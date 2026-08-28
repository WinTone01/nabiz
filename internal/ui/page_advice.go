package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/WinTone01/nabiz/internal/apply"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
)

// Advice is grouped by priority because the order *is* the advice: fixing a
// cable before tuning a buffer is not a preference.
//
// Items nabiz can perform itself carry a checkbox. Ticking them and pressing
// apply writes a snapshot, runs the batch under one privileged prompt, then
// verifies the connection and rolls itself back if it broke. Nothing here is
// applied without the dialog, and nothing is applied without an undo script
// already on disk.
type advicePage struct {
	body    scroller
	keys    keyMap
	rows    []adviceRow
	cursor  int
	focused bool

	width, height int
}

type adviceRow struct {
	advice     suite.Advice
	change     *apply.Change
	selectable bool
}

func newAdvicePage() Page { return &advicePage{body: newScroller(), keys: defaultKeys()} }

func (p *advicePage) ID() pageID            { return pageAdvice }
func (p *advicePage) SuiteName(*App) string { return "full" }

func (p *advicePage) Focus(focused bool) {
	p.focused = focused
	p.body.focused = focused
}

func (p *advicePage) Layout(width, height int) {
	p.width, p.height = width, height
	// The action bar and its gap live above the list.
	p.body.layout(width, max(height-2, 3))
}

func adviceZone(id string) string { return "adv:" + id }

func (p *advicePage) Reload(a *App) {
	p.keys = defaultKeys()
	p.rows = nil
	result := a.LastRun()
	if result == nil {
		p.body.setContent(emptyState("ui.press_run"))
		return
	}
	applicable := map[string]apply.Change{}
	for _, change := range apply.Available(*result) {
		applicable[change.ID] = change
	}
	a.Applicable = applicable
	for _, advice := range result.Advice {
		if a.Dismissed(advice.ID) {
			continue
		}
		row := adviceRow{advice: advice}
		if change, ok := applicable[advice.ID]; ok {
			copied := change
			row.change, row.selectable = &copied, true
		}
		p.rows = append(p.rows, row)
	}
	p.cursor = clamp(p.cursor, 0, max(len(p.rows)-1, 0))
	p.body.setContent(p.content(a))
}

func (p *advicePage) Update(a *App, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if isPress(msg) {
			if clicked(msg, zoneRestoreDismissed) {
				a.RestoreDismissed()
				p.Reload(a)
				return nil
			}
			for _, row := range p.rows {
				if clicked(msg, dismissZone(row.advice.ID)) {
					a.Dismiss(row.advice.ID)
					p.Reload(a)
					return nil
				}
			}
			for index, row := range p.rows {
				if !row.selectable || !clicked(msg, adviceZone(row.advice.ID)) {
					continue
				}
				p.cursor = index
				p.toggle(a, row.advice.ID)
				p.body.setContent(p.content(a))
				return nil
			}
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, p.keys.Toggle):
			if row := p.currentRow(); row != nil && row.selectable {
				p.toggle(a, row.advice.ID)
				p.body.setContent(p.content(a))
				return nil
			}
		case key.Matches(msg, p.keys.Dismiss):
			if row := p.currentRow(); row != nil {
				a.Dismiss(row.advice.ID)
				p.Reload(a)
				return nil
			}
		case key.Matches(msg, p.keys.Activate):
			// Enter on the list is the same approval the button offers, so the
			// keyboard path never has to reach for the mouse.
			if a.SelectedCount() > 0 {
				return func() tea.Msg { return applyRequestMsg{} }
			}
		case key.Matches(msg, p.keys.Down):
			if p.cursor < len(p.rows)-1 {
				p.cursor++
				p.body.setContent(p.content(a))
				p.body.vp.LineDown(4)
				return nil
			}
		case key.Matches(msg, p.keys.Up):
			if p.cursor > 0 {
				p.cursor--
				p.body.setContent(p.content(a))
				p.body.vp.LineUp(4)
				return nil
			}
		}
	}
	return p.body.update(msg)
}

// applyRequestMsg lets the page ask the root model to open the apply dialog
// without the page needing a reference back to it.
type applyRequestMsg struct{}

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

func (p *advicePage) View(a *App) string {
	if a.LastRun() == nil {
		return emptyState("ui.press_run")
	}
	p.body.setContent(p.content(a))
	return strings.Join([]string{p.actionBar(a), "", p.body.view()}, "\n")
}

// actionBar is the approval surface: how many changes are ticked, and the four
// controls that act on them. It is inside the page rather than in the global
// toolbar because applying a change is not a global action - it only means
// anything next to the list it applies to.
func dismissZone(id string) string { return "adv:dismiss:" + id }

const zoneRestoreDismissed = "adv:restore"

func (p *advicePage) actionBar(a *App) string {
	count := a.SelectedCount()
	controls := toolbar(
		button(zoneApply, fmt.Sprintf("%s (%d)", i18n.T("ui.apply"), count),
			btnSuccess, count > 0 && !a.Busy()),
		button(zoneRollback, i18n.T("apply.rollback"), btnGhost, a.HasSnapshots),
		button(zoneSelectAll, i18n.T("apply.select_all"), btnGhost, len(a.Applicable) > 0),
		button(zoneClearSel, i18n.T("apply.clear"), btnGhost, count > 0),
	)
	if hidden := a.DismissedCount(); hidden > 0 {
		controls = toolbar(controls,
			button(zoneRestoreDismissed, i18n.T("apply.restore_hidden", hidden),
				btnGhost, true))
	}
	label := sMuted.Render(i18n.T("apply.selected", count))
	gap := p.width - visWidth(controls) - visWidth(label)
	if gap < 2 {
		return controls
	}
	return controls + strings.Repeat(" ", gap) + label
}

func (p *advicePage) content(a *App) string {
	result := a.LastRun()
	width := p.body.contentWidth()
	inner := width - 4

	sections := []section{{
		title: i18n.T("sec.advice"),
		badge: fmt.Sprint(len(p.rows)),
		body: sFaint.Render(wrapText(i18n.T("misc.derived_from", result.Name,
			result.StartedAt.Format("15:04"), len(p.rows)), inner)),
	}}

	priority := -1
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		sections = append(sections, section{
			title: i18n.T("misc.priority", priority),
			body:  strings.TrimRight(strings.Join(current, "\n"), "\n"),
		})
		current = nil
	}
	for index, row := range p.rows {
		if row.advice.Priority != priority {
			flush()
			priority = row.advice.Priority
		}
		current = append(current, p.renderRow(a, index, row, inner), "")
	}
	flush()
	return stack(width, sections...)
}

func (p *advicePage) renderRow(a *App, index int, row adviceRow, width int) string {
	marker := "  "
	if index == p.cursor && p.focused {
		marker = sAcc.Render("▎ ")
	}
	head := marker
	indent := 2
	if row.selectable {
		head += checkbox(adviceZone(row.advice.ID), "", a.Selected[row.advice.ID], true) +
			" " + riskBadge(row.change.Risk) + " "
		indent = 4
	}
	// the dismiss control sits on the right of every row: a recommendation you
	// have already decided about should be closable wherever you are reading it
	close := zone.Mark(dismissZone(row.advice.ID),
		sFaint.Render(i18n.T("apply.dismiss_row")))
	head += sInfo.Render("["+suite.CategoryLabel(row.advice.Category)+"] ") +
		sBold.Render(truncate(row.advice.Title,
			max(width-visWidth(head)-visWidth(close)-4, 12)))
	if gap := width - visWidth(head) - visWidth(close); gap > 1 {
		head += strings.Repeat(" ", gap) + close
	}

	pad := strings.Repeat(" ", indent)
	detail := adviceBody(row.advice, width-indent)
	return head + "\n" + pad + strings.ReplaceAll(detail, "\n", "\n"+pad)
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
