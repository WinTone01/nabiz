package probe

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// The carrier counter in sysfs says *how many* times the link dropped. The
// kernel ring buffer says *when*, *for how long*, and often *why* - the r8169
// PHY, for instance, prints "Downshift occurred ... check cabling!" when
// gigabit autonegotiation keeps failing. That message is the difference between
// "your connection is unstable" and "one pair in your cable is broken".

// LinkEvent is one carrier transition read from the kernel log.
type LinkEvent struct {
	At          time.Time `json:"at"`
	Kind        string    `json:"kind"` // down | up
	Speed       string    `json:"speed,omitempty"`
	Downshifted bool      `json:"downshifted,omitempty"`
	Note        string    `json:"note,omitempty"`
}

// LinkHistory summarises every carrier transition since boot.
type LinkHistory struct {
	Available     bool        `json:"available"`
	Reason        string      `json:"reason,omitempty"`
	Events        []LinkEvent `json:"events,omitempty"`
	Drops         int         `json:"drops"`
	Downshifts    int         `json:"downshifts"`
	DownSeconds   float64     `json:"down_seconds"`
	LongestDown   float64     `json:"longest_down"`
	MeanGapMin    float64     `json:"mean_gap_min"`
	FirstAt       time.Time   `json:"first_at,omitempty"`
	LastAt        time.Time   `json:"last_at,omitempty"`
	WindowFrom    time.Time   `json:"window_from,omitempty"`
	WindowTo      time.Time   `json:"window_to,omitempty"`
	DownshiftNote string      `json:"downshift_note,omitempty"`
}

// SpanMinutes is the length of the log window we could actually read, not the
// span between the first and last drop. With a single drop those differ by a
// factor of infinity, and the rate computed from them would be nonsense.
func (h LinkHistory) SpanMinutes() float64 {
	if h.WindowFrom.IsZero() || h.WindowTo.IsZero() {
		return 0
	}
	return h.WindowTo.Sub(h.WindowFrom).Minutes()
}

// QuietMinutes is how long the link has held since the last drop. A cumulative
// count cannot answer "did the change I just made work?" - it only ever grows,
// so a fix looks identical to no fix until the next reboot. This is the number
// that moves the moment the flapping stops.
func (h LinkHistory) QuietMinutes() float64 {
	var lastDown time.Time
	for _, event := range h.Events {
		if event.Kind == "down" && event.At.After(lastDown) {
			lastDown = event.At
		}
	}
	if lastDown.IsZero() || h.WindowTo.IsZero() || h.WindowTo.Before(lastDown) {
		return 0
	}
	return h.WindowTo.Sub(lastDown).Minutes()
}

// Settled reports that the link has been quiet for long enough that the drop
// rate seen earlier in this boot no longer describes it: five times the mean
// gap between drops, and at least twenty minutes.
func (h LinkHistory) Settled() bool {
	quiet := h.QuietMinutes()
	return h.Drops > 2 && quiet >= 20 && (h.MeanGapMin <= 0 || quiet >= 5*h.MeanGapMin)
}

// DownPct is the share of the observed window spent with no carrier.
func (h LinkHistory) DownPct() float64 {
	span := h.SpanMinutes() * 60
	if span <= 0 {
		return 0
	}
	return h.DownSeconds / span * 100
}

var (
	linkDownRe  = regexp.MustCompile(`(\S+): Link is Down`)
	linkUpRe    = regexp.MustCompile(`(\S+): Link is Up - (\S+)`)
	downshiftRe = regexp.MustCompile(`Downshift occurred from negotiated speed ([^,\s]+) to actual speed ([^,\s]+)`)
)

// ReadLinkHistory parses the kernel log for carrier transitions on iface.
func ReadLinkHistory(iface string) LinkHistory {
	var history LinkHistory
	if iface == "" {
		iface, _ = util.DefaultRoute()
	}
	if iface == "" {
		history.Reason = i18n.T("ui.unknown")
		return history
	}
	out, ok := util.Run(10*time.Second, "journalctl", "-k", "-b", "--no-pager",
		"--output=short-iso", "-n", "20000")
	if !ok || strings.TrimSpace(out) == "" {
		history.Reason = i18n.T("err.journal")
		return history
	}
	history.Available = true

	var pendingDown time.Time
	var pendingDownshift string
	var bootStart time.Time
	sawFirstUp := false
	for _, line := range strings.Split(out, "\n") {
		if stamp := parseKernelStamp(line); !stamp.IsZero() {
			if bootStart.IsZero() {
				bootStart = stamp
				history.WindowFrom = stamp
			}
			history.WindowTo = stamp
		}
		if !strings.Contains(line, iface) && !strings.Contains(line, "Downshift occurred") {
			continue
		}
		stamp := parseKernelStamp(line)

		if match := downshiftRe.FindStringSubmatch(line); match != nil {
			history.Downshifts++
			pendingDownshift = match[1] + " → " + match[2]
			history.DownshiftNote = pendingDownshift
			continue
		}
		if linkDownRe.MatchString(line) {
			// the interface comes up during boot, which the kernel logs as a
			// down/up pair; counting that as an outage inflates every rate
			if !sawFirstUp && !bootStart.IsZero() && !stamp.IsZero() &&
				stamp.Sub(bootStart) < 90*time.Second {
				continue
			}
			history.Drops++
			pendingDown = stamp
			history.Events = append(history.Events,
				LinkEvent{At: stamp, Kind: "down"})
			if history.FirstAt.IsZero() {
				history.FirstAt = stamp
			}
			history.LastAt = stamp
			continue
		}
		if match := linkUpRe.FindStringSubmatch(line); match != nil {
			sawFirstUp = true
			event := LinkEvent{At: stamp, Kind: "up", Speed: match[2],
				Downshifted: strings.Contains(line, "downshifted")}
			if event.Downshifted {
				event.Note = pendingDownshift
			}
			pendingDownshift = ""
			history.Events = append(history.Events, event)
			if !pendingDown.IsZero() && !stamp.IsZero() {
				if seconds := stamp.Sub(pendingDown).Seconds(); seconds > 0 && seconds < 600 {
					history.DownSeconds += seconds
					if seconds > history.LongestDown {
						history.LongestDown = seconds
					}
				}
				pendingDown = time.Time{}
			}
			if history.FirstAt.IsZero() {
				history.FirstAt = stamp
			}
			history.LastAt = stamp
		}
	}
	if history.Drops > 1 && history.SpanMinutes() > 0 {
		history.MeanGapMin = history.SpanMinutes() / float64(history.Drops)
	}
	// keep the log bounded; the last transitions are the interesting ones
	if len(history.Events) > 60 {
		history.Events = history.Events[len(history.Events)-60:]
	}
	return history
}

func parseKernelStamp(line string) time.Time {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02T15:04:05-0700", time.RFC3339} {
		if stamp, err := time.Parse(layout, fields[0]); err == nil {
			return stamp
		}
	}
	return time.Time{}
}

// EEEStatus reports Energy Efficient Ethernet state. EEE on Realtek r8169 is a
// documented cause of link flapping, and it is a one-command experiment.
type EEEStatus struct {
	Supported bool `json:"supported"`
	Enabled   bool `json:"enabled"`
	Active    bool `json:"active"`
	// Checked is false when ethtool could not be asked at all. Without it an
	// unreadable NIC reports the same three false values as a healthy one with
	// EEE switched off, and a prime suspect for link flapping disappears from
	// the report without anyone being told it was never looked at.
	Checked bool   `json:"checked"`
	Reason  string `json:"reason,omitempty"`
}

// ReadEEE queries ethtool for the EEE state of an interface.
func ReadEEE(iface string) EEEStatus {
	var status EEEStatus
	if iface == "" {
		status.Reason = "no-interface"
		return status
	}
	out, outcome := util.RunDetail(4*time.Second, "ethtool", "--show-eee", iface)
	if !outcome.Readable() {
		status.Reason = outcome.String()
		return status
	}
	status.Checked = true
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "EEE status:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "EEE status:"))
			status.Supported = !strings.Contains(value, "not supported")
			status.Enabled = strings.Contains(value, "enabled")
			status.Active = strings.Contains(value, "active")
		}
	}
	return status
}

// BootLinkStats is the link-drop rate of one boot, used to answer the only
// question that separates a hardware fault from a regression: "did this start
// recently, and did it start with a specific boot?"
type BootLinkStats struct {
	Index      int       `json:"index"` // 0 = current, -1 = previous, ...
	Kernel     string    `json:"kernel,omitempty"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	Drops      int       `json:"drops"`
	Downshifts int       `json:"downshifts"`
	Truncated  bool      `json:"truncated"`
}

// Minutes is the length of the retained log window for this boot.
func (b BootLinkStats) Minutes() float64 {
	if b.From.IsZero() || b.To.IsZero() {
		return 0
	}
	return b.To.Sub(b.From).Minutes()
}

// DropsPerHour normalises the drop count so boots of different length compare.
func (b BootLinkStats) DropsPerHour() float64 {
	minutes := b.Minutes()
	if minutes < 1 {
		return 0
	}
	return float64(b.Drops) / (minutes / 60)
}

// ReadBootHistory walks back through the retained journal boots and counts link
// events in each. Boots whose kernel log has been rotated are marked truncated,
// because "no events" in a two-minute window proves nothing.
func ReadBootHistory(iface string, maxBoots int) []BootLinkStats {
	if iface == "" {
		iface, _ = util.DefaultRoute()
	}
	if iface == "" || util.Which("journalctl") == "" {
		return nil
	}
	var out []BootLinkStats
	for index := 0; index > -maxBoots; index-- {
		text, ok := util.Run(12*time.Second, "journalctl", "-k",
			"-b", strconv.Itoa(index), "--no-pager", "--output=short-iso", "-n", "200000")
		if !ok || strings.TrimSpace(text) == "" {
			break
		}
		stats := BootLinkStats{Index: index}
		var sawBanner bool
		for _, line := range strings.Split(text, "\n") {
			if line == "" {
				continue
			}
			stamp := parseKernelStamp(line)
			if !stamp.IsZero() {
				if stats.From.IsZero() {
					stats.From = stamp
				}
				stats.To = stamp
			}
			if strings.Contains(line, "Linux version ") {
				sawBanner = true
				if fields := strings.SplitN(line, "Linux version ", 2); len(fields) == 2 {
					stats.Kernel = strings.Fields(fields[1])[0]
				}
			}
			if strings.Contains(line, iface) && strings.Contains(line, "Link is Down") {
				stats.Drops++
			}
			if strings.Contains(line, "Downshift occurred") {
				stats.Downshifts++
			}
		}
		stats.Truncated = !sawBanner
		out = append(out, stats)
	}
	return out
}

// Regression is the structured form of "this started recently". Storing the
// numbers rather than a sentence means a saved run can be re-rendered in
// another language later.
type Regression struct {
	QuietMinutes float64 `json:"quiet_minutes"`
	QuietRate    float64 `json:"quiet_rate"`
	CurrentRate  float64 `json:"current_rate"`
	Kind         string  `json:"kind"` // window | worse
}

// RegressionOf is the structured counterpart of RegressionHint.
func RegressionOf(boots []BootLinkStats) (Regression, bool) {
	if len(boots) < 2 {
		return Regression{}, false
	}
	current := boots[0]
	if current.DropsPerHour() < 1 {
		return Regression{}, false
	}
	for _, previous := range boots[1:] {
		if previous.Minutes() < 60 {
			continue
		}
		if previous.Drops == 0 {
			return Regression{Kind: "window", QuietMinutes: previous.Minutes(),
				CurrentRate: current.DropsPerHour()}, true
		}
		if previous.DropsPerHour() > 0 && current.DropsPerHour() > previous.DropsPerHour()*3 {
			return Regression{Kind: "worse", QuietRate: previous.DropsPerHour(),
				CurrentRate: current.DropsPerHour()}, true
		}
	}
	return Regression{}, false
}

// RegressionHint compares the current boot against earlier ones and reports
// whether the flapping is new. It deliberately refuses to conclude anything
// from a truncated or very short window.
func RegressionHint(boots []BootLinkStats) (text string, recent bool) {
	if len(boots) < 2 {
		return "", false
	}
	current := boots[0]
	if current.DropsPerHour() < 1 {
		return "", false
	}
	for _, previous := range boots[1:] {
		if previous.Minutes() < 60 {
			continue // too short to prove anything
		}
		if previous.Drops == 0 {
			return i18n.T("fnd.link-regression.window",
				previous.Minutes(), current.DropsPerHour()), true
		}
		if previous.DropsPerHour() > 0 && current.DropsPerHour() > previous.DropsPerHour()*3 {
			return i18n.T("fnd.link-regression.worse",
				previous.DropsPerHour(), current.DropsPerHour()), true
		}
	}
	return "", false
}
