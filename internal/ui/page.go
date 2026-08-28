package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Page is one screen. Pages own their components so scrolling and selection
// behave natively instead of every screen being a pre-rendered blob of text.
//
// Reload is called when the data behind a page changes (a run finished, the
// environment was re-read, the language changed); Layout when the frame
// resizes. Keeping those separate means a resize does not re-derive data and a
// data refresh does not reset the scroll position.
type Page interface {
	ID() tabID
	Reload(a *App)
	Layout(width, height int)
	Update(a *App, msg tea.Msg) tea.Cmd
	View(a *App) string
}

// runnablePage reports which suite the run key starts on this page. Pages that
// do not measure anything simply do not implement it.
type runnablePage interface {
	SuiteName(a *App) string
}

// abPage marks pages where the A/B key applies, and to what.
type abPage interface {
	ABTarget() string
}
