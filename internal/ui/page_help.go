package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// The long-form explanation. The ? overlay is the key legend and this is the
// manual; keeping them apart means neither has to be a compromise.
type helpPage struct {
	body          scroller
	keys          keyMap
	width, height int
}

func newHelpPage() Page { return &helpPage{body: newScroller(), keys: defaultKeys()} }

func (p *helpPage) ID() pageID         { return pageHelp }
func (p *helpPage) Focus(focused bool) { p.body.focused = focused }

func (p *helpPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *helpPage) Reload(*App) {
	p.keys = defaultKeys()
	p.body.setContent(p.render())
}

func (p *helpPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *helpPage) View(*App) string                   { return p.body.view() }

func (p *helpPage) render() string {
	width := p.body.contentWidth()
	inner := width - 4

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

	sections := []section{{title: i18n.T("help.sec.keys"), body: p.keyTable(inner)}}
	for _, group := range groups {
		var lines []string
		for _, key := range group.keys {
			lines = append(lines, sText.Render(wrapIndent(i18n.T(key), inner, "  ")))
		}
		sections = append(sections, section{
			title: i18n.T(group.title),
			body:  strings.Join(lines, "\n\n"),
		})
	}
	return stack(width, sections...)
}

// keyTable is the printed version of the ? overlay, so the manual is complete
// on its own and does not send the reader to another screen mid-sentence.
func (p *helpPage) keyTable(width int) string {
	cols := []col{
		{title: "", width: 14},
		{title: "", width: 0},
	}
	var rows [][]cell
	for _, group := range p.keys.FullHelp() {
		for _, binding := range group {
			help := binding.Help()
			rows = append(rows, []cell{styled(help.Key, sAcc), dim(help.Desc)})
		}
		rows = append(rows, []cell{plain(""), plain("")})
	}
	return renderTable(width, cols, rows)
}
