package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/suite"
)

// A two-pane browser: the run list on the left, the selected run rendered on
// the right. ↑↓ moves the selection rather than scrolling the text, because
// picking a run is the action this page exists for.
type reportsPage struct {
	paths         []string
	index         int
	viewport      viewport.Model
	width, height int
	keys          keyMap
}

func newReportsPage() Page { return &reportsPage{keys: defaultKeys()} }

func (p *reportsPage) ID() tabID { return tabReports }

func (p *reportsPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width = max(width-32, 30)
	p.viewport.Height = height
}

func (p *reportsPage) Reload(a *App) {
	p.keys = defaultKeys()
	p.paths = report.ListRuns(60)
	if p.index >= len(p.paths) {
		p.index = max(len(p.paths)-1, 0)
	}
	p.loadSelection(a)
}

func (p *reportsPage) loadSelection(a *App) {
	if len(p.paths) == 0 {
		p.viewport.SetContent(emptyState("misc.no_runs"))
		return
	}
	result, err := report.LoadJSON(p.paths[p.index])
	if err != nil {
		p.viewport.SetContent(sBad.Render(err.Error()))
		return
	}
	// saved text may be in the other language; re-derive before rendering
	suite.Refresh(&result, a.Cfg)
	p.viewport.SetContent(renderResult(result, p.viewport.Width))
	p.viewport.GotoTop()
}

func reportZone(index int) string { return fmt.Sprintf("rep:%d", index) }

func (p *reportsPage) Update(a *App, msg tea.Msg) tea.Cmd {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && isPress(mouseMsg) {
		for index := range p.paths {
			if clicked(mouseMsg, reportZone(index)) {
				p.index = index
				p.loadSelection(a)
				return nil
			}
		}
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, p.keys.Up):
			if p.index > 0 {
				p.index--
				p.loadSelection(a)
			}
			return nil
		case key.Matches(keyMsg, p.keys.Down):
			if p.index < len(p.paths)-1 {
				p.index++
				p.loadSelection(a)
			}
			return nil
		}
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *reportsPage) SuiteName(*App) string { return "" } // r refreshes the list

func (p *reportsPage) View(a *App) string {
	listWidth := 30
	var lines []string
	for index, path := range p.paths {
		if index >= p.height {
			break
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if index == p.index {
			lines = append(lines, clickableRow(reportZone(index),
				sChipOn.Render(padRight(name, listWidth-2))))
			continue
		}
		lines = append(lines, clickableRow(reportZone(index),
			sFaint.Render(padRight("  "+name, listWidth-2))))
	}
	if len(lines) == 0 {
		lines = []string{emptyState("misc.no_runs")}
	}
	list := lipgloss.NewStyle().Width(listWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, list,
		sRule.Render(" │ "), p.viewport.View())
}

var _ = i18n.T
