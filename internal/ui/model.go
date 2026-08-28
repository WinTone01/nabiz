// Package ui is the terminal interface.
//
// The shape of it: a fixed frame - title row, navigation, page, status - with
// one Page rendered inside it at a time, and floating surfaces (a dialog, the
// command palette, the help sheet) composited over the top rather than instead
// of it. Pages own their own components, so each screen scrolls and selects on
// its own terms instead of every screen being one pre-rendered string.
//
// Three rules hold the whole thing together. Nothing outside widgets.go draws a
// border or picks a padding, so twelve screens written at different times look
// like one application. Nothing measures styled text with len(), because escape
// bytes are not columns. And no page changes state: pages render, the root
// model acts, which is why two screens can never disagree about what happened.
package ui

import (
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/WinTone01/nabiz/internal/apply"
	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/monitor"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/suite"
)

// The root model owns the frame, the focus ring and the message loop. What it
// can do to the machine lives in actions.go; how the frame is drawn lives in
// chrome.go. Layout is fixed so that moving between pages moves only the
// content: an interface whose furniture shifts as you walk through it reads as
// unstable even when every number in it is right.

type focusTarget int

const (
	focusNav focusTarget = iota
	focusContent
)

const (
	headerRows = 1
	ruleRows   = 1
	statusRows = 2
	toolbarRow = 1
	gapRow     = 1

	// Below this the sidebar costs more than it gives and navigation collapses
	// to a strip of shortcuts.
	sidebarBreakpoint = 100
)

type model struct {
	app  *App
	keys keyMap

	help    help.Model
	spin    spinner.Model
	prog    progress.Model
	nav     *nav
	pages   map[pageID]Page
	toasts  toastStack
	palette *palette
	dlg     *dialog

	focus    focusTarget
	showHelp bool
	sidebar  bool
	quitting bool

	width, height int
}

// Run starts the interactive interface.
func Run(cfg config.Config, version string) error {
	// One screen, one zone manager: every control marks itself at render time
	// and the manager is what turns a pixel back into an identity.
	zone.NewGlobal()
	program := tea.NewProgram(newModel(cfg, version),
		tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := program.Run()
	return err
}

func newModel(cfg config.Config, version string) *model {
	app := &App{
		Cfg:        cfg,
		Version:    version,
		Results:    map[string]suite.Result{},
		Selected:   map[string]bool{},
		Applicable: map[string]apply.Change{},
		msgCh:      make(chan tea.Msg, 64),
	}
	app.Env = snapshotLive()
	if previous, _, err := report.LatestRun(); err == nil {
		{
			// saved text carries whatever language it was measured in
			suite.Refresh(&previous, cfg)
			app.Results[previous.Name] = previous
			app.LastName = previous.Name
		}
	}
	if baseline, err := report.LoadBaseline(); err == nil {
		app.Baseline = baseline
	}
	app.History = report.History(40)
	if snapshots, err := apply.List(); err == nil {
		app.HasSnapshots = len(snapshots) > 0
	}

	helper := help.New()
	helper.Styles.ShortKey = sAcc
	helper.Styles.ShortDesc = sFaint
	helper.Styles.ShortSeparator = sLine
	helper.Styles.FullKey = sAcc
	helper.Styles.FullDesc = sMuted
	helper.Styles.FullSeparator = sLine

	spin := spinner.New()
	spin.Spinner = spinner.MiniDot
	spin.Style = sAcc

	m := &model{
		app:     app,
		keys:    defaultKeys(),
		help:    helper,
		spin:    spin,
		prog:    progress.New(progress.WithGradient("#8b5cf6", "#22b8d4"), progress.WithoutPercentage()),
		nav:     newNav(),
		palette: newPalette(),
		focus:   focusContent,
	}
	m.pages = map[pageID]Page{
		pageOverview: newOverviewPage(),
		pageMonitor:  newMonitorPage(),
		pageTest:     newTestPage(),
		pageLayers:   newLayersPage(),
		pageDNS:      newDNSPage(),
		pageKernel:   newKernelPage(),
		pageBpftune:  newBpftunePage(),
		pageUnwall:   newUnwallPage(),
		pageAdvice:   newAdvicePage(),
		pageHistory:  newHistoryPage(),
		pageReports:  newReportsPage(),
		pageHelp:     newHelpPage(),
	}
	return m
}

func (m *model) Init() tea.Cmd {
	m.setFocus(focusContent)
	return tea.Batch(tickCmd(), waitFor(m.app.msgCh), m.spin.Tick, m.startMonitor())
}

func tickCmd() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func refreshEnvCmd() tea.Cmd {
	return func() tea.Msg { return envMsg{env: snapshotLive()} }
}

// waitFor turns the worker channel into a Bubble Tea command. Measurements run
// on their own goroutine and post here, so a slow probe never blocks a redraw.
func waitFor(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m *model) startMonitor() tea.Cmd {
	if m.app.Watcher != nil {
		return nil
	}
	watcher := monitor.New(m.app.Cfg, nil)
	if err := watcher.Start(); err != nil {
		return func() tea.Msg {
			return noticeMsg{text: i18n.T("ui.error", err.Error()), level: toastBad}
		}
	}
	m.app.Watcher = watcher
	return nil
}

func (m *model) page() Page { return m.pages[m.nav.currentID()] }

func (m *model) isLivePage() bool {
	id := m.nav.currentID()
	return id == pageOverview || id == pageMonitor
}

func (m *model) reloadAll() {
	for _, page := range m.pages {
		page.Reload(m.app)
	}
}

func (m *model) notify(text string, level toastLevel) { m.toasts.push(text, level) }

// --- update -------------------------------------------------------------------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tea.MouseMsg:
		return m.onMouse(msg)

	case tea.KeyMsg:
		return m.onKey(msg)

	case tickMsg:
		cmds := []tea.Cmd{tickCmd()}
		if time.Now().Unix()%6 == 0 {
			cmds = append(cmds, refreshEnvCmd())
		}
		if m.isLivePage() {
			m.page().Reload(m.app)
		}
		m.toasts.prune()
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case envMsg:
		m.app.Env = msg.env
		m.page().Reload(m.app)
		return m, nil

	case progressMsg:
		m.app.Phase, m.app.Progress = msg.phase, msg.fraction
		return m, waitFor(m.app.msgCh)

	case runDoneMsg:
		return m, m.onRunDone(msg)

	case runFailedMsg:
		m.app.Running = ""
		m.notify(i18n.T("ui.error", msg.err), toastBad)
		return m, waitFor(m.app.msgCh)

	case abDoneMsg:
		m.app.Running = ""
		m.app.AB = &msg
		// remember when a comparison last ran, so the suggestion to run one is
		// not repeated after every measurement
		m.app.Cfg.LastComparison = time.Now().Format(time.RFC3339)
		_ = config.Save(m.app.Cfg)
		// the comparison is the only thing that can prove a hostlist entry is a
		// false positive, so carry that finding into the stored run and
		// re-derive: the advice list gains an item the single-run path cannot
		// produce
		if len(msg.notNeeded) > 0 {
			if result := m.app.LastRun(); result != nil {
				updated := *result
				updated.DesyncNotNeeded = msg.notNeeded
				suite.Refresh(&updated, m.app.Cfg)
				m.app.Results[updated.Name] = updated
				_, _ = report.Autosave(updated)
			}
		}
		m.notify(i18n.T("misc.ab_result"), toastGood)
		m.reloadAll()
		return m, tea.Batch(waitFor(m.app.msgCh), refreshEnvCmd())

	case applyDoneMsg:
		return m, m.onApplyDone(msg)

	case noticeMsg:
		m.notify(msg.text, msg.level)
		return m, waitFor(m.app.msgCh)

	case applyRequestMsg:
		m.askApply()
		return m, nil

	case cursor.BlinkMsg:
		// The palette's caret blinks on its own timer, which only reaches it if
		// the message is routed while the palette is the thing on screen.
		if m.palette.open {
			return m, m.palette.update(msg)
		}
		return m, nil
	}
	return m, m.page().Update(m.app, msg)
}

func (m *model) onRunDone(msg runDoneMsg) tea.Cmd {
	m.app.Running, m.app.Progress, m.app.Phase = "", 0, ""
	m.app.Results[msg.name] = msg.result
	m.app.LastName = msg.name
	if path, err := report.Autosave(msg.result); err == nil {
		m.notify(i18n.T("ui.saved", filepath.Base(path)), toastGood)
	}
	m.app.History = report.History(40)
	m.reloadAll()

	level := toastGood
	switch {
	case msg.result.CountFindings("bad") > 0:
		level = toastBad
	case msg.result.CountFindings("warn") > 0:
		level = toastWarn
	}
	m.notify(i18n.T("misc.score_line", msg.result.Score, msg.result.Grade), level)
	return tea.Batch(waitFor(m.app.msgCh), refreshEnvCmd())
}

func (m *model) onApplyDone(msg applyDoneMsg) tea.Cmd {
	m.app.Running = ""
	if snapshots, err := apply.List(); err == nil {
		m.app.HasSnapshots = len(snapshots) > 0
	}
	switch {
	case msg.rolledBack:
		m.notify(i18n.T("apply.rolledback"), toastWarn)
	case msg.err != "":
		m.notify(i18n.T("ui.error", msg.err), toastBad)
		return tea.Batch(waitFor(m.app.msgCh), refreshEnvCmd())
	default:
		m.notify(i18n.T("apply.ok"), toastGood)
	}
	// Advice is derived from a stored run, so applying a change does not remove
	// it from the list - the run still describes the machine as it was. Re-read
	// the environment into the stored result and re-derive: the measurements
	// stay, the machine state is current, and applied items drop off the list.
	m.app.Selected = map[string]bool{}
	if result := m.app.LastRun(); result != nil {
		updated := *result
		suite.RefreshEnv(&updated, m.app.Cfg)
		m.app.Results[updated.Name] = updated
		_, _ = report.Autosave(updated)
	}
	m.reloadAll()
	return tea.Batch(waitFor(m.app.msgCh), refreshEnvCmd())
}

// --- keyboard -------------------------------------------------------------------

// onKey routes by depth: whatever is floating gets the key first, then the
// global bindings, then the page. Nothing below can steal esc or enter from
// something above it.
func (m *model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.dlg != nil {
		return m, m.dialogKey(msg)
	}
	if m.palette.open {
		return m, m.paletteKey(msg)
	}
	if m.showHelp {
		if key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Help) ||
			key.Matches(msg, m.keys.Quit) || msg.String() == "enter" {
			m.showHelp = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		// Quitting mid-run throws away a measurement that took minutes, so that
		// is the one case worth a question. Idle, q just quits.
		if m.app.Busy() {
			m.askQuit()
			return m, nil
		}
		return m, m.quit()

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.Palette):
		m.palette.show(m.commands())
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Lang):
		return m, m.switchLanguage()

	case key.Matches(msg, m.keys.Focus):
		m.setFocus(focusTarget(1 - int(m.focus)))
		return m, nil

	case key.Matches(msg, m.keys.FocusBack):
		m.setFocus(focusTarget(1 - int(m.focus)))
		return m, nil

	case key.Matches(msg, m.keys.NextPage):
		m.gotoIndex(m.nav.active + 1)
		return m, nil

	case key.Matches(msg, m.keys.PrevPage):
		m.gotoIndex(m.nav.active - 1)
		return m, nil

	case key.Matches(msg, m.keys.Run):
		return m, m.actionRun()

	case key.Matches(msg, m.keys.Stop):
		m.actionStop()
		return m, nil

	case key.Matches(msg, m.keys.Export):
		m.actionExport()
		return m, nil

	case key.Matches(msg, m.keys.Baseline):
		m.actionBaseline()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		m.notify(i18n.T("ui.refreshed"), toastInfo)
		return m, refreshEnvCmd()

	case key.Matches(msg, m.keys.AB):
		if page, ok := m.page().(abPage); ok {
			m.askAB(page.ABTarget())
			return m, nil
		}

	case key.Matches(msg, m.keys.Back):
		// esc with nothing open means "go back to the page", which is the only
		// way out of the sidebar that does not require aiming at a tab key.
		if m.focus == focusNav {
			m.setFocus(focusContent)
			return m, nil
		}
	}

	// Direct shortcuts win over page keys: a screen is always one keystroke away.
	for index, item := range m.nav.items {
		if msg.String() == item.key {
			m.gotoIndex(index)
			return m, nil
		}
	}

	if m.focus == focusNav {
		return m, m.navKey(msg)
	}
	return m, m.page().Update(m.app, msg)
}

func (m *model) navKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Down):
		m.nav.moveCursor(1)
	case key.Matches(msg, m.keys.Up):
		m.nav.moveCursor(-1)
	case key.Matches(msg, m.keys.Home):
		m.nav.cursor = 0
	case key.Matches(msg, m.keys.End):
		m.nav.cursor = len(m.nav.items) - 1
	case key.Matches(msg, m.keys.Activate), key.Matches(msg, m.keys.Right):
		m.gotoIndex(m.nav.cursor)
		m.setFocus(focusContent)
	}
	return nil
}

func (m *model) dialogKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.dlg = nil
	case key.Matches(msg, m.keys.Left), key.Matches(msg, m.keys.FocusBack):
		m.dlg.move(-1)
	case key.Matches(msg, m.keys.Right), key.Matches(msg, m.keys.Focus):
		m.dlg.move(1)
	case key.Matches(msg, m.keys.Activate):
		dialog := m.dlg
		m.dlg = nil
		return dialog.activate()
	case msg.String() == "y":
		dialog := m.dlg
		m.dlg = nil
		dialog.focus = 0
		return dialog.activate()
	case msg.String() == "n":
		m.dlg = nil
	}
	return nil
}

func (m *model) paletteKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Palette):
		m.palette.hide()
		return nil
	case key.Matches(msg, m.keys.Activate):
		selected := m.palette.selected()
		m.palette.hide()
		if selected != nil && selected.run != nil {
			return selected.run()
		}
		return nil
	case msg.Type == tea.KeyDown:
		m.palette.move(1)
		return nil
	case msg.Type == tea.KeyUp:
		m.palette.move(-1)
		return nil
	}
	return m.palette.update(msg)
}

func (m *model) setFocus(target focusTarget) {
	m.focus = target
	m.nav.focused = target == focusNav
	if page, ok := m.page().(focusablePage); ok {
		page.Focus(target == focusContent)
	}
}

func (m *model) gotoIndex(index int) {
	count := len(m.nav.items)
	m.nav.selectIndex(((index % count) + count) % count)
	m.setFocus(m.focus)
	m.page().Reload(m.app)
}

func (m *model) gotoPage(id pageID) {
	m.nav.selectID(id)
	m.setFocus(m.focus)
	m.page().Reload(m.app)
}

// --- mouse ------------------------------------------------------------------------

func (m *model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.dlg != nil {
		if index := m.dlg.clickedButton(msg); index >= 0 {
			dialog := m.dlg
			dialog.focus = index
			m.dlg = nil
			return m, dialog.activate()
		}
		return m, nil
	}
	if m.palette.open {
		if row := m.palette.clickedIndex(msg, paletteRows); row >= 0 {
			m.palette.cursor = clamp(row, 0, max(len(m.palette.matches)-1, 0))
			selected := m.palette.selected()
			m.palette.hide()
			if selected != nil && selected.run != nil {
				return m, selected.run()
			}
		}
		return m, nil
	}
	if m.showHelp {
		if isPress(msg) {
			m.showHelp = false
		}
		return m, nil
	}

	if isPress(msg) {
		for index, item := range m.nav.items {
			if clicked(msg, navZone(item.id)) {
				m.gotoIndex(index)
				m.setFocus(focusContent)
				return m, nil
			}
		}
		if cmd, handled := m.toolbarClick(msg); handled {
			return m, cmd
		}
		// A click in the content region is a claim on it. Without this the
		// keyboard would still be driving the sidebar after the reader had
		// visibly started working somewhere else.
		m.setFocus(focusContent)
	}
	return m, m.page().Update(m.app, msg)
}
