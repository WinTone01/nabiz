package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// The page that separates a driver regression from a broken cable. Boot times
// and kernel versions come from wtmp, which outlives journal rotation; the drop
// counts come from the kernel log, so older boots honestly read "no coverage"
// instead of quietly reporting zero.
type kernelPage struct {
	viewport      viewport.Model
	width, height int
}

func newKernelPage() Page { return &kernelPage{} }

func (p *kernelPage) ID() tabID { return tabKernel }

func (p *kernelPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *kernelPage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *kernelPage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *kernelPage) View(a *App) string { return p.viewport.View() }

func (p *kernelPage) body(a *App) string {
	history := a.Env.linkLog
	dropStyle := sOK
	if history.Drops > 0 {
		dropStyle = sBad
	}
	summary := []string{
		sDim.Render(wrap(i18n.T("help.kernel.intro"), p.width-6)),
		"",
		kv(i18n.T("col.kernelv"), a.Env.kernelNow, sBold, 18),
	}
	if history.Available {
		summary = append(summary, kv(i18n.T("col.drops"),
			fmt.Sprintf("%d  (%.0f min, %.0f s)", history.Drops,
				history.SpanMinutes(), history.DownSeconds), dropStyle, 18))
		if history.Downshifts > 0 {
			summary = append(summary, kv("downshift", fmt.Sprintf("%d x %s",
				history.Downshifts, history.DownshiftNote), sBad, 18))
		}
	}

	var perKernel string
	if len(a.Env.kernels) > 0 {
		cols := []column{
			{title: "", width: 1},
			{title: i18n.T("col.kernelv"), width: 24},
			{title: i18n.T("col.boot"), width: 6, right: true},
			{title: i18n.T("col.hours"), width: 9, right: true},
			{title: i18n.T("misc.measured"), width: 16, right: true},
			{title: i18n.T("col.drops"), width: 8, right: true},
			{title: i18n.T("col.perhour"), width: 8, right: true},
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
				marker = ">"
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
		perKernel = strings.TrimRight(renderTable(cols, rows), "\n")
	}

	var boots string
	if len(a.Env.reboots) > 0 {
		cols := []column{
			{title: "", width: 1},
			{title: i18n.T("col.kernelv"), width: 24},
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
				marker = ">"
			}
			rows = append(rows, []cell{
				styled(marker, sAcc), plain(reboot.Kernel),
				plain(reboot.At.Format("2006-01-02 15:04")),
				plain(util.ShortDuration(reboot.Uptime)),
			})
		}
		boots = strings.TrimRight(renderTable(cols, rows), "\n")
		if len(a.Env.reboots) > 10 {
			boots += "\n" + sFaint.Render(i18n.T("ui.more", len(a.Env.reboots)-10))
		}
	}

	var log []string
	if events := a.Env.linkLog.Events; len(events) > 0 {
		if len(events) > 16 {
			events = events[len(events)-16:]
		}
		for _, event := range events {
			style, text := sBad, "link down"
			if event.Kind == "up" {
				style, text = sOK, "link up "+event.Speed
				if event.Downshifted {
					style, text = sWarn, text+" (downshifted)"
				}
			}
			log = append(log, sFaint.Render(event.At.Format("15:04:05"))+" "+style.Render(text))
		}
	}

	return stack(p.width,
		sectionSpec{i18n.T("nav.kernel"), strings.Join(summary, "\n")},
		sectionSpec{i18n.T("panel.stability"), perKernel},
		sectionSpec{i18n.T("panel.boots"), boots},
		sectionSpec{i18n.T("panel.log"), strings.Join(log, "\n")})
}
