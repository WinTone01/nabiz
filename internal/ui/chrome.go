package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// The chrome: everything around the page.
//
// It is fixed. The title row, the rules, the navigation column, the toolbar and
// the two status rows are in the same place on every screen and at every size,
// so moving between pages moves only the content. An interface whose furniture
// shifts as you walk through it reads as unstable even when every number in it
// is right.

// --- layout --------------------------------------------------------------------

func (m *model) layout() {
	m.sidebar = m.width >= sidebarBreakpoint

	navRows := 0
	if !m.sidebar {
		navRows = 1
	}
	bodyHeight := m.height - headerRows - navRows - ruleRows*2 - statusRows
	bodyHeight = max(bodyHeight, 4)

	contentWidth := m.width
	if m.sidebar {
		contentWidth -= sidebarWidth
	}
	contentWidth = max(contentWidth-2, 20) // one column of breathing room each side

	pageHeight := max(bodyHeight-toolbarRow-gapRow, 3)
	m.app.width, m.app.height = contentWidth, pageHeight

	m.prog.Width = clamp(contentWidth/3, 14, 44)
	m.help.Width = m.width - 4
	for _, page := range m.pages {
		page.Layout(contentWidth, pageHeight)
	}
	m.reloadAll()
}

// firstBodyRow is where the content region starts, which is where a toast is
// allowed to land: over the page, never over the frame's own furniture.
func (m *model) firstBodyRow() int {
	if m.sidebar {
		return headerRows + ruleRows
	}
	return headerRows + 1 + ruleRows
}

func (m *model) bodyHeight() int {
	navRows := 0
	if !m.sidebar {
		navRows = 1
	}
	return max(m.height-headerRows-navRows-ruleRows*2-statusRows, 4)
}

// --- view ----------------------------------------------------------------------

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if m.width < 60 || m.height < 16 {
		return sWarn.Render(i18n.T("ui.small_term", 60, 16))
	}

	screen := m.viewFrame()

	// Floating surfaces are composited over the frame, never instead of it: the
	// numbers that justify a change stay readable while the change is approved.
	if !m.toasts.empty() {
		block := m.toasts.view(m.width - 6)
		screen = overlay(screen, block, max(m.width-blockWidth(block)-2, 0), m.firstBodyRow())
	}
	if m.showHelp {
		screen = overlayCentre(screen, m.viewHelpSheet(), m.width, m.height)
	}
	if m.palette.open {
		screen = overlayCentre(screen, m.palette.view(m.width), m.width, m.height)
	}
	if m.dlg != nil {
		screen = overlayCentre(screen, m.dlg.view(m.width), m.width, m.height)
	}
	return zone.Scan(screen)
}

func (m *model) viewFrame() string {
	fullRule := sLine.Render(strings.Repeat("─", m.width))
	body := m.viewBody()

	rows := []string{m.viewHeader()}
	if !m.sidebar {
		rows = append(rows, " "+m.nav.viewStrip(m.width-2))
	}
	rows = append(rows, fullRule, body, fullRule, m.viewInsight(), m.viewStatus())
	return strings.Join(rows, "\n")
}

// viewBody puts the navigation beside the page, or above it on a narrow
// terminal where twenty columns of sidebar would cost more than they explain.
func (m *model) viewBody() string {
	height := m.bodyHeight()
	content := clampBlock(m.viewContent(), height)
	if !m.sidebar {
		return indentBlock(content, 1)
	}
	side := m.nav.viewSidebar(m.app, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, side, indentBlock(content, 1))
}

// indentBlock shifts a block right without letting lipgloss re-measure it,
// which is what a padded style would do to the coloured cells inside a table.
func indentBlock(block string, columns int) string {
	pad := strings.Repeat(" ", columns)
	lines := splitLines(block)
	for index, line := range lines {
		lines[index] = pad + line
	}
	return strings.Join(lines, "\n")
}

func (m *model) viewContent() string {
	page := m.page()
	// a rule under the global toolbar: without it the page's own action bar sits
	// directly beneath and the two rows read as one crowded strip
	return joinCol(m.viewToolbar(), "",
		sLine.Render(strings.Repeat("─", m.app.Width())), "", page.View(m.app))
}

// --- header ---------------------------------------------------------------------

func (m *model) viewHeader() string {
	left := sBrand.Render("NABIZ") + sFaint.Render(" "+m.app.Version) +
		sLine.Render("  │  ") + sBold.Render(m.nav.current().label)

	// The facts are dropped from the front as the terminal narrows rather than
	// truncated: half an interface name tells you nothing, whereas losing the
	// kernel version to keep the tool indicators is a real trade.
	facts := m.headerFacts()
	controls := m.headerControls()
	// The controls are never dropped - they are the only mouse route to the
	// language switch and to quitting - so they are joined onto a copy each
	// time rather than appended into the slice being trimmed.
	join := func(items []string) string {
		return strings.Join(append(append([]string{}, items...), controls),
			sLine.Render(" · "))
	}
	right := join(facts)
	for len(facts) > 0 && visWidth(left)+visWidth(right)+4 > m.width {
		facts = facts[1:]
		right = join(facts)
	}
	gap := m.width - visWidth(left) - visWidth(right) - 2
	if gap < 1 {
		left = truncate(left, max(m.width-visWidth(right)-4, 8))
		gap = max(m.width-visWidth(left)-visWidth(right)-2, 1)
	}
	return " " + left + strings.Repeat(" ", gap) + right + " "
}

// headerFacts are the four things worth knowing before reading any page: which
// interface, which kernel, whether the two tools that rewrite your networking
// are running, and which language the text is in.
func (m *model) headerFacts() []string {
	env := m.app.Env
	var parts []string
	if env.link.Iface != "" {
		speed := sText.Render(env.link.Iface)
		if env.link.SpeedMbit > 0 {
			style := sMuted
			// A gigabit card negotiating 100 Mbit is the single most common
			// cause of "my internet is broken", so it is coloured in the title.
			if env.link.SpeedMbit <= 100 && !env.link.Wireless {
				style = sWarn
			}
			speed += " " + style.Render(fmt.Sprintf("%d", env.link.SpeedMbit)+"M")
		}
		parts = append(parts, speed)
	}
	if env.kernelNow != "" {
		parts = append(parts, sFaint.Render(env.kernelNow))
	}
	if env.unwall.Installed {
		parts = append(parts, statusDot("unwall", env.unwall.Running))
	}
	if env.bpftune.Installed {
		parts = append(parts, statusDot("bpftune", env.bpftune.Running))
	}
	return parts
}

// headerControls keeps every global action reachable with the mouse alone.
//
// The language switch belongs here rather than in the toolbar because the
// header is where the current language is already stated, and the thing that
// tells you the setting should be the thing that changes it.
func (m *model) headerControls() string {
	return zone.Mark(zoneLang, sAcc.Bold(true).Render(
		i18n.Upper(string(i18n.Current())))) + sLine.Render(" · ") +
		zone.Mark(zoneHelp, sMuted.Render("?")) + sLine.Render(" · ") +
		zone.Mark(zoneQuit, sMuted.Render(i18n.T("key.quit")))
}

// --- toolbar ---------------------------------------------------------------------

const (
	zoneRun      = "tb:run"
	zoneStop     = "tb:stop"
	zoneExport   = "tb:export"
	zoneBaseline = "tb:baseline"
	zoneAB       = "tb:ab"
	zoneLang     = "tb:lang"
	zoneHelp     = "tb:help"
	zoneQuit     = "tb:quit"

	zoneApply     = "adv:apply"
	zoneRollback  = "adv:rollback"
	zoneSelectAll = "adv:selectall"
	zoneClearSel  = "adv:clear"
)

// viewToolbar is the row of real buttons. Everything on it also has a key; the
// point of the row is that none of them have to be remembered.
//
// The labels are words. A diagnostic tool that decorates its actions with
// pictographs looks like something a template produced, and the reader has to
// guess at the one glyph that means "set baseline".
func (m *model) viewToolbar() string {
	busy := m.app.Busy()
	_, canRun := m.page().(runnablePage)
	_, canAB := m.page().(abPage)
	hasRun := m.app.LastRun() != nil

	controls := []string{
		button(zoneRun, i18n.T("key.run"), btnPrimary, canRun && !busy),
		button(zoneStop, i18n.T("key.stop"), btnDanger, busy),
		button(zoneExport, i18n.T("key.export"), btnGhost, hasRun),
		button(zoneBaseline, i18n.T("key.baseline"), btnGhost, hasRun),
	}
	if canAB {
		controls = append(controls, button(zoneAB, i18n.T("key.ab"), btnGhost, !busy))
	}
	return toolbar(controls...)
}

func (m *model) toolbarClick(msg tea.MouseMsg) (tea.Cmd, bool) {
	switch {
	case clicked(msg, zoneRun):
		return m.actionRun(), true
	case clicked(msg, zoneStop):
		m.actionStop()
		return nil, true
	case clicked(msg, zoneExport):
		m.actionExport()
		return nil, true
	case clicked(msg, zoneBaseline):
		m.actionBaseline()
		return nil, true
	case clicked(msg, zoneAB):
		if page, ok := m.page().(abPage); ok {
			m.askAB(page.ABTarget())
		}
		return nil, true
	case clicked(msg, zoneLang):
		return m.switchLanguage(), true
	case clicked(msg, zoneHelp):
		m.showHelp = true
		return nil, true
	case clicked(msg, zoneQuit):
		if m.app.Busy() {
			m.askQuit()
			return nil, true
		}
		return m.quit(), true
	case clicked(msg, zoneApply):
		m.askApply()
		return nil, true
	case clicked(msg, zoneRollback):
		m.askRollback()
		return nil, true
	case clicked(msg, zoneSelectAll):
		// Everything, including the changes that can take the link down. The
		// risk badge sits next to each row and the dialog lists them again
		// before anything runs, so hiding them behind a second button only
		// made them harder to find.
		for id := range m.app.Applicable {
			m.app.Selected[id] = true
		}
		m.reloadAll()
		return nil, true
	case clicked(msg, zoneClearSel):
		m.app.Selected = map[string]bool{}
		m.reloadAll()
		return nil, true
	}
	return nil, false
}

// --- status rows --------------------------------------------------------------------

// viewInsight is the row that changes: what is running right now, or what the
// last run concluded. Reserving it in both states is what keeps the frame from
// jumping by a row every time a measurement starts.
func (m *model) viewInsight() string {
	if m.app.Busy() {
		return " " + m.viewRunning()
	}
	result := m.app.LastRun()
	if result == nil {
		return " " + sFaint.Render(i18n.T("ui.press_run"))
	}

	parts := []string{
		sFaint.Render(i18n.T("misc.last_run")) + " " +
			sText.Render(result.Name) + sFaint.Render(" · "+
			result.StartedAt.Format("15:04")),
	}
	if bad := result.CountFindings("bad"); bad > 0 {
		parts = append(parts, sBad.Render(fmt.Sprintf("● %d", bad)))
	}
	if warn := result.CountFindings("warn"); warn > 0 {
		parts = append(parts, sWarn.Render(fmt.Sprintf("▲ %d", warn)))
	}
	if bad, warn := result.CountFindings("bad"), result.CountFindings("warn"); bad+warn == 0 {
		parts = append(parts, sOK.Render("✓ "+i18n.T("fnd.clean.title")))
	}
	if len(result.Advice) > 0 {
		parts = append(parts, sAcc.Render("→ ")+
			sText.Render(truncate(result.Advice[0].Title, max(m.width/2, 20))))
	}
	return " " + truncate(strings.Join(parts, sLine.Render("  ·  ")), m.width-2)
}

func (m *model) viewRunning() string {
	label := m.app.Running
	if m.app.Phase != "" {
		label += sFaint.Render(" · ") + sMuted.Render(m.app.Phase)
	}
	head := m.spin.View() + " " + sAcc.Bold(true).Render(label)
	bar := m.prog.ViewAs(m.app.Progress)
	tail := sBold.Render(fmt.Sprintf("%3d%%", int(m.app.Progress*100))) +
		sFaint.Render("  "+util.ShortDuration(m.app.Elapsed()))

	gap := m.width - visWidth(head) - visWidth(bar) - visWidth(tail) - 4
	if gap < 2 {
		return truncate(head+"  "+tail, m.width-2)
	}
	return head + strings.Repeat(" ", gap) + bar + "  " + tail
}

// viewStatus is the key legend on the left and the standing verdict on the
// right: the two things you want without having to look away from the page.
func (m *model) viewStatus() string {
	left := m.help.ShortHelpView(m.keys.ShortHelp())
	right := m.viewVerdict()
	gap := m.width - visWidth(left) - visWidth(right) - 2
	if gap < 1 {
		left = truncate(left, max(m.width-visWidth(right)-3, 0))
		gap = max(m.width-visWidth(left)-visWidth(right)-2, 1)
	}
	return " " + left + strings.Repeat(" ", gap) + right + " "
}

func (m *model) viewVerdict() string {
	result := m.app.LastRun()
	if result == nil {
		return sFaint.Render(i18n.T("ui.idle"))
	}
	score := scoreStyle(result.Score).Bold(true).
		Render(fmt.Sprintf("%.1f", result.Score))
	grade := pill(result.Grade, colInvert, gradeColour(result.Score))
	out := sFaint.Render(i18n.T("f.score")+" ") + score + " " + grade
	if result.Baseline != nil {
		delta := result.Score - result.Baseline.Score
		style, arrow := sOK, "▲"
		if delta < 0 {
			style, arrow = sBad, "▼"
		}
		out += "  " + style.Render(fmt.Sprintf("%s%+.1f", arrow, delta))
	}
	return out
}

func gradeColour(score float64) lipgloss.TerminalColor {
	switch {
	case score >= 80:
		return colOK
	case score >= 60:
		return colWarn
	}
	return colBad
}

// --- help sheet ------------------------------------------------------------------

func (m *model) viewHelpSheet() string {
	width := clamp(m.width-16, 52, 88)
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.help.FullHelpView(m.keys.FullHelp()),
		"",
		sLine.Render(strings.Repeat("╌", width-4)),
		sMuted.Render(wrapText(i18n.T("ui.help_hint"), width-4)),
		"",
		sFaint.Render(i18n.T("ui.any_key")))
	return card{title: i18n.T("nav.help"), width: width, accent: colAccent}.render(body)
}
