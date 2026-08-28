package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
)

// DNS is where "the internet is slow" most often turns out to mean "name
// resolution is slow", and where an encrypted resolver can be quietly bypassed.
// The page puts the configuration and the measurement side by side, because
// either one alone can look fine while the pair is broken.
type dnsPage struct {
	body          scroller
	width, height int
}

func newDNSPage() Page { return &dnsPage{body: newScroller()} }

func (p *dnsPage) ID() pageID            { return pageDNS }
func (p *dnsPage) SuiteName(*App) string { return "dns" }
func (p *dnsPage) Focus(focused bool)    { p.body.focused = focused }

func (p *dnsPage) Layout(width, height int) {
	p.width, p.height = width, height
	p.body.layout(width, height)
}

func (p *dnsPage) Reload(a *App)                      { p.body.setContent(p.render(a)) }
func (p *dnsPage) Update(a *App, msg tea.Msg) tea.Cmd { return p.body.update(msg) }
func (p *dnsPage) View(a *App) string                 { return p.body.view() }

func (p *dnsPage) render(a *App) string {
	width := p.body.contentWidth()
	inner := width - 4

	// A plaintext resolver in the fallback list defeats encrypted DNS entirely,
	// so it is coloured as a warning rather than listed neutrally.
	leakStyle := sText
	if a.Env.unwall.DNSEncrypted {
		for _, server := range a.Env.upstream {
			if !strings.HasPrefix(server, "127.") {
				leakStyle = sWarn
			}
		}
	}
	var list kvList
	list.add(i18n.T("f.system"), strings.Join(a.Env.systemDNS, ", "))
	list.addStyled(i18n.T("f.upstream"), strings.Join(a.Env.upstream, ", "), leakStyle)
	list.add(i18n.T("f.encryption"), dnsText(a.Env.unwall))
	config := list.render(20)

	result, ok := a.Results["dns"]
	if !ok {
		// Any run that benchmarked resolvers will do; the dns suite is just the
		// one that does it thoroughly.
		for _, name := range []string{"deep", "full", "quick"} {
			if alternative, found := a.Results[name]; found {
				result, ok = alternative, true
				break
			}
		}
	}
	if !ok || len(result.DNSBench) == 0 {
		return stack(width,
			section{title: "DNS", body: config},
			section{title: i18n.T("sec.dns"), body: emptyState("ui.press_run")})
	}

	return stack(width,
		section{title: "DNS", body: config},
		section{title: i18n.T("sec.dns"), badge: fastestBadge(result),
			body: dnsTable(result.DNSBench, inner)},
		section{title: i18n.T("sec.dnschecks"), body: checksTable(result.DNSChecks, inner)},
		section{title: i18n.T("sec.dnscompare"), body: p.compare(result, inner)})
}

// fastestBadge names the quickest resolver that actually answered, which is the
// only recommendation this table can make on its own.
func fastestBadge(result suite.Result) string {
	best := ""
	bestMs := 0.0
	for _, row := range result.DNSBench {
		if row.Failures == row.Queries || row.AvgMs <= 0 {
			continue
		}
		if best == "" || row.AvgMs < bestMs {
			best, bestMs = row.Label, row.AvgMs
		}
	}
	if best == "" {
		return ""
	}
	return sOK.Render(best)
}

func (p *dnsPage) compare(result suite.Result, width int) string {
	if len(result.DNSCompare) == 0 {
		return ""
	}
	cols := []col{
		{title: i18n.T("col.domain"), width: 24},
		{title: i18n.T("f.system"), width: 30},
		{title: i18n.T("f.encryption"), width: 0},
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
	return renderTable(width, cols, rows)
}
