package sysinfo

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// bpftune (github.com/oracle/bpftune) watches the kernel through BPF and edits
// sysctls on its own. That makes it invisible in the usual places: the values
// in /etc/sysctl.d no longer describe the running system, and a tunable can
// change three times during a single test.
//
// We read what it actually did - from its journal, which is authoritative and
// timestamped - and cross-check the live values against the kernel defaults.
// The judgement call at the end is whether the buffers it grew make sense for
// the bandwidth-delay product we just measured, because "bigger" and "better"
// are not the same thing on a home line.

// dropInPath is the override this tool writes; its presence explains a
// failure that would otherwise look like a packaging problem.
const dropInPath = "/etc/systemd/system/bpftune.service.d/99-nabiz-tuners.conf"

const (
	unitName = "bpftune.service"
	binary   = "bpftune"
)

// Change is the aggregate story of one tunable: where it started, where it
// ended up, and how many times bpftune moved it. Showing every individual step
// is noise - a buffer that grew in seven increments is one decision, not seven.
type Change struct {
	At      time.Time `json:"at"`
	Tunable string    `json:"tunable"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	Count   int       `json:"count"`
	Reason  string    `json:"reason,omitempty"`
}

// Step is a single raw edit, kept for the detail view.
type Step struct {
	At      time.Time `json:"at"`
	Tunable string    `json:"tunable"`
	From    string    `json:"from"`
	To      string    `json:"to"`
}

// Tunable pairs the running value with the stock kernel default.
type Tunable struct {
	Key      string `json:"key"`
	Current  string `json:"current"`
	Default  string `json:"default"`
	Modified bool   `json:"modified"`
}

// BpftuneState is everything we know about the auto-tuner.
type BpftuneState struct {
	Installed  bool           `json:"installed"`
	Running    bool           `json:"running"`
	Failed     bool           `json:"failed"`
	FailDetail string         `json:"fail_detail,omitempty"`
	Overridden bool           `json:"overridden,omitempty"`
	Enabled    bool           `json:"enabled"`
	Version    string         `json:"version,omitempty"`
	Changes    []Change       `json:"changes"`
	History    []Step         `json:"history,omitempty"`
	CCVotes    map[string]int `json:"cc_votes,omitempty"`
	Tunables   []Tunable      `json:"tunables"`
	Tuners     []string       `json:"tuners,omitempty"`
	JournalErr string         `json:"journal_err,omitempty"`
}

// Label renders the state for a status bar.
func (s BpftuneState) Label() string {
	switch {
	case !s.Installed:
		return i18n.T("ui.notinstalled")
	case s.Failed:
		return i18n.T("ui.failed")
	case !s.Running:
		return i18n.T("ui.stopped")
	case len(s.Changes) == 0:
		return i18n.T("ui.running")
	default:
		return i18n.T("bpftune.label", len(s.Changes))
	}
}

// kernelDefaults are the upstream defaults for the tunables bpftune touches.
// A distro sysctl.d snippet can also move these, so a difference is evidence,
// not proof - the journal is what attributes a change to bpftune.
var kernelDefaults = map[string]string{
	"net.ipv4.tcp_rmem":                       "4096 131072 6291456",
	"net.ipv4.tcp_wmem":                       "4096 16384 4194304",
	"net.core.rmem_max":                       "212992",
	"net.core.wmem_max":                       "212992",
	"net.core.netdev_max_backlog":             "1000",
	"net.ipv4.tcp_max_syn_backlog":            "256",
	"net.ipv4.tcp_congestion_control":         "cubic",
	"net.ipv4.tcp_allowed_congestion_control": "reno cubic",
	"net.ipv4.tcp_no_metrics_save":            "0",
	"net.ipv4.tcp_moderate_rcvbuf":            "1",
	"net.ipv4.tcp_syn_retries":                "6",
	"net.ipv4.tcp_thin_linear_timeouts":       "0",
	"net.core.somaxconn":                      "4096",
}

var (
	sysctlChangeRe = regexp.MustCompile(`sysctl '([^']+)' changed from \(([^)]*)\) -> \(([^)]*)\)`)
	bufferChangeRe = regexp.MustCompile(`change (net\.[a-z0-9_.]+)\([^)]*\) from \(([^)]*)\) -> \(([^)]*)\)`)
	ccVoteRe       = regexp.MustCompile(`tcp_conn_tuner:\s+(\w+)\s+(\d+)\s*$`)
	scenarioRe     = regexp.MustCompile(`Scenario '([^']+)' occurred for tunable '([^']+)'`)
	stampRe        = regexp.MustCompile(`^(\w{3}\s+\d+\s+\d+:\d+:\d+|\S+\s+\d+\s+\d+:\d+:\d+)`)
)

// ReadBpftune gathers service state, journal history and live tunables.
func ReadBpftune() BpftuneState {
	state := BpftuneState{CCVotes: map[string]int{}}
	if util.Which(binary) == "" {
		// the daemon may still be packaged without the CLI on PATH
		if out, _ := util.Run(4*time.Second, "systemctl", "is-enabled", unitName); strings.TrimSpace(out) == "" {
			return state
		}
	}
	state.Installed = true
	if out, ok := util.Run(4*time.Second, binary, "-V"); ok {
		state.Version = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "bpftune "))
	}
	if out, _ := util.Run(5*time.Second, "systemctl", "is-active", unitName); strings.TrimSpace(out) == "active" {
		state.Running = true
	}
	if out, _ := util.Run(5*time.Second, "systemctl", "is-enabled", unitName); strings.TrimSpace(out) == "enabled" {
		state.Enabled = true
	}
	// "not running" and "tried to run and died" are different problems, and the
	// second one is usually something that was done to the machine on purpose.
	if out, _ := util.Run(5*time.Second, "systemctl", "is-failed", unitName); strings.TrimSpace(out) == "failed" {
		state.Failed = true
		if detail, ok := util.Run(6*time.Second, "systemctl", "show", "-p", "ExecMainStatus",
			"-p", "Result", "--value", unitName); ok {
			state.FailDetail = strings.Join(strings.Fields(detail), " ")
		}
	}
	if _, err := os.Stat(dropInPath); err == nil {
		state.Overridden = true
	}
	state.readJournal()
	state.readTunables()
	return state
}

func (s *BpftuneState) readJournal() {
	out, ok := util.Run(8*time.Second, "journalctl", "-u", unitName,
		"--no-pager", "--output=short-iso", "-n", "1500")
	if !ok || strings.TrimSpace(out) == "" {
		s.JournalErr = "günlük okunamadı (journalctl erişimi yok olabilir)"
		return
	}
	reasons := map[string]string{}
	type agg struct {
		first, last string
		at          time.Time
		count       int
		order       int
	}
	aggregates := map[string]*agg{}
	order := 0

	for _, line := range strings.Split(out, "\n") {
		if match := scenarioRe.FindStringSubmatch(line); match != nil {
			reasons[match[2]] = match[1]
		}
		if match := ccVoteRe.FindStringSubmatch(line); match != nil {
			if count, err := strconv.Atoi(match[2]); err == nil && match[1] != "Count" {
				// the summary prints cumulative counts; keep the largest seen
				if count > s.CCVotes[match[1]] {
					s.CCVotes[match[1]] = count
				}
			}
		}
		var tunable, from, to string
		if match := sysctlChangeRe.FindStringSubmatch(line); match != nil {
			tunable, from, to = match[1], match[2], match[3]
		} else if match := bufferChangeRe.FindStringSubmatch(line); match != nil {
			tunable, from, to = match[1], match[2], match[3]
		} else {
			continue
		}
		from = strings.Join(strings.Fields(from), " ")
		to = strings.Join(strings.Fields(to), " ")
		if from == to {
			continue
		}
		stamp := parseStamp(line)
		s.History = append(s.History, Step{At: stamp, Tunable: tunable, From: from, To: to})

		entry := aggregates[tunable]
		if entry == nil {
			order++
			entry = &agg{first: from, order: order}
			aggregates[tunable] = entry
		}
		entry.last = to
		entry.at = stamp
		entry.count++
	}

	for tunable, entry := range aggregates {
		s.Changes = append(s.Changes, Change{
			At: entry.at, Tunable: tunable, From: entry.first, To: entry.last,
			Count: entry.count, Reason: reasons[tunable],
		})
	}
	sort.Slice(s.Changes, func(i, j int) bool {
		return s.Changes[i].At.After(s.Changes[j].At)
	})
	if len(s.History) > 60 {
		s.History = s.History[len(s.History)-60:]
	}
}

func parseStamp(line string) time.Time {
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

func (s *BpftuneState) readTunables() {
	keys := make([]string, 0, len(kernelDefaults))
	for key := range kernelDefaults {
		keys = append(keys, key)
	}
	// keep a stable, meaningful order rather than map order
	order := []string{
		"net.ipv4.tcp_rmem", "net.ipv4.tcp_wmem", "net.core.rmem_max", "net.core.wmem_max",
		"net.ipv4.tcp_congestion_control", "net.ipv4.tcp_allowed_congestion_control",
		"net.core.netdev_max_backlog", "net.ipv4.tcp_max_syn_backlog",
		"net.ipv4.tcp_no_metrics_save", "net.ipv4.tcp_moderate_rcvbuf",
		"net.ipv4.tcp_thin_linear_timeouts", "net.core.somaxconn", "net.ipv4.tcp_syn_retries",
	}
	seen := map[string]bool{}
	appendKey := func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		current := util.Sysctl(key)
		if current == "" {
			return
		}
		def := kernelDefaults[key]
		s.Tunables = append(s.Tunables, Tunable{
			Key: key, Current: current, Default: def,
			Modified: def != "" && current != def,
		})
	}
	for _, key := range order {
		appendKey(key)
	}
	for _, key := range keys {
		appendKey(key)
	}
}

// ChangedByBpftune reports whether the journal shows this tunable being edited.
func (s BpftuneState) ChangedByBpftune(key string) (Change, bool) {
	for i := len(s.Changes) - 1; i >= 0; i-- {
		if s.Changes[i].Tunable == key {
			return s.Changes[i], true
		}
	}
	return Change{}, false
}

// BDPBytes is the bandwidth-delay product: how much data can be in flight on
// this path at once. Buffers meaningfully larger than this buy nothing.
func BDPBytes(linkMbit int, rttMs float64) float64 {
	if linkMbit <= 0 || rttMs <= 0 {
		return 0
	}
	return float64(linkMbit) * 1e6 / 8 * (rttMs / 1000)
}

// AssessBpftune judges the auto-tuner against the line we just measured.
func AssessBpftune(state BpftuneState, linkMbit int, rttMs float64, retransPct float64) []Note {
	var notes []Note
	if !state.Installed {
		return notes
	}
	note := func(level, key, text, hint string) {
		notes = append(notes, Note{Level: level, Key: key, Source: "bpftune", Text: text, Hint: hint})
	}
	if state.Failed {
		hint := ""
		if state.Overridden {
			hint = i18n.T("fnd.bpftune-failed.override")
		}
		note("bad", "bpftune-failed",
			i18n.T("fnd.bpftune-failed.title", state.FailDetail), hint)
		return notes
	}
	if !state.Running {
		note("info", "bpftune-stopped", i18n.T("fnd.bpftune-stopped.title"), "")
		return notes
	}
	if len(state.Changes) == 0 {
		note("info", "bpftune-idle", i18n.T("fnd.bpftune-idle.title"), "")
	}

	// The buffer question: is the receive-buffer ceiling sane for this path?
	bdp := BDPBytes(linkMbit, rttMs)
	for _, tunable := range state.Tunables {
		if tunable.Key != "net.ipv4.tcp_rmem" {
			continue
		}
		fields := strings.Fields(tunable.Current)
		if len(fields) != 3 {
			continue
		}
		maxBuf, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || bdp <= 0 {
			continue
		}
		ratio := maxBuf / bdp
		origin := ""
		if change, fromJournal := state.ChangedByBpftune(tunable.Key); fromJournal {
			origin = i18n.T("fnd.bpftune-buffers.origin",
				lastField(change.From), lastField(change.To))
		}
		switch {
		case ratio > 50:
			note("warn", "bpftune-buffers",
				i18n.T("fnd.bpftune-buffers-high.title", util.HumanBytes(maxBuf), ratio, origin),
				i18n.T("fnd.bpftune-buffers-high.hint", linkMbit, rttMs,
					util.HumanBytes(bdp), int(bdp*4)))
		case ratio > 4:
			note("ok", "bpftune-buffers",
				i18n.T("fnd.bpftune-buffers-ok.title", util.HumanBytes(maxBuf), ratio, origin), "")
		default:
			note("info", "bpftune-buffers",
				i18n.T("fnd.bpftune-buffers-info.title", util.HumanBytes(maxBuf),
					util.HumanBytes(bdp)), "")
		}
	}

	if len(state.CCVotes) > 0 {
		var parts []string
		for name, count := range state.CCVotes {
			parts = append(parts, fmt.Sprintf("%s=%d", name, count))
		}
		sort.Strings(parts)
		note("info", "bpftune-cc",
			i18n.T("fnd.bpftune-cc.title", strings.Join(parts, ", ")),
			i18n.T("fnd.bpftune-cc.hint"))
	}
	if _, ok := state.ChangedByBpftune("net.ipv4.tcp_allowed_congestion_control"); ok {
		note("info", "bpftune-cc-allowed",
			i18n.T("fnd.bpftune-cc-allowed.title"), i18n.T("fnd.bpftune-cc-allowed.hint"))
	}
	if retransPct > 2 {
		note("warn", "bpftune-retrans",
			i18n.T("fnd.bpftune-retrans.title", retransPct), i18n.T("fnd.bpftune-retrans.hint"))
	}
	return notes
}

func lastField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return value
	}
	return fields[len(fields)-1]
}

// --- control (used by the A/B screen, always opt-in) --------------------

func privileged(args []string) []string {
	if util.IsRoot() {
		return args
	}
	if util.Which("pkexec") != "" {
		return append([]string{"pkexec"}, args...)
	}
	if util.Which("sudo") != "" {
		return append([]string{"sudo", "-n"}, args...)
	}
	return nil
}

// CanControlBpftune reports whether we can start/stop the service.
func CanControlBpftune() bool {
	return util.Which("systemctl") != "" && privileged([]string{"true"}) != nil
}

// SetBpftune starts or stops the daemon. Stopping does not undo the sysctls it
// already wrote; RollbackBpftune does that.
func SetBpftune(running bool) (bool, string) {
	action := "stop"
	if running {
		action = "start"
	}
	command := privileged([]string{"systemctl", action, unitName})
	if command == nil {
		return false, "yetki yok / no privilege escalation available"
	}
	out, ok := util.Run(30*time.Second, command[0], command[1:]...)
	if !ok {
		return false, strings.TrimSpace(out)
	}
	return true, "ok"
}

// RollbackBpftune asks bpftune to restore the values it changed.
func RollbackBpftune() (bool, string) {
	command := privileged([]string{binary, "-R"})
	if command == nil {
		return false, "yetki yok / no privilege escalation available"
	}
	out, ok := util.Run(30*time.Second, command[0], command[1:]...)
	if !ok {
		return false, strings.TrimSpace(out)
	}
	return true, "ok"
}
