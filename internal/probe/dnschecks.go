package probe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// Check is one pass/fail environmental observation.
type Check struct {
	Name    string            `json:"name"`
	Verdict string            `json:"verdict"` // ok | warn | bad | info
	Detail  string            `json:"detail"`
	Data    map[string]string `json:"data,omitempty"`
}

// sinkholes are documentation-only ranges: nothing there runs a resolver, so a
// reply can only have come from something on the path.
var sinkholes = []string{"192.0.2.1", "198.51.100.1"}

// CheckTransparentRedirect proves whether UDP/53 is being intercepted. If an
// address that runs no DNS answers a question, the answer was manufactured on
// the path and picking a different resolver changes nothing.
func CheckTransparentRedirect(ctx context.Context, timeout time.Duration) Check {
	for _, sink := range sinkholes {
		res := QueryUDP(ctx, sink, "example.com", "A", timeout, false)
		if res.OK && len(res.Addrs) > 0 {
			return Check{
				Name:    "transparent-dns",
				Verdict: "bad",
				Detail: i18n.T("chk.transparent-dns.bad",
					sink, strings.Join(res.Addrs, ", ")),
				Data: map[string]string{"sinkhole": sink},
			}
		}
	}
	return Check{Name: "transparent-dns", Verdict: "ok", Detail: i18n.T("chk.transparent-dns.ok")}
}

// CheckNXDOMAINHijack looks for an ISP turning "no such name" into an ad page.
func CheckNXDOMAINHijack(ctx context.Context, timeout time.Duration) Check {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	name := "nabiz-" + hex.EncodeToString(buf) + ".invalid"
	servers := SystemResolvers()
	if len(servers) == 0 {
		servers = []string{"1.1.1.1"}
	}
	res := QueryUDP(ctx, servers[0], name, "A", timeout, false)
	switch {
	case len(res.Addrs) > 0:
		return Check{Name: "nxdomain-hijack", Verdict: "bad",
			Detail: i18n.T("chk.nxdomain-hijack.bad", res.Addrs[0])}
	case res.RCode == "NXDOMAIN" || res.RCode == "SERVFAIL" || res.RCode == "REFUSED":
		return Check{Name: "nxdomain-hijack", Verdict: "ok", Detail: res.RCode}
	default:
		return Check{Name: "nxdomain-hijack", Verdict: "warn", Detail: res.Summary()}
	}
}

// CheckDNSSEC resolves a deliberately broken zone: an answer means the resolver
// is not validating, SERVFAIL means it is.
func CheckDNSSEC(ctx context.Context, resolver Resolver, timeout time.Duration) Check {
	broken := Query(ctx, resolver, "dnssec-failed.org", "A", timeout, true)
	good := Query(ctx, resolver, "cloudflare.com", "A", timeout, true)
	switch {
	case broken.OK && len(broken.Addrs) > 0:
		return Check{Name: "dnssec", Verdict: "warn",
			Detail: i18n.T("chk.dnssec.warn")}
	case broken.RCode == "SERVFAIL" && good.OK:
		return Check{Name: "dnssec", Verdict: "ok",
			Detail: i18n.T("chk.dnssec.ok")}
	default:
		return Check{Name: "dnssec", Verdict: "info", Detail: broken.Summary()}
	}
}

// CheckUDPvsTCP catches middleboxes that only rewrite the cheap UDP path.
func CheckUDPvsTCP(ctx context.Context, server, domain string, timeout time.Duration) Check {
	udp := QueryUDP(ctx, server, domain, "A", timeout, false)
	tcp := QueryTCP(ctx, server, domain, "A", timeout, false)
	if !udp.OK || !tcp.OK {
		return Check{Name: "udp-vs-tcp", Verdict: "info",
			Detail: fmt.Sprintf("udp:%s tcp:%s", udp.Summary(), tcp.Summary())}
	}
	if overlap(udp.Addrs, tcp.Addrs) {
		return Check{Name: "udp-vs-tcp", Verdict: "ok", Detail: i18n.T("chk.udp-vs-tcp.ok")}
	}
	return Check{Name: "udp-vs-tcp", Verdict: "warn",
		Detail: i18n.T("chk.udp-vs-tcp.differ",
			domain, strings.Join(udp.Addrs, ","), strings.Join(tcp.Addrs, ","))}
}

// CheckInjection listens past the first answer. Censorship boxes race the real
// resolver: the forged packet arrives first, the genuine one a few ms later.
// Two different answers to one question is proof, not inference.
func CheckInjection(ctx context.Context, server, domain string, window time.Duration) Check {
	target := hostPort(server, "53")
	packed, id, err := buildQuery(domain, "A", false)
	if err != nil {
		return Check{Name: "dns-injection", Verdict: "info", Detail: err.Error()}
	}
	dialer := net.Dialer{Timeout: window}
	conn, err := dialer.DialContext(ctx, "udp", target)
	if err != nil {
		return Check{Name: "dns-injection", Verdict: "info", Detail: shortErr(err)}
	}
	defer conn.Close()
	deadline := time.Now().Add(window)
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write(packed); err != nil {
		return Check{Name: "dns-injection", Verdict: "info", Detail: shortErr(err)}
	}
	type reply struct {
		addrs []string
		at    time.Duration
	}
	start := time.Now()
	var replies []reply
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		res := parseResponse(buf[:n], id, true)
		if res.Err != "" {
			continue
		}
		set := append([]string(nil), res.Addrs...)
		sort.Strings(set)
		replies = append(replies, reply{addrs: set, at: time.Since(start)})
	}
	if len(replies) < 2 {
		return Check{Name: "dns-injection", Verdict: "ok", Detail: i18n.T("chk.injection.ok")}
	}
	first := strings.Join(replies[0].addrs, ",")
	for _, other := range replies[1:] {
		if strings.Join(other.addrs, ",") != first {
			return Check{Name: "dns-injection", Verdict: "bad",
				Detail: i18n.T("chk.injection.two",
					first, replies[0].at.Seconds()*1000,
					strings.Join(other.addrs, ","), other.at.Seconds()*1000)}
		}
	}
	return Check{Name: "dns-injection", Verdict: "info",
		Detail: i18n.T("chk.injection.dupe", len(replies))}
}

// CheckEDNS verifies that large, EDNS0-flagged answers survive the path; a
// middlebox that drops them makes DNSSEC and modern resolvers fail oddly.
func CheckEDNS(ctx context.Context, resolver Resolver, timeout time.Duration) Check {
	res := Query(ctx, resolver, "cloudflare.com", "A", timeout, true)
	switch {
	case res.Truncated:
		return Check{Name: "edns", Verdict: "warn",
			Detail: i18n.T("chk.edns.info")}
	case res.OK:
		return Check{Name: "edns", Verdict: "ok", Detail: i18n.T("chk.edns.ok")}
	default:
		return Check{Name: "edns", Verdict: "warn",
			Detail: i18n.T("chk.edns.warn", res.Summary())}
	}
}

func overlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, item := range a {
		set[item] = true
	}
	for _, item := range b {
		if set[item] {
			return true
		}
	}
	return false
}

func boolWord(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// AnswerComparison records what two resolvers said about the same name.
type AnswerComparison struct {
	Domain       string   `json:"domain"`
	System       []string `json:"system"`
	SystemRCode  string   `json:"system_rcode"`
	Trusted      []string `json:"trusted"`
	TrustedRCode string   `json:"trusted_rcode"`
	Overlap      bool     `json:"overlap"`
}

// CompareAnswers asks the system resolver and a trusted encrypted resolver the
// same question. CDNs legitimately answer differently per resolver, so a
// mismatch is a hint rather than proof - the TLS probe decides whether the
// system's answer actually works.
func CompareAnswers(ctx context.Context, domain string, trusted Resolver,
	timeout time.Duration,
) AnswerComparison {
	system := Query(ctx, Resolver{Label: "system", Kind: "system"}, domain, "A", timeout, false)
	secure := Query(ctx, trusted, domain, "A", timeout+time.Second, false)
	out := AnswerComparison{
		Domain:       domain,
		System:       system.Addrs,
		SystemRCode:  firstNonEmpty(system.RCode, system.Err),
		Trusted:      secure.Addrs,
		TrustedRCode: firstNonEmpty(secure.RCode, secure.Err),
	}
	set := make(map[string]bool, len(secure.Addrs))
	for _, addr := range secure.Addrs {
		set[addr] = true
	}
	for _, addr := range system.Addrs {
		if set[addr] {
			out.Overlap = true
			break
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
