package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/apply"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
)

// Advice is grouped by priority because the order *is* the advice: fixing a
// cable before tuning a buffer is not a preference.
//
// Items nabiz can perform itself carry a checkbox. Selecting them and pressing
// Apply writes a snapshot, runs the batch under one privileged prompt, then
// verifies the connection and rolls back on its own if it broke. Nothing here
// is applied without the dialog, and nothing is applied without an undo script
// already on disk.
type advicePage struct {
	viewport      viewport.Model
	width, height int
	keys          keyMap
	rows          []adviceRow
	cursor        int
}

type adviceRow struct {
	advice     suite.Advice
	change     *apply.Change
	selectable bool
}

func newAdvicePage() Page { return &advicePage{keys: defaultKeys()} }

func (p *advicePage) ID() tabID { return tabAdvice }

func (p *advicePage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width = width
	p.viewport.Height = max(height-3, 3)
}

func (p *advicePage) Reload(a *App) {
	p.keys = defaultKeys()
	p.rows = nil
	result := a.LastRun()
	if result == nil {
		p.viewport.SetContent(emptyState("ui.press_run"))
		return
	}
	applicable := map[string]apply.Change{}
	for _, change := range apply.Available(*result) {
		applicable[change.ID] = change
	}
	a.Applicable = applicable
	for _, advice := range result.Advice {
		row := adviceRow{advice: advice}
		if change, ok := applicable[advice.ID]; ok {
			copied := change
			row.change, row.selectable = &copied, true
		}
		p.rows = append(p.rows, row)
	}
	if p.cursor >= len(p.rows) {
		p.cursor = max(len(p.rows)-1, 0)
	}
	p.viewport.SetContent(p.body(a))
}

func (p *advicePage) SuiteName(*App) string { return "full" }

func (p *advicePage) Update(a *App, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if isPress(msg) {
			for index, row := range p.rows {
				if !row.selectable {
					continue
				}
				if clicked(msg, adviceZone(row.advice.ID)) {
					p.toggle(a, row.advice.ID)
					p.cursor = index
					p.viewport.SetContent(p.body(a))
					return nil
				}
			}
		}

	case tea.KeyMsg:
		switch {
		case msg.String() == " ":
			if row := p.currentRow(); row != nil && row.selectable {
				p.toggle(a, row.advice.ID)
				p.viewport.SetContent(p.body(a))
				return nil
			}
		case key.Matches(msg, p.keys.Down):
			if p.cursor < len(p.rows)-1 {
				p.cursor++
				p.viewport.SetContent(p.body(a))
				p.viewport.LineDown(3)
				return nil
			}
		case key.Matches(msg, p.keys.Up):
			if p.cursor > 0 {
				p.cursor--
				p.viewport.SetContent(p.body(a))
				p.viewport.LineUp(3)
				return nil
			}
		}
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *advicePage) currentRow() *adviceRow {
	if p.cursor < 0 || p.cursor >= len(p.rows) {
		return nil
	}
	return &p.rows[p.cursor]
}

func (p *advicePage) toggle(a *App, id string) {
	if a.Selected == nil {
		a.Selected = map[string]bool{}
	}
	a.Selected[id] = !a.Selected[id]
}

func adviceZone(id string) string { return "adv:" + id }

func (p *advicePage) View(a *App) string {
	if a.LastRun() == nil {
		return emptyState("ui.press_run")
	}
	p.viewport.SetContent(p.body(a))
	return lipgloss.JoinVertical(lipgloss.Left,
		p.actionBar(a), "", p.viewport.View())
}

// actionBar is the approval surface: how many changes are ticked, and the two
// buttons that act on them.
func (p *advicePage) actionBar(a *App) string {
	count := 0
	for id, on := range a.Selected {
		if on && a.Applicable[id].ID != "" {
			count++
		}
	}
	label := i18n.T("apply.selected", count)
	buttons := []string{
		button(zoneApply, i18n.T("ui.apply")+" ("+fmt.Sprint(count)+")",
			btnSuccess, count > 0 && a.Running == ""),
		button(zoneRollback, i18n.T("apply.rollback"), btnGhost, a.HasSnapshots),
		button(zoneSelectSafe, i18n.T("apply.select_safe"), btnGhost, len(a.Applicable) > 0),
		button(zoneClearSel, i18n.T("apply.clear"), btnGhost, count > 0),
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		toolbar(buttons...), "  ", sDim.Render(label))
}

func (p *advicePage) body(a *App) string {
	result := a.LastRun()
	var specs []sectionSpec
	specs = append(specs, sectionSpec{i18n.T("sec.advice"),
		sFaint.Render(i18n.T("misc.derived_from", result.Name,
			result.StartedAt.Format("15:04"), len(result.Advice)))})

	priority := -1
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		specs = append(specs, sectionSpec{i18n.T("misc.priority", priority),
			strings.TrimRight(strings.Join(current, "\n"), "\n")})
		current = nil
	}
	for index, row := range p.rows {
		if row.advice.Priority != priority {
			flush()
			priority = row.advice.Priority
		}
		marker := "  "
		if index == p.cursor {
			marker = sAcc.Render("| ")
		}
		head := marker
		if row.selectable {
			head += checkbox(adviceZone(row.advice.ID), "", a.Selected[row.advice.ID], true) + " "
			head += riskBadge(row.change.Risk) + " "
		} else {
			head += "     "
		}
		head += sInfo.Render("["+suite.CategoryLabel(row.advice.Category)+"] ") +
			sBold.Render(row.advice.Title)
		current = append(current, head, strings.TrimRight(adviceBody(row.advice, p.width-6), "\n"), "")
	}
	flush()
	return stack(p.width, specs...)
}

func riskBadge(risk string) string {
	switch risk {
	case apply.RiskLow:
		return sOK.Render("[" + i18n.T("apply.risk.low") + "]")
	case apply.RiskMedium:
		return sWarn.Render("[" + i18n.T("apply.risk.medium") + "]")
	case apply.RiskLink:
		return sBad.Render("[" + i18n.T("apply.risk.link") + "]")
	}
	return ""
}

// adviceBody is the detail under the title, without repeating the title.
func adviceBody(advice suite.Advice, width int) string {
	var b strings.Builder
	b.WriteString("     " + sText.Render(wrapIndent(advice.Why, width-10, "     ")) + "\n")
	for _, step := range advice.How {
		if suite.IsCommand(step) {
			b.WriteString("       " + sFaint.Render("$ ") + sAcc.Render(step) + "\n")
			continue
		}
		b.WriteString("       " + sFaint.Render("• ") +
			sText.Render(wrapIndent(step, width-12, "         ")) + "\n")
	}
	if advice.Gain != "" {
		b.WriteString("     " + sOK.Render("→ ") +
			sText.Render(wrapIndent(advice.Gain, width-10, "       ")) + "\n")
	}
	if advice.Risk != "" {
		b.WriteString("     " + sWarn.Render("! ") +
			sText.Render(wrapIndent(advice.Risk, width-10, "       ")) + "\n")
	}
	return b.String()
}
