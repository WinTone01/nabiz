package probe

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// The journal gets vacuumed; wtmp does not. `last -x reboot` records the kernel
// release of every boot going back weeks, which is the only cheap way to answer
// "did this start with a kernel update?" - the question that separates a driver
// regression from a hardware fault. Without it, a truncated journal makes a
// clean older boot look like it never happened.

// Reboot is one entry from wtmp.
type Reboot struct {
	Kernel  string        `json:"kernel"`
	At      time.Time     `json:"at"`
	Uptime  time.Duration `json:"uptime"`
	Current bool          `json:"current"`
}

// KernelStability aggregates link drops per kernel release.
type KernelStability struct {
	Kernel     string  `json:"kernel"`
	Boots      int     `json:"boots"`
	Hours      float64 `json:"hours"`
	Drops      int     `json:"drops"`
	Measured   bool    `json:"measured"` // do we have journal coverage?
	DropsPerHr float64 `json:"drops_per_hour"`
	CoveredHrs float64 `json:"covered_hours"`
}

// ReadReboots parses `last -x reboot`. Returns newest first.
func ReadReboots(limit int) []Reboot {
	if util.Which("last") == "" {
		return nil
	}
	// -w keeps last(1) from truncating longer kernel release strings, which it
	// otherwise marks with a trailing asterisk
	out, ok := util.Run(6*time.Second, "last", "-w", "-x", "reboot", "--time-format", "iso")
	if !ok || strings.TrimSpace(out) == "" {
		return nil
	}
	var reboots []Reboot
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		// reboot system boot <kernel> <iso-start> - <iso-end> (duration)
		if len(fields) < 5 || fields[0] != "reboot" {
			continue
		}
		entry := Reboot{Kernel: fields[3]}
		for _, layout := range []string{"2006-01-02T15:04:05-07:00", time.RFC3339} {
			if stamp, err := time.Parse(layout, fields[4]); err == nil {
				entry.At = stamp
				break
			}
		}
		if entry.At.IsZero() {
			continue
		}
		if strings.Contains(line, "still running") {
			entry.Current = true
			entry.Uptime = time.Since(entry.At)
		} else if index := strings.LastIndex(line, "("); index >= 0 {
			entry.Uptime = parseLastDuration(line[index+1:])
		}
		reboots = append(reboots, entry)
		if limit > 0 && len(reboots) >= limit {
			break
		}
	}
	return reboots
}

// parseLastDuration reads last(1)'s "(1+12:35)" / "(05:17)" format.
func parseLastDuration(text string) time.Duration {
	text = strings.TrimSuffix(strings.TrimSpace(text), ")")
	days := 0
	if plus := strings.Index(text, "+"); plus >= 0 {
		fmt.Sscanf(text[:plus], "%d", &days)
		text = text[plus+1:]
	}
	var hours, minutes int
	fmt.Sscanf(text, "%d:%d", &hours, &minutes)
	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
}

// StabilityByKernel joins wtmp's kernel history with the per-boot drop counts
// we can still read from the journal.
func StabilityByKernel(reboots []Reboot, boots []BootLinkStats) []KernelStability {
	byKernel := map[string]*KernelStability{}
	get := func(kernel string) *KernelStability {
		entry := byKernel[kernel]
		if entry == nil {
			entry = &KernelStability{Kernel: kernel}
			byKernel[kernel] = entry
		}
		return entry
	}
	// Both lists are newest-first and describe the same sequence of boots, so
	// they pair up by position. Matching on timestamps looks safer but breaks
	// exactly when it matters: a vacuumed journal reports a boot as starting
	// hours after it really did, and that is precisely the old, quiet boot we
	// need in the comparison.
	for index, reboot := range reboots {
		entry := get(reboot.Kernel)
		entry.Boots++
		entry.Hours += reboot.Uptime.Hours()
		if index >= len(boots) {
			continue
		}
		boot := boots[index]
		if boot.Minutes() < 1 {
			continue
		}
		// sanity: the journal window must fall inside this boot's lifetime
		if !boot.To.IsZero() && boot.To.Before(reboot.At) {
			continue
		}
		entry.Measured = true
		entry.Drops += boot.Drops
		entry.CoveredHrs += boot.Minutes() / 60
	}
	out := make([]KernelStability, 0, len(byKernel))
	for _, entry := range byKernel {
		if entry.CoveredHrs > 0 {
			entry.DropsPerHr = float64(entry.Drops) / entry.CoveredHrs
		}
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kernel > out[j].Kernel })
	return out
}

// cleanlyMeasured reports whether a kernel can be called good. It has to have
// been *watched* to be called quiet: wtmp knows how long a kernel ran, but with
// a volatile journal its logs die with the boot, and an unwatched kernel then
// scores a perfect zero drops out of zero observed minutes. Reading that as
// evidence recommends downgrading to whichever kernel was logged least - which
// is usually the oldest one, and quite possibly the one that started the fault.
// Half an hour is the shortest window in which a fault at one drop per hour has
// a fair chance of showing itself.
func cleanlyMeasured(entry *KernelStability) bool {
	return entry.Measured && entry.CoveredHrs >= 0.5 && entry.DropsPerHr < 1
}

// KernelRegressionData is the structured verdict, kept alongside the rendered
// sentence so a saved run can be re-read in either language.
type KernelRegressionData struct {
	GoodKernel  string  `json:"good_kernel"`
	GoodHours   float64 `json:"good_hours"`
	Running     string  `json:"running"`
	CurrentRate float64 `json:"current_rate"`
}

// KernelRegressionOf is the structured counterpart of KernelRegression.
func KernelRegressionOf(stats []KernelStability, running string) (KernelRegressionData, bool) {
	var current *KernelStability
	for i := range stats {
		if stats[i].Kernel == running {
			current = &stats[i]
		}
	}
	if current == nil || !current.Measured || current.CoveredHrs < 0.5 || current.DropsPerHr < 1 {
		return KernelRegressionData{}, false
	}
	for i := range stats {
		other := &stats[i]
		if other.Kernel == running || other.Hours < 4 {
			continue
		}
		if !cleanlyMeasured(other) {
			continue
		}
		return KernelRegressionData{GoodKernel: other.Kernel, GoodHours: other.Hours,
			Running: running, CurrentRate: current.DropsPerHr}, true
	}
	return KernelRegressionData{}, false
}

// KernelRegression compares the running kernel against the previous one and
// reports a regression only when both sides were actually measured.
func KernelRegression(stats []KernelStability, running string) (string, string, bool) {
	var current *KernelStability
	for i := range stats {
		if stats[i].Kernel == running {
			current = &stats[i]
		}
	}
	if current == nil || !current.Measured || current.CoveredHrs < 0.5 || current.DropsPerHr < 1 {
		return "", "", false
	}
	for i := range stats {
		other := &stats[i]
		if other.Kernel == running {
			continue
		}
		// a long clean run on another kernel is the evidence we want
		if other.Hours < 4 {
			continue
		}
		if !cleanlyMeasured(other) {
			continue // it flapped too, or nobody was watching
		}
		detail := i18n.T("fnd.kernel-regression.detail",
			other.Kernel, other.Hours, running, current.DropsPerHr)
		return other.Kernel, detail, true
	}
	return "", "", false
}

// CachedKernelPackages finds pacman's cached packages for a kernel release, so
// the advice can offer a downgrade the user can actually run rather than a
// vague "roll back the kernel".
func CachedKernelPackages(release string) []string {
	// "7.1.8-1-cachyos" -> package version "7.1.8-1"
	parts := strings.Split(release, "-")
	if len(parts) < 2 {
		return nil
	}
	version := parts[0] + "-" + parts[1]
	var found []string
	for _, name := range []string{"linux-cachyos", "linux-cachyos-headers",
		"linux-cachyos-nvidia-open", "linux", "linux-headers"} {
		matches, err := filepath.Glob("/var/cache/pacman/pkg/" + name + "-" + version + "-*.pkg.tar.zst")
		if err != nil {
			continue
		}
		for _, match := range matches {
			if !strings.HasSuffix(match, ".sig") {
				found = append(found, match)
			}
		}
	}
	sort.Strings(found)
	return found
}
