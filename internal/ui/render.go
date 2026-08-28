package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Rendering primitives that know about escape sequences.
//
// Everything drawn here is already coloured by the time it is measured, cut or
// padded, so len() and slicing are both wrong: they count escape bytes as
// columns and cut sequences in half. Every helper in this file goes through
// ansi.* instead, which is the difference between a table that lines up and one
// that drifts a column further right on every coloured cell.

// visWidth is the printed width of an already-styled string.
func visWidth(text string) int { return ansi.StringWidth(text) }

// truncate cuts to a column count, adding an ellipsis only when it actually cut.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if visWidth(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(text, width, "…")
}

func padRight(text string, width int) string {
	if gap := width - visWidth(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

func padLeft(text string, width int) string {
	if gap := width - visWidth(text); gap > 0 {
		return strings.Repeat(" ", gap) + text
	}
	return text
}

// fit truncates and pads in one step, which is what every cell in every table
// actually wants.
func fit(text string, width int) string { return padRight(truncate(text, width), width) }

// fitRight is fit for numeric columns.
func fitRight(text string, width int) string { return padLeft(truncate(text, width), width) }

// wrapText re-flows prose to a column width, breaking on spaces and falling
// back to a hard break for words that do not fit at all (long URLs, sysctl
// names) rather than letting them push the panel border off screen.
func wrapText(text string, width int) string {
	if width < 8 {
		width = 8
	}
	return ansi.Wrap(text, width, " -/")
}

// wrapIndent wraps and indents every line after the first, so a hint reads as
// one paragraph attached to its bullet instead of a new column of text.
func wrapIndent(text string, width int, indent string) string {
	wrapped := wrapText(text, width)
	return strings.ReplaceAll(wrapped, "\n", "\n"+indent)
}

func splitLines(block string) []string { return strings.Split(block, "\n") }

func blockWidth(block string) int {
	widest := 0
	for _, line := range splitLines(block) {
		if width := visWidth(line); width > widest {
			widest = width
		}
	}
	return widest
}

func blockHeight(block string) int {
	if block == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

// clampBlock forces a block to an exact height by padding or cutting, so the
// frame never changes height between pages. An interface that grows and shrinks
// by a row as you move between tabs reads as unstable even when it is correct.
func clampBlock(block string, height int) string {
	lines := splitLines(block)
	if len(lines) > height {
		lines = lines[:max(height, 0)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// overlay draws one block on top of another at a column and row, keeping the
// background visible around it.
//
// lipgloss.Place cannot do this: it composes onto blank space, so anything
// drawn over the screen erases the screen. A dialog that hides the numbers you
// are being asked about is a dialog nobody can answer, which is why this exists.
func overlay(background, foreground string, column, row int) string {
	bgLines := splitLines(background)
	fgLines := splitLines(foreground)
	for index, fgLine := range fgLines {
		target := row + index
		if target < 0 || target >= len(bgLines) {
			continue
		}
		bgLine := bgLines[target]
		bgWidth := visWidth(bgLine)
		fgWidth := visWidth(fgLine)

		left := ""
		switch {
		case column <= 0:
			left = ""
		case bgWidth >= column:
			left = ansi.Truncate(bgLine, column, "")
		default:
			left = bgLine + strings.Repeat(" ", column-bgWidth)
		}
		right := ""
		if bgWidth > column+fgWidth {
			right = ansi.TruncateLeft(bgLine, column+fgWidth, "")
		}
		bgLines[target] = left + fgLine + right
	}
	return strings.Join(bgLines, "\n")
}

// overlayCentre drops a block into the middle of the screen with a soft drop
// shadow. The shadow is not decoration: it separates a floating surface from
// the text behind it in a terminal that has no blur and no transparency.
func overlayCentre(background, foreground string, screenWidth, screenHeight int) string {
	width, height := blockWidth(foreground), blockHeight(foreground)
	column := max((screenWidth-width)/2, 0)
	row := max((screenHeight-height)/2, 0)

	shadowLine := sShadow.Render(strings.Repeat(" ", width))
	shadow := strings.TrimSuffix(strings.Repeat(shadowLine+"\n", height), "\n")
	out := overlay(background, shadow, column+1, row+1)
	return overlay(out, foreground, column, row)
}

// joinRow places blocks side by side with a fixed gap, top-aligned.
func joinRow(gap int, blocks ...string) string {
	kept := make([]string, 0, len(blocks)*2)
	spacer := strings.Repeat(" ", max(gap, 0))
	for index, block := range blocks {
		if block == "" {
			continue
		}
		if index > 0 && gap > 0 {
			kept = append(kept, spacer)
		}
		kept = append(kept, block)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, kept...)
}

// joinCol stacks blocks, dropping empty ones so a missing section leaves no gap.
func joinCol(blocks ...string) string {
	kept := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		kept = append(kept, block)
	}
	return lipgloss.JoinVertical(lipgloss.Left, kept...)
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func orStar(value string) string {
	if value == "" {
		return "*"
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(value, low, high int) int { return max(low, min(value, high)) }
