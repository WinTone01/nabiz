// Package ui is the Bubble Tea interface.
//
// The layout is a frame (header, tab bar, body, footer) around one Page at a
// time. Pages own their own components - tables, viewports, lists - so each one
// scrolls and selects on its own terms instead of everything being flattened
// into a single pre-rendered string.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/stats"
)

// One adaptive palette. Colour carries meaning: red is a fault, amber is worth
// a look, green is confirmed healthy. Everything else stays neutral so a screen
// with no colour is a screen with no problems.
var (
	cText   = lipgloss.AdaptiveColor{Light: "#2c2c38", Dark: "#d6d6e6"}
	cDim    = lipgloss.AdaptiveColor{Light: "#6f6f82", Dark: "#8b8ba4"}
	cFaint  = lipgloss.AdaptiveColor{Light: "#9b9bae", Dark: "#5c5c74"}
	cBrand  = lipgloss.AdaptiveColor{Light: "#6d28d9", Dark: "#b79cff"}
	cBrand2 = lipgloss.AdaptiveColor{Light: "#0e7490", Dark: "#61d4ec"}
	cOK     = lipgloss.AdaptiveColor{Light: "#14803f", Dark: "#5fdf90"}
	cWarn   = lipgloss.AdaptiveColor{Light: "#8a6200", Dark: "#f3c552"}
	cBad    = lipgloss.AdaptiveColor{Light: "#b31f28", Dark: "#ff7378"}
	cInfo   = lipgloss.AdaptiveColor{Light: "#2456b0", Dark: "#87b6ff"}
	cLine   = lipgloss.AdaptiveColor{Light: "#d5d5e2", Dark: "#32324a"}
	cInvert = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#12121a"}
	cPanel  = lipgloss.AdaptiveColor{Light: "#f2f2f8", Dark: "#1a1a26"}
)

var (
	sText  = lipgloss.NewStyle().Foreground(cText)
	sDim   = lipgloss.NewStyle().Foreground(cDim)
	sFaint = lipgloss.NewStyle().Foreground(cFaint)
	sBold  = lipgloss.NewStyle().Foreground(cText).Bold(true)
	sOK    = lipgloss.NewStyle().Foreground(cOK)
	sWarn  = lipgloss.NewStyle().Foreground(cWarn)
	sBad   = lipgloss.NewStyle().Foreground(cBad)
	sInfo  = lipgloss.NewStyle().Foreground(cInfo)
	sAcc   = lipgloss.NewStyle().Foreground(cBrand)
	sAcc2  = lipgloss.NewStyle().Foreground(cBrand2)

	sBrandTag = lipgloss.NewStyle().Bold(true).
			Foreground(cInvert).Background(cBrand).Padding(0, 1)

	sTabOn = lipgloss.NewStyle().Bold(true).
		Foreground(cInvert).Background(cBrand).Padding(0, 1)
	sTabOff = lipgloss.NewStyle().Foreground(cDim).Padding(0, 1)

	sFrame = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(cLine)

	sPanel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(cLine).Padding(0, 1)

	sPanelTitle = lipgloss.NewStyle().Bold(true).Foreground(cBrand2)
	sSection    = lipgloss.NewStyle().Bold(true).Foreground(cBrand2)
	sHead       = lipgloss.NewStyle().Bold(true).Foreground(cDim)
	sRule       = lipgloss.NewStyle().Foreground(cLine)

	sChipOn = lipgloss.NewStyle().Bold(true).
		Foreground(cInvert).Background(cBrand).Padding(0, 1)
	sChipOff = lipgloss.NewStyle().Foreground(cDim).Background(cPanel).Padding(0, 1)

	sOverlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(cBrand).Padding(1, 2)
)

// tableStyles gives every bubbles/table the same look.
func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(cLine).
		BorderBottom(true).
		Bold(true).
		Foreground(cDim)
	styles.Selected = styles.Selected.
		Foreground(cInvert).
		Background(cBrand).
		Bold(true)
	styles.Cell = styles.Cell.Foreground(cText)
	return styles
}

// staticTableStyles is for tables that are read, not navigated: no selection
// highlight, because a highlighted row implies you can act on it.
func staticTableStyles() table.Styles {
	styles := tableStyles()
	styles.Selected = lipgloss.NewStyle().Foreground(cText)
	return styles
}

func levelStyle(level string) lipgloss.Style {
	switch level {
	case "bad", "critical":
		return sBad
	case "warn":
		return sWarn
	case "ok":
		return sOK
	case "info":
		return sInfo
	default:
		return sText
	}
}

func levelMark(level string) string {
	switch level {
	case "bad", "critical":
		return "●"
	case "warn":
		return "▲"
	case "ok":
		return "✓"
	case "info":
		return "·"
	}
	return " "
}

func scoreStyle(score float64) lipgloss.Style {
	switch {
	case score >= 80:
		return sOK
	case score >= 60:
		return sWarn
	}
	return sBad
}

func deltaStyle(delta float64) lipgloss.Style {
	switch {
	case delta >= 100:
		return sBad
	case delta >= 30:
		return sWarn
	}
	return sOK
}

// --- small widgets --------------------------------------------------------

func padRight(text string, width int) string {
	if gap := width - lipgloss.Width(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

func padLeft(text string, width int) string {
	if gap := width - lipgloss.Width(text); gap > 0 {
		return strings.Repeat(" ", gap) + text
	}
	return text
}

// panel is a titled rounded box sized to an outer width.
func panel(title string, outerWidth int, body string) string {
	inner := outerWidth - 4
	if inner < 10 {
		inner = 10
	}
	return sPanel.Width(inner).Render(sPanelTitle.Render(title) + "\n" + body)
}

// kv renders an aligned "key   value" line.
func kv(key, value string, style lipgloss.Style, keyWidth int) string {
	return sDim.Render(padRight(key, keyWidth)) + " " + style.Render(value)
}

// rule draws a labelled separator.
func rule(label string, width int) string {
	if label == "" {
		return sRule.Render(strings.Repeat("─", max(width, 0)))
	}
	prefix := sAcc.Render("── " + label + " ")
	return prefix + sRule.Render(strings.Repeat("─", max(width-lipgloss.Width(prefix), 0)))
}

// sparkline renders losses in red and everything else in green.
func sparkline(samples []stats.Sample, width int) string {
	raw := stats.Sparkline(samples, width)
	var b strings.Builder
	for _, r := range raw {
		if r == '!' {
			b.WriteString(sBad.Render("!"))
			continue
		}
		b.WriteString(sOK.Render(string(r)))
	}
	return b.String()
}

// meter is a proportional bar for distributions.
func meter(fraction float64, width int, style lipgloss.Style) string {
	return style.Render(stats.Bar(fraction, width))
}

// wrap re-flows prose, with an optional hanging indent.
func wrap(text string, width int) string { return wrapIndent(text, width, "") }

func wrapIndent(text string, width int, indent string) string {
	if width < 24 {
		width = 24
	}
	var lines []string
	current := ""
	for _, word := range strings.Fields(text) {
		switch {
		case current == "":
			current = word
		case len(current)+1+len(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n"+indent)
}

func orDash(value string) string {
	if value == "" {
		return "-"
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
