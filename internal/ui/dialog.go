package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// Dialogs.
//
// The only element allowed to take focus away from the page, and only for
// things that change the machine. A confirmation is worth the interruption
// exactly when the answer cannot be undone by pressing the same key again.
//
// The dialog is drawn over the screen rather than instead of it, so the numbers
// that justify the change stay visible while the change is being approved.

type dialogButton struct {
	id     string
	label  string
	kind   int
	action func() tea.Cmd
}

type dialog struct {
	title   string
	body    string
	accent  lipgloss.TerminalColor
	buttons []dialogButton
	focus   int
	width   int
}

func (d *dialog) move(delta int) {
	count := len(d.buttons)
	if count == 0 {
		return
	}
	d.focus = ((d.focus+delta)%count + count) % count
}

func (d *dialog) activate() tea.Cmd {
	if d.focus < 0 || d.focus >= len(d.buttons) {
		return nil
	}
	if action := d.buttons[d.focus].action; action != nil {
		return action()
	}
	return nil
}

// clickedButton reports which button a press landed on, so the mouse and the
// keyboard reach the same code rather than two copies of it.
func (d *dialog) clickedButton(msg tea.MouseMsg) int {
	for index, item := range d.buttons {
		if clicked(msg, item.id) {
			return index
		}
	}
	return -1
}

func (d *dialog) view(screenWidth int) string {
	width := d.width
	if width <= 0 {
		width = clamp(screenWidth-24, 44, 78)
	}
	inner := width - 4

	var controls []string
	for index, item := range d.buttons {
		kind := item.kind
		if index != d.focus {
			kind = btnGhost
		}
		label := item.label
		if index == d.focus {
			label = "› " + label + " ‹"
		} else {
			label = "  " + label + "  "
		}
		controls = append(controls, button(item.id, label, kind, true))
	}
	row := padLeft(toolbar(controls...), inner)

	body := lipgloss.JoinVertical(lipgloss.Left,
		wrapText(d.body, inner),
		"",
		row)
	accent := d.accent
	if accent == nil {
		accent = colAccent
	}
	return card{title: d.title, width: width, accent: accent}.render(body)
}

// confirmDialog builds the two-button shape every destructive action uses:
// the action itself on the left with its own colour, cancel on the right.
func confirmDialog(title, body, okLabel string, kind int, accent lipgloss.TerminalColor,
	action func() tea.Cmd) *dialog {
	return &dialog{
		title:  title,
		body:   body,
		accent: accent,
		buttons: []dialogButton{
			{id: "dlg:ok", label: okLabel, kind: kind, action: action},
			{id: "dlg:cancel", label: i18n.T("ui.cancel"), kind: btnGhost},
		},
	}
}
