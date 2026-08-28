package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/stats"
)

// History exists so a change can be judged. A single score is an opinion; a
// score next to the one from before you changed something is evidence.
type historyPage struct {
	viewport      viewport.Model
	width, height int
}

func newHistoryPage() Page { return &historyPage{} }

func (p *historyPage) ID() tabID { return tabHistory }

func (p *historyPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *historyPage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *historyPage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *historyPage) View(a *App) string { return p.viewport.View() }

func (p *historyPage) body(a *App) string {
	if len(a.History) == 0 {
		return stack(p.width, sectionSpec{i18n.T("sec.trend"), emptyState("misc.no_runs")})
	}
	series := report.ScoreSeries(a.History)
	trend := []string{sOK.Render(stats.SparklineF(series, min(len(series), p.width-10)))}
	if len(series) > 0 {
		trend = append(trend, sFaint.Render(fmt.Sprintf("%.0f … %.0f",
			series[0], series[len(series)-1])))
	}
	if a.Baseline != nil {
		trend = append(trend, "", kv(i18n.T("f.baseline"),
			fmt.Sprintf("%.1f (%s) · %s", a.Baseline.Score, a.Baseline.Grade,
				a.Baseline.StartedAt.Format("2006-01-02 15:04")), sAcc, 14))
	}

	cols := []column{
		{title: i18n.T("col.date"), width: 17},
		{title: i18n.T("col.suite"), width: 8},
		{title: i18n.T("f.score"), width: 7, right: true},
		{title: i18n.T("col.grade"), width: 6, right: true},
		{title: "bad", width: 5, right: true},
		{title: "warn", width: 5, right: true},
	}
	var rows [][]cell
	for index := len(a.History) - 1; index >= 0; index-- {
		entry := a.History[index]
		rows = append(rows, []cell{
			plain(entry.StartedAt.Format("2006-01-02 15:04")),
			dim(entry.Name),
			styled(fmt.Sprintf("%.1f", entry.Score), scoreStyle(entry.Score)),
			plain(entry.Grade),
			styled(fmt.Sprint(entry.Bad), sBad),
			styled(fmt.Sprint(entry.Warn), sWarn),
		})
	}
	return stack(p.width,
		sectionSpec{i18n.T("sec.trend"), strings.Join(trend, "\n")},
		sectionSpec{i18n.T("nav.reports"), strings.TrimRight(renderTable(cols, rows), "\n")})
}
