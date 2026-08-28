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
	if !state.Installed {
		return stack(p.width, sectionSpec{"bpftune", emptyState("ui.notinstalled")})
	}
	rtt := liveRTT(a)
	bdp := sysinfo.BDPBytes(a.Env.link.SpeedMbit, rtt)

	status := []string{
		sDim.Render(wrap(i18n.T("help.bpftune.intro"), p.width-6)),
		"",
		sDim.Render(padRight(i18n.T("f.status"), 16)) + " " + runningTag(state.Running),
		kv(i18n.T("f.version"), util.Truncate(state.Version, p.width-28), sText, 16),
		kv(i18n.T("f.autostart"), boolText(state.Enabled), sText, 16),
	}
	if bdp > 0 {
		status = append(status, "",
			kv(i18n.T("f.bdp"), fmt.Sprintf("%d Mbit x %.0f ms = %s",
				a.Env.link.SpeedMbit, rtt, util.HumanBytes(bdp)), sBold, 16),
			sFaint.Render(i18n.T("help.bpftune.bdp")))
	}

	var changes string
	if len(state.Changes) > 0 {
		cols := []column{
			{title: i18n.T("col.tunable"), width: 36},
			{title: i18n.T("col.from"), width: 20},
			{title: i18n.T("col.to"), width: 20},
			{title: i18n.T("col.times"), width: 5, right: true},
		}
		var rows [][]cell
		for _, change := range state.Changes {
			rows = append(rows, []cell{
				styled(change.Tunable, sAcc), dim(change.From),
				plain(change.To), numf("%d", change.Count),
			})
		}
		changes = strings.TrimRight(renderTable(cols, rows), "\n")
		for _, change := range state.Changes {
			if change.Reason != "" {
				changes += "\n" + sFaint.Render(change.Tunable+": "+change.Reason)
			}
		}
	}

	var congestion string
	if len(state.CCVotes) > 0 {
		total := 0
		names := make([]string, 0, len(state.CCVotes))
		for name, count := range state.CCVotes {
			total += count
			names = append(names, name)
		}
		sort.Strings(names)
		var lines []string
		for _, name := range names {
			count := state.CCVotes[name]
			share := 0.0
			if total > 0 {
				share = float64(count) / float64(total)
			}
			lines = append(lines, fmt.Sprintf("%s %s %s", padRight(name, 10),
				meter(share, 26, sAcc),
				sText.Render(fmt.Sprintf("%d  (%.0f%%)", count, share*100))))
		}
		congestion = strings.Join(lines, "\n")
	}

	var assessment []string
	for _, note := range sysinfo.AssessBpftune(state, a.Env.link.SpeedMbit, rtt,
		a.Env.health.RetransPct) {
		assessment = append(assessment, fmt.Sprintf("%s %s",
			levelStyle(note.Level).Render(levelMark(note.Level)),
			sText.Render(wrapIndent(note.Text, p.width-10, "  "))))
		if note.Hint != "" {
			assessment = append(assessment,
				"  "+sFaint.Render(wrapIndent(note.Hint, p.width-12, "  ")))
		}
	}

	specs := []sectionSpec{
		{"bpftune", strings.Join(status, "\n")},
		{i18n.T("panel.changes"), changes},
		{i18n.T("panel.cc"), congestion},
		{i18n.T("sec.assessment"), strings.Join(assessment, "\n")},
	}
	if a.AB != nil && a.AB.target == "bpftune" {
		specs = append(specs, sectionSpec{i18n.T("misc.ab_result"),
			strings.TrimRight(abRows(a, "bpftune ON", "bpftune OFF"), "\n")})
	}
	return stack(p.width, specs...)
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

// abRows renders the A/B comparison body; the caller wraps it in a panel.
func abRows(a *App, labelA, labelB string) string {
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
	return renderTable(cols, rows)
}
