package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/monitor"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/util"
)

// The monitor page is the one you leave open. Its job is to make a pattern
// visible without anyone having to read a log: a minute-by-minute loss strip
// and an hour-of-day histogram beside the raw events.
type monitorPage struct {
	width, height int
}

func newMonitorPage() Page { return &monitorPage{} }

func (p *monitorPage) ID() tabID                    { return tabMonitor }
func (p *monitorPage) Reload(*App)                  {}
func (p *monitorPage) Layout(width, height int)     { p.width, p.height = width, height }
func (p *monitorPage) Update(*App, tea.Msg) tea.Cmd { return nil }

func (p *monitorPage) View(a *App) string {
	if a.Watcher == nil {
		return emptyState("ui.nodata")
	}
	snapshot := a.Watcher.Snapshot()

	availStyle := sOK
	if snapshot.Availability <= 99.9 {
		availStyle = sWarn
	}
	outageStyle := sOK
	if len(snapshot.Outages) > 0 {
		outageStyle = sBad
	}
	flapStyle := sOK
	if snapshot.CarrierFlaps > 0 {
		flapStyle = sBad
	}

	head := []string{
		fmt.Sprintf("%s   %s   %s",
			kv(i18n.T("f.uptime"), util.ShortDuration(snapshot.Uptime), sBold, 12),
			kv(i18n.T("f.avail"), fmt.Sprintf("%.3f%%", snapshot.Availability), availStyle, 14),
			kv(i18n.T("f.outages"), fmt.Sprintf("%d (%.1f s)", len(snapshot.Outages),
				totalOutage(snapshot)), outageStyle, 10)),
		fmt.Sprintf("%s   %s   %s",
			kv(i18n.T("f.dnsfail"), fmt.Sprint(snapshot.DNSFailures), sText, 12),
			kv(i18n.T("f.reachfail"), fmt.Sprint(snapshot.ReachFailures), sText, 14),
			kv(i18n.T("f.flaps"), fmt.Sprint(snapshot.CarrierFlaps), flapStyle, 10)),
		kv(i18n.T("f.lastreach"),
			util.Truncate(orDash(snapshot.ReachLast), p.width-20), sText, 12),
		"",
	}

	chart := max(p.width-52, 10)
	cols := []column{
		{title: i18n.T("col.target"), width: 18},
		{title: i18n.T("col.loss"), width: 8, right: true},
		{title: i18n.T("col.avg"), width: 6, right: true},
		{title: i18n.T("col.p95"), width: 6, right: true},
		{title: "", width: chart},
	}
	var rows [][]cell
	for _, target := range snapshot.Targets {
		lossStyle := sOK
		if target.LossPct >= 1 {
			lossStyle = sBad
		}
		rows = append(rows, []cell{
			plain(target.Label),
			styled(fmt.Sprintf("%.2f%%", target.LossPct), lossStyle),
			numf("%.0f", target.Avg),
			numf("%.0f", target.P95),
			rendered(sparkline(target.Recent, chart)),
		})
	}
	body := strings.Join(head, "\n") + renderTable(cols, rows)

	timeline := renderTimeline(snapshot, p.width-24)
	hours := a.Watcher.HourlyHistogram()
	var histogram strings.Builder
	for hour := 0; hour < 24; hour++ {
		if hours[hour] == 0 {
			histogram.WriteString(sFaint.Render("· "))
			continue
		}
		histogram.WriteString(sBad.Render(string(stats.Spark[min(hours[hour], 7)]) + " "))
	}

	used := lipgloss.Height(body) + lipgloss.Height(timeline) + 8
	logHeight := max(p.height-used, 3)
	events := snapshot.Events
	if len(events) > logHeight {
		events = events[len(events)-logHeight:]
	}
	var log []string
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		log = append(log, fmt.Sprintf("%s %s %s %s",
			sFaint.Render(event.Stamp()),
			levelStyle(event.Severity).Render(padRight(event.Kind, 15)),
			sAcc.Render(padRight(util.Truncate(event.Target, 18), 18)),
			sText.Render(util.Truncate(event.Detail, max(p.width-62, 10)))))
	}
	if len(log) == 0 {
		log = []string{emptyState("misc.no_events")}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		panel(i18n.T("panel.stability"), p.width, body),
		panel(i18n.T("panel.timeline"), p.width, timeline+"\n"+
			sDim.Render(padRight(i18n.T("misc.hourly"), 22))+histogram.String()),
		panel(i18n.T("panel.log"), p.width, strings.Join(log, "\n")))
}

func renderTimeline(snapshot monitor.Snapshot, width int) string {
	if len(snapshot.Timeline) == 0 {
		return emptyState("ui.nodata")
	}
	timeline := snapshot.Timeline
	if len(timeline) > width {
		timeline = timeline[len(timeline)-width:]
	}
	var b strings.Builder
	b.WriteString(sDim.Render(padRight(i18n.T("misc.per_min_loss"), 22)))
	for _, minute := range timeline {
		switch {
		case minute.LossPct > 20:
			b.WriteString(sBad.Render("█"))
		case minute.LossPct > 2:
			b.WriteString(sWarn.Render("▄"))
		default:
			b.WriteString(sOK.Render("▁"))
		}
	}
	return b.String()
}

func totalOutage(snapshot monitor.Snapshot) float64 {
	total := 0.0
	for _, outage := range snapshot.Outages {
		total += outage.Duration
	}
	return total
}
