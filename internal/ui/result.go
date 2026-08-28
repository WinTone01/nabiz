package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/util"
)

// One result, one layout.
//
// The test page and the reports browser render the same thing through the same
// code, so a saved run can never look different from the run that produced it -
// which matters, because the whole point of saving one is to compare them.

// scoreHeader is the headline: the number, the grade, how far along the bar it
// sits, and what it is being compared against.
func scoreHeader(result suite.Result, width int) string {
	style := scoreStyle(result.Score)
	headline := style.Bold(true).Render(fmt.Sprintf("%5.1f", result.Score)) + " " +
		pill(result.Grade, colInvert, gradeColour(result.Score))

	barWidth := clamp(width-visWidth(headline)-4, 10, 40)
	headline += "  " + meter(result.Score/100, barWidth, style)

	facts := []string{
		sText.Render(result.Name),
		sFaint.Render(fmt.Sprintf("%.0f s", result.Duration)),
		sFaint.Render(result.StartedAt.Format("2006-01-02 15:04:05")),
	}
	if result.Cancelled {
		facts = append(facts, sWarn.Render(i18n.T("ui.cancelled")))
	}
	if result.Baseline != nil {
		delta := result.Score - result.Baseline.Score
		deltaStyle, arrow := sOK, "▲"
		if delta < 0 {
			deltaStyle, arrow = sBad, "▼"
		}
		facts = append(facts, sFaint.Render(i18n.T("misc.vs_baseline"))+" "+
			deltaStyle.Render(fmt.Sprintf("%s%+.1f", arrow, delta)))
	}
	return headline + "\n" + strings.Join(facts, sLine.Render(" · "))
}

// scoreLine is the one-line form, for lists and footers.
func scoreLine(result suite.Result) string {
	line := scoreStyle(result.Score).Bold(true).
		Render(i18n.T("misc.score_line", result.Score, result.Grade)) +
		sFaint.Render(fmt.Sprintf("   %s · %.0f s · %s", result.Name, result.Duration,
			result.StartedAt.Format("15:04:05")))
	if result.Baseline != nil {
		delta := result.Score - result.Baseline.Score
		style := sOK
		if delta < 0 {
			style = sBad
		}
		line += sMuted.Render("   "+i18n.T("misc.vs_baseline")+" ") +
			style.Render(fmt.Sprintf("%+.1f", delta))
	}
	return line
}

// renderResult lays a whole run out as a sequence of cards.
func renderResult(result suite.Result, width int) string {
	sections := []section{{title: i18n.T("sec.score"), body: scoreHeader(result, width-4)}}

	if len(result.Latency) > 0 {
		sections = append(sections, section{
			title: i18n.T("sec.latency"),
			body:  latencyTable(result.Latency, width-4),
		})
	}
	if result.Load != nil {
		sections = append(sections, section{
			title: i18n.T("sec.load"),
			badge: bloatBadge(result.Load.Grade),
			body:  loadBlock(*result.Load, width-4),
		})
	}
	if len(result.DNSBench) > 0 {
		sections = append(sections, section{
			title: i18n.T("sec.dns"), body: dnsTable(result.DNSBench, width-4)})
	}
	if len(result.DNSChecks) > 0 {
		sections = append(sections, section{
			title: i18n.T("sec.dnschecks"), body: checksTable(result.DNSChecks, width-4)})
	}
	if len(result.DPI) > 0 {
		sections = append(sections, section{
			title: i18n.T("sec.dpi"), body: dpiTable(result.DPI, width-4)})
	}
	if len(result.Hops) > 0 {
		body := hopsTable(result.Hops, width-4)
		if result.MTU != nil {
			style := sText
			if result.MTU.Blackhole {
				style = sBad
			}
			body += "\n\n" + style.Render(i18n.T("misc.mtu_line",
				result.MTU.IfaceMTU, result.MTU.ProbedMTU, result.MTU.Detail))
		}
		sections = append(sections, section{title: i18n.T("sec.path"), body: body})
	}
	sections = append(sections, section{
		title: i18n.T("sec.findings"),
		badge: findingsBadge(result),
		body:  findingsBlock(result.Findings, width-4),
	})
	return stack(width, sections...)
}

func findingsBadge(result suite.Result) string {
	var parts []string
	if bad := result.CountFindings("bad"); bad > 0 {
		parts = append(parts, sBad.Render(fmt.Sprintf("● %d", bad)))
	}
	if warn := result.CountFindings("warn"); warn > 0 {
		parts = append(parts, sWarn.Render(fmt.Sprintf("▲ %d", warn)))
	}
	if len(parts) == 0 {
		return sOK.Render("✓")
	}
	return strings.Join(parts, " ")
}

func bloatBadge(grade string) string {
	style := sOK
	if grade != "A+" && grade != "A" {
		style = sWarn
	}
	return style.Render(grade)
}

// --- tables ------------------------------------------------------------------

func latencyTable(rows []stats.Summary, width int) string {
	cols := []col{
		{title: i18n.T("col.target"), width: 20},
		{title: i18n.T("col.loss"), width: 7, right: true},
		{title: i18n.T("col.avg"), width: 7, right: true},
		{title: i18n.T("col.p95"), width: 7, right: true},
		{title: i18n.T("col.jitter"), width: 7, right: true},
		{title: i18n.T("col.mos"), width: 5, right: true},
	}
	// Saved runs do not carry their raw samples, so the chart column is only
	// added when there is something to draw in it; an empty flexible column
	// would otherwise push the whole table apart for nothing.
	chart := 0
	for _, summary := range rows {
		if len(summary.Samples) > 0 {
			chart = max(width-64, 8)
			cols = append(cols, col{title: ""})
			break
		}
	}
	var out [][]cell
	for _, summary := range rows {
		style := sText
		switch {
		case summary.LossPct >= 2:
			style = sBad
		case summary.LossPct >= 0.5 || summary.Jitter >= 8:
			style = sWarn
		}
		row := []cell{
			plain(summary.Label),
			styled(fmt.Sprintf("%.1f%%", summary.LossPct), style),
			numf("%.1f", summary.Avg),
			numf("%.1f", summary.P95),
			numf("%.1f", summary.Jitter),
			numf("%.2f", summary.MOS),
		}
		if chart > 0 {
			row = append(row, rawCell(sparkline(summary.Samples, chart)))
		}
		out = append(out, row)
	}
	return renderTable(width, cols, out)
}

func loadBlock(load probe.BloatResult, width int) string {
	lines := []string{
		kv(i18n.T("load.idle"), fmt.Sprintf("%.1f ms", load.Idle.P50), sText, 12),
	}
	if load.Download != nil {
		lines = append(lines, kv(i18n.T("load.download"),
			fmt.Sprintf("%-12s  p95 %6.1f ms  %s", util.HumanRate(load.Download.Bps),
				load.DownLatency.P95,
				deltaStyle(load.DownDelta).Render(fmt.Sprintf("+%.0f ms", load.DownDelta))),
			sText, 12))
	}
	if load.Upload != nil {
		lines = append(lines, kv(i18n.T("load.upload"),
			fmt.Sprintf("%-12s  p95 %6.1f ms  %s", util.HumanRate(load.Upload.Bps),
				load.UpLatency.P95,
				deltaStyle(load.UpDelta).Render(fmt.Sprintf("+%.0f ms", load.UpDelta))),
			sText, 12))
	}
	gradeStyle := sOK
	if load.Grade != "A+" && load.Grade != "A" {
		gradeStyle = sWarn
	}
	lines = append(lines, kv(i18n.T("load.bloat"), load.Grade, gradeStyle, 12))
	if load.Download != nil && load.Download.Sockets.Count > 0 {
		sockets := load.Download.Sockets
		lines = append(lines, "", sFaint.Render(wrapText(i18n.T("load.kernel",
			strings.Join(sockets.CCAlgorithms, ","), sockets.AvgCWnd,
			sockets.RetransPct, sockets.MinRTTms, sockets.Count), width)))
	}
	return strings.Join(lines, "\n")
}

func dnsTable(rows []suite.DNSRow, width int) string {
	cols := []col{
		{title: i18n.T("col.resolver"), width: 0},
		{title: i18n.T("col.kind"), width: 7},
		{title: i18n.T("col.avgms"), width: 9, right: true},
		{title: i18n.T("col.p95"), width: 9, right: true},
		{title: i18n.T("col.errors"), width: 8, right: true},
		{title: "", width: 14},
	}
	fastest := 0.0
	for _, row := range rows {
		if row.AvgMs > fastest {
			fastest = row.AvgMs
		}
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
		share := 0.0
		if fastest > 0 {
			share = row.AvgMs / fastest
		}
		out = append(out, []cell{
			plain(row.Label), dim(row.Kind),
			styled(fmt.Sprintf("%.1f", row.AvgMs), style),
			numf("%.1f", row.P95Ms),
			numf("%d/%d", row.Failures, row.Queries),
			rawCell(meter(share, 13, style)),
		})
	}
	return renderTable(width, cols, out)
}

func checksTable(checks []probe.Check, width int) string {
	cols := []col{
		{title: i18n.T("col.metric"), width: 22},
		{title: i18n.T("col.verdict"), width: 9},
		{title: "", width: 0},
	}
	var out [][]cell
	for _, check := range checks {
		out = append(out, []cell{
			plain(check.Name),
			styled(levelMark(check.Verdict)+" "+check.Verdict, levelStyle(check.Verdict)),
			dim(check.Detail),
		})
	}
	return renderTable(width, cols, out)
}

func dpiTable(verdicts []probe.DomainVerdict, width int) string {
	cols := []col{
		{title: i18n.T("col.domain"), width: 26},
		{title: i18n.T("col.verdict"), width: 17},
		{title: i18n.T("col.tls"), width: 9},
		{title: i18n.T("col.split"), width: 13},
		{title: i18n.T("col.quic"), width: 0},
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
		split := "—"
		if verdict.SplitHdr.Kind != "" || verdict.SplitSNI.Kind != "" {
			split = orDash(verdict.SplitHdr.Kind) + "/" + orDash(verdict.SplitSNI.Kind)
		}
		out = append(out, []cell{
			plain(verdict.Domain),
			styled(verdict.Verdict, style),
			plain(orDash(verdict.Whole.Kind)),
			plain(split),
			dim(verdict.QUIC),
		})
	}
	return renderTable(width, cols, out)
}

func hopsTable(hops []probe.Hop, width int) string {
	cols := []col{
		{title: "#", width: 3, right: true},
		{title: i18n.T("col.ip"), width: 20},
		{title: i18n.T("col.loss"), width: 7, right: true},
		{title: i18n.T("col.best"), width: 8, right: true},
		{title: i18n.T("col.avg"), width: 8, right: true},
		{title: i18n.T("col.worst"), width: 8, right: true},
		{title: "", width: 0},
	}
	worst := 1.0
	for _, hop := range hops {
		if hop.Avg() > worst {
			worst = hop.Avg()
		}
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
			rawCell(meter(hop.Avg()/worst, max(width-70, 6), sInfo)),
		})
	}
	return renderTable(width, cols, out)
}

// --- findings and advice --------------------------------------------------------

// findingsBlock lists what the run concluded, worst first. The level glyph is
// coloured and shaped, so severity survives both a monochrome terminal and a
// reader who cannot tell the red from the green.
func findingsBlock(findings []suite.Finding, width int) string {
	var b strings.Builder
	for _, finding := range findings {
		b.WriteString(fmt.Sprintf("%s %s  %s\n",
			levelStyle(finding.Level).Render(levelMark(finding.Level)),
			sBold.Render(fit(finding.Key, 22)),
			sText.Render(truncate(finding.Title, max(width-27, 20)))))
		if finding.Hint != "" {
			b.WriteString("   " + sFaint.Render(wrapIndent(finding.Hint, width-6, "   ")) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// adviceBody is the detail under an advice title, without repeating the title.
func adviceBody(advice suite.Advice, width int) string {
	var b strings.Builder
	b.WriteString(sText.Render(wrapText(advice.Why, width)) + "\n")
	for _, step := range advice.How {
		if suite.IsCommand(step) {
			b.WriteString("  " + sFaint.Render("$ ") + sAcc.Render(truncate(step, width-4)) + "\n")
			continue
		}
		b.WriteString("  " + sFaint.Render("• ") +
			sText.Render(wrapIndent(step, width-4, "    ")) + "\n")
	}
	if advice.Gain != "" {
		b.WriteString(sOK.Render("→ ") + sText.Render(wrapIndent(advice.Gain, width-2, "  ")) + "\n")
	}
	if advice.Risk != "" {
		b.WriteString(sWarn.Render("! ") + sText.Render(wrapIndent(advice.Risk, width-2, "  ")) + "\n")
	}
	if len(advice.Revert) > 0 {
		b.WriteString(sFaint.Render(wrapIndent(i18n.T("misc.revert")+": "+
			strings.Join(advice.Revert, " ; "), width, "  ")) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- A/B ---------------------------------------------------------------------------

// abRows renders the comparison body; the caller wraps it in a card.
func abRows(a *App, labelA, labelB string, width int) string {
	if a.AB == nil {
		return ""
	}
	cols := []col{
		{title: i18n.T("col.metric"), width: 0},
		{title: labelA, width: 16, right: true},
		{title: labelB, width: 16, right: true},
		{title: i18n.T("col.delta"), width: 12, right: true},
	}
	var rows [][]cell
	for _, row := range report.DiffRows(a.AB.before, a.AB.after) {
		style := sFaint
		if row.Significant() {
			style = sBad
			if row.Better() {
				style = sOK
			}
		}
		rows = append(rows, []cell{
			plain(row.Metric),
			numf("%.2f %s", row.Before, row.Unit),
			numf("%.2f %s", row.After, row.Unit),
			styled(fmt.Sprintf("%+.2f", row.Delta), style),
		})
	}
	return renderTable(width, cols, rows)
}

// --- shared readings ------------------------------------------------------------------

// errorCounters is sorted because it is read from a map and rendered on a
// timer: unsorted, the same unchanged counters reorder themselves on every
// refresh, and a line that rewrites itself twice a second reads as a fault in
// the interface rather than as a stable reading.
func errorCounters(link probe.LinkInfo) []string {
	var out []string
	for key, value := range link.Stats {
		if probe.ErrorKeys[key] && value > 0 {
			out = append(out, fmt.Sprintf("%s=%d", key, value))
		}
	}
	sort.Strings(out)
	return out
}

func nfqText(a *App) string {
	nfq := a.Env.nfqueue
	switch {
	case !nfq.Available:
		return i18n.T("ui.needs_root")
	case len(nfq.Queues) == 0:
		return i18n.T("ui.none")
	}
	var drops int64
	for _, queue := range nfq.Queues {
		drops += queue.QueueDropped + queue.UserDropped
	}
	return fmt.Sprint(drops)
}

func nfqStyle(a *App) lipgloss.Style {
	nfq := a.Env.nfqueue
	if !nfq.Available {
		return sFaint
	}
	for _, queue := range nfq.Queues {
		if queue.QueueDropped+queue.UserDropped > 0 {
			return sBad
		}
	}
	return sOK
}

// liveRTT is the best non-gateway round trip the watcher currently sees. The
// gateway says nothing about the WAN, which is the whole reason it is excluded.
func liveRTT(a *App) float64 {
	if a.Watcher == nil {
		return 0
	}
	for _, target := range a.Watcher.Snapshot().Targets {
		if target.Label != "Modem / Gateway" && target.Avg > 0 {
			return target.Avg
		}
	}
	return 0
}

func eeeText(status probe.EEEStatus) string {
	switch {
	case !status.Supported:
		return i18n.T("ui.unknown")
	case status.Active:
		return i18n.T("ui.enabled")
	}
	return i18n.T("ui.disabled")
}
