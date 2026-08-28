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
	var b strings.Builder
	b.WriteString(sSection.Render("Unwall — zapret / DPI") + "\n\n")
	if !state.Installed {
		b.WriteString(emptyState("ui.notinstalled") + "\n")
		return b.String()
	}
	b.WriteString("  " + kv(i18n.T("f.status"), "", sText, 18) + runningTag(state.Running) + "\n")
	for _, row := range [][2]string{
		{i18n.T("f.engine"), state.Engine + " · " + state.Strategy},
		{i18n.T("f.hostlist"), fmt.Sprintf("%s · %d / %d", state.HostlistMode,
			state.HostlistN, state.AutoHostlistN)},
		{i18n.T("f.ports"), fmt.Sprintf("TCP %s · UDP %s · queue %d",
			state.PortsTCP, state.PortsUDP, state.QNum)},
		{i18n.T("f.gatewaymode"), boolText(state.GatewayMode)},
		{i18n.T("f.dns"), dnsText(state)},
	} {
		b.WriteString("  " + kv(row[0], row[1], sText, 18) + "\n")
	}
	b.WriteString("  " + kv(i18n.T("f.nfqueue"), nfqText(a), nfqStyle(a), 18) + "\n\n")

	if result, ok := a.Results["dpi"]; ok && len(result.DPI) > 0 {
		b.WriteString(sSection.Render(i18n.T("sec.dpi")) + "\n" +
			dpiTable(result.DPI, p.width) + "\n")
	} else {
		b.WriteString(emptyState("ui.press_run") + "\n\n")
	}
	b.WriteString(sDim.Render(wrap(i18n.T("help.dpi.explainer"), p.width-2)) + "\n\n")
	b.WriteString(sFaint.Render("a → "+i18n.T("key.ab")) + "\n")
	if a.AB != nil && a.AB.target == "unwall" {
		b.WriteString("\n" + abTable(a, "zapret ON", "zapret OFF"))
	}
	return b.String()
}

func dnsText(state sysinfo.UnwallState) string {
	if !state.DNSEncrypted {
		return i18n.T("ui.off")
	}
	return state.DNSBackend + " / " + state.DNSProvider
}
