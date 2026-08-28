package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
)

// Layers walks the stack from the cable upwards.
//
// Every line is read from the kernel rather than measured, so the page costs
// nothing to refresh and can be trusted while a run is in flight. The numbering
// is the point: when something is wrong you want to know how far up it starts.
type layersPage struct {
	body          scroller
	width, height int
}

func newLayersPage() Page { return &layersPage{body: newScroller()} }

func (p *layersPage) ID() pageID            { return pageLayers }
func (p *layersPage) SuiteName(*App) string { return "deep" }
func (p *layersPage) Focus(focused bool)    { p.body.focused = focused }

func (p *layersPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *layersPage) Reload(a *App)                      { p.body.setContent(p.render(a)) }
func (p *layersPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *layersPage) View(a *App) string                 { return p.body.view() }

func (p *layersPage) render(a *App) string {
	width := p.body.contentWidth()
	inner := width - 4
	link := a.Env.link

	physical := kvBlock([][2]string{
		{i18n.T("f.iface"), fmt.Sprintf("%s (%s) %s", link.Iface, link.Address, orDash(link.Driver))},
		{i18n.T("f.speed"), fmt.Sprintf("%d Mbit/s · %s", link.SpeedMbit, link.Duplex)},
		{i18n.T("f.mtu"), fmt.Sprint(link.MTU)},
		{i18n.T("f.eee"), eeeText(a.Env.eee)},
		{i18n.T("f.qdisc"), fmt.Sprintf("%s · drops %d · backlog %d · overlimit %d",
			orDash(link.Qdisc), link.QdiscStats.Drops, link.QdiscStats.Backlog,
			link.QdiscStats.Overlimits)},
		{i18n.T("f.counters"), orDash(strings.Join(errorCounters(link), "  "))},
	}, 20)

	health := a.Env.health
	counters := kvBlock([][2]string{
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
	}, 20)

	return stack(width,
		section{title: "1 · " + i18n.T("sec.physical"),
			badge: linkBadge(link), body: physical},
		section{title: "2 · " + i18n.T("sec.tcpcounters"),
			badge: retransBadge(health.RetransPct), body: counters},
		section{title: "3 · " + i18n.T("panel.sockets") + " · netlink INET_DIAG",
			body: p.sockets(inner)},
		section{title: "4 · " + i18n.T("panel.netfilter"),
			body: p.netfilter(a, inner)},
		section{title: "5 · " + i18n.T("panel.sysctl"),
			body: p.sysctls(a, inner)})
}

func linkBadge(link probe.LinkInfo) string {
	if link.SpeedMbit > 0 && link.SpeedMbit <= 100 && !link.Wireless {
		return sWarn.Render(fmt.Sprintf("%d Mbit", link.SpeedMbit))
	}
	return sOK.Render(fmt.Sprintf("%d Mbit", link.SpeedMbit))
}

func retransBadge(pct float64) string {
	style := sOK
	switch {
	case pct >= 3:
		style = sBad
	case pct >= 1:
		style = sWarn
	}
	return style.Render(fmt.Sprintf("%.2f%%", pct))
}

// sockets reads the live socket table rather than the cached snapshot: this is
// the one place where a per-connection view is the answer, and it goes stale in
// seconds.
func (p *layersPage) sockets(width int) string {
	sockets, err := probe.TCPSockets("")
	switch {
	case err != nil:
		return sFaint.Render(err.Error())
	case len(sockets) == 0:
		return emptyState("ui.none")
	}
	cols := []col{
		// The peer column takes the slack, but only up to a point: a hostname
		// column forty characters wide pushes the numbers - which are the
		// reason to look - off to the far right of the card.
		{title: i18n.T("col.peer"), width: clamp(width-72, 20, 38)},
		{title: i18n.T("f.cc"), width: 14},
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
	body := renderTable(width, cols, rows)
	if len(sockets) > 12 {
		body += "\n" + sFaint.Render(i18n.T("ui.more", len(sockets)-12))
	}
	return body
}

func (p *layersPage) netfilter(a *App, width int) string {
	var lines []string
	nfq := a.Env.nfqueue
	switch {
	case !nfq.Available:
		lines = append(lines, sFaint.Render(i18n.T("ui.needs_root")+" ("+nfq.Reason+")"))
	case len(nfq.Queues) == 0:
		lines = append(lines, sWarn.Render(i18n.T("ui.none")))
	default:
		for _, queue := range nfq.Queues {
			style := sOK
			if queue.QueueDropped+queue.UserDropped > 0 {
				style = sBad
			}
			lines = append(lines, fmt.Sprintf("queue %-4d queued %-8d %s",
				queue.QNum, queue.Queued,
				style.Render(fmt.Sprintf("dropped %d/%d",
					queue.QueueDropped, queue.UserDropped))))
		}
	}
	return strings.Join(append(lines, kv(i18n.T("f.conntrack"),
		fmt.Sprintf("%d / %d", a.Env.conntrack.Count, a.Env.conntrack.Max), sText, 20)), "\n")
}

// sysctls marks the values bpftune has rewritten, because /etc/sysctl.d stops
// describing the running system the moment it starts.
func (p *layersPage) sysctls(a *App, width int) string {
	var lines []string
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
		lines = append(lines, kv(key, value, style, 42))
	}
	if len(lines) == 0 {
		return emptyState("ui.nodata")
	}
	return strings.Join(lines, "\n")
}
