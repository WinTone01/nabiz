package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// The help page is the long-form explanation; the ? overlay is the key legend.
// Keeping them separate means neither has to be a compromise.
type helpPage struct {
	viewport      viewport.Model
	width, height int
}

func newHelpPage() Page { return &helpPage{} }

func (p *helpPage) ID() tabID { return tabHelp }

func (p *helpPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *helpPage) Reload(*App) { p.viewport.SetContent(p.body()) }

func (p *helpPage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *helpPage) View(*App) string { return p.viewport.View() }

func (p *helpPage) body() string {
	groups := []struct {
		title string
		keys  []string
	}{
		{"help.sec.sections", []string{
			"help.sections.overview", "help.sections.test", "help.sections.layers",
			"help.sections.kernel", "help.sections.bpftune", "help.sections.unwall",
			"help.sections.dns", "help.sections.monitor", "help.sections.advice",
			"help.sections.history", "help.sections.reports",
		}},
		{"help.sec.reading", []string{
			"help.reading.1", "help.reading.2", "help.reading.3",
			"help.reading.4", "help.reading.5", "help.reading.6",
		}},
		{"help.sec.limits", []string{"help.limits.1", "help.limits.2", "help.limits.3"}},
	}
	var specs []sectionSpec
	for _, group := range groups {
		var lines []string
		for _, key := range group.keys {
			lines = append(lines, sText.Render(wrapIndent(i18n.T(key), p.width-8, "  ")))
		}
		specs = append(specs, sectionSpec{i18n.T(group.title), strings.Join(lines, "\n\n")})
	}
	return stack(p.width, specs...)
}
