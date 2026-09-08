package suite

import "github.com/WinTone01/nabiz/internal/i18n"

// Every probe in this tool is built to explain a connection that works badly.
// None of them is built to notice that there is no connection at all, and the
// difference matters more than anything else here: with nothing on the wire,
// each probe fails, and a failed probe looks exactly like a blocked one.
//
// Measured before this existed: on a machine with no network, `doctor` scored
// 97/100 (A+) because the score only replaces its perfect starting point once a
// packet has come back, and every link check is guarded by "if speed > 0" which
// a missing interface skips. `quick` scored 77/100 and reported DNS hijacking,
// a DPI block and QUIC blocking - a censorship diagnosis invented entirely out
// of the absence of a network. That is the worst possible answer for the person
// most likely to be running it.

// Presence is the answer to "is there a network here at all", and why not.
type Presence struct {
	Down   bool   `json:"down"`
	Reason string `json:"reason,omitempty"` // catalog key suffix
	Iface  string `json:"iface,omitempty"`
}

// Detail renders the reason in the active language.
func (p Presence) Detail() string {
	if !p.Down {
		return ""
	}
	return i18n.T("fnd.no-network." + p.Reason)
}

// CheckPresence decides whether this run had a network to measure.
//
// It insists on unambiguous evidence, because a false positive here suppresses
// every other finding: either the machine has no route to send through, or the
// interface it would send through is down, or packets went out to every anchor
// and nothing at all came back.
func CheckPresence(result Result) Presence {
	link := result.Env.Link

	if link.Iface == "" && link.Gateway == "" {
		return Presence{Down: true, Reason: "nointerface"}
	}
	// A wireless interface reports carrier only while associated, and a wired
	// one only while the cable is live; either way "down" is not a judgement
	// call.
	if link.Iface != "" && !link.Carrier {
		return Presence{Down: true, Reason: "nocarrier", Iface: link.Iface}
	}
	if link.Gateway == "" {
		return Presence{Down: true, Reason: "noroute", Iface: link.Iface}
	}

	// Probes that were run and answered by nothing. One silent anchor is a
	// firewall; every anchor silent, including the modem, is not a diagnosis
	// about the internet - it is the absence of one.
	if len(result.Latency) > 0 {
		sent, received := 0, 0
		for _, summary := range result.Latency {
			sent += summary.Sent
			received += summary.Received
		}
		if sent > 0 && received == 0 {
			return Presence{Down: true, Reason: "noreply", Iface: link.Iface}
		}
	}
	return Presence{}
}

// meaningless lists the findings that describe *how* traffic is interfered
// with. With no network they all fire at once off failed probes, which is how
// a dead link turns into a censorship report.
var meaningless = map[string]bool{
	"dpi-blocked": true, "quic-blocked": true, "dpi-sni": true,
	"dns-fail": true, "dns-nxdomain-hijack": true, "dns-edns": true,
	"dns-redirect": true, "dns-mismatch": true, "dns-slow": true,
	"loss": true, "loss-small": true, "loss-burst": true, "jitter": true,
	"jitter-small": true, "spikes": true, "gateway-loss": true,
	"unwall-quic": true, "ipv6-missing": true, "ipv6-broken": true,
}
