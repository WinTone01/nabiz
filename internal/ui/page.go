package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A page is one screen.
//
// Pages own their components, so scrolling and selection behave natively
// instead of every screen being a pre-rendered blob of text. Reload is called
// when the data behind a page changes; Layout when the frame resizes. Keeping
// those apart means a resize does not re-derive data and a data refresh does
// not throw away the reader's scroll position.
type Page interface {
	ID() pageID
	Reload(a *App)
	Layout(width, height int)
	Update(a *App, msg tea.Msg) tea.Cmd
	View(a *App) string
}

// runnablePage names the suite the run key starts here. Pages that measure
// nothing simply do not implement it, and the run button greys out.
type runnablePage interface {
	SuiteName(a *App) string
}

// abPage marks the pages where an A/B comparison means something, and what it
// would toggle.
type abPage interface {
	ABTarget() string
}

// focusablePage is told when the content region holds focus, so it can show a
// cursor only while that cursor does anything.
type focusablePage interface {
	Focus(focused bool)
}

// --- the scrolling content region ---------------------------------------------

// scroller is a viewport with a gutter. The gutter is always reserved, even
// when the content fits: a scrollbar that appears and disappears reflows the
// text under the reader's eyes every time a table gains a row.
type scroller struct {
	vp      viewport.Model
	focused bool
}

func newScroller() scroller {
	view := viewport.New(10, 10)
	view.MouseWheelEnabled = true
	return scroller{vp: view}
}

const gutterWidth = 2

func (s *scroller) layout(width, height int) {
	s.vp.Width = max(width-gutterWidth, 10)
	s.vp.Height = max(height, 1)
}

func (s *scroller) contentWidth() int { return s.vp.Width }

func (s *scroller) setContent(body string) { s.vp.SetContent(body) }

func (s *scroller) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return cmd
}

func (s *scroller) view() string {
	body := s.vp.View()
	if s.vp.TotalLineCount() <= s.vp.VisibleLineCount() {
		return body
	}
	visible := 1.0
	if total := s.vp.TotalLineCount(); total > 0 {
		visible = float64(s.vp.VisibleLineCount()) / float64(total)
	}
	bar := scrollbar(s.vp.Height, s.vp.ScrollPercent(), visible, s.focused)
	return lipgloss.JoinHorizontal(lipgloss.Top, body, " ", bar)
}

// --- section stacking ----------------------------------------------------------

// section is one titled block on a page.
type section struct {
	title string
	badge string
	body  string
}

// stack renders sections as cards, one under the other, dropping the empty
// ones. Every page is built from this, which is why no screen is a wall of text
// with a rule through it: if something has a heading, it has a border.
func stack(width int, items ...section) string {
	var parts []string
	for _, item := range items {
		if strings.TrimSpace(item.body) == "" {
			continue
		}
		parts = append(parts, card{title: item.title, badge: item.badge, width: width}.
			render(item.body))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// grid lays blocks out row by row in a fixed number of columns. Callers size
// their blocks with columnWidth first, so the grid never has to re-measure
// content that is already styled.
func grid(width, columns, gap int, blocks ...string) string {
	if columns < 1 {
		columns = 1
	}
	var rows []string
	for start := 0; start < len(blocks); start += columns {
		end := min(start+columns, len(blocks))
		rows = append(rows, joinRow(gap, blocks[start:end]...))
	}
	return strings.Join(rows, "\n")
}

// columnWidth is the width one cell of an n-column grid gets.
func columnWidth(width, columns, gap int) int {
	if columns < 1 {
		columns = 1
	}
	return (width - gap*(columns-1)) / columns
}
