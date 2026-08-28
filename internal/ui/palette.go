package ui

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// The command palette.
//
// Twelve screens and a dozen actions is past the point where a key legend can
// carry the whole interface. ctrl+k lists everything the program can do, by
// name, in the current language, filtered as you type - which also makes it the
// honest answer to "what else does this thing do", and the reason the toolbar
// does not have to grow a button for every feature.

type command struct {
	id      string
	title   string
	section string
	hint    string // the key that does the same thing
	enabled bool
	run     func() tea.Cmd
}

type palette struct {
	input    textinput.Model
	commands []command
	matches  []int
	cursor   int
	// offset is the first match drawn by the last render. A click reports the
	// row it landed on, and without this the ninth visible row would select the
	// ninth command rather than the ninth one currently on screen.
	offset int
	open   bool
}

func newPalette() *palette {
	input := textinput.New()
	input.Prompt = "› "
	input.PromptStyle = sAcc
	input.TextStyle = sText
	input.PlaceholderStyle = sFaint
	input.Cursor.Style = sAcc
	input.CharLimit = 48
	return &palette{input: input}
}

func (p *palette) show(commands []command) {
	p.commands = commands
	p.open = true
	p.cursor = 0
	p.input.Placeholder = i18n.T("ui.palette.placeholder")
	p.input.SetValue("")
	p.input.Focus()
	p.filter()
}

func (p *palette) hide() {
	p.open = false
	p.input.Blur()
}

func (p *palette) move(delta int) {
	if len(p.matches) == 0 {
		return
	}
	p.cursor = clamp(p.cursor+delta, 0, len(p.matches)-1)
}

func (p *palette) selected() *command {
	if p.cursor < 0 || p.cursor >= len(p.matches) {
		return nil
	}
	return &p.commands[p.matches[p.cursor]]
}

// update feeds the text field and re-filters. Navigation keys are handled by
// the caller so enter and the arrows never reach the input.
func (p *palette) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	before := p.input.Value()
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.cursor = 0
		p.filter()
	}
	return cmd
}

func (p *palette) filter() {
	query := strings.ToLower(strings.TrimSpace(p.input.Value()))
	type scored struct {
		index, score int
	}
	var found []scored
	for index, item := range p.commands {
		score, ok := fuzzyScore(strings.ToLower(item.title+" "+item.section), query)
		if !ok {
			continue
		}
		if !item.enabled {
			score -= 50
		}
		found = append(found, scored{index, score})
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].score > found[b].score })
	p.matches = p.matches[:0]
	for _, item := range found {
		p.matches = append(p.matches, item.index)
	}
	p.cursor = clamp(p.cursor, 0, max(len(p.matches)-1, 0))
}

// fuzzyScore is a subsequence match that rewards matching the start of a word,
// so "ba" finds "set baseline" before it finds "database".
//
// Both sides are compared as runes rather than bytes: the command titles are
// Turkish half the time, and a byte-wise scan would never match a query that
// starts with "ç" or "ö".
func fuzzyScore(text, query string) (int, bool) {
	if query == "" {
		return 0, true
	}
	haystack := []rune(text)
	score, at := 0, 0
	previousMatch := -2
	for _, want := range query {
		if want == ' ' {
			continue
		}
		found := -1
		for index := at; index < len(haystack); index++ {
			if haystack[index] != want {
				continue
			}
			found = index
			break
		}
		if found < 0 {
			return 0, false
		}
		switch {
		case found == 0:
			score += 12
		case found == previousMatch+1:
			score += 8
		case !unicode.IsLetter(haystack[found-1]):
			score += 10
		default:
			score++
		}
		previousMatch, at = found, found+1
	}
	return score, true
}

func paletteZone(row int) string { return "pal:" + strconv.Itoa(row) }

// clickedIndex maps a press onto the match a visible row stands for.
func (p *palette) clickedIndex(msg tea.MouseMsg, visible int) int {
	for row := 0; row < visible; row++ {
		if clicked(msg, paletteZone(row)) {
			return p.offset + row
		}
	}
	return -1
}

const paletteRows = 9

func (p *palette) view(screenWidth int) string {
	width := clamp(screenWidth-20, 46, 72)
	inner := width - 4

	rows := make([]string, 0, paletteRows+2)
	rows = append(rows, p.input.View(), sLine.Render(strings.Repeat("╌", inner)))

	if len(p.matches) == 0 {
		rows = append(rows, sFaint.Render("  "+i18n.T("ui.palette.empty")))
	}
	start := max(p.cursor-paletteRows+1, 0)
	p.offset = start
	shown := 0
	for offset, index := range p.matches[min(start, len(p.matches)):] {
		if shown >= paletteRows {
			break
		}
		item := p.commands[index]
		rows = append(rows, p.renderRow(offset, item, start+offset == p.cursor, inner))
		shown++
	}
	if extra := len(p.matches) - start - shown; extra > 0 {
		rows = append(rows, sFaint.Render("  "+i18n.T("ui.more", extra)))
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return card{title: i18n.T("ui.palette.title"), width: width, accent: colAccent}.render(body)
}

func (p *palette) renderRow(row int, item command, selected bool, inner int) string {
	// Both states are laid out first and styled second, so the selected row is
	// exactly as wide as the others and the list does not shift as it moves.
	tail := item.section
	if item.hint != "" {
		tail += "  " + item.hint
	}
	titleRoom := max(inner-visWidth(tail)-4, 8)
	tailRoom := max(inner-titleRoom-4, 1)
	line := " " + fit(item.title, titleRoom) + "  " + padLeft(tail, tailRoom) + " "

	switch {
	case selected:
		line = sSelected.Render(line)
	case !item.enabled:
		line = sFaint.Render(line)
	default:
		line = sText.Render(" "+fit(item.title, titleRoom)) + "  " +
			sFaint.Render(padLeft(tail, tailRoom)) + " "
	}
	return clickableRow(paletteZone(row), line)
}
