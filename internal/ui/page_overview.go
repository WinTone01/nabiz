package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/util"
)

// The overview answers three questions in the order people ask them: is
// something wrong right now, what is the link doing, and what did the last run
// conclude. Everything else lives one keystroke away.
type overviewPage struct {
	width, height int
}

func newOverviewPage() Page { return &overviewPage{} }

func (p *overviewPage) ID() tabID                    { return tabOverview }
func (p *overviewPage) Reload(*App)                  {}
func (p *overviewPage) Layout(width, height int)     { p.width, p.height = width, height }
func (p *overviewPage) Update(*App, tea.Msg) tea.Cmd { return nil }
func (p *overviewPage) SuiteName(*App) string        { return "quick" }

func (p *overviewPage) View(a *App) string {
	if p.width < 92 {
		return lipgloss.JoinVertical(lipgloss.Left,
			panel(i18n.T("panel.latency"), p.width, p.latency(a, p.width-6)),
			panel(i18n.T("panel.verdict"), p.width, p.verdict(a, p.width-6)))
	}
	leftWidth := min(max(p.width/3, 36), 46)
	rightWidth := p.width - leftWidth - 1

	left := lipgloss.JoinVertical(lipgloss.Left,
		panel(i18n.T("panel.link"), leftWidth, p.link(a)),
		panel(i18n.T("panel.kernel"), leftWidth, p.kernel(a)),
		panel(i18n.T("panel.tools"), leftWidth, p.tools(a)))
	usedRows := lipgloss.Height(left)

	right := lipgloss.JoinVertical(lipgloss.Left,
		panel(i18n.T("panel.latency"), rightWidth, p.latency(a, rightWidth-6)),
		panel(i18n.T("panel.verdict"), rightWidth, p.verdict(a, rightWidth-6)))
	eventRows := usedRows - lipgloss.Height(right) - 2
	if eventRows >= 3 {
		right = lipgloss.JoinVertical(lipgloss.Left, right,
			panel(i18n.T("panel.events"), rightWidth, p.events(a, rightWidth-6, eventRows)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (p *overviewPage) link(a *App) string {
	link := a.Env.link
	speedStyle := sText
	if link.SpeedMbit > 0 && link.SpeedMbit <= 100 && !link.Wireless {
		speedStyle = sWarn
	}
	dropStyle, dropText := sOK, "0"
	if a.Env.linkLog.Available {
		dropText = fmt.Sprint(a.Env.linkLog.Drops)
		if a.Env.linkLog.Drops > 0 {
			dropStyle = sBad
			dropText += fmt.Sprintf("  (%.0f min)", a.Env.linkLog.SpanMinutes())
		}
	}
	lines := []string{
		kv(i18n.T("f.iface"), link.Iface+"  "+link.Address, sText, 11),
		kv(i18n.T("f.gateway"), link.Gateway, sText, 11),
		kv(i18n.T("f.speed"), fmt.Sprintf("%d Mbit · %d · %s",
			link.SpeedMbit, link.MTU, link.Duplex), speedStyle, 11),
		kv(i18n.T("f.qdisc"), orDash(link.Qdisc), sText, 11),
		kv(i18n.T("f.flaps"), dropText, dropStyle, 11),
	}
	if link.Wireless {
		style := sText
		if link.SignalDBm < -70 {
			style = sWarn
		}
		lines = append(lines, kv(i18n.T("f.wifi"),
			fmt.Sprintf("%s %.0f dBm", link.SSID, link.SignalDBm), style, 11))
	}
	style, text := sOK, i18n.T("ui.clean")
	if errs := errorCounters(link); len(errs) > 0 {
		style, text = sWarn, util.Truncate(strings.Join(errs, " "), 24)
	}
	return strings.Join(append(lines, kv(i18n.T("f.counters"), text, style, 11)), "\n")
}

func errorCounters(link probe.LinkInfo) []string {
	var out []string
	for key, value := range link.Stats {
		if probe.ErrorKeys[key] && value > 0 {
			out = append(out, fmt.Sprintf("%s=%d", key, value))
		}
	}
	return out
}

func (p *overviewPage) kernel(a *App) string {
	health := a.Env.health
	retransStyle := sOK
	switch {
	case health.RetransPct >= 3:
		retransStyle = sBad
	case health.RetransPct >= 1:
		retransStyle = sWarn
	}
	cc := strings.Join(a.Env.sockets.CCAlgorithms, ",")
	if cc == "" {
		cc = a.Env.sysctls["net.ipv4.tcp_congestion_control"]
	}
	return strings.Join([]string{
		kv(i18n.T("f.retransmit"), fmt.Sprintf("%.2f%%  (%d/%d)", health.RetransPct,
			health.RetransSegs, health.OutSegs), retransStyle, 11),
		kv(i18n.T("f.timeouts"), fmt.Sprint(health.Timeouts), sText, 11),
		kv(i18n.T("f.ooo"), fmt.Sprint(health.OFOQueue), sText, 11),
		kv(i18n.T("f.sockets"), fmt.Sprintf("%d · cwnd %.0f",
			a.Env.sockets.Count, a.Env.sockets.AvgCWnd), sText, 11),
		kv(i18n.T("f.cc"), util.Truncate(cc, 22), sText, 11),
		kv(i18n.T("f.conntrack"), fmt.Sprintf("%d / %d",
			a.Env.conntrack.Count, a.Env.conntrack.Max), sText, 11),
	}, "\n")
}

func (p *overviewPage) tools(a *App) string {
	var lines []string
	if unwall := a.Env.unwall; unwall.Installed {
		lines = append(lines,
			sBold.Render("Unwall")+"  "+runningTag(unwall.Running),
			kv(i18n.T("f.engine"), unwall.Engine+" · "+unwall.Strategy, sText, 11),
			kv(i18n.T("f.hostlist"), fmt.Sprintf("%s (%d/%d)", unwall.HostlistMode,
				unwall.HostlistN, unwall.AutoHostlistN), sText, 11))
		dns := sWarn.Render(i18n.T("ui.off"))
		if unwall.DNSEncrypted {
			dns = sOK.Render(unwall.DNSBackend + " / " + unwall.DNSProvider)
		}
		lines = append(lines, kv(i18n.T("f.dns"), dns, sText, 11),
			kv(i18n.T("f.nfqueue"), nfqText(a), nfqStyle(a), 11))
	} else {
		lines = append(lines, sBold.Render("Unwall")+"  "+sFaint.Render(i18n.T("ui.notinstalled")))
	}
	lines = append(lines, "")
	if bpftune := a.Env.bpftune; bpftune.Installed {
		lines = append(lines, sBold.Render("bpftune")+"  "+runningTag(bpftune.Running),
			kv(i18n.T("f.changes"), fmt.Sprint(len(bpftune.Changes)), sText, 11))
		if rmem := a.Env.sysctls["net.ipv4.tcp_rmem"]; rmem != "" {
			lines = append(lines, kv("tcp_rmem",
				util.HumanBytes(float64(suite.SysctlInt(rmem, 2))), sText, 11))
		}
	} else {
		lines = append(lines, sBold.Render("bpftune")+"  "+sFaint.Render(i18n.T("ui.notinstalled")))
	}
	return strings.Join(lines, "\n")
}

func runningTag(running bool) string {
	if running {
		return sOK.Render(i18n.T("ui.running"))
	}
	return sWarn.Render(i18n.T("ui.stopped"))
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

func (p *overviewPage) latency(a *App, width int) string {
	if a.Watcher == nil {
		return emptyState("ui.nodata")
	}
	snapshot := a.Watcher.Snapshot()
	chart := max(width-46, 8)
	cols := []column{
		{title: i18n.T("col.target"), width: 17},
		{title: i18n.T("col.last"), width: 6, right: true},
		{title: i18n.T("col.avg"), width: 6, right: true},
		{title: i18n.T("col.p95"), width: 6, right: true},
		{title: i18n.T("col.loss"), width: 7, right: true},
		{title: "", width: chart},
	}
	var rows [][]cell
	for _, target := range snapshot.Targets {
		last, lastStyle := "—", sBad
		if target.Last != nil {
			last, lastStyle = fmt.Sprintf("%.0f", *target.Last), sText
		}
		lossStyle := sOK
		switch {
		case target.LossPct >= 2:
			lossStyle = sBad
		case target.LossPct >= 0.5:
			lossStyle = sWarn
		}
		rows = append(rows, []cell{
			plain(target.Label),
			styled(last, lastStyle),
			numf("%.0f", target.Avg),
			numf("%.0f", target.P95),
			styled(fmt.Sprintf("%.1f%%", target.LossPct), lossStyle),
			rendered(sparkline(target.Recent, chart)),
		})
	}
	outageStyle := sOK
	if len(snapshot.Outages) > 0 {
		outageStyle = sBad
	}
	summary := fmt.Sprintf("%s  %s  %s  %s",
		sDim.Render(i18n.T("f.uptime")+" ")+sText.Render(util.ShortDuration(snapshot.Uptime)),
		sDim.Render(i18n.T("f.outages")+" ")+outageStyle.Render(fmt.Sprint(len(snapshot.Outages))),
		sDim.Render(i18n.T("f.avail")+" ")+sText.Render(fmt.Sprintf("%.3f%%", snapshot.Availability)),
		sDim.Render(i18n.T("f.dnsfail")+" ")+sText.Render(fmt.Sprint(snapshot.DNSFailures)))
	return renderTable(cols, rows) + "\n" + summary
}

func (p *overviewPage) verdict(a *App, width int) string {
	result := a.LastRun()
	if result == nil {
		return emptyState("ui.press_run")
	}
	lines := []string{scoreLine(*result), ""}
	shown := 0
	for _, finding := range result.Findings {
		if finding.Level != "bad" && finding.Level != "warn" {
			continue
		}
		if shown >= 4 {
			break
		}
		shown++
		lines = append(lines, fmt.Sprintf(" %s %s",
			levelStyle(finding.Level).Render(levelMark(finding.Level)),
			sText.Render(util.Truncate(finding.Title, width-3))))
	}
	if shown == 0 {
		lines = append(lines, " "+sOK.Render("✓ ")+sText.Render(i18n.T("fnd.clean.title")))
	}
	if len(result.Advice) > 0 {
		lines = append(lines, "", sAcc.Render("→ ")+
			sBold.Render(util.Truncate(result.Advice[0].Title, width-3)))
	}
	return strings.Join(lines, "\n")
}

func (p *overviewPage) events(a *App, width, height int) string {
	if a.Watcher == nil {
		return emptyState("ui.nodata")
	}
	events := a.Watcher.Snapshot().Events
	if len(events) > height {
		events = events[len(events)-height:]
	}
	if len(events) == 0 {
		return emptyState("misc.no_events")
	}
	var lines []string
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		lines = append(lines, fmt.Sprintf("%s %s %s",
			sFaint.Render(event.Stamp()),
			levelStyle(event.Severity).Render(padRight(event.Kind, 14)),
			sText.Render(util.Truncate(
				strings.TrimSpace(event.Target+" "+event.Detail), max(width-26, 10)))))
	}
	return strings.Join(lines, "\n")
}
