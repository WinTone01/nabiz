package ui

import (
	"context"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/apply"
	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/monitor"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

// App is the state every page reads and no page writes.
//
// Pages render; the root model acts. One run can be in flight, one language is
// current, one baseline is pinned, and there is exactly one place each of those
// changes - which is why two screens can never disagree about what happened.
type App struct {
	Cfg     config.Config
	Version string
	Env     liveEnv
	Watcher *monitor.Monitor

	Results  map[string]suite.Result
	LastName string
	Baseline *suite.Result
	AB       *abDoneMsg
	History  []report.RunSummary

	SuitePick    int
	Selected     map[string]bool
	Applicable   map[string]apply.Change
	HasSnapshots bool

	Running   string
	Phase     string
	Progress  float64
	StartedAt time.Time

	width, height int
	cancel        context.CancelFunc
	msgCh         chan tea.Msg
}

// LastRun is the most recent result, or nil when nothing has been measured.
func (a *App) LastRun() *suite.Result {
	if a.LastName == "" {
		return nil
	}
	result, ok := a.Results[a.LastName]
	if !ok {
		return nil
	}
	return &result
}

// Busy reports whether a measurement or an apply is in flight. Buttons that
// would start a second one are disabled rather than hidden, so the interface
// does not rearrange itself while it works.
func (a *App) Busy() bool { return a.Running != "" }

func (a *App) Elapsed() time.Duration {
	if a.StartedAt.IsZero() {
		return 0
	}
	return time.Since(a.StartedAt)
}

// Width and Height are the usable dimensions of the content region.
func (a *App) Width() int  { return a.width }
func (a *App) Height() int { return a.height }

// SelectedCount counts ticked advice that nabiz can actually perform.
func (a *App) SelectedCount() int {
	count := 0
	for id, on := range a.Selected {
		if on && a.Applicable[id].ID != "" {
			count++
		}
	}
	return count
}

// --- the live environment ----------------------------------------------------

// liveEnv is the cheap snapshot refreshed on a timer. Everything in it is read
// from the kernel rather than measured, so refreshing it costs nothing and can
// be trusted while a run is in flight.
type liveEnv struct {
	link      probe.LinkInfo
	linkLog   probe.LinkHistory
	boots     []probe.BootLinkStats
	kernels   []probe.KernelStability
	reboots   []probe.Reboot
	kernelNow string
	nfqueue   probe.NFQueueInfo
	conntrack probe.Conntrack
	health    probe.TCPHealth
	sockets   probe.SocketSummary
	unwall    sysinfo.UnwallState
	bpftune   sysinfo.BpftuneState
	eee       probe.EEEStatus
	systemDNS []string
	upstream  []string
	sysctls   map[string]string
}

func snapshotLive() liveEnv {
	env := liveEnv{
		link:      probe.ReadLink(""),
		nfqueue:   probe.ReadNFQueues(),
		conntrack: probe.ReadConntrack(),
		health:    probe.ReadSNMP().Health(),
		unwall:    sysinfo.ReadUnwall(),
		bpftune:   sysinfo.ReadBpftune(),
		systemDNS: probe.SystemResolvers(),
		upstream:  probe.UpstreamResolvers(),
		sysctls:   probe.ReadSysctls(),
	}
	env.eee = probe.ReadEEE(env.link.Iface)
	env.linkLog = probe.ReadLinkHistory(env.link.Iface)
	env.boots = probe.ReadBootHistory(env.link.Iface, 6)
	env.reboots = probe.ReadReboots(30)
	env.kernels = probe.StabilityByKernel(env.reboots, env.boots)
	env.kernelNow = strings.TrimSpace(readText("/proc/sys/kernel/osrelease"))
	if sockets, err := probe.TCPSockets(""); err == nil {
		env.sockets = probe.Summarize(sockets)
	}
	return env
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// --- messages -----------------------------------------------------------------

type tickMsg time.Time
type envMsg struct{ env liveEnv }

type progressMsg struct {
	phase    string
	fraction float64
}

type runDoneMsg struct {
	name   string
	result suite.Result
}

type runFailedMsg struct{ err string }

type abDoneMsg struct {
	target    string
	before    suite.Result
	after     suite.Result
	notNeeded []string
}

type applyDoneMsg struct {
	rolledBack bool
	err        string
	snapshot   string
}

type noticeMsg struct {
	text  string
	level toastLevel
}

// Dismissed reports whether a recommendation has been closed by hand.
//
// Not every recommendation describes a fault. Some ask a question only the
// person at the keyboard can answer - is a 100 Mbit link expected here, is IPv6
// wanted at all - and one that cannot be answered stays on the list forever and
// teaches the reader to skim past the ones that matter.
func (a *App) Dismissed(id string) bool {
	for _, dismissed := range a.Cfg.Dismissed {
		if dismissed == id {
			return true
		}
	}
	return false
}

// Dismiss hides a recommendation and remembers the choice.
func (a *App) Dismiss(id string) {
	if id == "" || a.Dismissed(id) {
		return
	}
	a.Cfg.Dismissed = append(a.Cfg.Dismissed, id)
	_ = config.Save(a.Cfg)
}

// RestoreDismissed brings every hidden recommendation back.
func (a *App) RestoreDismissed() {
	a.Cfg.Dismissed = nil
	_ = config.Save(a.Cfg)
}

// DismissedCount is what the page shows next to the restore control.
func (a *App) DismissedCount() int { return len(a.Cfg.Dismissed) }
