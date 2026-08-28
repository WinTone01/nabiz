package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

// The component library.
//
// Every surface in the program is built from these, and nothing outside this
// file draws a border or picks a padding. That is the only way twelve screens
// written at different times end up looking like one application: when a panel
// is a call rather than a convention, it cannot drift.

// --- cards -----------------------------------------------------------------

// card is the standard surface: a rounded box whose title sits in the top
// border and whose right corner can carry a status badge. Anything with a
// heading gets one, so no screen is ever a wall of text with a rule through it.
type card struct {
	title  string
	badge  string                 // already styled; sits at the right of the title border
	width  int                    // total outer width
	accent lipgloss.TerminalColor // border colour override, for severity
}

func (c card) render(body string) string {
	if c.width < 8 {
		c.width = 8
	}
	var border lipgloss.TerminalColor = colLine
	titleStyle := sHeading
	if c.accent != nil {
		border = c.accent
		titleStyle = lipgloss.NewStyle().Foreground(c.accent).Bold(true)
	}
	edge := lipgloss.NewStyle().Foreground(border)
	inner := c.width - 4

	var b strings.Builder
	b.WriteString(c.topBorder(edge, titleStyle, inner))
	b.WriteString("\n")
	for _, line := range splitLines(body) {
		b.WriteString(edge.Render("│") + " " + fit(line, inner) + " " + edge.Render("│") + "\n")
	}
	b.WriteString(edge.Render("╰" + strings.Repeat("─", c.width-2) + "╯"))
	return b.String()
}

func (c card) topBorder(edge, titleStyle lipgloss.Style, inner int) string {
	if c.title == "" {
		return edge.Render("╭" + strings.Repeat("─", c.width-2) + "╮")
	}
	title := truncate(c.title, max(inner-2, 4))
	left := edge.Render("╭─ ") + titleStyle.Render(title) + " "
	right := edge.Render("╮")
	rightWidth := 1
	if c.badge != "" {
		badge := truncate(c.badge, max(inner-visWidth(title)-6, 0))
		if badge != "" {
			right = " " + badge + edge.Render(" ─╮")
			rightWidth = visWidth(badge) + 4
		}
	}
	fill := c.width - 3 - visWidth(title) - 1 - rightWidth
	return left + edge.Render(strings.Repeat("─", max(fill, 0))) + right
}

// panel is the one-liner most callers want.
func panel(title string, width int, body string) string {
	return card{title: title, width: width}.render(body)
}

// tile is a headline number: a label, the figure itself, and one line of
// context under it. Three of them across the top of a page answer "is anything
// wrong" before the reader has parsed a single table.
func tile(label, value string, valueStyle lipgloss.Style, note string, width int) string {
	inner := width - 4
	body := strings.Join([]string{
		sEyebrow.Render(fit(i18n.Upper(label), inner)),
		valueStyle.Bold(true).Render(fit(value, inner)),
		sFaint.Render(fit(note, inner)),
	}, "\n")
	return card{width: width}.render(body)
}

// --- text bits --------------------------------------------------------------

// kv renders one aligned "label   value" line, the workhorse of every detail
// panel. The label column is fixed by the caller so a panel reads as a table
// even though it is a list.
func kv(key, value string, style lipgloss.Style, keyWidth int) string {
	return sMuted.Render(padRight(key, keyWidth)) + " " + style.Render(value)
}

// kvList is a detail panel that sizes its own label column.
//
// A fixed width is a bug waiting for a translation: "retransmit" is ten columns
// in English and sixteen in Turkish, and a hardcoded fourteen turns the second
// one into a table with one row out of line. Measuring the labels that are
// actually present costs nothing and is right in every language.
type kvList struct {
	rows []kvEntry
}

type kvEntry struct {
	key, value string
	style      lipgloss.Style
}

func (l *kvList) add(key, value string) {
	l.rows = append(l.rows, kvEntry{key: key, value: value, style: sText})
}

func (l *kvList) addStyled(key, value string, style lipgloss.Style) {
	l.rows = append(l.rows, kvEntry{key: key, value: value, style: style})
}

// addRaw takes a value that is already styled per character.
func (l *kvList) addRaw(key, value string) {
	l.rows = append(l.rows, kvEntry{key: key, value: value, style: lipgloss.NewStyle()})
}

// addRule draws a separator inside the list, for panels that cover two things.
func (l *kvList) addRule(width int) {
	l.rows = append(l.rows, kvEntry{key: "\x00rule", value: strings.Repeat("╌", max(width, 4))})
}

// addHead is a bold sub-heading inside the list.
func (l *kvList) addHead(text string) {
	l.rows = append(l.rows, kvEntry{key: "\x00head", value: text})
}

func (l *kvList) render(maxKey int) string {
	keyWidth := 0
	for _, row := range l.rows {
		if strings.HasPrefix(row.key, "\x00") {
			continue
		}
		keyWidth = max(keyWidth, visWidth(row.key))
	}
	keyWidth = min(keyWidth, maxKey)

	lines := make([]string, 0, len(l.rows))
	for _, row := range l.rows {
		switch row.key {
		case "\x00rule":
			lines = append(lines, sLine.Render(row.value))
		case "\x00head":
			lines = append(lines, row.value)
		default:
			lines = append(lines, sMuted.Render(fit(row.key, keyWidth))+" "+
				row.style.Render(row.value))
		}
	}
	return strings.Join(lines, "\n")
}

// kvBlock is the one-shot form for plain label/value pairs.
func kvBlock(rows [][2]string, maxKey int) string {
	var list kvList
	for _, row := range rows {
		list.add(row[0], row[1])
	}
	return list.render(maxKey)
}

// emptyState is the single "nothing here yet" voice. Every page uses it so the
// answer to "why is this blank" is always in the same place and the same tone.
func emptyState(key string) string {
	return sFaint.Render("  " + i18n.T(key))
}

// --- badges and status ------------------------------------------------------

// pill is a filled label. Used for counts and states that need to read as one
// object rather than as two words that happen to be adjacent.
func pill(text string, foreground, background lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Foreground(foreground).Background(background).
		Bold(true).Padding(0, 1).Render(text)
}

// statusDot is the compact "installed and running" indicator used in the header
// and in card badges.
func statusDot(name string, running bool) string {
	if running {
		return sOK.Render("●") + " " + sText.Render(name)
	}
	return sWarn.Render("○") + " " + sMuted.Render(name)
}

func runningTag(running bool) string {
	if running {
		return sOK.Render("● " + i18n.T("ui.running"))
	}
	return sWarn.Render("○ " + i18n.T("ui.stopped"))
}

// bpftuneTag reports the unit the way BpftuneState describes it. Failed,
// stopped on purpose and merely stopped are three different situations and
// only the first is a warning.
func bpftuneTag(state sysinfo.BpftuneState) string {
	switch {
	case state.Failed:
		return sBad.Render("!! " + state.Label())
	case state.StoppedByNabiz:
		return sMuted.Render("○ " + state.Label())
	case state.Running:
		return sOK.Render("● " + state.Label())
	default:
		return sWarn.Render("○ " + state.Label())
	}
}

func boolText(value bool) string {
	if value {
		return i18n.T("ui.yes")
	}
	return i18n.T("ui.no")
}

// --- charts -----------------------------------------------------------------

// sparkline colours losses red and everything else green, per character, so a
// dropped packet is visible in a chart eight columns wide.
func sparkline(samples []stats.Sample, width int) string {
	raw := stats.Sparkline(samples, width)
	var b strings.Builder
	for _, glyph := range raw {
		if glyph == '!' {
			b.WriteString(sBad.Render("!"))
			continue
		}
		b.WriteString(sOK.Render(string(glyph)))
	}
	return b.String()
}

// meter is a proportional bar with sub-character resolution, drawn on a track
// so an empty meter still reads as a meter rather than as missing data.
func meter(fraction float64, width int, style lipgloss.Style) string {
	// stats.Bar pads its own tail with spaces; the track is drawn instead, so a
	// bar at zero still reads as a bar rather than as a missing value.
	filled := strings.TrimRight(stats.Bar(fraction, width), " ")
	trail := max(width-visWidth(filled), 0)
	return style.Render(filled) + sLine.Render(strings.Repeat("╌", trail))
}

// scrollbar shows both position and how much of the content is on screen, so a
// long page announces its length instead of surprising the reader with it.
func scrollbar(height int, offsetFraction, visibleFraction float64, focused bool) string {
	if height <= 0 {
		return ""
	}
	thumb := clamp(int(visibleFraction*float64(height)+0.5), 1, height)
	top := clamp(int(offsetFraction*float64(height-thumb)+0.5), 0, height-thumb)
	// The thumb takes the accent only while the pane has focus, so the accent
	// keeps meaning "this is what the arrow keys are pointed at".
	thumbStyle := sEdge
	if focused {
		thumbStyle = sAcc
	}
	var b strings.Builder
	for row := 0; row < height; row++ {
		if row >= top && row < top+thumb {
			b.WriteString(thumbStyle.Render("┃"))
		} else {
			b.WriteString(sLine.Render("│"))
		}
		if row < height-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// --- tables -----------------------------------------------------------------

// col describes one column. A zero width means "take what is left", shared
// between every flexible column, which is how one table definition survives a
// terminal that is 80 columns wide and one that is 200.
type col struct {
	title string
	width int
	right bool
}

// cell pairs text with the style it renders in, so a red loss figure can sit
// next to a neutral average without the caller pre-rendering escape codes into
// a column width and getting the alignment wrong.
type cell struct {
	text  string
	style lipgloss.Style
	// raw marks content already styled per character - a sparkline - which must
	// not be truncated by rune count or restyled, because both would cut into
	// the escape sequences.
	raw bool
}

func plain(text string) cell                    { return cell{text: text, style: sText} }
func styled(text string, s lipgloss.Style) cell { return cell{text: text, style: s} }
func dim(text string) cell                      { return cell{text: text, style: sFaint} }
func rawCell(text string) cell                  { return cell{text: text, raw: true} }
func numf(format string, args ...any) cell      { return plain(fmt.Sprintf(format, args...)) }

// renderTable draws an aligned table with a ruled header. These tables are read
// rather than navigated, so there is no selection highlight: a highlighted row
// you cannot act on is a lie about the interface.
func renderTable(width int, cols []col, rows [][]cell) string {
	widths := resolveWidths(width, cols)

	var b strings.Builder
	if labelled(cols) {
		head := make([]string, len(cols))
		for index, column := range cols {
			if column.right {
				head[index] = fitRight(column.title, widths[index])
				continue
			}
			head[index] = fit(column.title, widths[index])
		}
		b.WriteString(sColHead.Render(strings.Join(head, " ")) + "\n")
		b.WriteString(sLine.Render(strings.Repeat("╌",
			min(sum(widths)+len(widths)-1, width))) + "\n")
	}

	for _, row := range rows {
		parts := make([]string, len(cols))
		for index, column := range cols {
			if index >= len(row) {
				parts[index] = strings.Repeat(" ", widths[index])
				continue
			}
			current := row[index]
			if current.raw {
				parts[index] = padRight(current.text, widths[index])
				continue
			}
			if column.right {
				parts[index] = current.style.Render(fitRight(current.text, widths[index]))
				continue
			}
			parts[index] = current.style.Render(fit(current.text, widths[index]))
		}
		b.WriteString(strings.Join(parts, " ") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// labelled reports whether a table has headings at all. Some of them are plain
// two-column layouts - a key sheet, a definition list - and a blank header row
// with a rule under it is furniture around nothing.
func labelled(cols []col) bool {
	for _, column := range cols {
		if column.title != "" {
			return true
		}
	}
	return false
}

// resolveWidths shares the leftover columns between the flexible ones and, when
// even the fixed columns do not fit, shrinks the widest ones one at a time
// rather than letting the table run off the edge of the panel.
func resolveWidths(width int, cols []col) []int {
	widths := make([]int, len(cols))
	fixed, flexible := 0, 0
	for index, column := range cols {
		widths[index] = column.width
		if column.width == 0 {
			flexible++
			continue
		}
		fixed += column.width
	}
	spare := width - fixed - max(len(cols)-1, 0)

	if flexible > 0 {
		share := max(spare/flexible, 6)
		remainder := max(spare-share*flexible, 0)
		for index, column := range cols {
			if column.width != 0 {
				continue
			}
			widths[index] = share
			if remainder > 0 {
				widths[index]++
				remainder--
			}
		}
		return widths
	}
	for spare < 0 {
		widest, at := 0, -1
		for index, current := range widths {
			if current > widest {
				widest, at = current, index
			}
		}
		if at < 0 || widest <= 4 {
			break
		}
		widths[at]--
		spare++
	}
	return widths
}

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

// --- clickable controls ------------------------------------------------------

// Bubble Tea reports mouse coordinates but has no idea what lives at them, so
// every control marks itself with a named zone at render time and asks that
// zone whether a click landed inside it. The id is the whole contract between
// View and Update.

const (
	btnPrimary = iota
	btnGhost
	btnDanger
	btnSuccess
)

// button is a single row, filled. Bordered buttons are three rows tall, and a
// toolbar three rows tall costs more of the screen than the actions are worth.
func button(id, label string, kind int, enabled bool) string {
	base := lipgloss.NewStyle().Padding(0, 2).Bold(true)
	var style lipgloss.Style
	switch {
	case !enabled:
		style = base.Foreground(colFaint).Background(colSurf).Bold(false)
	case kind == btnPrimary:
		style = base.Foreground(colInvert).Background(colAccent)
	case kind == btnSuccess:
		style = base.Foreground(colInvert).Background(colOK)
	case kind == btnDanger:
		style = base.Foreground(colBad).Background(colSurf2)
	default:
		style = base.Foreground(colText).Background(colSurf2).Bold(false)
	}
	return zone.Mark(id, style.Render(label))
}

// checkbox is the approval surface for advice: nothing is applied that was not
// ticked here first.
func checkbox(id, label string, checked, enabled bool) string {
	box, style := "[ ]", sMuted
	if checked {
		box, style = "[x]", sAcc
	}
	if !enabled {
		style = sFaint
	}
	text := box
	if label != "" {
		text += " " + label
	}
	return zone.Mark(id, style.Render(text))
}

// chip is one option in a segmented control.
func chip(id, label string, active bool) string {
	style := lipgloss.NewStyle().Padding(0, 1).Foreground(colMuted).Background(colSurf)
	if active {
		style = style.Foreground(colInvert).Background(colAccent).Bold(true)
	}
	return zone.Mark(id, style.Render(label))
}

// segmented lays chips out as one control, which is what a suite picker is: a
// row of mutually exclusive options, not seven separate buttons.
func segmented(ids, labels []string, active int) string {
	parts := make([]string, 0, len(ids))
	for index := range ids {
		parts = append(parts, chip(ids[index], labels[index], index == active))
	}
	return strings.Join(parts, " ")
}

// toolbar spaces controls on one row. Two cells rather than one: filled buttons
// sitting a single space apart read as one striped block instead of separate
// things you can press.
func toolbar(controls ...string) string {
	kept := make([]string, 0, len(controls))
	for _, control := range controls {
		if control != "" {
			kept = append(kept, control)
		}
	}
	return strings.Join(kept, "  ")
}

// clickableRow marks a whole line so a list can be driven with the mouse.
func clickableRow(id, content string) string { return zone.Mark(id, content) }
