package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
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
	link := a.Env.link
	physical := kvBlock(16, [][2]string{
		{i18n.T("f.iface"), fmt.Sprintf("%s (%s) %s", link.Iface, link.Address, orDash(link.Driver))},
		{i18n.T("f.speed"), fmt.Sprintf("%d Mbit/s · %s", link.SpeedMbit, link.Duplex)},
		{i18n.T("f.mtu"), fmt.Sprint(link.MTU)},
		{i18n.T("f.eee"), eeeText(a.Env.eee)},
		{i18n.T("f.qdisc"), fmt.Sprintf("%s · drops %d · backlog %d · overlimit %d",
			orDash(link.Qdisc), link.QdiscStats.Drops, link.QdiscStats.Backlog,
			link.QdiscStats.Overlimits)},
		{i18n.T("f.counters"), orDash(strings.Join(errorCounters(link), "  "))},
	})

	health := a.Env.health
	counters := kvBlock(18, [][2]string{
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
	})

	var socketBody string
	sockets, err := probe.TCPSockets("")
	switch {
	case err != nil:
		socketBody = sFaint.Render(err.Error())
	case len(sockets) == 0:
		socketBody = emptyState("ui.none")
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
			if index >= 12 {
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
		socketBody = strings.TrimRight(renderTable(cols, rows), "\n")
		if len(sockets) > 12 {
			socketBody += "\n" + sFaint.Render(i18n.T("ui.more", len(sockets)-12))
		}
	}

	var netfilter []string
	nfq := a.Env.nfqueue
	switch {
	case !nfq.Available:
		netfilter = append(netfilter, sFaint.Render(i18n.T("ui.needs_root")+" ("+nfq.Reason+")"))
	case len(nfq.Queues) == 0:
		netfilter = append(netfilter, sWarn.Render(i18n.T("ui.none")))
	default:
		for _, queue := range nfq.Queues {
			style := sOK
			if queue.QueueDropped+queue.UserDropped > 0 {
				style = sBad
			}
			netfilter = append(netfilter, fmt.Sprintf("queue %-4d queued %-6d %s",
				queue.QNum, queue.Queued,
				style.Render(fmt.Sprintf("dropped %d/%d",
					queue.QueueDropped, queue.UserDropped))))
		}
	}
	netfilter = append(netfilter, kv(i18n.T("f.conntrack"),
		fmt.Sprintf("%d / %d", a.Env.conntrack.Count, a.Env.conntrack.Max), sText, 18))

	var tunables []string
	for _, key := range probe.SysctlKeys {
		value, ok := a.Env.sysctls[key]
		if !ok {
			continue
		}
		style := sText
		if change, changed := a.Env.bpftune.ChangedByBpftune(key); changed {
			style = sAcc
			value += sFaint.Render(fmt.Sprintf("   <- bpftune x%d", change.Count))
		}
		tunables = append(tunables, kv(key, value, style, 42))
	}

	return stack(p.width,
		sectionSpec{"1 · " + i18n.T("sec.physical"), physical},
		sectionSpec{"2 · " + i18n.T("sec.tcpcounters"), counters},
		sectionSpec{"3 · " + i18n.T("panel.sockets") + " · netlink INET_DIAG", socketBody},
		sectionSpec{"4 · " + i18n.T("panel.netfilter"), strings.Join(netfilter, "\n")},
		sectionSpec{"5 · " + i18n.T("panel.sysctl"), strings.Join(tunables, "\n")})
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
