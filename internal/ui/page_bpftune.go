package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// bpftune edits sysctls on its own, so /etc/sysctl.d stops describing the
// running system. This page shows what its journal actually recorded and judges
// those values against the bandwidth-delay product we measured, because "bigger
// buffer" and "better" are not the same thing on a home line.
type bpftunePage struct {
	body          scroller
	width, height int
}

func newBpftunePage() Page { return &bpftunePage{body: newScroller()} }

func (p *bpftunePage) ID() pageID            { return pageBpftune }
func (p *bpftunePage) ABTarget() string      { return "bpftune" }
func (p *bpftunePage) SuiteName(*App) string { return "load" }
func (p *bpftunePage) Focus(focused bool)    { p.body.focused = focused }

func (p *bpftunePage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *bpftunePage) Reload(a *App)                      { p.body.setContent(p.render(a)) }
func (p *bpftunePage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *bpftunePage) View(a *App) string                 { return p.body.view() }

func (p *bpftunePage) render(a *App) string {
	width := p.body.contentWidth()
	inner := width - 4
	state := a.Env.bpftune
	if !state.Installed {
		return stack(width, section{title: "bpftune", body: emptyState("ui.notinstalled")})
	}
	rtt := liveRTT(a)
	bdp := sysinfo.BDPBytes(a.Env.link.SpeedMbit, rtt)

	var list kvList
	list.addRaw(i18n.T("f.status"), runningTag(state.Running))
	list.add(i18n.T("f.version"), truncate(state.Version, inner-22))
	list.add(i18n.T("f.autostart"), boolText(state.Enabled))
	if bdp > 0 {
		list.addStyled(i18n.T("f.bdp"), fmt.Sprintf("%d Mbit × %.0f ms = %s",
			a.Env.link.SpeedMbit, rtt, util.HumanBytes(bdp)), sBold)
	}
	status := []string{
		sMuted.Render(wrapText(i18n.T("help.bpftune.intro"), inner)),
		"",
		list.render(20),
	}
	if bdp > 0 {
		status = append(status, "", sFaint.Render(wrapText(i18n.T("help.bpftune.bdp"), inner)))
	}

	sections := []section{
		{title: "bpftune", badge: runningTag(state.Running),
			body: strings.Join(status, "\n")},
		{title: i18n.T("panel.changes"), badge: fmt.Sprint(len(state.Changes)),
			body: p.changes(state, inner)},
		{title: i18n.T("panel.cc"), body: p.congestion(state, inner)},
		{title: i18n.T("sec.assessment"), body: p.assessment(a, rtt, inner)},
	}
	if a.AB != nil && a.AB.target == "bpftune" {
		sections = append(sections, section{title: i18n.T("misc.ab_result"),
			body: abRows(a, "bpftune ON", "bpftune OFF", inner)})
	}
	return stack(width, sections...)
}

func (p *bpftunePage) changes(state sysinfo.BpftuneState, width int) string {
	if len(state.Changes) == 0 {
		return ""
	}
	cols := []col{
		{title: i18n.T("col.tunable"), width: 0},
		{title: i18n.T("col.from"), width: 20, right: true},
		{title: i18n.T("col.to"), width: 20, right: true},
		{title: i18n.T("col.times"), width: 6, right: true},
	}
	var rows [][]cell
	for _, change := range state.Changes {
		rows = append(rows, []cell{
			styled(change.Tunable, sAcc), dim(change.From),
			plain(change.To), numf("%d", change.Count),
		})
	}
	body := renderTable(width, cols, rows)
	reasons := ""
	for _, change := range state.Changes {
		if change.Reason != "" {
			reasons += "\n" + sFaint.Render(wrapIndent(change.Tunable+": "+change.Reason,
				width, "  "))
		}
	}
	if reasons != "" {
		body += "\n" + reasons
	}
	return body
}

// congestion shows which algorithm bpftune actually picked, per connection.
// A single global setting is a guess; the vote is what happened.
func (p *bpftunePage) congestion(state sysinfo.BpftuneState, width int) string {
	if len(state.CCVotes) == 0 {
		return ""
	}
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
		lines = append(lines, fmt.Sprintf("%s %s %s", padRight(name, 12),
			meter(share, min(max(width-32, 8), 30), sAcc),
			sText.Render(fmt.Sprintf("%d  (%.0f%%)", count, share*100))))
	}
	return strings.Join(lines, "\n")
}

func (p *bpftunePage) assessment(a *App, rtt float64, width int) string {
	var lines []string
	for _, note := range sysinfo.AssessBpftune(a.Env.bpftune, a.Env.link.SpeedMbit, rtt,
		a.Env.health.RetransPct) {
		lines = append(lines, fmt.Sprintf("%s %s",
			levelStyle(note.Level).Render(levelMark(note.Level)),
			sText.Render(wrapIndent(note.Text, width-2, "  "))))
		if note.Hint != "" {
			lines = append(lines, "  "+sFaint.Render(wrapIndent(note.Hint, width-4, "  ")))
		}
	}
	return strings.Join(lines, "\n")
}
