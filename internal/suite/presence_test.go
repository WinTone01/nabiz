package suite

import (
	"testing"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/stats"
)

// The failure this guards against, measured before it was fixed: on a machine
// with no network at all, `doctor` scored 97/100 (A+) and `quick` scored 77/100
// while reporting DNS hijacking, a DPI block and QUIC blocking - a censorship
// diagnosis invented out of the absence of a network, handed to the one person
// most likely to be running the tool.
func TestADeadNetworkIsNotAGoodConnection(t *testing.T) {
	cfg := config.Default()
	result := Result{
		Env:      Env{Link: probe.LinkInfo{}}, // no interface, no route, no carrier
		Findings: nil,
	}
	Finalize(&result, cfg)

	if result.Score != 0 {
		t.Errorf("score %.0f on a machine with no network, want 0", result.Score)
	}
	if !hasFinding(result, "no-network") {
		t.Error("nothing said there was no network")
	}
}

// A failed probe and a blocked one look identical from here, so the
// interference findings have to be attributed rather than left standing alone.
func TestInterferenceIsNotDiagnosedWithoutANetwork(t *testing.T) {
	cfg := config.Default()
	result := Result{
		Env: Env{Link: probe.LinkInfo{}},
		DPI: []probe.DomainVerdict{
			{Domain: "a.example", Verdict: "blocked"},
			{Domain: "b.example", Verdict: "blocked"},
		},
	}
	Finalize(&result, cfg)
	for _, finding := range result.Findings {
		if meaningless[finding.Key] && finding.Because != "no-network" {
			t.Errorf("%q stands as its own diagnosis with no network to test", finding.Key)
		}
	}
}

// Anchors that were probed and answered by nothing - including the modem - are
// the absence of a network, not a fact about the internet.
func TestEveryAnchorSilentCountsAsNoNetwork(t *testing.T) {
	result := Result{
		Env: Env{Link: probe.LinkInfo{Iface: "eth0", Gateway: "192.168.0.1", Carrier: true}},
		Latency: []stats.Summary{
			{Label: "Modem / Gateway", Sent: 10, Received: 0},
			{Label: "Cloudflare", Sent: 10, Received: 0},
		},
	}
	if presence := CheckPresence(result); !presence.Down || presence.Reason != "noreply" {
		t.Errorf("presence = %+v, want down with noreply", presence)
	}

	// one silent anchor is a firewall, not a dead link
	result.Latency[0].Received = 10
	if presence := CheckPresence(result); presence.Down {
		t.Errorf("called the network dead while the modem answered: %+v", presence)
	}
}

// A working link must be unaffected, or the cure is worse than the disease.
func TestAWorkingLinkIsNotCalledDown(t *testing.T) {
	result := Result{Env: Env{Link: probe.LinkInfo{
		Iface: "enp3s0", Gateway: "192.168.0.1", Carrier: true, SpeedMbit: 100, MTU: 1500,
	}}}
	if presence := CheckPresence(result); presence.Down {
		t.Errorf("a healthy link was called down: %+v", presence)
	}
}

func hasFinding(result Result, key string) bool {
	for _, finding := range result.Findings {
		if finding.Key == key {
			return true
		}
	}
	return false
}
