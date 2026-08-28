package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

type dnsPage struct {
	viewport      viewport.Model
	width, height int
}

func newDNSPage() Page { return &dnsPage{} }

func (p *dnsPage) ID() tabID { return tabDNS }

func (p *dnsPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = width, height
}

func (p *dnsPage) Reload(a *App) { p.viewport.SetContent(p.body(a)) }

func (p *dnsPage) Update(a *App, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *dnsPage) SuiteName(*App) string { return "dns" }

func (p *dnsPage) View(a *App) string { return p.viewport.View() }

func (p *dnsPage) body(a *App) string {
	var b strings.Builder
	b.WriteString(sSection.Render("DNS") + "\n")
	b.WriteString("  " + kv(i18n.T("f.system"),
		strings.Join(a.Env.systemDNS, ", "), sText, 16) + "\n")

	// a plaintext resolver in the fallback list defeats encrypted DNS entirely,
	// so it is coloured as a warning rather than listed neutrally
	leakStyle := sText
	if a.Env.unwall.DNSEncrypted {
		for _, server := range a.Env.upstream {
			if !strings.HasPrefix(server, "127.") {
				leakStyle = sWarn
			}
		}
	}
	b.WriteString("  " + kv(i18n.T("f.upstream"),
		strings.Join(a.Env.upstream, ", "), leakStyle, 16) + "\n")
	b.WriteString("  " + kv(i18n.T("f.encryption"), dnsText(a.Env.unwall), sText, 16) + "\n\n")

	result, ok := a.Results["dns"]
	if !ok {
		for _, name := range []string{"deep", "full", "quick"} {
			if alternative, found := a.Results[name]; found {
				result, ok = alternative, true
				break
			}
		}
	}
	if !ok || len(result.DNSBench) == 0 {
		b.WriteString(emptyState("ui.press_run") + "\n")
		return b.String()
	}
	b.WriteString(dnsTable(result.DNSBench) + "\n")
	if len(result.DNSChecks) > 0 {
		b.WriteString(sSection.Render(i18n.T("sec.dnschecks")) + "\n" +
			checksTable(result.DNSChecks) + "\n")
	}
	if len(result.DNSCompare) > 0 {
		cols := []column{
			{title: i18n.T("col.domain"), width: 24},
			{title: i18n.T("f.system"), width: 30},
			{title: i18n.T("f.encryption"), width: max(p.width-58, 12)},
		}
		var rows [][]cell
		for _, comparison := range result.DNSCompare {
			style := sOK
			if !comparison.Overlap {
				style = sWarn
			}
			rows = append(rows, []cell{
				plain(comparison.Domain),
				styled(strings.Join(comparison.System, ","), style),
				plain(strings.Join(comparison.Trusted, ",")),
			})
		}
		b.WriteString(sSection.Render(i18n.T("sec.dnscompare")) + "\n" +
			renderTable(cols, rows))
	}
	return b.String()
}

var _ = util.Truncate
