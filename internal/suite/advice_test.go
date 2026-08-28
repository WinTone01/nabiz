package suite

import (
	"strings"
	"testing"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/stats"
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

// Reported twice: applying the offload change never removed the recommendation.
// It was judged against the kernel's cumulative retransmission counter, which
// only ever grows, so nothing the user did could satisfy it.
func TestOffloadAdviceRespectsTheCurrentState(t *testing.T) {
	cfg := config.Default()
	build := func(offloads probe.Offloads) Result {
		return Result{
			Env: Env{
				Link: probe.LinkInfo{Iface: "eth0", SpeedMbit: 1000, Duplex: "full",
					MTU: 1500, Offloads: offloads},
				TCPHealth: probe.TCPHealth{RetransPct: 13, OutSegs: 500000, RetransSegs: 65000},
			},
			Latency: []stats.Summary{
				{Label: "Modem / Gateway", Target: "192.168.0.1", Received: 10, Avg: 1},
				{Label: "Cloudflare", Target: "1.1.1.1", Received: 10, Avg: 25},
			},
		}
	}
	on := build(probe.Offloads{Known: true, GRO: true, GSO: true, TSO: true})
	Finalize(&on, cfg)
	if !adviceIDs(on)["nic-offload"] {
		t.Fatal("expected the offload advice while the offloads are on")
	}

	off := build(probe.Offloads{Known: true})
	Finalize(&off, cfg)
	if adviceIDs(off)["nic-offload"] {
		t.Error("the offload advice survived the offloads being turned off")
	}
}

// The other half of the same report: the leak fix cleared the global fallback
// while the addresses were on a link, so it could never take effect. It also
// counted a MagicDNS address as a leak, which is a resolver reached over an
// encrypted tunnel.
func TestDNSLeakLooksAtScopesAndIgnoresLocalResolvers(t *testing.T) {
	cfg := config.Default()
	encrypted := sysinfo.UnwallState{Installed: true, Running: true, DNSEncrypted: true,
		DNSBackend: "dnscrypt"}

	clean := Result{Env: Env{
		Link:   probe.LinkInfo{Iface: "eth0", MTU: 1500},
		Unwall: encrypted,
		DNSPaths: []probe.ResolverPath{
			{Servers: []string{"127.0.0.1"}},
			{Link: "tailscale0", Servers: []string{"100.100.100.100", "fd7a:115c:a1e0::53"}},
		},
	}}
	Finalize(&clean, cfg)
	if adviceIDs(clean)["dns-leak"] {
		t.Error("a loopback stub and MagicDNS are not a plaintext leak")
	}

	leaking := clean
	leaking.Env.DNSPaths = append(leaking.Env.DNSPaths,
		probe.ResolverPath{Link: "eth0", Servers: []string{"46.196.235.227"}})
	Finalize(&leaking, cfg)
	if !adviceIDs(leaking)["dns-leak"] {
		t.Fatal("a resolver on a link is still a leak")
	}
	for _, advice := range leaking.Advice {
		if advice.ID != "dns-leak" {
			continue
		}
		if !strings.Contains(advice.Why, "eth0") {
			t.Errorf("the advice must name the scope that leaks, got %q", advice.Why)
		}
	}
}
