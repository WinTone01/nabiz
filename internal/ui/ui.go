package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/monitor"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

type tabID int

const (
	tabOverview tabID = iota
	tabTest
	tabLayers
	tabKernel
	tabBpftune
	tabUnwall
	tabDNS
	tabMonitor
	tabAdvice
	tabHistory
	tabReports
	tabHelp
)

type tabInfo struct {
	id    tabID
	key   string
	label string
}

func tabList() []tabInfo {
	return []tabInfo{
		{tabOverview, "1", i18n.T("nav.overview")},
		{tabTest, "2", i18n.T("nav.test")},
		{tabLayers, "3", i18n.T("nav.layers")},
		{tabKernel, "4", i18n.T("nav.kernel.short")},
		{tabBpftune, "5", "bpftune"},
		{tabUnwall, "6", i18n.T("nav.unwall.short")},
		{tabDNS, "7", "DNS"},
		{tabMonitor, "8", i18n.T("nav.monitor")},
		{tabAdvice, "9", i18n.T("nav.advice")},
		{tabHistory, "0", i18n.T("nav.history")},
		{tabReports, "p", i18n.T("nav.reports")},
		{tabHelp, "?", i18n.T("nav.help")},
	}
}

// --- messages -------------------------------------------------------------

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
	target string
	before suite.Result
	after  suite.Result
}
type toastMsg string

// liveEnv is the cheap snapshot refreshed on a timer for the live pages.
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

// --- app ------------------------------------------------------------------

// App is the shared state every page reads from. Pages never mutate it except
// through the helpers here, so there is exactly one place where a run starts,
// a language changes or a baseline is pinned.
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

	SuitePick int
	Running   string
	Phase     string
	Progress  float64

	width, height int
	cancel        context.CancelFunc
	msgCh         chan tea.Msg
}

// LastRun returns the most recent result, or nil.
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

// Width and Height are the usable body dimensions.
func (a *App) Width() int  { return a.width }
func (a *App) Height() int { return a.height }

type model struct {
	app   *App
	keys  keyMap
	help  help.Model
	spin  spinner.Model
	prog  progress.Model
	pages map[tabID]Page

	tab      tabID
	tabIndex int

	showHelp   bool
	confirming string
	toast      string
	toastTill  time.Time
	quitting   bool

	width, height int
}

// Run starts the interactive interface.
func Run(cfg config.Config, version string) error {
	program := tea.NewProgram(newModel(cfg, version),
		tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := program.Run()
	return err
}

func newModel(cfg config.Config, version string) *model {
	app := &App{
		Cfg:     cfg,
		Version: version,
		Results: map[string]suite.Result{},
		msgCh:   make(chan tea.Msg, 64),
	}
	app.Env = snapshotLive()
	if runs := report.ListRuns(1); len(runs) > 0 {
		if previous, err := report.LoadJSON(runs[0]); err == nil {
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

	helpModel := help.New()
	helpModel.Styles.ShortKey = sAcc
	helpModel.Styles.ShortDesc = sDim
	helpModel.Styles.FullKey = sAcc
	helpModel.Styles.FullDesc = sDim

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = sAcc

	m := &model{
		app:  app,
		keys: defaultKeys(),
		help: helpModel,
		spin: spin,
		prog: progress.New(progress.WithDefaultGradient(), progress.WithoutPercentage()),
	}
	m.pages = map[tabID]Page{
		tabOverview: newOverviewPage(),
		tabTest:     newTestPage(),
		tabLayers:   newLayersPage(),
		tabKernel:   newKernelPage(),
		tabBpftune:  newBpftunePage(),
		tabUnwall:   newUnwallPage(),
		tabDNS:      newDNSPage(),
		tabMonitor:  newMonitorPage(),
		tabAdvice:   newAdvicePage(),
		tabHistory:  newHistoryPage(),
		tabReports:  newReportsPage(),
		tabHelp:     newHelpPage(),
	}
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitFor(m.app.msgCh), m.spin.Tick, m.startMonitor())
}

func tickCmd() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func refreshEnvCmd() tea.Cmd {
	return func() tea.Msg { return envMsg{env: snapshotLive()} }
}

func waitFor(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m *model) startMonitor() tea.Cmd {
	if m.app.Watcher != nil {
		return nil
	}
	watcher := monitor.New(m.app.Cfg, nil)
	if err := watcher.Start(); err != nil {
		return func() tea.Msg { return toastMsg(i18n.T("ui.error", err.Error())) }
	}
	m.app.Watcher = watcher
	return nil
}

// --- update ---------------------------------------------------------------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		return m.onKey(msg)

	case tickMsg:
		cmds := []tea.Cmd{tickCmd()}
		if time.Now().Unix()%6 == 0 {
			cmds = append(cmds, refreshEnvCmd())
		}
		if m.isLivePage() {
			m.currentPage().Reload(m.app)
		}
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case envMsg:
		m.app.Env = msg.env
		m.currentPage().Reload(m.app)
		return m, nil

	case progressMsg:
		m.app.Phase, m.app.Progress = msg.phase, msg.fraction
		return m, waitFor(m.app.msgCh)

	case runDoneMsg:
		m.app.Running, m.app.Progress = "", 0
		m.app.Results[msg.name] = msg.result
		m.app.LastName = msg.name
		if path, err := report.Autosave(msg.result); err == nil {
			m.flash(i18n.T("ui.saved", filepath.Base(path)))
		}
		m.app.History = report.History(40)
		m.reloadAll()
		return m, tea.Batch(waitFor(m.app.msgCh), refreshEnvCmd())

	case runFailedMsg:
		m.app.Running = ""
		m.flash(i18n.T("ui.error", msg.err))
		return m, waitFor(m.app.msgCh)

	case abDoneMsg:
		m.app.Running = ""
		m.app.AB = &msg
		m.flash(i18n.T("misc.ab_result"))
		m.reloadAll()
		return m, tea.Batch(waitFor(m.app.msgCh), refreshEnvCmd())

	case toastMsg:
		m.flash(string(msg))
		return m, waitFor(m.app.msgCh)
	}

	return m, m.currentPage().Update(m.app, msg)
}

func (m *model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirming != "" {
		switch msg.String() {
		case "y", "e", "enter":
			target := m.confirming
			m.confirming = ""
			return m, m.startAB(target)
		default:
			m.confirming = ""
			return m, nil
		}
	}
	if m.showHelp {
		// any key closes the overlay; that is the whole contract
		m.showHelp = false
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		if m.app.cancel != nil {
			m.app.cancel()
		}
		if m.app.Watcher != nil {
			m.app.Watcher.Stop()
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.Lang):
		return m, m.switchLanguage()

	case key.Matches(msg, m.keys.NextTab):
		m.gotoTab((m.tabIndex + 1) % len(tabList()))
		return m, nil

	case key.Matches(msg, m.keys.PrevTab):
		m.gotoTab((m.tabIndex - 1 + len(tabList())) % len(tabList()))
		return m, nil

	case key.Matches(msg, m.keys.Run):
		return m, m.actionRun()

	case key.Matches(msg, m.keys.Stop):
		if m.app.cancel != nil {
			m.app.cancel()
			m.flash(i18n.T("ui.cancelled"))
		}
		return m, nil

	case key.Matches(msg, m.keys.Export):
		m.export()
		return m, nil

	case key.Matches(msg, m.keys.Baseline):
		m.pinBaseline()
		return m, nil

	case key.Matches(msg, m.keys.AB):
		if page, ok := m.currentPage().(abPage); ok {
			m.confirming = page.ABTarget()
			return m, nil
		}
	}

	// direct tab hotkeys
	for index, info := range tabList() {
		if msg.String() == info.key {
			m.gotoTab(index)
			return m, nil
		}
	}

	return m, m.currentPage().Update(m.app, msg)
}

// switchLanguage flips the language, re-derives every cached string and stores
// the choice so the next launch keeps it.
func (m *model) switchLanguage() tea.Cmd {
	lang := i18n.Toggle()
	m.keys.rebuild()
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
		m.flash(i18n.T("ui.error", err.Error()))
	} else {
		m.flash(i18n.T("ui.language_now", i18n.Name(lang)))
	}
	m.reloadAll()
	m.layout()
	return nil
}

func (m *model) gotoTab(index int) {
	list := tabList()
	if index < 0 || index >= len(list) {
		return
	}
	m.tabIndex, m.tab = index, list[index].id
	m.currentPage().Reload(m.app)
}

func (m *model) currentPage() Page { return m.pages[m.tab] }

func (m *model) isLivePage() bool {
	return m.tab == tabOverview || m.tab == tabMonitor
}

func (m *model) reloadAll() {
	for _, page := range m.pages {
		page.Reload(m.app)
	}
}

func (m *model) actionRun() tea.Cmd {
	if m.app.Running != "" {
		return nil
	}
	page, ok := m.currentPage().(runnablePage)
	if !ok {
		return nil
	}
	name := page.SuiteName(m.app)
	if name == "" {
		m.currentPage().Reload(m.app)
		return nil
	}
	return m.startSuite(name)
}

func (m *model) startSuite(name string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.app.cancel = cancel
	m.app.Running, m.app.Phase, m.app.Progress = name, "", 0
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
	if m.app.Running != "" {
		return nil
	}
	var (
		wasRunning bool
		setState   func(bool) (bool, string)
	)
	switch target {
	case "unwall":
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
			results[index] = suite.Run(ctx, cfg, "quick", func(phase string, fraction float64) {
				select {
				case ch <- progressMsg{phase: phase,
					fraction: (float64(index) + fraction) / 2}:
				default:
				}
			})
			if ctx.Err() != nil {
				ch <- runFailedMsg{err: i18n.T("ui.cancelled")}
				return
			}
		}
		ch <- abDoneMsg{target: target, before: results[0], after: results[1]}
	}()
	return waitFor(m.app.msgCh)
}

func (m *model) export() {
	result := m.app.LastRun()
	if result == nil {
		m.flash(i18n.T("ui.run_first"))
		return
	}
	stamp := result.StartedAt.Format("20060102-150405")
	dir := config.ExportDir()
	mdPath := filepath.Join(dir, fmt.Sprintf("nabiz-%s-%s.md", result.Name, stamp))
	if err := report.SaveMarkdown(*result, mdPath); err != nil {
		m.flash(i18n.T("ui.error", err.Error()))
		return
	}
	_ = report.SaveJSON(*result,
		filepath.Join(dir, fmt.Sprintf("nabiz-%s-%s.json", result.Name, stamp)))
	m.flash(i18n.T("ui.saved", mdPath))
}

func (m *model) pinBaseline() {
	result := m.app.LastRun()
	if result == nil {
		m.flash(i18n.T("ui.run_first"))
		return
	}
	if err := report.SaveBaseline(*result); err != nil {
		m.flash(i18n.T("ui.error", err.Error()))
		return
	}
	copied := *result
	m.app.Baseline = &copied
	m.flash(i18n.T("misc.baseline_set"))
	m.reloadAll()
}

func (m *model) flash(text string) {
	m.toast = text
	m.toastTill = time.Now().Add(5 * time.Second)
}

// --- layout ----------------------------------------------------------------

const (
	headerRows = 1 // brand line
	tabRows    = 2 // tab bar + separator
	footerRows = 1
	framePad   = 2 // rounded border top and bottom
)

func (m *model) layout() {
	bodyWidth := max(m.width-2, 20) // frame border
	bodyHeight := max(m.height-headerRows-tabRows-footerRows-framePad, 4)
	m.app.width, m.app.height = bodyWidth, bodyHeight
	m.prog.Width = min(max(bodyWidth/3, 12), 40)
	m.help.Width = bodyWidth
	for _, page := range m.pages {
		page.Layout(bodyWidth, bodyHeight)
	}
	m.reloadAll()
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if m.width < 64 || m.height < 18 {
		return sWarn.Render(i18n.T("ui.small_term", 64, 18))
	}
	if m.showHelp {
		return m.viewHelpOverlay()
	}
	body := m.currentPage().View(m.app)
	inner := lipgloss.JoinVertical(lipgloss.Left, m.viewTabs(), clip(body, m.app.height))
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewHeader(),
		sFrame.Width(m.app.width).Render(inner),
		m.viewFooter())
}

// clip pads or truncates a page body so the frame height never jumps between
// pages, which otherwise makes the whole interface twitch on every tab change.
func clip(text string, height int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m *model) viewHeader() string {
	left := sBrandTag.Render("Nabız") + sDim.Render(" "+m.app.Version)
	var parts []string
	if iface := m.app.Env.link.Iface; iface != "" {
		parts = append(parts, sText.Render(iface))
	}
	if m.app.Env.kernelNow != "" {
		parts = append(parts, sDim.Render(m.app.Env.kernelNow))
	}
	if m.app.Env.unwall.Installed {
		parts = append(parts, badge("unwall", m.app.Env.unwall.Running))
	}
	if m.app.Env.bpftune.Installed {
		parts = append(parts, badge("bpftune", m.app.Env.bpftune.Running))
	}
	parts = append(parts, sAcc.Render(strings.ToUpper(string(i18n.Current()))))
	right := strings.Join(parts, sFaint.Render(" · "))
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return " " + left + strings.Repeat(" ", gap-1) + right
}

func badge(name string, running bool) string {
	if running {
		return sText.Render(name) + " " + sOK.Render("●")
	}
	return sText.Render(name) + " " + sWarn.Render("○")
}

func (m *model) viewTabs() string {
	var rendered []string
	for _, info := range tabList() {
		label := info.key + " " + info.label
		if info.id == m.tab {
			rendered = append(rendered, sTabOn.Render(label))
			continue
		}
		rendered = append(rendered, sTabOff.Render(label))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, rendered...)
	if lipgloss.Width(row) > m.app.width {
		// narrow terminals lose the labels before they lose the numbers
		rendered = rendered[:0]
		for _, info := range tabList() {
			style := sTabOff
			if info.id == m.tab {
				style = sTabOn
			}
			rendered = append(rendered, style.Render(info.key))
		}
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, rendered...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, row,
		sRule.Render(strings.Repeat("─", m.app.width)))
}

func (m *model) viewFooter() string {
	left := m.help.ShortHelpView(m.keys.ShortHelp())
	right := m.viewStatus()
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return " " + left + strings.Repeat(" ", gap-1) + right
}

func (m *model) viewStatus() string {
	switch {
	case m.confirming != "":
		return sAcc.Bold(true).Render(i18n.T("ui.confirm_ab", m.confirming))
	case m.toast != "" && time.Now().Before(m.toastTill):
		return sAcc.Bold(true).Render(m.toast)
	case m.app.Running != "":
		return m.spin.View() + " " + sAcc.Render(m.app.Running) + " " +
			m.prog.ViewAs(m.app.Progress) + " " +
			sDim.Render(fmt.Sprintf("%3d%%", int(m.app.Progress*100)))
	}
	if result := m.app.LastRun(); result != nil {
		return sDim.Render(i18n.T("f.score")+" ") +
			scoreStyle(result.Score).Render(
				fmt.Sprintf("%.1f (%s)", result.Score, result.Grade))
	}
	return ""
}

func (m *model) viewHelpOverlay() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		sSection.Render(i18n.T("nav.help")),
		"",
		m.help.FullHelpView(m.keys.FullHelp()),
		"",
		sFaint.Render(i18n.T("ui.any_key")))
	box := sOverlay.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
