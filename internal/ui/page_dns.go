package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
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
	config := strings.Join([]string{
		kv(i18n.T("f.system"), strings.Join(a.Env.systemDNS, ", "), sText, 16),
		kv(i18n.T("f.upstream"), strings.Join(a.Env.upstream, ", "), leakStyle, 16),
		kv(i18n.T("f.encryption"), dnsText(a.Env.unwall), sText, 16),
	}, "\n")

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
		return stack(p.width,
			sectionSpec{"DNS", config},
			sectionSpec{i18n.T("sec.dns"), emptyState("ui.press_run")})
	}

	var compare string
	if len(result.DNSCompare) > 0 {
		cols := []column{
			{title: i18n.T("col.domain"), width: 24},
			{title: i18n.T("f.system"), width: 30},
			{title: i18n.T("f.encryption"), width: max(p.width-64, 12)},
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
		compare = strings.TrimRight(renderTable(cols, rows), "\n")
	}

	return stack(p.width,
		sectionSpec{"DNS", config},
		sectionSpec{i18n.T("sec.dns"), strings.TrimRight(dnsTable(result.DNSBench), "\n")},
		sectionSpec{i18n.T("sec.dnschecks"), strings.TrimRight(checksTable(result.DNSChecks), "\n")},
		sectionSpec{i18n.T("sec.dnscompare"), compare})
}
