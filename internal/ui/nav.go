package ui

import (
	"fmt"
	"strings"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// Navigation.
//
// Twelve screens is too many for a tab strip and exactly right for a grouped
// sidebar: the groups say what kind of thing each screen is, which is the part a
// row of twelve equal chips cannot tell you. On a terminal too narrow to spend
// twenty columns on navigation the same list degrades to a strip of shortcuts,
// because the numbers are the real navigation and the labels are the reminder.

type pageID int

const (
	pageOverview pageID = iota
	pageMonitor
	pageTest
	pageLayers
	pageDNS
	pageKernel
	pageBpftune
	pageUnwall
	pageAdvice
	pageHistory
	pageReports
	pageHelp
)

const sidebarWidth = 22

type navGroup int

const (
	groupLive navGroup = iota
	groupTest
	groupSystem
	groupResults
	groupMeta
)

func groupLabel(group navGroup) string {
	switch group {
	case groupLive:
		return i18n.T("nav.group.live")
	case groupTest:
		return i18n.T("nav.group.test")
	case groupSystem:
		return i18n.T("nav.group.system")
	case groupResults:
		return i18n.T("nav.group.results")
	}
	return ""
}

type navItem struct {
	id    pageID
	key   string
	group navGroup
	label string
}

// navItems is the single source of truth for page order, shortcuts and titles.
// The page ring, the sidebar, the narrow strip and the command palette all read
// it, so a screen cannot exist in one of them and be missing from another.
func navItems() []navItem {
	return []navItem{
		{pageOverview, "1", groupLive, i18n.T("nav.overview")},
		{pageMonitor, "2", groupLive, i18n.T("nav.monitor")},

		{pageTest, "3", groupTest, i18n.T("nav.test")},
		{pageLayers, "4", groupTest, i18n.T("nav.layers")},
		{pageDNS, "5", groupTest, "DNS"},

		{pageKernel, "6", groupSystem, i18n.T("nav.kernel.short")},
		{pageBpftune, "7", groupSystem, "bpftune"},
		{pageUnwall, "8", groupSystem, i18n.T("nav.unwall.short")},

		{pageAdvice, "9", groupResults, i18n.T("nav.advice")},
		{pageHistory, "0", groupResults, i18n.T("nav.history")},
		{pageReports, "p", groupResults, i18n.T("nav.reports")},

		// H rather than ?: the question mark is the universal key for a key
		// sheet, and it opens the overlay from anywhere. This is the manual.
		{pageHelp, "H", groupMeta, i18n.T("nav.help")},
	}
}

type nav struct {
	items   []navItem
	active  int
	cursor  int
	focused bool
}

func newNav() *nav {
	return &nav{items: navItems()}
}

func (n *nav) rebuild() {
	n.items = navItems()
}

func (n *nav) current() navItem { return n.items[n.active] }
func (n *nav) currentID() pageID {
	return n.items[n.active].id
}

func (n *nav) selectIndex(index int) {
	if index < 0 || index >= len(n.items) {
		return
	}
	n.active, n.cursor = index, index
}

func (n *nav) selectID(id pageID) {
	for index, item := range n.items {
		if item.id == id {
			n.selectIndex(index)
			return
		}
	}
}

// step moves through the page ring and takes the selection with it, which is
// what ctrl+n means everywhere else in the world.
func (n *nav) step(delta int) {
	count := len(n.items)
	n.selectIndex(((n.active+delta)%count + count) % count)
}

// moveCursor walks the sidebar without switching page. The page changes on
// enter, so arrowing past ten screens does not fire ten reloads.
func (n *nav) moveCursor(delta int) {
	count := len(n.items)
	n.cursor = ((n.cursor+delta)%count + count) % count
}

func navZone(id pageID) string { return fmt.Sprintf("nav:%d", id) }

// badges are the counters that make the sidebar worth looking at rather than
// just clicking through: unread problems, essentially.
func (n *nav) badge(a *App, id pageID) (string, bool) {
	switch id {
	case pageAdvice:
		// dismissed advice is not shown on the page, so counting it here left
		// the badge promising items the list does not have
		if result := a.LastRun(); result != nil {
			open := 0
			for _, advice := range result.Advice {
				if !a.Dismissed(advice.ID) {
					open++
				}
			}
			if open > 0 {
				return fmt.Sprint(open), true
			}
		}
	case pageMonitor:
		if a.Watcher != nil {
			if outages := len(a.Watcher.Snapshot().Outages); outages > 0 {
				return fmt.Sprint(outages), false
			}
		}
	case pageTest:
		if result := a.LastRun(); result != nil {
			if bad := result.CountFindings("bad"); bad > 0 {
				return fmt.Sprint(bad), false
			}
		}
	}
	return "", false
}

// viewSidebar draws the grouped list. Group headings are not clickable and are
// styled so they cannot be mistaken for items.
func (n *nav) viewSidebar(a *App, height int) string {
	inner := sidebarWidth - 2
	var lines []string
	lastGroup := navGroup(-1)
	for index, item := range n.items {
		if item.group != lastGroup {
			if lastGroup != navGroup(-1) {
				lines = append(lines, "")
			}
			if label := groupLabel(item.group); label != "" {
				lines = append(lines, " "+sEyebrow.Render(fit(i18n.Upper(label), inner-1)))
			}
			lastGroup = item.group
		}
		lines = append(lines, n.renderItem(a, index, item, inner))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	body := strings.Join(lines[:min(len(lines), height)], "\n")

	edge := sLine
	if n.focused {
		edge = sAcc
	}
	var out strings.Builder
	for index, line := range splitLines(body) {
		out.WriteString(padRight(line, sidebarWidth-1) + edge.Render("│"))
		if index < height-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

func (n *nav) renderItem(a *App, index int, item navItem, inner int) string {
	badgeText, urgent := n.badge(a, item.id)
	label := item.label
	room := inner - 5
	if badgeText != "" {
		room -= visWidth(badgeText) + 1
	}
	label = fit(label, max(room, 4))

	var line string
	switch {
	case index == n.active:
		content := sSelected.Render(" " + padRight(item.key, 2) + label + " ")
		line = sAcc.Render("▎") + content
	case n.focused && index == n.cursor:
		line = sAcc.Render("▎") + sHovered.Render(" "+sFaint.Render(padRight(item.key, 2))+sText.Render(label)+" ")
	default:
		line = "  " + sFaint.Render(padRight(item.key, 2)) + sMuted.Render(label) + " "
	}
	if badgeText != "" {
		style := sInfo
		if urgent {
			style = sAcc
		}
		if index == n.active {
			style = sSelected
		}
		line += style.Render(badgeText)
	}
	return padRight(clickableRow(navZone(item.id), line), inner)
}

// viewStrip is the narrow-terminal fallback. It gives up labels in two stages:
// first every label but the current one, then all of them. Keeping the active
// label as long as possible means the strip still says where you are, which is
// the one thing a row of bare numbers cannot.
func (n *nav) viewStrip(width int) string {
	for _, mode := range []int{stripAll, stripActive, stripKeys} {
		strip := n.strip(mode)
		if visWidth(strip) <= width {
			return strip
		}
	}
	return truncate(n.strip(stripKeys), width)
}

const (
	stripAll = iota
	stripActive
	stripKeys
)

func (n *nav) strip(mode int) string {
	parts := make([]string, 0, len(n.items))
	for index, item := range n.items {
		label := item.key
		if mode == stripAll || (mode == stripActive && index == n.active) {
			label += " " + item.label
		}
		parts = append(parts, chip(navZone(item.id), label, index == n.active))
	}
	return strings.Join(parts, " ")
}
