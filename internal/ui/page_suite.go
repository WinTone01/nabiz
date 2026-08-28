package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
)

// The test page is a suite picker and the result of that suite.
//
// Results are kept per suite rather than "the last one", so switching between
// quick and deep does not throw away the other's numbers - which is the
// difference between comparing two runs and remembering one of them.
type testPage struct {
	body          scroller
	keys          keyMap
	width, height int
}

func newTestPage() Page { return &testPage{body: newScroller(), keys: defaultKeys()} }

func (p *testPage) ID() pageID              { return pageTest }
func (p *testPage) SuiteName(a *App) string { return suite.Names[a.SuitePick] }
func (p *testPage) Focus(focused bool)      { p.body.focused = focused }

func (p *testPage) Layout(width, height int) {
	p.width, p.height = width, height
	// Two rows for the picker and its description, one for the gap.
	p.body.layout(width, max(height-3, 3))
}

func (p *testPage) Reload(a *App) {
	p.keys = defaultKeys()
	p.body.setContent(p.content(a))
}

func suiteZone(name string) string { return "suite:" + name }

func (p *testPage) Update(a *App, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if isPress(msg) {
			for index, name := range suite.Names {
				if clicked(msg, suiteZone(name)) {
					p.pick(a, index)
					return nil
				}
			}
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, p.keys.Left):
			p.pick(a, a.SuitePick-1)
			return nil
		case key.Matches(msg, p.keys.Right):
			p.pick(a, a.SuitePick+1)
			return nil
		}
	}
	return p.body.update(msg)
}

func (p *testPage) pick(a *App, index int) {
	count := len(suite.Names)
	a.SuitePick = ((index % count) + count) % count
	p.body.setContent(p.content(a))
	p.body.vp.GotoTop()
}

func (p *testPage) View(a *App) string {
	name := suite.Names[a.SuitePick]
	turkish, english := suite.Describe(name)
	description := english
	if i18n.Current() == i18n.TR {
		description = turkish
	}

	ids := make([]string, len(suite.Names))
	for index, candidate := range suite.Names {
		ids[index] = suiteZone(candidate)
	}
	picker := segmented(ids, suite.Names, a.SuitePick)

	p.body.setContent(p.content(a))
	return strings.Join([]string{
		picker,
		sMuted.Render(truncate(description, p.width-1)),
		"",
		p.body.view(),
	}, "\n")
}

func (p *testPage) content(a *App) string {
	name := suite.Names[a.SuitePick]
	result, ok := a.Results[name]
	if !ok {
		return stack(p.body.contentWidth(),
			section{title: name, body: emptyState("ui.press_run")})
	}
	return renderResult(result, p.body.contentWidth())
}
