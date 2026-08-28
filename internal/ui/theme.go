package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// The palette is one set of semantic tokens, not a set of colours. Nothing in
// the interface picks "purple"; it picks accent, or bad, or line. That is what
// makes a light terminal and a dark terminal both look deliberate, and it is
// what stops a new panel from inventing a new shade of grey.
//
// Colour carries meaning and only meaning: red is a fault, amber is worth a
// look, green is confirmed healthy, violet is "you can act on this". A screen
// with no colour is a screen with nothing wrong.
var (
	colText   = lipgloss.AdaptiveColor{Light: "#24242e", Dark: "#e2e2ee"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#5c5c70", Dark: "#9a9ab4"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#9494a8", Dark: "#5f5f78"}
	colLine   = lipgloss.AdaptiveColor{Light: "#dcdce8", Dark: "#2b2b3d"}
	colEdge   = lipgloss.AdaptiveColor{Light: "#c0c0d2", Dark: "#3f3f58"}
	colSurf   = lipgloss.AdaptiveColor{Light: "#f2f2f8", Dark: "#191922"}
	colSurf2  = lipgloss.AdaptiveColor{Light: "#e8e8f2", Dark: "#22222e"}
	colInvert = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#12121a"}

	colAccent = lipgloss.AdaptiveColor{Light: "#6d28d9", Dark: "#b79cff"}
	colCyan   = lipgloss.AdaptiveColor{Light: "#0e7490", Dark: "#61d4ec"}
	colOK     = lipgloss.AdaptiveColor{Light: "#12803c", Dark: "#5fdf90"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#8a6200", Dark: "#f3c552"}
	colBad    = lipgloss.AdaptiveColor{Light: "#b31f28", Dark: "#ff7378"}
	colInfo   = lipgloss.AdaptiveColor{Light: "#2456b0", Dark: "#87b6ff"}
)

// Text styles. Every one of them is foreground-only: backgrounds are applied by
// the widget that owns the surface, never by the text inside it, so a value can
// be reused on a panel, in a tile and inside a selected row without fighting.
var (
	sText  = lipgloss.NewStyle().Foreground(colText)
	sBold  = lipgloss.NewStyle().Foreground(colText).Bold(true)
	sMuted = lipgloss.NewStyle().Foreground(colMuted)
	sFaint = lipgloss.NewStyle().Foreground(colFaint)
	sOK    = lipgloss.NewStyle().Foreground(colOK)
	sWarn  = lipgloss.NewStyle().Foreground(colWarn)
	sBad   = lipgloss.NewStyle().Foreground(colBad)
	sInfo  = lipgloss.NewStyle().Foreground(colInfo)
	sAcc   = lipgloss.NewStyle().Foreground(colAccent)
	sCyan  = lipgloss.NewStyle().Foreground(colCyan)
	sLine  = lipgloss.NewStyle().Foreground(colLine)
	sEdge  = lipgloss.NewStyle().Foreground(colEdge)

	// Headings. Section headings are cyan because the accent is reserved for
	// things that respond to a click; a title that looks clickable and is not
	// is the cheapest way to make an interface feel unreliable.
	sHeading = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	sEyebrow = lipgloss.NewStyle().Foreground(colFaint).Bold(true)
	sColHead = lipgloss.NewStyle().Foreground(colMuted)

	sBrand = lipgloss.NewStyle().Foreground(colInvert).Background(colAccent).
		Bold(true).Padding(0, 1)

	sSurface  = lipgloss.NewStyle().Background(colSurf)
	sSelected = lipgloss.NewStyle().Foreground(colInvert).Background(colAccent).Bold(true)
	sHovered  = lipgloss.NewStyle().Foreground(colText).Background(colSurf2)
	sShadow   = lipgloss.NewStyle().Background(colSurf2)
)

// levelStyle maps a finding level onto its colour. Every severity in the whole
// program funnels through here so "warn" is the same amber everywhere.
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
	}
	return sText
}

// levelMark is the glyph that goes with the colour, so severity survives a
// monochrome terminal and a colour-blind reader.
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

// deltaStyle grades added latency under load: the numbers are milliseconds of
// bufferbloat, where 30 is noticeable in a call and 100 ruins it.
func deltaStyle(delta float64) lipgloss.Style {
	switch {
	case delta >= 100:
		return sBad
	case delta >= 30:
		return sWarn
	}
	return sOK
}

// lossStyle is the one place that decides what counts as a bad loss figure.
func lossStyle(pct float64) lipgloss.Style {
	switch {
	case pct >= 2:
		return sBad
	case pct >= 0.5:
		return sWarn
	}
	return sOK
}
