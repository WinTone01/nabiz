package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/util"
)

// cell pairs a value with the style it should be rendered in, so a table row
// can carry per-cell meaning (a red loss figure next to a neutral average)
// without the caller pre-rendering escape codes into column widths.
type cell struct {
	text  string
	style lipgloss.Style
	// pre marks content that is already styled per character (a sparkline, for
	// instance). Such a string must not be truncated by rune count or restyled,
	// because both operations would cut into the escape sequences and the cell
	// would render one column too wide.
	pre bool
}

func plain(text string) cell                    { return cell{text: text, style: sText} }
func styled(text string, s lipgloss.Style) cell { return cell{text: text, style: s} }
func dim(text string) cell                      { return cell{text: text, style: sFaint} }
func rendered(text string) cell                 { return cell{text: text, pre: true} }
func numf(format string, args ...any) cell      { return plain(fmt.Sprintf(format, args...)) }

// column describes one column of a lightweight table.
type column struct {
	title string
	width int
	right bool
}

// renderTable draws an aligned, coloured table. It is deliberately not
// bubbles/table: these tables are read, not navigated, and a selection
// highlight on a row you cannot act on is a lie about the interface.
func renderTable(cols []column, rows [][]cell) string {
	var b strings.Builder
	var header []string
	for _, col := range cols {
		title := util.Truncate(col.title, col.width)
		if col.right {
			header = append(header, padLeft(title, col.width))
			continue
		}
		header = append(header, padRight(title, col.width))
	}
	b.WriteString(sHead.Render(strings.Join(header, " ")) + "\n")
	for _, row := range rows {
		var parts []string
		for index, col := range cols {
			if index >= len(row) {
				parts = append(parts, strings.Repeat(" ", col.width))
				continue
			}
			current := row[index]
			if current.pre {
				parts = append(parts, padRight(current.text, col.width))
				continue
			}
			text := util.Truncate(current.text, col.width)
			if col.right {
				parts = append(parts, current.style.Render(padLeft(text, col.width)))
				continue
			}
			parts = append(parts, current.style.Render(padRight(text, col.width)))
		}
		b.WriteString(strings.Join(parts, " ") + "\n")
	}
	return b.String()
}

// newDataTable builds a scrollable bubbles/table for the pages where the rows
// are numerous enough that scrolling within the page beats scrolling the page.
func newDataTable(cols []table.Column, rows []table.Row, height int) table.Model {
	model := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(max(height, 3)),
		table.WithStyles(staticTableStyles()),
	)
	model.Blur()
	return model
}

// findingsBlock renders a findings list with hints.
func findingsBlock(findings []suite.Finding, width int) string {
	var b strings.Builder
	for _, finding := range findings {
		b.WriteString(fmt.Sprintf(" %s %s  %s\n",
			levelStyle(finding.Level).Render(levelMark(finding.Level)),
			sBold.Render(padRight(util.Truncate(finding.Key, 22), 22)),
			sText.Render(util.Truncate(finding.Title, max(width-28, 20)))))
		if finding.Hint != "" {
			b.WriteString("   " + sFaint.Render(wrapIndent(finding.Hint, width-8, "   ")) + "\n")
		}
	}
	return b.String()
}

// adviceBlock renders one advice item in full.
func adviceBlock(advice suite.Advice, width int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(" %s %s\n",
		sInfo.Render("["+suite.CategoryLabel(advice.Category)+"]"),
		sBold.Render(advice.Title)))
	b.WriteString("   " + sText.Render(wrapIndent(advice.Why, width-8, "   ")) + "\n")
	for _, step := range advice.How {
		if suite.IsCommand(step) {
			b.WriteString("     " + sFaint.Render("$ ") + sAcc.Render(step) + "\n")
			continue
		}
		b.WriteString("     " + sFaint.Render("• ") +
			sText.Render(wrapIndent(step, width-10, "       ")) + "\n")
	}
	if advice.Gain != "" {
		b.WriteString("   " + sOK.Render("→ ") +
			sText.Render(wrapIndent(advice.Gain, width-8, "     ")) + "\n")
	}
	if advice.Risk != "" {
		b.WriteString("   " + sWarn.Render("! ") +
			sText.Render(wrapIndent(advice.Risk, width-8, "     ")) + "\n")
	}
	if len(advice.Revert) > 0 {
		b.WriteString("   " + sFaint.Render(i18n.T("misc.revert")+": "+
			strings.Join(advice.Revert, " ; ")) + "\n")
	}
	return b.String()
}

// scoreLine is the headline of any result.
func scoreLine(result suite.Result) string {
	line := scoreStyle(result.Score).Render(
		i18n.T("misc.score_line", result.Score, result.Grade)) +
		sFaint.Render(fmt.Sprintf("   %s · %.0f s · %s", result.Name, result.Duration,
			result.StartedAt.Format("15:04:05")))
	if result.Baseline != nil {
		delta := result.Score - result.Baseline.Score
		style := sOK
		if delta < 0 {
			style = sBad
		}
		line += sDim.Render("   "+i18n.T("misc.vs_baseline")+" ") +
			style.Render(fmt.Sprintf("%+.1f", delta))
	}
	return line
}

// emptyState is the consistent "nothing here yet" line.
func emptyState(key string) string { return sFaint.Render(i18n.T(key)) }

// sectionSpec is one titled block on a page.
type sectionSpec struct {
	title string
	body  string
}

// stack renders sections as titled panels, one under the other. Every page is
// built from these so no screen is a wall of text with a rule through it: if
// something has a heading, it has a border.
func stack(width int, items ...sectionSpec) string {
	var parts []string
	for _, item := range items {
		if strings.TrimSpace(item.body) == "" {
			continue
		}
		parts = append(parts, panel(item.title, width, item.body))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// kvBlock renders aligned key/value lines as a panel body.
func kvBlock(keyWidth int, rows [][2]string) string {
	var lines []string
	for _, row := range rows {
		lines = append(lines, kv(row[0], row[1], sText, keyWidth))
	}
	return strings.Join(lines, "\n")
}
