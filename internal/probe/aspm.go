package probe

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/util"
)

// PCIe power management is the quietest way for a network card to break. When a
// driver knows its chip misbehaves under ASPM it disables it at probe time - but
// that call only works if the firmware handed ASPM control to the OS through
// ACPI _OSC. When it did not, the kernel prints
//
//	r8169 0000:03:00.0: can't disable ASPM; OS doesn't have ASPM control
//
// and carries on with the exact power saving the driver just asked to turn off.
// That line is a fault the machine reports about itself, so it deserves a
// finding of its own rather than being guessed at from the drop rate.

// ASPMStatus describes who controls PCIe ASPM for the network card.
type ASPMStatus struct {
	Checked bool `json:"checked"`
	// Blocked is true when the driver asked to disable ASPM and was refused.
	Blocked bool   `json:"blocked"`
	Driver  string `json:"driver,omitempty"`
	Slot    string `json:"slot,omitempty"`
	// Policy is the contents of /sys/module/pcie_aspm/parameters/policy.
	Policy string `json:"policy,omitempty"`
	// OSControl is true when per-device ASPM state is exposed in sysfs, which
	// only happens once the kernel actually owns ASPM.
	OSControl bool `json:"os_control"`
	// CmdlineParam is the pcie_aspm= value already on the kernel command line.
	CmdlineParam string `json:"cmdline_param,omitempty"`
}

// Forced reports whether the kernel was told to take ASPM control regardless of
// what the firmware granted.
func (a ASPMStatus) Forced() bool { return a.CmdlineParam == "force" }

var aspmBlockedRe = regexp.MustCompile(
	`(\S+) (\S+): can't disable ASPM; OS doesn't have ASPM control`)

// ReadASPM reports the ASPM control state for iface's PCI device.
func ReadASPM(iface string) ASPMStatus {
	var status ASPMStatus
	if iface == "" {
		iface, _ = util.DefaultRoute()
	}
	if iface == "" {
		return status
	}

	// The slot lets us check sysfs and lets the advice name the right device.
	if target, err := os.Readlink(filepath.Join("/sys/class/net", iface, "device")); err == nil {
		status.Slot = filepath.Base(target)
	}

	if raw, err := os.ReadFile("/sys/module/pcie_aspm/parameters/policy"); err == nil {
		status.Policy = strings.TrimSpace(string(raw))
	}
	// Per-device ASPM knobs exist only when the kernel owns ASPM; their absence
	// is the same statement as the log line, read from a different place. The
	// "link" directory itself is created either way and is simply left empty
	// when the kernel has no control, so its contents are what to look at.
	if status.Slot != "" {
		entries, err := os.ReadDir(filepath.Join("/sys/bus/pci/devices", status.Slot, "link"))
		status.OSControl = err == nil && len(entries) > 0
	}
	if raw, err := os.ReadFile("/proc/cmdline"); err == nil {
		for _, field := range strings.Fields(string(raw)) {
			if value, ok := strings.CutPrefix(field, "pcie_aspm="); ok {
				status.CmdlineParam = value
			}
		}
	}

	out, ok := util.Run(10*time.Second, "journalctl", "-k", "-b", "--no-pager", "-n", "20000")
	if !ok {
		// sysfs still answered, so report what we have rather than nothing
		status.Checked = status.Slot != "" || status.Policy != ""
		return status
	}
	status.Checked = true
	for _, line := range strings.Split(out, "\n") {
		match := aspmBlockedRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		// only the card we route through matters; other devices may share the fault
		if status.Slot != "" && match[2] != status.Slot {
			continue
		}
		status.Blocked = true
		status.Driver = match[1]
		if status.Slot == "" {
			status.Slot = match[2]
		}
	}
	return status
}

// A volatile journal is why a kernel comparison can be impossible: without
// /var/log/journal the kernel log dies with the boot, so every earlier kernel
// looks flawless simply because nothing about it was kept. Detecting that is
// what lets the tool say "I cannot tell yet" instead of guessing.

// JournalStorage reports whether the journal survives reboots and how many
// boots are actually retained.
type JournalStorage struct {
	Persistent bool `json:"persistent"`
	Boots      int  `json:"boots"`
}

// ReadJournalStorage inspects journald's on-disk state.
func ReadJournalStorage() JournalStorage {
	var storage JournalStorage
	if entries, err := os.ReadDir("/var/log/journal"); err == nil && len(entries) > 0 {
		storage.Persistent = true
	}
	if util.Which("journalctl") == "" {
		return storage
	}
	out, ok := util.Run(8*time.Second, "journalctl", "--list-boots", "--no-pager")
	if !ok {
		return storage
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) != "" {
			storage.Boots++
		}
	}
	return storage
}
