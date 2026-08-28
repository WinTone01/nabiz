package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/apply"
	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// Actions.
//
// Everything the interface can do to the machine lives here: starting and
// stopping a measurement, exporting, pinning a baseline, switching language,
// and the two dialogs that guard applying a change and rolling one back.
//
// Pages never call any of it directly. They report what they are - runnable,
// A/B-able - and the root model decides, which is why there is exactly one
// place a run can start and exactly one place a change can be applied.

// --- actions ------------------------------------------------------------------------

func (m *model) quit() tea.Cmd {
	m.quitting = true
	if m.app.cancel != nil {
		m.app.cancel()
	}
	if m.app.Watcher != nil {
		m.app.Watcher.Stop()
	}
	return tea.Quit
}

func (m *model) actionRun() tea.Cmd {
	if m.app.Busy() {
		return nil
	}
	page, ok := m.page().(runnablePage)
	if !ok {
		m.notify(i18n.T("ui.not_here"), toastInfo)
		return nil
	}
	name := page.SuiteName(m.app)
	if name == "" {
		m.page().Reload(m.app)
		return nil
	}
	return m.startSuite(name)
}

func (m *model) actionStop() {
	if m.app.cancel == nil || !m.app.Busy() {
		return
	}
	m.app.cancel()
	m.notify(i18n.T("ui.cancelled"), toastWarn)
}

func (m *model) startSuite(name string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.app.cancel = cancel
	m.app.Running, m.app.Phase, m.app.Progress = name, "", 0
	m.app.StartedAt = time.Now()
	cfg, ch, baseline := m.app.Cfg, m.app.msgCh, m.app.Baseline
	go func() {
		defer cancel()
		result := suite.Run(ctx, cfg, name, func(phase string, fraction float64) {
			select {
			case ch <- progressMsg{phase: phase, fraction: fraction}:
			default: // never block a measurement on a slow renderer
			}
		})
		if baseline != nil {
			result.Baseline = &suite.BaselineRef{
				Score: baseline.Score, Grade: baseline.Grade,
				StartedAt: baseline.StartedAt.Format(time.RFC3339),
			}
		}
		ch <- runDoneMsg{name: name, result: result}
	}()
	return waitFor(m.app.msgCh)
}

func (m *model) startAB(target string) tea.Cmd {
	if m.app.Busy() {
		return nil
	}
	// The suite is chosen for what the comparison is meant to show: stopping
	// zapret is only interesting against the domains it was protecting, and
	// stopping bpftune is only interesting under load.
	abSuite := "quick"
	var (
		wasRunning bool
		setState   func(bool) (bool, string)
	)
	switch target {
	case "unwall":
		abSuite = "dpi"
		state := sysinfo.ReadUnwall()
		wasRunning, setState = state.Running, sysinfo.SetUnwall
	case "bpftune":
		state := sysinfo.ReadBpftune()
		wasRunning, setState = state.Running, sysinfo.SetBpftune
	default:
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.app.cancel = cancel
	m.app.Running = "ab:" + target
	m.app.StartedAt = time.Now()
	cfg, ch := m.app.Cfg, m.app.msgCh
	go func() {
		defer cancel()
		// the service is restored whatever happens, including a panic path
		defer func() { _, _ = setState(wasRunning) }()
		var results [2]suite.Result
		for index, want := range []bool{true, false} {
			if ok, message := setState(want); !ok {
				ch <- runFailedMsg{err: message}
				return
			}
			time.Sleep(3 * time.Second)
			results[index] = suite.Run(ctx, cfg, abSuite, func(phase string, fraction float64) {
				select {
				case ch <- progressMsg{phase: phase, fraction: (float64(index) + fraction) / 2}:
				default:
				}
			})
			if ctx.Err() != nil {
				ch <- runFailedMsg{err: i18n.T("ui.cancelled")}
				return
			}
		}
		message := abDoneMsg{target: target, before: results[0], after: results[1]}
		if target == "unwall" {
			message.notNeeded = suite.DomainsWithoutDesync(results[1])
		}
		ch <- message
	}()
	return waitFor(m.app.msgCh)
}

func (m *model) actionExport() {
	result := m.app.LastRun()
	if result == nil {
		m.notify(i18n.T("ui.run_first"), toastWarn)
		return
	}
	stamp := result.StartedAt.Format("20060102-150405")
	dir := config.ExportDir()
	mdPath := filepath.Join(dir, fmt.Sprintf("nabiz-%s-%s.md", result.Name, stamp))
	if err := report.SaveMarkdown(*result, mdPath); err != nil {
		m.notify(i18n.T("ui.error", err.Error()), toastBad)
		return
	}
	_ = report.SaveJSON(*result,
		filepath.Join(dir, fmt.Sprintf("nabiz-%s-%s.json", result.Name, stamp)))
	m.notify(i18n.T("ui.saved", mdPath), toastGood)
}

func (m *model) actionBaseline() {
	result := m.app.LastRun()
	if result == nil {
		m.notify(i18n.T("ui.run_first"), toastWarn)
		return
	}
	if err := report.SaveBaseline(*result); err != nil {
		m.notify(i18n.T("ui.error", err.Error()), toastBad)
		return
	}
	copied := *result
	m.app.Baseline = &copied
	m.notify(i18n.T("misc.baseline_set"), toastGood)
	m.reloadAll()
}

// switchLanguage flips the language, re-derives every cached string and stores
// the choice so the next launch keeps it.
func (m *model) switchLanguage() tea.Cmd {
	lang := i18n.Toggle()
	m.keys.rebuild()
	m.nav.rebuild()
	for name, result := range m.app.Results {
		suite.Refresh(&result, m.app.Cfg)
		m.app.Results[name] = result
	}
	if m.app.Baseline != nil {
		suite.Refresh(m.app.Baseline, m.app.Cfg)
	}
	if m.app.AB != nil {
		suite.Refresh(&m.app.AB.before, m.app.Cfg)
		suite.Refresh(&m.app.AB.after, m.app.Cfg)
	}
	m.app.Cfg.Lang = string(lang)
	if err := config.Save(m.app.Cfg); err != nil {
		m.notify(i18n.T("ui.error", err.Error()), toastBad)
	} else {
		m.notify(i18n.T("ui.language_now", i18n.Name(lang)), toastGood)
	}
	m.layout()
	return nil
}

// --- dialogs -----------------------------------------------------------------------

func (m *model) askAB(target string) {
	if target == "" {
		return
	}
	m.dlg = confirmDialog(i18n.T("key.ab"), i18n.T("ui.confirm_ab", target),
		i18n.T("ui.start"), btnPrimary, colAccent,
		func() tea.Cmd { return m.startAB(target) })
}

// askApply shows exactly what is about to happen before anything runs, and
// names the snapshot directory: the promise being made is that the change is
// undoable, so the reader should be able to see where the undo lives.
func (m *model) askApply() {
	var selected []apply.Change
	for id, on := range m.app.Selected {
		if !on {
			continue
		}
		if change, ok := m.app.Applicable[id]; ok {
			selected = append(selected, change)
		}
	}
	if len(selected) == 0 {
		return
	}
	if !util.IsRoot() && util.Which("pkexec") == "" {
		m.notify(i18n.T("apply.needs_pkexec"), toastBad)
		return
	}
	snapshot, err := apply.Prepare(selected)
	if err != nil {
		m.notify(i18n.T("ui.error", err.Error()), toastBad)
		return
	}
	var names []string
	for _, change := range selected {
		names = append(names, "• "+change.ID+" — "+change.Title)
	}
	body := i18n.T("apply.dialog.body", strings.Join(names, "\n")+"\n\n"+snapshot.Dir)
	m.dlg = confirmDialog(i18n.T("apply.dialog.title", len(selected)), body,
		i18n.T("ui.apply"), btnSuccess, colOK,
		func() tea.Cmd { return m.runApply(snapshot) })
}

func (m *model) runApply(snapshot *apply.Snapshot) tea.Cmd {
	m.app.Running, m.app.Progress = i18n.T("apply.running"), 0
	m.app.StartedAt = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	m.app.cancel = cancel
	ch := m.app.msgCh
	go func() {
		defer cancel()
		outcome := apply.Apply(ctx, snapshot, true)
		message := applyDoneMsg{rolledBack: outcome.RolledBack, snapshot: snapshot.Dir}
		if outcome.Err != nil {
			message.err = outcome.Err.Error()
		}
		ch <- message
	}()
	return waitFor(m.app.msgCh)
}

func (m *model) askRollback() {
	snapshots, err := apply.List()
	if err != nil || len(snapshots) == 0 {
		m.notify(i18n.T("apply.nosnapshots"), toastWarn)
		return
	}
	latest := snapshots[len(snapshots)-1]
	var names []string
	for _, change := range latest.Changes {
		names = append(names, "• "+change.ID)
	}
	body := i18n.T("apply.dialog.rollback", strings.Join(names, "\n")+"\n\n"+latest.Dir)
	m.dlg = confirmDialog(i18n.T("apply.rollback"), body, i18n.T("apply.rollback"),
		btnDanger, colBad, func() tea.Cmd { return m.runRollback(latest.Dir) })
}

func (m *model) runRollback(dir string) tea.Cmd {
	m.app.Running = i18n.T("apply.running")
	m.app.StartedAt = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	m.app.cancel = cancel
	ch := m.app.msgCh
	go func() {
		defer cancel()
		_, err := apply.Rollback(ctx, dir)
		message := applyDoneMsg{snapshot: dir}
		if err != nil {
			message.err = err.Error()
		}
		ch <- message
	}()
	return waitFor(m.app.msgCh)
}

// askQuit guards the one irreversible thing in the interface: a run in flight
// is minutes of measurement that cannot be recovered by starting again.
func (m *model) askQuit() {
	m.dlg = confirmDialog(i18n.T("key.quit"), i18n.T("ui.confirm_quit"),
		i18n.T("key.quit"), btnDanger, colBad, func() tea.Cmd { return m.quit() })
}
