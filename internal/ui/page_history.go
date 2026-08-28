package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/stats"
)

// History exists so a change can be judged. A single score is an opinion; a
// score next to the one from before you changed something is evidence.
type historyPage struct {
	body          scroller
	width, height int
}

func newHistoryPage() Page { return &historyPage{body: newScroller()} }

func (p *historyPage) ID() pageID         { return pageHistory }
func (p *historyPage) Focus(focused bool) { p.body.focused = focused }

func (p *historyPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *historyPage) Reload(a *App)                      { p.body.setContent(p.render(a)) }
func (p *historyPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *historyPage) View(a *App) string                 { return p.body.view() }

func (p *historyPage) render(a *App) string {
	width := p.body.contentWidth()
	inner := width - 4
	if len(a.History) == 0 {
		return stack(width, section{title: i18n.T("sec.trend"), body: emptyState("misc.no_runs")})
	}

	series := report.ScoreSeries(a.History)
	trend := []string{sOK.Render(stats.SparklineF(series, min(len(series), inner)))}
	if len(series) > 0 {
		first, last := series[0], series[len(series)-1]
		style, arrow := sOK, "▲"
		if last < first {
			style, arrow = sBad, "▼"
		}
		trend = append(trend, fmt.Sprintf("%s %s %s",
			sFaint.Render(fmt.Sprintf("%.1f", first)),
			style.Render(fmt.Sprintf("%s %+.1f", arrow, last-first)),
			sBold.Render(fmt.Sprintf("%.1f", last))))
	}
	if a.Baseline != nil {
		trend = append(trend, "", kv(i18n.T("f.baseline"),
			fmt.Sprintf("%.1f (%s) · %s", a.Baseline.Score, a.Baseline.Grade,
				a.Baseline.StartedAt.Format("2006-01-02 15:04")), sAcc, 14))
	}

	cols := []col{
		{title: i18n.T("col.date"), width: 17},
		{title: i18n.T("col.suite"), width: 8},
		{title: i18n.T("f.score"), width: 7, right: true},
		{title: i18n.T("col.grade"), width: 6, right: true},
		{title: "●", width: 4, right: true},
		{title: "▲", width: 4, right: true},
		{title: "", width: 0},
	}
	var rows [][]cell
	for index := len(a.History) - 1; index >= 0; index-- {
		entry := a.History[index]
		badCell, warnCell := dim("·"), dim("·")
		if entry.Bad > 0 {
			badCell = styled(fmt.Sprint(entry.Bad), sBad)
		}
		if entry.Warn > 0 {
			warnCell = styled(fmt.Sprint(entry.Warn), sWarn)
		}
		rows = append(rows, []cell{
			plain(entry.StartedAt.Format("2006-01-02 15:04")),
			dim(entry.Name),
			styled(fmt.Sprintf("%.1f", entry.Score), scoreStyle(entry.Score)),
			plain(entry.Grade),
			badCell, warnCell,
			rawCell(meter(entry.Score/100, max(inner-60, 6), scoreStyle(entry.Score))),
		})
	}
	return stack(width,
		section{title: i18n.T("sec.trend"), badge: fmt.Sprint(len(a.History)),
			body: strings.Join(trend, "\n")},
		section{title: i18n.T("nav.reports"), body: renderTable(inner, cols, rows)})
}
