package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// bpftune edits sysctls on its own, so /etc/sysctl.d stops describing the
// running system. This page shows what its journal actually recorded and判
// judges those values against the bandwidth-delay product we measured, because
// "bigger buffer" and "better" are not the same thing on a home line.
type bpftunePage struct {
	viewport      viewport.Model
	width, height int
}

func newBpftunePage() Page { return &bpftunePage{} }

func (p *bpftunePage) ID() tabID        { return tabBpftune }
func (p *bpftunePage) ABTarget() string { return "bpftune" }

func (p *bpftunePage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *bpftunePage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *bpftunePage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *bpftunePage) SuiteName(*App) string { return "load" }

func (p *bpftunePage) View(a *App) string { return p.viewport.View() }

func (p *bpftunePage) body(a *App) string {
	state := a.Env.bpftune
	var b strings.Builder
	b.WriteString(sSection.Render("bpftune") + "\n")
	b.WriteString(sDim.Render(wrap(i18n.T("help.bpftune.intro"), p.width-2)) + "\n\n")
	if !state.Installed {
		b.WriteString(emptyState("ui.notinstalled") + "\n")
		return b.String()
	}
	b.WriteString("  " + kv(i18n.T("f.status"), "", sText, 16) + runningTag(state.Running) + "\n")
	b.WriteString("  " + kv(i18n.T("f.version"),
		util.Truncate(state.Version, p.width-24), sText, 16) + "\n")
	b.WriteString("  " + kv(i18n.T("f.autostart"), boolText(state.Enabled), sText, 16) + "\n\n")

	rtt := liveRTT(a)
	if bdp := sysinfo.BDPBytes(a.Env.link.SpeedMbit, rtt); bdp > 0 {
		b.WriteString("  " + kv(i18n.T("f.bdp"), fmt.Sprintf("%d Mbit × %.0f ms = %s",
			a.Env.link.SpeedMbit, rtt, util.HumanBytes(bdp)), sBold, 24) + "\n")
		b.WriteString("  " + sFaint.Render(i18n.T("help.bpftune.bdp")) + "\n\n")
	}

	if len(state.Changes) > 0 {
		cols := []column{
			{title: i18n.T("col.tunable"), width: 38},
			{title: i18n.T("col.from"), width: 22},
			{title: i18n.T("col.to"), width: 22},
			{title: i18n.T("col.times"), width: 5, right: true},
		}
		var rows [][]cell
		for _, change := range state.Changes {
			rows = append(rows, []cell{
				styled(change.Tunable, sAcc), dim(change.From),
				plain(change.To), numf("%d", change.Count),
			})
		}
		b.WriteString(sSection.Render(i18n.T("panel.changes")) + "\n" +
			renderTable(cols, rows))
		for _, change := range state.Changes {
			if change.Reason != "" {
				b.WriteString("  " + sFaint.Render(change.Tunable+": "+change.Reason) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if len(state.CCVotes) > 0 {
		b.WriteString(sSection.Render(i18n.T("panel.cc")) + "\n")
		total := 0
		names := make([]string, 0, len(state.CCVotes))
		for name, count := range state.CCVotes {
			total += count
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			count := state.CCVotes[name]
			share := 0.0
			if total > 0 {
				share = float64(count) / float64(total)
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n", padRight(name, 10),
				meter(share, 26, sAcc),
				sText.Render(fmt.Sprintf("%d  (%.0f%%)", count, share*100))))
		}
		b.WriteString("\n")
	}

	b.WriteString(sSection.Render(i18n.T("sec.assessment")) + "\n")
	for _, note := range sysinfo.AssessBpftune(state, a.Env.link.SpeedMbit, rtt,
		a.Env.health.RetransPct) {
		b.WriteString(fmt.Sprintf(" %s %s\n",
			levelStyle(note.Level).Render(levelMark(note.Level)),
			sText.Render(wrapIndent(note.Text, p.width-6, "   "))))
		if note.Hint != "" {
			b.WriteString("   " + sFaint.Render(wrapIndent(note.Hint, p.width-8, "   ")) + "\n")
		}
	}
	b.WriteString("\n" + sFaint.Render("a → "+i18n.T("key.ab")+"    r → "+i18n.T("key.run")) + "\n")
	if a.AB != nil && a.AB.target == "bpftune" {
		b.WriteString("\n" + abTable(a, "bpftune ON", "bpftune OFF"))
	}
	return b.String()
}

func liveRTT(a *App) float64 {
	if a.Watcher == nil {
		return 0
	}
	for _, target := range a.Watcher.Snapshot().Targets {
		if target.Label != "Modem / Gateway" && target.Avg > 0 {
			return target.Avg
		}
	}
	return 0
}

func boolText(value bool) string {
	if value {
		return i18n.T("ui.yes")
	}
	return i18n.T("ui.no")
}

// abTable renders an A/B comparison; shared by the bpftune and Unwall pages.
func abTable(a *App, labelA, labelB string) string {
	if a.AB == nil {
		return ""
	}
	cols := []column{
		{title: i18n.T("col.metric"), width: 24},
		{title: labelA, width: 15, right: true},
		{title: labelB, width: 15, right: true},
		{title: i18n.T("col.delta"), width: 12, right: true},
	}
	var rows [][]cell
	for _, row := range report.DiffRows(a.AB.before, a.AB.after) {
		style := sFaint
		if row.Significant() {
			style = sBad
			if row.Better() {
				style = sOK
			}
		}
		rows = append(rows, []cell{
			plain(row.Metric),
			numf("%.2f %s", row.Before, row.Unit),
			numf("%.2f %s", row.After, row.Unit),
			styled(fmt.Sprintf("%+.2f", row.Delta), style),
		})
	}
	return sSection.Render(i18n.T("misc.ab_result")) + "\n" + renderTable(cols, rows)
}
