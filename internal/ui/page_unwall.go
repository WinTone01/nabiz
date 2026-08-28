package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

// Unwall / zapret sits in the packet path and rewrites what leaves the machine.
// When it is on, "the site does not load" and "the site loads slowly" have
// completely different causes, and this page is where the difference is
// visible: what the engine is doing, and what the DPI scan found.
type unwallPage struct {
	body          scroller
	width, height int
}

func newUnwallPage() Page { return &unwallPage{body: newScroller()} }

func (p *unwallPage) ID() pageID            { return pageUnwall }
func (p *unwallPage) ABTarget() string      { return "unwall" }
func (p *unwallPage) SuiteName(*App) string { return "dpi" }
func (p *unwallPage) Focus(focused bool)    { p.body.focused = focused }

func (p *unwallPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *unwallPage) Reload(a *App)                      { p.body.setContent(p.render(a)) }
func (p *unwallPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *unwallPage) View(a *App) string                 { return p.body.view() }

func (p *unwallPage) render(a *App) string {
	width := p.body.contentWidth()
	inner := width - 4
	state := a.Env.unwall
	if !state.Installed {
		return stack(width, section{title: "Unwall", body: emptyState("ui.notinstalled")})
	}
	var list kvList
	list.addRaw(i18n.T("f.status"), runningTag(state.Running))
	list.add(i18n.T("f.engine"), state.Engine+" · "+state.Strategy)
	list.add(i18n.T("f.hostlist"), fmt.Sprintf("%s · %d / %d", state.HostlistMode,
		state.HostlistN, state.AutoHostlistN))
	list.add(i18n.T("f.ports"), fmt.Sprintf("TCP %s · UDP %s · queue %d",
		state.PortsTCP, state.PortsUDP, state.QNum))
	list.add(i18n.T("f.gatewaymode"), boolText(state.GatewayMode))
	list.add(i18n.T("f.dns"), dnsText(state))
	list.addStyled(i18n.T("f.nfqueue"), nfqText(a), nfqStyle(a))
	status := list.render(22)

	scan := emptyState("ui.press_run")
	badge := ""
	if result, ok := a.Results["dpi"]; ok && len(result.DPI) > 0 {
		scan = dpiTable(result.DPI, inner)
		badge = dpiBadge(result.DPI)
	}

	sections := []section{
		{title: "Unwall — zapret / DPI", badge: runningTag(state.Running), body: status},
		{title: i18n.T("sec.dpi"), badge: badge, body: scan},
		{title: i18n.T("help.sec.reading"),
			body: sMuted.Render(wrapText(i18n.T("help.dpi.explainer"), inner))},
	}
	if a.AB != nil && a.AB.target == "unwall" {
		sections = append(sections, section{title: i18n.T("misc.ab_result"),
			body: abRows(a, "zapret ON", "zapret OFF", inner)})
	}
	return stack(width, sections...)
}

// dpiBadge counts the domains that are not simply reachable, which is the one
// number the scan exists to produce.
func dpiBadge(verdicts []probe.DomainVerdict) string {
	blocked := 0
	for _, verdict := range verdicts {
		if verdict.Verdict != "clean" {
			blocked++
		}
	}
	if blocked == 0 {
		return sOK.Render(fmt.Sprintf("✓ %d", len(verdicts)))
	}
	return sWarn.Render(fmt.Sprintf("%d / %d", blocked, len(verdicts)))
}

func dnsText(state sysinfo.UnwallState) string {
	if !state.DNSEncrypted {
		return i18n.T("ui.off")
	}
	return state.DNSBackend + " / " + state.DNSProvider
}
