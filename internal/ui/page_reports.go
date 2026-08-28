package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/suite"
)

// A two-pane browser: the run list on the left, the selected run rendered on
// the right, in exactly the layout the test page uses. ↑↓ moves the selection
// rather than scrolling the text, because picking a run is the reason this page
// exists; ←→ moves between the two panes.
type reportsPage struct {
	paths   []string
	index   int
	detail  scroller
	pane    int // 0 = list, 1 = detail
	focused bool

	width, height int
}

const reportListWidth = 30

func newReportsPage() Page { return &reportsPage{detail: newScroller()} }

func (p *reportsPage) ID() pageID { return pageReports }

// The run key refreshes the list here rather than starting a measurement: this
// page is about runs that already happened.
func (p *reportsPage) SuiteName(*App) string { return "" }

func (p *reportsPage) Focus(focused bool) {
	p.focused = focused
	p.detail.focused = focused && p.pane == 1
}

func (p *reportsPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.detail.layout(max(width-reportListWidth-3, 30), height)
}

func (p *reportsPage) Reload(a *App) {
	p.paths = report.ListRuns(60)
	p.index = clamp(p.index, 0, max(len(p.paths)-1, 0))
	p.load(a)
}

func (p *reportsPage) load(a *App) {
	if len(p.paths) == 0 {
		p.detail.setContent(emptyState("misc.no_runs"))
		return
	}
	result, err := report.LoadJSON(p.paths[p.index])
	if err != nil {
		p.detail.setContent(sBad.Render(err.Error()))
		return
	}
	// Saved text may be in the other language; re-derive before rendering so a
	// report never argues with the interface it is displayed in.
	suite.Refresh(&result, a.Cfg)
	p.detail.setContent(renderResult(result, p.detail.contentWidth()))
	p.detail.vp.GotoTop()
}

func reportZone(index int) string { return fmt.Sprintf("rep:%d", index) }

func (p *reportsPage) Update(a *App, msg tea.Msg) tea.Cmd {
	keys := defaultKeys()
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if isPress(msg) {
			for index := range p.paths {
				if clicked(msg, reportZone(index)) {
					p.index, p.pane = index, 0
					p.detail.focused = false
					p.load(a)
					return nil
				}
			}
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Right):
			p.pane = 1
			p.detail.focused = p.focused
			return nil
		case key.Matches(msg, keys.Left):
			p.pane = 0
			p.detail.focused = false
			return nil
		case key.Matches(msg, keys.Up) && p.pane == 0:
			if p.index > 0 {
				p.index--
				p.load(a)
			}
			return nil
		case key.Matches(msg, keys.Down) && p.pane == 0:
			if p.index < len(p.paths)-1 {
				p.index++
				p.load(a)
			}
			return nil
		}
	}
	return p.detail.update(msg)
}

func (p *reportsPage) View(a *App) string {
	list := p.viewList()
	divider := p.divider()
	return lipgloss.JoinHorizontal(lipgloss.Top, list, divider, p.detail.view())
}

func (p *reportsPage) viewList() string {
	var lines []string
	for index, path := range p.paths {
		if index >= p.height {
			break
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		switch {
		case index == p.index:
			style := sSelected
			if p.pane != 0 || !p.focused {
				style = sHovered
			}
			lines = append(lines, clickableRow(reportZone(index),
				style.Render(" "+fit(name, reportListWidth-2))))
		default:
			lines = append(lines, clickableRow(reportZone(index),
				" "+sFaint.Render(fit(name, reportListWidth-2))))
		}
	}
	if len(lines) == 0 {
		lines = []string{emptyState("misc.no_runs")}
	}
	for len(lines) < p.height {
		lines = append(lines, strings.Repeat(" ", reportListWidth))
	}
	return strings.Join(lines[:p.height], "\n")
}

// divider is a column rather than a border so the focused pane can be shown by
// colouring it, which is cheaper to read than two different border styles.
func (p *reportsPage) divider() string {
	style := sLine
	if p.focused {
		style = sEdge
	}
	rows := make([]string, p.height)
	for index := range rows {
		rows[index] = style.Render(" │ ")
	}
	return strings.Join(rows, "\n")
}
