package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/util"
)

// The test page is a suite picker plus the result of that suite. Results are
// kept per suite rather than "the last one", so switching between quick and
// deep does not lose the other's numbers.
type testPage struct {
	viewport      viewport.Model
	width, height int
	keys          keyMap
}

func newTestPage() Page { return &testPage{keys: defaultKeys()} }

func (p *testPage) ID() tabID { return tabTest }

func (p *testPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width = width
	p.viewport.Height = max(height-4, 3)
}

func (p *testPage) Reload(a *App) {
	p.keys = defaultKeys()
	p.viewport.SetContent(p.body(a))
}

func suiteZone(name string) string { return "suite:" + name }

func (p *testPage) Update(a *App, msg tea.Msg) tea.Cmd {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && isPress(mouseMsg) {
		for index, name := range suite.Names {
			if clicked(mouseMsg, suiteZone(name)) {
				a.SuitePick = index
				p.Reload(a)
				return nil
			}
		}
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, p.keys.PrevSel):
			a.SuitePick = (a.SuitePick - 1 + len(suite.Names)) % len(suite.Names)
			p.Reload(a)
			return nil
		case key.Matches(keyMsg, p.keys.NextSel):
			a.SuitePick = (a.SuitePick + 1) % len(suite.Names)
			p.Reload(a)
			return nil
		}
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *testPage) SuiteName(a *App) string { return suite.Names[a.SuitePick] }

func (p *testPage) View(a *App) string {
	var chips []string
	for index, name := range suite.Names {
		chips = append(chips, chip(suiteZone(name), name, index == a.SuitePick))
	}
	description, _ := suite.Describe(suite.Names[a.SuitePick])
	if i18n.Current() != i18n.TR {
		_, description = suite.Describe(suite.Names[a.SuitePick])
	}
	header := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(chips, " "),
		sDim.Render(wrap(description, p.width-2)))
	p.viewport.SetContent(p.body(a))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", p.viewport.View())
}

func (p *testPage) body(a *App) string {
	result, ok := a.Results[suite.Names[a.SuitePick]]
	if !ok {
		return emptyState("ui.press_run")
	}
	return renderResult(result, p.width)
}

// renderResult is shared with the reports page: one result, one layout.
func renderResult(result suite.Result, width int) string {
	var b strings.Builder
	b.WriteString(scoreLine(result) + "\n\n")

	if len(result.Latency) > 0 {
		chart := max(width-64, 8)
		cols := []column{
			{title: i18n.T("col.target"), width: 20},
			{title: i18n.T("col.loss"), width: 7, right: true},
			{title: i18n.T("col.avg"), width: 7, right: true},
			{title: i18n.T("col.p95"), width: 7, right: true},
			{title: i18n.T("col.jitter"), width: 7, right: true},
			{title: i18n.T("col.mos"), width: 5, right: true},
			{title: "", width: chart},
		}
		var rows [][]cell
		for _, summary := range result.Latency {
			style := sText
			switch {
			case summary.LossPct >= 2:
				style = sBad
			case summary.LossPct >= 0.5 || summary.Jitter >= 8:
				style = sWarn
			}
			rows = append(rows, []cell{
				plain(summary.Label),
				styled(fmt.Sprintf("%.1f%%", summary.LossPct), style),
				numf("%.1f", summary.Avg),
				numf("%.1f", summary.P95),
				numf("%.1f", summary.Jitter),
				numf("%.2f", summary.MOS),
				rendered(sparkline(summary.Samples, chart)),
			})
		}
		b.WriteString(sSection.Render(i18n.T("sec.latency")) + "\n")
		b.WriteString(renderTable(cols, rows) + "\n")
	}

	if load := result.Load; load != nil {
		b.WriteString(sSection.Render(i18n.T("sec.load")) + "\n")
		b.WriteString("  " + kv(i18n.T("load.idle"),
			fmt.Sprintf("%.1f ms", load.Idle.P50), sText, 12) + "\n")
		if load.Download != nil {
			b.WriteString("  " + kv(i18n.T("load.download"),
				fmt.Sprintf("%-12s p95 %6.1f ms  %s", util.HumanRate(load.Download.Bps),
					load.DownLatency.P95,
					deltaStyle(load.DownDelta).Render(fmt.Sprintf("+%.0f ms", load.DownDelta))),
				sText, 12) + "\n")
		}
		if load.Upload != nil {
			b.WriteString("  " + kv(i18n.T("load.upload"),
				fmt.Sprintf("%-12s p95 %6.1f ms  %s", util.HumanRate(load.Upload.Bps),
					load.UpLatency.P95,
					deltaStyle(load.UpDelta).Render(fmt.Sprintf("+%.0f ms", load.UpDelta))),
				sText, 12) + "\n")
		}
		gradeStyle := sOK
		if load.Grade != "A+" && load.Grade != "A" {
			gradeStyle = sWarn
		}
		b.WriteString("  " + kv(i18n.T("load.bloat"), load.Grade, gradeStyle, 12) + "\n")
		if load.Download != nil && load.Download.Sockets.Count > 0 {
			sockets := load.Download.Sockets
			b.WriteString("  " + sFaint.Render(i18n.T("load.kernel",
				strings.Join(sockets.CCAlgorithms, ","), sockets.AvgCWnd,
				sockets.RetransPct, sockets.MinRTTms, sockets.Count)) + "\n")
		}
		b.WriteString("\n")
	}

	if len(result.DNSBench) > 0 {
		b.WriteString(sSection.Render(i18n.T("sec.dns")) + "\n" + dnsTable(result.DNSBench) + "\n")
	}
	if len(result.DNSChecks) > 0 {
		b.WriteString(sSection.Render(i18n.T("sec.dnschecks")) + "\n" +
			checksTable(result.DNSChecks) + "\n")
	}
	if len(result.DPI) > 0 {
		b.WriteString(sSection.Render(i18n.T("sec.dpi")) + "\n" +
			dpiTable(result.DPI, width) + "\n")
	}
	if len(result.Hops) > 0 {
		b.WriteString(sSection.Render(i18n.T("sec.path")) + "\n" +
			hopsTable(result.Hops) + "\n")
	}
	if result.MTU != nil {
		style := sText
		if result.MTU.Blackhole {
			style = sBad
		}
		b.WriteString(style.Render(i18n.T("misc.mtu_line", result.MTU.IfaceMTU,
			result.MTU.ProbedMTU, result.MTU.Detail)) + "\n\n")
	}

	b.WriteString(sSection.Render(i18n.T("sec.findings")) + "\n")
	b.WriteString(findingsBlock(result.Findings, width))
	return b.String()
}

func dnsTable(rows []suite.DNSRow) string {
	cols := []column{
		{title: i18n.T("col.resolver"), width: 26},
		{title: i18n.T("col.kind"), width: 7},
		{title: i18n.T("col.avgms"), width: 9, right: true},
		{title: i18n.T("col.p95"), width: 9, right: true},
		{title: i18n.T("col.errors"), width: 8, right: true},
	}
	var out [][]cell
	for _, row := range rows {
		style := sText
		switch {
		case row.Failures == row.Queries && row.Queries > 0:
			style = sBad
		case row.AvgMs > 250:
			style = sWarn
		case row.Encrypted:
			style = sInfo
		}
		out = append(out, []cell{
			plain(row.Label), dim(row.Kind),
			styled(fmt.Sprintf("%.1f", row.AvgMs), style),
			numf("%.1f", row.P95Ms),
			numf("%d/%d", row.Failures, row.Queries),
		})
	}
	return renderTable(cols, out)
}

func checksTable(checks []probe.Check) string {
	cols := []column{
		{title: i18n.T("col.metric"), width: 20},
		{title: i18n.T("col.verdict"), width: 7},
		{title: "", width: 70},
	}
	var out [][]cell
	for _, check := range checks {
		out = append(out, []cell{
			plain(check.Name),
			styled(check.Verdict, levelStyle(check.Verdict)),
			plain(check.Detail),
		})
	}
	return renderTable(cols, out)
}

func dpiTable(verdicts []probe.DomainVerdict, width int) string {
	cols := []column{
		{title: i18n.T("col.domain"), width: 28},
		{title: i18n.T("col.verdict"), width: 17},
		{title: i18n.T("col.tls"), width: 9},
		{title: i18n.T("col.split"), width: 13},
		{title: i18n.T("col.quic"), width: max(width-82, 8)},
	}
	var out [][]cell
	for _, verdict := range verdicts {
		style := sInfo
		switch verdict.Verdict {
		case "clean":
			style = sOK
		case "dpi-split-helps":
			style = sWarn
		case "dpi-hard", "ip-block", "unreachable":
			style = sBad
		}
		split := "-"
		if verdict.SplitHdr.Kind != "" || verdict.SplitSNI.Kind != "" {
			split = orDash(verdict.SplitHdr.Kind) + "/" + orDash(verdict.SplitSNI.Kind)
		}
		out = append(out, []cell{
			plain(verdict.Domain), styled(verdict.Verdict, style),
			plain(orDash(verdict.Whole.Kind)), plain(split), dim(verdict.QUIC),
		})
	}
	return renderTable(cols, out)
}

func hopsTable(hops []probe.Hop) string {
	cols := []column{
		{title: i18n.T("col.hop"), width: 3, right: true},
		{title: i18n.T("col.ip"), width: 18},
		{title: i18n.T("col.loss"), width: 7, right: true},
		{title: i18n.T("col.best"), width: 8, right: true},
		{title: i18n.T("col.avg"), width: 8, right: true},
		{title: i18n.T("col.worst"), width: 8, right: true},
	}
	var out [][]cell
	for _, hop := range hops {
		style := sText
		switch {
		case hop.LossPct() >= 20:
			style = sBad
		case hop.LossPct() > 0:
			style = sWarn
		}
		out = append(out, []cell{
			numf("%d", hop.TTL), plain(orStar(hop.IP)),
			styled(fmt.Sprintf("%.0f%%", hop.LossPct()), style),
			numf("%.1f", hop.Best()), numf("%.1f", hop.Avg()), numf("%.1f", hop.Worst()),
		})
	}
	return renderTable(cols, out)
}
