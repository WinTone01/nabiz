package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Mouse helpers. Bubble Tea gives coordinates; bubblezone gives them meaning.

// clicked reports whether a left press landed inside the named zone.
func clicked(msg tea.MouseMsg, id string) bool {
	if !isPress(msg) {
		return false
	}
	area := zone.Get(id)
	if area == nil {
		return false
	}
	return area.InBounds(msg)
}

// isPress is a plain left click, wherever it landed.
func isPress(msg tea.MouseMsg) bool {
	return msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft
}
