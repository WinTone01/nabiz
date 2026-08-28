package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

type unwallPage struct {
	viewport      viewport.Model
	width, height int
}

func newUnwallPage() Page { return &unwallPage{} }

func (p *unwallPage) ID() tabID        { return tabUnwall }
func (p *unwallPage) ABTarget() string { return "unwall" }

func (p *unwallPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *unwallPage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *unwallPage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *unwallPage) SuiteName(*App) string { return "dpi" }

func (p *unwallPage) View(a *App) string { return p.viewport.View() }

func (p *unwallPage) body(a *App) string {
	state := a.Env.unwall
	if !state.Installed {
		return stack(p.width, sectionSpec{"Unwall", emptyState("ui.notinstalled")})
	}
	status := []string{
		sDim.Render(padRight(i18n.T("f.status"), 18)) + " " + runningTag(state.Running),
		kv(i18n.T("f.engine"), state.Engine+" · "+state.Strategy, sText, 18),
		kv(i18n.T("f.hostlist"), fmt.Sprintf("%s · %d / %d", state.HostlistMode,
			state.HostlistN, state.AutoHostlistN), sText, 18),
		kv(i18n.T("f.ports"), fmt.Sprintf("TCP %s · UDP %s · queue %d",
			state.PortsTCP, state.PortsUDP, state.QNum), sText, 18),
		kv(i18n.T("f.gatewaymode"), boolText(state.GatewayMode), sText, 18),
		kv(i18n.T("f.dns"), dnsText(state), sText, 18),
		kv(i18n.T("f.nfqueue"), nfqText(a), nfqStyle(a), 18),
	}

	scan := emptyState("ui.press_run")
	if result, ok := a.Results["dpi"]; ok && len(result.DPI) > 0 {
		scan = strings.TrimRight(dpiTable(result.DPI, p.width-6), "\n")
	}

	specs := []sectionSpec{
		{"Unwall — zapret / DPI", strings.Join(status, "\n")},
		{i18n.T("sec.dpi"), scan},
		{i18n.T("help.sec.reading"), sDim.Render(wrap(i18n.T("help.dpi.explainer"), p.width-6))},
	}
	if a.AB != nil && a.AB.target == "unwall" {
		specs = append(specs, sectionSpec{i18n.T("misc.ab_result"),
			strings.TrimRight(abRows(a, "zapret ON", "zapret OFF"), "\n")})
	}
	return stack(p.width, specs...)
}

func dnsText(state sysinfo.UnwallState) string {
	if !state.DNSEncrypted {
		return i18n.T("ui.off")
	}
	return state.DNSBackend + " / " + state.DNSProvider
}
