package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/monitor"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/util"
)

// The overview answers three questions in the order people actually ask them:
// is something wrong right now, what is the link doing, and what did the last
// run conclude. The four tiles across the top answer the first one before a
// single table has been read; everything else is one keystroke away.
type overviewPage struct {
	body          scroller
	width, height int
}

func newOverviewPage() Page { return &overviewPage{body: newScroller()} }

func (p *overviewPage) ID() pageID            { return pageOverview }
func (p *overviewPage) SuiteName(*App) string { return "quick" }

func (p *overviewPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *overviewPage) Reload(a *App) { p.body.setContent(p.render(a)) }

func (p *overviewPage) Focus(focused bool) { p.body.focused = focused }

func (p *overviewPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }

func (p *overviewPage) View(a *App) string {
	p.body.setContent(p.render(a))
	return p.body.view()
}

func (p *overviewPage) render(a *App) string {
	width := p.body.contentWidth()
	blocks := []string{p.tiles(a, width)}

	// Below this the two columns are narrower than the tables inside them, and
	// a table that wraps is worse than a page that scrolls.
	if width < 96 {
		blocks = append(blocks,
			panel(i18n.T("panel.latency"), width, p.latency(a, width-4)),
			panel(i18n.T("panel.verdict"), width, p.verdict(a, width-4)),
			panel(i18n.T("panel.link"), width, p.link(a)),
			panel(i18n.T("panel.kernel"), width, p.kernel(a)),
			panel(i18n.T("panel.tools"), width, p.tools(a, width-4)))
		return strings.Join(blocks, "\n")
	}

	leftWidth := clamp(width/3, 34, 44)
	rightWidth := width - leftWidth - 1

	left := joinCol(
		panel(i18n.T("panel.link"), leftWidth, p.link(a)),
		panel(i18n.T("panel.kernel"), leftWidth, p.kernel(a)),
		panel(i18n.T("panel.tools"), leftWidth, p.tools(a, leftWidth-4)))

	right := joinCol(
		panel(i18n.T("panel.latency"), rightWidth, p.latency(a, rightWidth-4)),
		panel(i18n.T("panel.verdict"), rightWidth, p.verdict(a, rightWidth-4)))

	// The event log takes whatever height the other column left over, so the
	// two sides end level instead of one trailing off into blank space.
	if spare := blockHeight(left) - blockHeight(right) - 2; spare >= 3 {
		right = joinCol(right,
			panel(i18n.T("panel.events"), rightWidth, p.events(a, rightWidth-4, spare)))
	}
	return strings.Join(append(blocks, joinRow(1, left, right)), "\n")
}

// --- tiles -------------------------------------------------------------------

func (p *overviewPage) tiles(a *App, width int) string {
	columns := 4
	if width < 76 {
		columns = 2
	}
	cellWidth := columnWidth(width, columns, 1)

	availValue, availNote, availStyle := i18n.T("ui.nodata"), "", sFaint
	latencyValue, latencyNote, latencyStyle := "—", "", sFaint
	if a.Watcher != nil {
		snapshot := a.Watcher.Snapshot()
		availStyle = sOK
		if snapshot.Availability <= 99.9 {
			availStyle = sWarn
		}
		if snapshot.OutageActive {
			availStyle = sBad
		}
		availValue = fmt.Sprintf("%.3f %%", snapshot.Availability)
		availNote = fmt.Sprintf("%s · %s %d", util.ShortDuration(snapshot.Uptime),
			i18n.T("f.outages"), len(snapshot.Outages))
		latencyValue, latencyNote, latencyStyle = internetTile(snapshot)
	}

	link := a.Env.link
	linkStyle := sOK
	if link.SpeedMbit > 0 && link.SpeedMbit <= 100 && !link.Wireless {
		linkStyle = sWarn
	}
	flaps := 0
	if a.Env.linkLog.Available {
		flaps = a.Env.linkLog.Drops
	}
	if flaps > 0 {
		linkStyle = sBad
	}
	linkNote := fmt.Sprintf("%s · %d %s", orDash(link.Iface), flaps, i18n.T("tile.drops"))

	scoreValue, scoreNote, style := "—", i18n.T("ui.press_run"), sFaint
	if result := a.LastRun(); result != nil {
		scoreValue = fmt.Sprintf("%.1f  %s", result.Score, result.Grade)
		scoreNote = result.Name + " · " + result.StartedAt.Format("15:04")
		style = scoreStyle(result.Score)
	}

	return grid(width, columns, 1,
		tile(i18n.T("f.avail"), availValue, availStyle, availNote, cellWidth),
		tile(i18n.T("tile.latency"), latencyValue, latencyStyle, latencyNote, cellWidth),
		tile(i18n.T("tile.link"), fmt.Sprintf("%d Mbit", link.SpeedMbit), linkStyle,
			linkNote, cellWidth),
		tile(i18n.T("f.score"), scoreValue, style, scoreNote, cellWidth))
}

// internetTile picks the best internet anchor rather than the gateway: the
// modem answering in 1 ms says nothing about whether the internet is reachable.
func internetTile(snapshot monitor.Snapshot) (string, string, lipgloss.Style) {
	for _, target := range snapshot.Targets {
		if target.Label == "Modem / Gateway" || target.Avg <= 0 {
			continue
		}
		style := sOK
		switch {
		case target.LossPct >= 2 || target.Avg >= 150:
			style = sBad
		case target.LossPct >= 0.5 || target.Avg >= 80:
			style = sWarn
		}
		note := fmt.Sprintf("p95 %.0f · %s %.1f%%", target.P95, i18n.T("col.loss"),
			target.LossPct)
		return fmt.Sprintf("%.0f ms", target.Avg), note, style
	}
	return "—", "", sFaint
}

// --- panels --------------------------------------------------------------------

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
	var list kvList
	list.add(i18n.T("f.iface"), link.Iface+"  "+link.Address)
	list.add(i18n.T("f.gateway"), orDash(link.Gateway))
	list.addStyled(i18n.T("f.speed"), fmt.Sprintf("%d Mbit · %d · %s",
		link.SpeedMbit, link.MTU, link.Duplex), speedStyle)
	list.add(i18n.T("f.qdisc"), orDash(link.Qdisc))
	list.addStyled(i18n.T("f.flaps"), dropText, dropStyle)
	if link.Wireless {
		style := sText
		if link.SignalDBm < -70 {
			style = sWarn
		}
		list.addStyled(i18n.T("f.wifi"),
			fmt.Sprintf("%s %.0f dBm", link.SSID, link.SignalDBm), style)
	}
	style, text := sOK, i18n.T("ui.clean")
	if errs := errorCounters(link); len(errs) > 0 {
		style, text = sWarn, strings.Join(errs, " ")
	}
	list.addStyled(i18n.T("f.counters"), text, style)
	return list.render(16)
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
	var list kvList
	list.addStyled(i18n.T("f.retransmit"), fmt.Sprintf("%.2f%%  (%d/%d)",
		health.RetransPct, health.RetransSegs, health.OutSegs), retransStyle)
	list.add(i18n.T("f.timeouts"), fmt.Sprint(health.Timeouts))
	list.add(i18n.T("f.ooo"), fmt.Sprint(health.OFOQueue))
	list.add(i18n.T("f.sockets"), fmt.Sprintf("%d · cwnd %.0f",
		a.Env.sockets.Count, a.Env.sockets.AvgCWnd))
	list.add(i18n.T("f.cc"), cc)
	list.add(i18n.T("f.conntrack"), fmt.Sprintf("%d / %d",
		a.Env.conntrack.Count, a.Env.conntrack.Max))
	return list.render(16)
}

func (p *overviewPage) tools(a *App, width int) string {
	var list kvList
	if unwall := a.Env.unwall; unwall.Installed {
		list.addHead(sBold.Render("Unwall") + "  " + runningTag(unwall.Running))
		list.add(i18n.T("f.engine"), unwall.Engine+" · "+unwall.Strategy)
		list.add(i18n.T("f.hostlist"), fmt.Sprintf("%s (%d/%d)", unwall.HostlistMode,
			unwall.HostlistN, unwall.AutoHostlistN))
		dns := sWarn.Render(i18n.T("ui.off"))
		if unwall.DNSEncrypted {
			dns = sOK.Render(unwall.DNSBackend + " / " + unwall.DNSProvider)
		}
		list.addRaw(i18n.T("f.dns"), dns)
		list.addStyled(i18n.T("f.nfqueue"), nfqText(a), nfqStyle(a))
	} else {
		list.addHead(sBold.Render("Unwall") + "  " + sFaint.Render(i18n.T("ui.notinstalled")))
	}
	list.addRule(width)
	if bpftune := a.Env.bpftune; bpftune.Installed {
		list.addHead(sBold.Render("bpftune") + "  " + runningTag(bpftune.Running))
		list.add(i18n.T("f.changes"), fmt.Sprint(len(bpftune.Changes)))
		if rmem := a.Env.sysctls["net.ipv4.tcp_rmem"]; rmem != "" {
			list.add("tcp_rmem", util.HumanBytes(float64(suite.SysctlInt(rmem, 2))))
		}
	} else {
		list.addHead(sBold.Render("bpftune") + "  " + sFaint.Render(i18n.T("ui.notinstalled")))
	}
	return list.render(16)
}

func (p *overviewPage) latency(a *App, width int) string {
	if a.Watcher == nil {
		return emptyState("ui.nodata")
	}
	snapshot := a.Watcher.Snapshot()
	cols := []col{
		{title: i18n.T("col.target"), width: 17},
		{title: i18n.T("col.last"), width: 6, right: true},
		{title: i18n.T("col.avg"), width: 6, right: true},
		{title: i18n.T("col.p95"), width: 6, right: true},
		{title: i18n.T("col.loss"), width: 7, right: true},
		{title: "", width: 0},
	}
	chart := max(width-48, 8)
	var rows [][]cell
	for _, target := range snapshot.Targets {
		last, lastStyle := "—", sBad
		if target.Last != nil {
			last, lastStyle = fmt.Sprintf("%.0f", *target.Last), sText
		}
		rows = append(rows, []cell{
			plain(target.Label),
			styled(last, lastStyle),
			numf("%.0f", target.Avg),
			numf("%.0f", target.P95),
			styled(fmt.Sprintf("%.1f%%", target.LossPct), lossStyle(target.LossPct)),
			rawCell(sparkline(target.Recent, chart)),
		})
	}
	outageStyle := sOK
	if len(snapshot.Outages) > 0 {
		outageStyle = sBad
	}
	summary := strings.Join([]string{
		sMuted.Render(i18n.T("f.uptime")+" ") + sText.Render(util.ShortDuration(snapshot.Uptime)),
		sMuted.Render(i18n.T("f.outages")+" ") + outageStyle.Render(fmt.Sprint(len(snapshot.Outages))),
		sMuted.Render(i18n.T("f.avail")+" ") + sText.Render(fmt.Sprintf("%.3f%%", snapshot.Availability)),
		sMuted.Render(i18n.T("f.dnsfail")+" ") + sText.Render(fmt.Sprint(snapshot.DNSFailures)),
	}, sLine.Render("  ·  "))
	return renderTable(width, cols, rows) + "\n\n" + summary
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
		lines = append(lines, fmt.Sprintf("%s %s",
			levelStyle(finding.Level).Render(levelMark(finding.Level)),
			sText.Render(truncate(finding.Title, width-2))))
	}
	if shown == 0 {
		lines = append(lines, sOK.Render("✓ ")+sText.Render(i18n.T("fnd.clean.title")))
	}
	if len(result.Advice) > 0 {
		lines = append(lines, "", sAcc.Render("→ ")+
			sBold.Render(truncate(result.Advice[0].Title, width-2)))
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
			levelStyle(event.Severity).Render(fit(event.Kind, 14)),
			sText.Render(truncate(strings.TrimSpace(event.Target+" "+event.Detail),
				max(width-26, 10)))))
	}
	return strings.Join(lines, "\n")
}
