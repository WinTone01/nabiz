package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// Advice is grouped by priority because the order is the advice: fixing a
// cable before tuning a buffer is not a preference, it is the whole point.
type advicePage struct {
	viewport      viewport.Model
	width, height int
}

func newAdvicePage() Page { return &advicePage{} }

func (p *advicePage) ID() tabID { return tabAdvice }

func (p *advicePage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *advicePage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *advicePage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *advicePage) SuiteName(*App) string { return "full" }

func (p *advicePage) View(a *App) string { return p.viewport.View() }

func (p *advicePage) body(a *App) string {
	result := a.LastRun()
	if result == nil {
		return emptyState("ui.press_run")
	}
	var b strings.Builder
	b.WriteString(sSection.Render(i18n.T("sec.advice")) + "\n")
	b.WriteString(sFaint.Render(i18n.T("misc.derived_from", result.Name,
		result.StartedAt.Format("15:04"), len(result.Advice))) + "\n")

	priority := -1
	for _, advice := range result.Advice {
		if advice.Priority != priority {
			priority = advice.Priority
			b.WriteString("\n" + rule(i18n.T("misc.priority", priority), p.width-2) + "\n\n")
		}
		b.WriteString(adviceBlock(advice, p.width) + "\n")
	}
	return b.String()
}
