package suite

import (
	"testing"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

func adviceIDs(result Result) map[string]bool {
	out := map[string]bool{}
	for _, advice := range result.Advice {
		out[advice.ID] = true
	}
	return out
}

// The bug this guards against: advice is derived from a stored run, so applying
// a change used to leave it in the list forever. Deriving twice from the same
// run with only the environment changed must drop the item.
func TestAppliedChangeLeavesTheAdviceList(t *testing.T) {
	cfg := config.Default()
	base := Result{Env: Env{
		Link:    probe.LinkInfo{Iface: "eth0", SpeedMbit: 1000, Duplex: "full", MTU: 1500},
		Sysctls: map[string]string{"net.ipv4.tcp_slow_start_after_idle": "1"},
	}}

	before := base
	Finalize(&before, cfg)
	if !adviceIDs(before)["ssaio"] {
		t.Fatal("expected the slow-start advice before the change is applied")
	}

	after := base
	after.Env.Sysctls = map[string]string{"net.ipv4.tcp_slow_start_after_idle": "0"}
	Finalize(&after, cfg)
	if adviceIDs(after)["ssaio"] {
		t.Error("the slow-start advice survived the setting being applied")
	}
}

// A hostlist entry that opens on its own with the engine stopped is a false
// positive, and that can only be seen by comparing against a run without it.
func TestDomainsWithoutDesyncNeedsTheEngineStopped(t *testing.T) {
	withEngineUp := Result{
		Env: Env{Unwall: sysinfo.UnwallState{Installed: true, Running: true}},
		DPI: []probe.DomainVerdict{{Domain: "discord.com", Verdict: "clean"}},
	}
	if got := DomainsWithoutDesync(withEngineUp); len(got) != 0 {
		t.Errorf("a run with the engine up proves nothing, got %v", got)
	}
}

// Every advice item must carry enough for someone to act on it. An empty Why or
// How is a recommendation nobody can follow.
func TestEveryAdviceItemIsActionable(t *testing.T) {
	cfg := config.Default()
	result := Result{Env: Env{
		Link: probe.LinkInfo{Iface: "eth0", SpeedMbit: 100, Duplex: "full", MTU: 1500,
			CarrierUps: 12, Stats: map[string]int64{"rx_crc_errors": 4, "rx_packets": 1000}},
		Sysctls:   map[string]string{"net.ipv4.tcp_congestion_control": "cubic"},
		TCPHealth: probe.TCPHealth{RetransPct: 4, OutSegs: 100000, RetransSegs: 4000},
		Unwall: sysinfo.UnwallState{Installed: true, Running: true,
			AutoHostlistN: 900, GatewayMode: true, PortsUDP: "443"},
	}}
	Finalize(&result, cfg)
	if len(result.Advice) == 0 {
		t.Fatal("a machine in this state should produce advice")
	}
	for _, advice := range result.Advice {
		if advice.Title == "" || advice.Why == "" {
			t.Errorf("%s: missing title or reason", advice.ID)
		}
		if len(advice.How) == 0 {
			t.Errorf("%s: no steps to follow", advice.ID)
		}
		if advice.Priority < 1 || advice.Priority > 5 {
			t.Errorf("%s: priority %d out of range", advice.ID, advice.Priority)
		}
	}
}
