// Package suite composes probes into named test runs and turns the raw numbers
// into findings, a score and concrete advice.
package suite

import (
	"time"

	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

// Env is the machine-side snapshot taken at the start of every run. Half the
// findings come from here without a single packet being sent.
type Env struct {
	Host        string                      `json:"host"`
	Kernel      string                      `json:"kernel"`
	Link        probe.LinkInfo              `json:"link"`
	LinkLog     probe.LinkHistory           `json:"link_history"`
	LinkBoots   []probe.BootLinkStats       `json:"link_boots,omitempty"`
	LinkRegress string                      `json:"link_regression,omitempty"`
	LinkRegData *probe.Regression           `json:"link_regression_data,omitempty"`
	KernRegData *probe.KernelRegressionData `json:"kernel_regression_data,omitempty"`
	Reboots     []probe.Reboot              `json:"reboots,omitempty"`
	KernelStats []probe.KernelStability     `json:"kernel_stability,omitempty"`
	GoodKernel  string                      `json:"good_kernel,omitempty"`
	KernelRegr  string                      `json:"kernel_regression,omitempty"`
	EEE         probe.EEEStatus             `json:"eee"`
	ASPM        probe.ASPMStatus            `json:"aspm"`
	Journal     probe.JournalStorage        `json:"journal"`
	SQM         probe.SQMState              `json:"sqm"`
	IPv6        probe.IPv6Status            `json:"ipv6"`
	Firewall    probe.FirewallICMP          `json:"firewall_icmp"`
	NFQueue     probe.NFQueueInfo           `json:"nfqueue"`
	Conntrack   probe.Conntrack             `json:"conntrack"`
	Sysctls     map[string]string           `json:"sysctl"`
	TCPHealth   probe.TCPHealth             `json:"tcp_health"`
	Unwall      sysinfo.UnwallState         `json:"unwall"`
	Bpftune     sysinfo.BpftuneState        `json:"bpftune"`
	SystemDNS   []string                    `json:"system_dns"`
	UpstreamDNS []string                    `json:"upstream_dns"`
	DNSPaths    []probe.ResolverPath        `json:"dns_paths,omitempty"`
	Sockets     probe.SocketSummary         `json:"sockets"`
}

// DNSRow is one resolver's benchmark line.
type DNSRow struct {
	Label     string  `json:"label"`
	Kind      string  `json:"kind"`
	Address   string  `json:"address"`
	Encrypted bool    `json:"encrypted"`
	Queries   int     `json:"queries"`
	Failures  int     `json:"failures"`
	AvgMs     float64 `json:"avg_ms"`
	P95Ms     float64 `json:"p95_ms"`
	MinMs     float64 `json:"min_ms"`
}

// Finding is one problem (or confirmation) derived from the measurements.
type Finding struct {
	Level  string `json:"level"` // bad | warn | info | ok
	Key    string `json:"key"`
	Title  string `json:"title"`
	Hint   string `json:"hint,omitempty"`
	Source string `json:"source,omitempty"`
}

// Advice is one actionable recommendation, ranked and reversible.
type Advice struct {
	ID       string   `json:"id"`
	Priority int      `json:"priority"` // 1 = do this first
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Why      string   `json:"why"`
	How      []string `json:"how"`
	Gain     string   `json:"gain,omitempty"`
	Risk     string   `json:"risk,omitempty"`
	Revert   []string `json:"revert,omitempty"`
}

// IsCommand reports whether a How step is a shell command rather than a
// physical instruction, so the renderers can prompt it correctly.
func IsCommand(step string) bool {
	for _, prefix := range []string{
		"sudo ", "echo ", "watch ", "nabiz ", "unwallctl ", "tc ", "ip ", "ethtool ",
		"sysctl ", "systemctl ", "iw ", "cat ", "journalctl ", "bpftune ", "#",
	} {
		if len(step) >= len(prefix) && step[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// Result is one complete run, serialisable and diffable.
type Result struct {
	Name       string                   `json:"name"`
	Lang       string                   `json:"lang,omitempty"`
	StartedAt  time.Time                `json:"started_at"`
	Duration   float64                  `json:"duration"`
	Env        Env                      `json:"env"`
	Latency    []stats.Summary          `json:"latency,omitempty"`
	DNSBench   []DNSRow                 `json:"dns_bench,omitempty"`
	DNSChecks  []probe.Check            `json:"dns_checks,omitempty"`
	DNSCompare []probe.AnswerComparison `json:"dns_compare,omitempty"`
	DPI        []probe.DomainVerdict    `json:"dpi,omitempty"`
	Hops       []probe.Hop              `json:"hops,omitempty"`
	MTU        *probe.MTUResult         `json:"mtu,omitempty"`
	Load       *probe.BloatResult       `json:"load,omitempty"`
	Findings   []Finding                `json:"findings"`
	Advice     []Advice                 `json:"advice"`
	// DesyncNotNeeded lists hostlist domains that opened cleanly with the
	// bypass engine stopped. They are false positives: every packet they cost
	// the engine buys nothing.
	DesyncNotNeeded []string     `json:"desync_not_needed,omitempty"`
	Baseline        *BaselineRef `json:"baseline,omitempty"`
	Score           float64      `json:"score"`
	Grade           string       `json:"grade"`
	Cancelled       bool         `json:"cancelled"`
}

// InternetLatency is the best non-gateway anchor; the gateway says nothing
// about the WAN.
func (r Result) InternetLatency() *stats.Summary {
	var best *stats.Summary
	for i := range r.Latency {
		summary := &r.Latency[i]
		if summary.Received == 0 || isGateway(summary.Label) {
			continue
		}
		if best == nil || summary.Avg < best.Avg {
			best = summary
		}
	}
	return best
}

// GatewayLatency is the modem hop, which separates "my house" from "my ISP".
func (r Result) GatewayLatency() *stats.Summary {
	for i := range r.Latency {
		if isGateway(r.Latency[i].Label) {
			return &r.Latency[i]
		}
	}
	return nil
}

func isGateway(label string) bool {
	return label == "Modem / Gateway"
}

// CountFindings returns how many findings sit at the given level.
func (r Result) CountFindings(level string) int {
	count := 0
	for _, finding := range r.Findings {
		if finding.Level == level {
			count++
		}
	}
	return count
}

// BaselineRef is a pointer to the run this one is compared against.
type BaselineRef struct {
	Path      string  `json:"path"`
	Score     float64 `json:"score"`
	Grade     string  `json:"grade"`
	StartedAt string  `json:"started_at"`
}
