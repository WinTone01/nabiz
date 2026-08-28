package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/util"
)

// Layers walks the stack from the cable upwards. Every line here is read from
// the kernel rather than measured, so the page costs nothing to refresh and can
// be trusted while a run is in flight.
type layersPage struct {
	viewport      viewport.Model
	width, height int
}

func newLayersPage() Page { return &layersPage{} }

func (p *layersPage) ID() tabID { return tabLayers }

func (p *layersPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *layersPage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *layersPage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *layersPage) SuiteName(*App) string { return "deep" }

func (p *layersPage) View(a *App) string { return p.viewport.View() }

func (p *layersPage) body(a *App) string {
	var b strings.Builder
	link := a.Env.link

	b.WriteString(rule("1 · "+i18n.T("sec.physical"), p.width) + "\n")
	for _, row := range [][2]string{
		{i18n.T("f.iface"), fmt.Sprintf("%s (%s) %s", link.Iface, link.Address, orDash(link.Driver))},
		{i18n.T("f.speed"), fmt.Sprintf("%d Mbit/s · %s", link.SpeedMbit, link.Duplex)},
		{i18n.T("f.mtu"), fmt.Sprint(link.MTU)},
		{i18n.T("f.eee"), eeeText(a.Env.eee)},
		{i18n.T("f.qdisc"), fmt.Sprintf("%s · drops %d · backlog %d · overlimit %d",
			orDash(link.Qdisc), link.QdiscStats.Drops, link.QdiscStats.Backlog,
			link.QdiscStats.Overlimits)},
		{i18n.T("f.counters"), orDash(strings.Join(errorCounters(link), "  "))},
	} {
		b.WriteString("  " + kv(row[0], row[1], sText, 16) + "\n")
	}

	health := a.Env.health
	b.WriteString("\n" + rule("2 · "+i18n.T("sec.tcpcounters"), p.width) + "\n")
	for _, row := range [][2]string{
		{"segments", fmt.Sprintf("out %d · in %d", health.OutSegs, health.InSegs)},
		{i18n.T("f.retransmit"), fmt.Sprintf("%d (%.2f%%)", health.RetransSegs, health.RetransPct)},
		{"lost retrans", fmt.Sprint(health.LostRetransmit)},
		{i18n.T("f.timeouts"), fmt.Sprint(health.Timeouts)},
		{"SYN retrans", fmt.Sprint(health.SynRetrans)},
		{i18n.T("f.ooo"), fmt.Sprint(health.OFOQueue)},
		{"prune / pruned", fmt.Sprintf("%d / %d", health.PruneCalled, health.RcvPruned)},
		{"spurious RTO", fmt.Sprint(health.SpuriousRTOs)},
		{"established", fmt.Sprint(health.CurrEstab)},
		{"failed / reset", fmt.Sprintf("%d / %d", health.AttemptFails, health.EstabResets)},
	} {
		b.WriteString("  " + kv(row[0], row[1], sText, 18) + "\n")
	}

	b.WriteString("\n" + rule("3 · "+i18n.T("panel.sockets")+" · netlink INET_DIAG", p.width) + "\n")
	sockets, err := probe.TCPSockets("")
	switch {
	case err != nil:
		b.WriteString("  " + sFaint.Render(err.Error()) + "\n")
	case len(sockets) == 0:
		b.WriteString("  " + emptyState("ui.none") + "\n")
	default:
		cols := []column{
			{title: i18n.T("col.peer"), width: 24},
			{title: i18n.T("f.cc"), width: 11},
			{title: i18n.T("col.rtt"), width: 8, right: true},
			{title: i18n.T("col.minrtt"), width: 8, right: true},
			{title: i18n.T("col.cwnd"), width: 6, right: true},
			{title: i18n.T("col.retrans"), width: 7, right: true},
			{title: i18n.T("col.rate"), width: 12, right: true},
		}
		var rows [][]cell
		for index, socket := range sockets {
			if index >= 14 {
				break
			}
			style := sText
			if socket.TotalRetrans > 0 {
				style = sWarn
			}
			rows = append(rows, []cell{
				plain(socket.Remote), styled(orDash(socket.CC), sAcc),
				numf("%.1f", socket.RTTms), numf("%.1f", socket.MinRTTms),
				numf("%d", socket.CWnd),
				styled(fmt.Sprint(socket.TotalRetrans), style),
				numf("%.2f Mbps", socket.DeliveryMbps()),
			})
		}
		b.WriteString(renderTable(cols, rows))
		if len(sockets) > 14 {
			b.WriteString(sFaint.Render(i18n.T("ui.more", len(sockets)-14)) + "\n")
		}
	}

	b.WriteString("\n" + rule("4 · "+i18n.T("panel.netfilter"), p.width) + "\n")
	nfq := a.Env.nfqueue
	switch {
	case !nfq.Available:
		b.WriteString("  " + sFaint.Render(i18n.T("ui.needs_root")+" ("+nfq.Reason+")") + "\n")
	case len(nfq.Queues) == 0:
		b.WriteString("  " + sWarn.Render(i18n.T("ui.none")) + "\n")
	default:
		for _, queue := range nfq.Queues {
			style := sOK
			if queue.QueueDropped+queue.UserDropped > 0 {
				style = sBad
			}
			b.WriteString(fmt.Sprintf("  queue %-4d queued %-6d %s\n",
				queue.QNum, queue.Queued,
				style.Render(fmt.Sprintf("dropped %d/%d",
					queue.QueueDropped, queue.UserDropped))))
		}
	}
	b.WriteString("  " + kv(i18n.T("f.conntrack"), fmt.Sprintf("%d / %d",
		a.Env.conntrack.Count, a.Env.conntrack.Max), sText, 18) + "\n")

	b.WriteString("\n" + rule("5 · "+i18n.T("panel.sysctl"), p.width) + "\n")
	for _, key := range probe.SysctlKeys {
		value, ok := a.Env.sysctls[key]
		if !ok {
			continue
		}
		style := sText
		if change, changed := a.Env.bpftune.ChangedByBpftune(key); changed {
			style = sAcc
			value += sFaint.Render(fmt.Sprintf("   ← bpftune ×%d", change.Count))
		}
		b.WriteString("  " + kv(key, value, style, 42) + "\n")
	}
	return b.String()
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

var _ = lipgloss.JoinVertical
var _ = util.Truncate
