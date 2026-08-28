package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// Clickable primitives.
//
// Bubble Tea reports mouse coordinates but has no notion of what lives at them,
// so every interactive element wraps itself in a named zone at render time and
// asks that zone whether a click landed inside it. The id is the contract
// between the two halves: render marks it, Update matches it.

const (
	btnPrimary = iota
	btnGhost
	btnDanger
	btnSuccess
)

// Every button is the same shape. Mixing filled and outlined buttons in one row
// makes them different heights, and a toolbar whose items do not share a
// baseline reads as broken rather than as emphasis.
var (
	sBtnBase = lipgloss.NewStyle().Padding(0, 1).
			Border(lipgloss.RoundedBorder())

	sBtnPrimary = sBtnBase.Foreground(cBrand).BorderForeground(cBrand).Bold(true)
	sBtnGhost   = sBtnBase.Foreground(cText).BorderForeground(cLine)
	sBtnDanger  = sBtnBase.Foreground(cBad).BorderForeground(cBad).Bold(true)
	sBtnSuccess = sBtnBase.Foreground(cOK).BorderForeground(cOK).Bold(true)
	sBtnOff     = sBtnBase.Foreground(cFaint).BorderForeground(cLine)
)

// button renders a clickable button and registers its zone.
func button(id, label string, kind int, enabled bool) string {
	style := sBtnGhost
	switch {
	case !enabled:
		style = sBtnOff
	case kind == btnPrimary:
		style = sBtnPrimary
	case kind == btnDanger:
		style = sBtnDanger
	case kind == btnSuccess:
		style = sBtnSuccess
	}
	return zone.Mark(id, style.Render(label))
}

// checkbox renders a clickable checkbox with a label.
func checkbox(id, label string, checked, enabled bool) string {
	box := "☐"
	style := sText
	if checked {
		box, style = "☑", sAcc
	}
	if !enabled {
		style = sFaint
	}
	return zone.Mark(id, style.Render(box+" ")+style.Render(label))
}

// chip renders a clickable selector chip.
func chip(id, label string, active bool) string {
	style := sChipOff
	if active {
		style = sChipOn
	}
	return zone.Mark(id, style.Render(label))
}

// clickableRow marks a whole row so a list can be selected with the mouse.
func clickableRow(id, content string) string { return zone.Mark(id, content) }

// clicked reports whether a left press landed inside the named zone.
func clicked(msg tea.MouseMsg, id string) bool {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return false
	}
	area := zone.Get(id)
	if area == nil {
		return false
	}
	return area.InBounds(msg)
}

// isPress reports a plain left click, wherever it landed.
func isPress(msg tea.MouseMsg) bool {
	return msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft
}

// toolbar lays buttons out in a row with consistent spacing.
func toolbar(buttons ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Center, spaced(buttons)...)
}

func spaced(items []string) []string {
	out := make([]string, 0, len(items)*2)
	for index, item := range items {
		if index > 0 {
			out = append(out, " ")
		}
		out = append(out, item)
	}
	return out
}

// modal is a centred dialog drawn over the whole frame. It is deliberately the
// only element that can take focus away from the page: a confirmation that can
// be scrolled off screen is a confirmation nobody reads.
type modal struct {
	title   string
	body    string
	buttons []modalButton
	width   int
}

type modalButton struct {
	id    string
	label string
	kind  int
}

func (m modal) view(frameWidth, frameHeight int) string {
	width := m.width
	if width <= 0 {
		width = min(max(frameWidth-20, 40), 76)
	}
	var rendered []string
	for _, item := range m.buttons {
		rendered = append(rendered, button(item.id, item.label, item.kind, true))
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		sSection.Render(m.title),
		"",
		lipgloss.NewStyle().Width(width).Render(m.body),
		"",
		toolbar(rendered...))
	box := sOverlay.Width(width).Render(content)
	return lipgloss.Place(frameWidth, frameHeight,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "))
}

// scrollHint shows where a viewport sits, so a mouse user can tell there is
// more content without discovering it by accident.
func scrollHint(percent float64, height int) string {
	if height < 3 {
		return ""
	}
	position := int(percent * float64(height-1))
	var b strings.Builder
	for row := 0; row < height; row++ {
		if row == position {
			b.WriteString(sAcc.Render("█"))
		} else {
			b.WriteString(sRule.Render("│"))
		}
		if row < height-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
