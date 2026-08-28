package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// The page that separates a driver regression from a broken cable.
//
// Boot times and kernel versions come from wtmp, which outlives journal
// rotation; the drop counts come from the kernel log, so older boots honestly
// read "no coverage" instead of quietly reporting zero. A tool that reports an
// unmeasured period as clean is worse than one that reports nothing.
type kernelPage struct {
	body          scroller
	width, height int
}

func newKernelPage() Page { return &kernelPage{body: newScroller()} }

func (p *kernelPage) ID() pageID         { return pageKernel }
func (p *kernelPage) Focus(focused bool) { p.body.focused = focused }

func (p *kernelPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *kernelPage) Reload(a *App)                      { p.body.setContent(p.render(a)) }
func (p *kernelPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *kernelPage) View(a *App) string                 { return p.body.view() }

func (p *kernelPage) render(a *App) string {
	width := p.body.contentWidth()
	inner := width - 4
	history := a.Env.linkLog

	dropStyle := sOK
	if history.Drops > 0 {
		dropStyle = sBad
	}
	var list kvList
	list.addStyled(i18n.T("col.kernelv"), a.Env.kernelNow, sBold)
	if history.Available {
		list.addStyled(i18n.T("col.drops"), fmt.Sprintf("%d  (%.0f min, %.0f s)",
			history.Drops, history.SpanMinutes(), history.DownSeconds), dropStyle)
		if history.Downshifts > 0 {
			list.addStyled("downshift", fmt.Sprintf("%d × %s",
				history.Downshifts, history.DownshiftNote), sBad)
		}
	}
	summary := []string{
		sMuted.Render(wrapText(i18n.T("help.kernel.intro"), inner)),
		"",
		list.render(20),
	}

	return stack(width,
		section{title: i18n.T("nav.kernel"), badge: p.badge(a),
			body: strings.Join(summary, "\n")},
		section{title: i18n.T("panel.stability"), body: p.perKernel(a, inner)},
		section{title: i18n.T("panel.boots"), body: p.boots(a, inner)},
		section{title: i18n.T("panel.log"), body: p.log(a)})
}

func (p *kernelPage) badge(a *App) string {
	if !a.Env.linkLog.Available {
		return sFaint.Render(i18n.T("misc.unmeasured"))
	}
	if a.Env.linkLog.Drops > 0 {
		return sBad.Render(fmt.Sprintf("● %d", a.Env.linkLog.Drops))
	}
	return sOK.Render("✓")
}

// perKernel is the regression detector: drops per hour, per kernel release,
// with the unmeasured boots greyed out rather than counted as clean.
func (p *kernelPage) perKernel(a *App, width int) string {
	if len(a.Env.kernels) == 0 {
		return ""
	}
	cols := []col{
		{title: "", width: 1},
		// The release name is given room but not the whole card: the numbers to
		// its right are the ones that decide whether this is a regression.
		{title: i18n.T("col.kernelv"), width: clamp(width-58, 18, 32)},
		{title: i18n.T("col.boot"), width: 6, right: true},
		{title: i18n.T("col.hours"), width: 9, right: true},
		{title: i18n.T("misc.measured"), width: 16, right: true},
		{title: i18n.T("col.drops"), width: 8, right: true},
		{title: i18n.T("col.perhour"), width: 9, right: true},
	}
	var rows [][]cell
	for _, entry := range a.Env.kernels {
		style := sText
		switch {
		case !entry.Measured:
			style = sFaint
		case entry.DropsPerHr >= 1:
			style = sBad
		case entry.Drops == 0:
			style = sOK
		}
		marker := ""
		if entry.Kernel == a.Env.kernelNow {
			marker = "▸"
		}
		measured := i18n.T("misc.unmeasured")
		if entry.Measured {
			measured = fmt.Sprintf("%.1f h", entry.CoveredHrs)
		}
		rows = append(rows, []cell{
			styled(marker, sAcc), plain(entry.Kernel),
			numf("%d", entry.Boots), numf("%.1f", entry.Hours),
			dim(measured),
			styled(fmt.Sprint(entry.Drops), style),
			styled(fmt.Sprintf("%.1f", entry.DropsPerHr), style),
		})
	}
	return renderTable(width, cols, rows)
}

func (p *kernelPage) boots(a *App, width int) string {
	if len(a.Env.reboots) == 0 {
		return ""
	}
	cols := []col{
		{title: "", width: 1},
		{title: i18n.T("col.kernelv"), width: clamp(width-34, 18, 32)},
		{title: i18n.T("col.when"), width: 18},
		{title: i18n.T("f.uptime"), width: 10, right: true},
	}
	var rows [][]cell
	for index, reboot := range a.Env.reboots {
		if index >= 10 {
			break
		}
		marker := ""
		if reboot.Current {
			marker = "▸"
		}
		rows = append(rows, []cell{
			styled(marker, sAcc), plain(reboot.Kernel),
			plain(reboot.At.Format("2006-01-02 15:04")),
			plain(util.ShortDuration(reboot.Uptime)),
		})
	}
	body := renderTable(width, cols, rows)
	if len(a.Env.reboots) > 10 {
		body += "\n" + sFaint.Render(i18n.T("ui.more", len(a.Env.reboots)-10))
	}
	return body
}

func (p *kernelPage) log(a *App) string {
	events := a.Env.linkLog.Events
	if len(events) == 0 {
		return ""
	}
	if len(events) > 16 {
		events = events[len(events)-16:]
	}
	var lines []string
	for _, event := range events {
		style, text := sBad, "link down"
		if event.Kind == "up" {
			style, text = sOK, "link up "+event.Speed
			if event.Downshifted {
				style, text = sWarn, text+" (downshifted)"
			}
		}
		lines = append(lines, sFaint.Render(event.At.Format("15:04:05"))+" "+style.Render(text))
	}
	return strings.Join(lines, "\n")
}
