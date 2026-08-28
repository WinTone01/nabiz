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

// The page you leave open.
//
// Its job is to make a pattern visible without anyone reading a log: a
// minute-by-minute loss strip, an hour-of-day histogram, and the events
// underneath with timestamps. "It drops every evening at eight" is a sentence
// this page can produce and a speed test never can.
type monitorPage struct {
	body          scroller
	width, height int
}

func newMonitorPage() Page { return &monitorPage{body: newScroller()} }

func (p *monitorPage) ID() pageID            { return pageMonitor }
func (p *monitorPage) Focus(focused bool)    { p.body.focused = focused }
func (p *monitorPage) SuiteName(*App) string { return "quick" }

func (p *monitorPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *monitorPage) Reload(a *App) { p.body.setContent(p.render(a)) }

func (p *monitorPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }

func (p *monitorPage) View(a *App) string {
	p.body.setContent(p.render(a))
	return p.body.view()
}

func (p *monitorPage) render(a *App) string {
	if a.Watcher == nil {
		return emptyState("ui.nodata")
	}
	width := p.body.contentWidth()
	snapshot := a.Watcher.Snapshot()

	return stack(width,
		section{title: i18n.T("panel.stability"),
			badge: p.headline(snapshot),
			body:  p.summary(snapshot, width-4)},
		section{title: i18n.T("panel.timeline"),
			body: p.timeline(a, snapshot, width-4)},
		section{title: i18n.T("panel.log"),
			badge: fmt.Sprint(len(snapshot.Events)),
			body:  p.log(snapshot, width-4)})
}

func (p *monitorPage) headline(snapshot monitor.Snapshot) string {
	if snapshot.OutageActive {
		return sBad.Render("● " + i18n.T("f.outages"))
	}
	if snapshot.Availability <= 99.9 {
		return sWarn.Render(fmt.Sprintf("%.3f%%", snapshot.Availability))
	}
	return sOK.Render(fmt.Sprintf("%.3f%%", snapshot.Availability))
}

func (p *monitorPage) summary(snapshot monitor.Snapshot, width int) string {
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

	// These read as sentences rather than as a table: the labels are different
	// lengths in every language, and three columns forced to a common width
	// would leave one of them stranded in the middle of the card.
	stat := func(label, value string, style lipgloss.Style) string {
		return sMuted.Render(label) + " " + style.Render(value)
	}
	separator := sLine.Render("   ·   ")
	head := []string{
		strings.Join([]string{
			stat(i18n.T("f.uptime"), util.ShortDuration(snapshot.Uptime), sBold),
			stat(i18n.T("f.avail"), fmt.Sprintf("%.3f%%", snapshot.Availability), availStyle),
			stat(i18n.T("f.outages"), fmt.Sprintf("%d (%.1f s)", len(snapshot.Outages),
				totalOutage(snapshot)), outageStyle),
		}, separator),
		strings.Join([]string{
			stat(i18n.T("f.dnsfail"), fmt.Sprint(snapshot.DNSFailures), sText),
			stat(i18n.T("f.reachfail"), fmt.Sprint(snapshot.ReachFailures), sText),
			stat(i18n.T("f.flaps"), fmt.Sprint(snapshot.CarrierFlaps), flapStyle),
		}, separator),
		stat(i18n.T("f.lastreach"), truncate(orDash(snapshot.ReachLast), width-24), sText),
		"",
	}

	cols := []col{
		{title: i18n.T("col.target"), width: 18},
		{title: i18n.T("col.loss"), width: 8, right: true},
		{title: i18n.T("col.avg"), width: 6, right: true},
		{title: i18n.T("col.p95"), width: 6, right: true},
		{title: "", width: 0},
	}
	chart := max(width-46, 10)
	var rows [][]cell
	for _, target := range snapshot.Targets {
		rows = append(rows, []cell{
			plain(target.Label),
			styled(fmt.Sprintf("%.2f%%", target.LossPct), lossStyle(target.LossPct)),
			numf("%.0f", target.Avg),
			numf("%.0f", target.P95),
			rawCell(sparkline(target.Recent, chart)),
		})
	}
	return strings.Join(head, "\n") + renderTable(width, cols, rows)
}

func (p *monitorPage) timeline(a *App, snapshot monitor.Snapshot, width int) string {
	label := 22
	strip := sMuted.Render(padRight(i18n.T("misc.per_min_loss"), label)) +
		renderTimeline(snapshot, max(width-label, 10))

	hours := a.Watcher.HourlyHistogram()
	var histogram strings.Builder
	for hour := 0; hour < 24; hour++ {
		if hours[hour] == 0 {
			histogram.WriteString(sFaint.Render("· "))
			continue
		}
		histogram.WriteString(sBad.Render(string(stats.Spark[min(hours[hour], 7)]) + " "))
	}
	scale := sFaint.Render(padRight("", label))
	for hour := 0; hour < 24; hour += 6 {
		scale += sFaint.Render(padRight(fmt.Sprintf("%02d", hour), 12))
	}
	return strip + "\n" +
		sMuted.Render(padRight(i18n.T("misc.hourly"), label)) + histogram.String() + "\n" +
		scale
}

func (p *monitorPage) log(snapshot monitor.Snapshot, width int) string {
	events := snapshot.Events
	if len(events) > 40 {
		events = events[len(events)-40:]
	}
	if len(events) == 0 {
		return emptyState("misc.no_events")
	}
	var lines []string
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		lines = append(lines, fmt.Sprintf("%s %s %s %s",
			sFaint.Render(event.Stamp()),
			levelStyle(event.Severity).Render(fit(levelMark(event.Severity)+" "+event.Kind, 17)),
			sAcc.Render(fit(event.Target, 18)),
			sText.Render(truncate(event.Detail, max(width-60, 10)))))
	}
	return strings.Join(lines, "\n")
}

// renderTimeline draws one column per minute: a floor of green, amber where a
// couple of packets went missing, a solid red block where the line was gone.
func renderTimeline(snapshot monitor.Snapshot, width int) string {
	if len(snapshot.Timeline) == 0 {
		return sFaint.Render(i18n.T("ui.nodata"))
	}
	timeline := snapshot.Timeline
	if len(timeline) > width {
		timeline = timeline[len(timeline)-width:]
	}
	var b strings.Builder
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
