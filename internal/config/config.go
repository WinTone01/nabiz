// Package config holds the defaults, the target catalogue and the user's
// on-disk overrides.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// Anchor is one ping target with a role attached; the role decides how a
// finding is worded ("your cable" versus "your ISP").
type Anchor struct {
	Label string `json:"label"`
	Host  string `json:"host"`
	Kind  string `json:"kind"` // gateway | internet
}

// Thresholds are the lines between fine, worth-a-look and broken.
type Thresholds struct {
	LossWarn   float64 `json:"loss_warn"`
	LossBad    float64 `json:"loss_bad"`
	JitterWarn float64 `json:"jitter_warn"`
	JitterBad  float64 `json:"jitter_bad"`
	RTTWarn    float64 `json:"rtt_warn"`
	RTTBad     float64 `json:"rtt_bad"`
	DNSWarnMs  float64 `json:"dns_warn_ms"`
	DNSBadMs   float64 `json:"dns_bad_ms"`
	BloatWarn  float64 `json:"bloat_warn"`
	BloatBad   float64 `json:"bloat_bad"`
	RetransPct float64 `json:"retrans_pct"`
}

// Config is the whole tunable surface of nabiz.
type Config struct {
	Lang              string           `json:"lang"`
	PingIntervalMs    int              `json:"ping_interval_ms"`
	QuickProbes       int              `json:"quick_probes"`
	FullProbes        int              `json:"full_probes"`
	PingTimeoutMs     int              `json:"ping_timeout_ms"`
	ConnectTimeoutS   int              `json:"connect_timeout_s"`
	DNSTimeoutS       int              `json:"dns_timeout_s"`
	LoadSeconds       int              `json:"load_seconds"`
	IdleSeconds       int              `json:"idle_seconds"`
	Streams           int              `json:"streams"`
	SpeedBytes        int64            `json:"speed_bytes"`
	MonitorInterval   int              `json:"monitor_interval_ms"`
	MonitorDNSEvery   int              `json:"monitor_dns_every_s"`
	MonitorReachEvery int              `json:"monitor_reach_every_s"`
	Anchors           []Anchor         `json:"anchors"`
	Resolvers         []probe.Resolver `json:"resolvers"`
	ControlDomains    []string         `json:"control_domains"`
	SensitiveDomains  []string         `json:"sensitive_domains"`
	DownURL           string           `json:"down_url"`
	UpURL             string           `json:"up_url"`
	Thresholds        Thresholds       `json:"thresholds"`
	UseUnwallHostlist bool             `json:"use_unwall_hostlist"`
	// Dismissed advice ids. Some recommendations answer a question rather than
	// describe a fault - whether a 100 Mbit link is expected, whether IPv6 is
	// wanted - and only the person at the keyboard can close those.
	Dismissed []string `json:"dismissed,omitempty"`
	// LastComparison is when an A/B last ran, so the suggestion to run one is
	// not repeated after every single measurement.
	LastComparison string `json:"last_comparison,omitempty"`
}

// Default returns the shipped configuration.
func Default() Config {
	return Config{
		Lang:              "auto",
		PingIntervalMs:    250,
		QuickProbes:       60,
		FullProbes:        200,
		PingTimeoutMs:     1500,
		ConnectTimeoutS:   6,
		DNSTimeoutS:       3,
		LoadSeconds:       8,
		IdleSeconds:       4,
		Streams:           4,
		SpeedBytes:        50_000_000, // Cloudflare's __down rejects much more
		MonitorInterval:   1000,
		MonitorDNSEvery:   30,
		MonitorReachEvery: 60,
		Anchors: []Anchor{
			{Label: "Cloudflare", Host: "1.1.1.1", Kind: "internet"},
			{Label: "Google", Host: "8.8.8.8", Kind: "internet"},
			{Label: "Quad9", Host: "9.9.9.9", Kind: "internet"},
		},
		Resolvers: []probe.Resolver{
			{Label: "Sistem / System", Kind: "system"},
			{Label: "Cloudflare UDP", Kind: "udp", Address: "1.1.1.1:53"},
			{Label: "Google UDP", Kind: "udp", Address: "8.8.8.8:53"},
			{Label: "Quad9 UDP", Kind: "udp", Address: "9.9.9.9:53"},
			{Label: "Cloudflare DoT", Kind: "dot", Address: "1.1.1.1:853", Hostname: "cloudflare-dns.com", Trusted: true},
			{Label: "Quad9 DoT", Kind: "dot", Address: "9.9.9.9:853", Hostname: "dns.quad9.net", Trusted: true},
			{Label: "Cloudflare DoH", Kind: "doh", Address: "https://cloudflare-dns.com/dns-query", Trusted: true},
			{Label: "Google DoH", Kind: "doh", Address: "https://dns.google/dns-query", Trusted: true},
			{Label: "Quad9 DoH", Kind: "doh", Address: "https://dns.quad9.net/dns-query", Trusted: true},
		},
		ControlDomains:   []string{"example.com", "cloudflare.com", "wikipedia.org", "github.com"},
		SensitiveDomains: []string{"discord.com", "gateway.discord.gg", "media.discordapp.net", "cdn.discordapp.com", "www.roblox.com", "x.com"},
		DownURL:          "https://speed.cloudflare.com/__down?bytes=50000000",
		UpURL:            "https://speed.cloudflare.com/__up",
		Thresholds: Thresholds{
			LossWarn: 0.5, LossBad: 2, JitterWarn: 8, JitterBad: 25,
			RTTWarn: 80, RTTBad: 150, DNSWarnMs: 80, DNSBadMs: 250,
			BloatWarn: 60, BloatBad: 200, RetransPct: 1,
		},
		UseUnwallHostlist: true,
	}
}

// Dir is where the config file lives.
func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "nabiz")
}

// DataDir is where runs and the monitor log are stored.
func DataDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "nabiz")
}

// RunsDir holds one JSON file per completed run.
func RunsDir() string { return filepath.Join(DataDir(), "runs") }

// ExportDir holds user-requested markdown/json exports.
func ExportDir() string { return filepath.Join(DataDir(), "exports") }

// MonitorLog is the append-only event log.
func MonitorLog() string { return filepath.Join(DataDir(), "monitor.jsonl") }

// File is the config path.
func File() string { return filepath.Join(Dir(), "config.json") }

// Load reads the user config over the defaults and merges in Unwall's hostlist.
func Load() Config {
	cfg := Default()
	if data, err := os.ReadFile(File()); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.UseUnwallHostlist {
		if extra := sysinfo.UnwallDomains(12); len(extra) > 0 {
			// built-ins first: they are known to resolve and serve TLS, so the
			// quick suite never wastes a probe on a dead hostlist entry
			cfg.SensitiveDomains = util.Uniq(append(cfg.SensitiveDomains, extra...))
			if len(cfg.SensitiveDomains) > 24 {
				cfg.SensitiveDomains = cfg.SensitiveDomains[:24]
			}
		}
	}
	return cfg
}

// Save writes the config back to disk.
func Save(cfg Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(File(), data, 0o644)
}

// LiveAnchors prepends the actual default gateway to the configured anchors.
func (c Config) LiveAnchors() []Anchor {
	var out []Anchor
	if _, gateway := util.DefaultRoute(); gateway != "" {
		out = append(out, Anchor{Label: "Modem / Gateway", Host: gateway, Kind: "gateway"})
	}
	return append(out, c.Anchors...)
}

// AnchorHosts is LiveAnchors reduced to addresses.
func (c Config) AnchorHosts() []string {
	anchors := c.LiveAnchors()
	out := make([]string, len(anchors))
	for i, anchor := range anchors {
		out[i] = anchor.Host
	}
	return out
}

// PrimaryAnchor is the first internet anchor, used for latency-under-load.
func (c Config) PrimaryAnchor() string {
	for _, anchor := range c.Anchors {
		return anchor.Host
	}
	return "1.1.1.1"
}

// LiveResolvers adds whatever is actually listening locally and on the router.
func (c Config) LiveResolvers() []probe.Resolver {
	var out []probe.Resolver
	for _, candidate := range []struct{ label, address string }{
		{"Unwall dnscrypt", "127.0.0.1:5300"},
		{"systemd-resolved stub", "127.0.0.53:53"},
	} {
		if udpListening(candidate.address) {
			out = append(out, probe.Resolver{Label: candidate.label, Kind: "udp", Address: candidate.address})
		}
	}
	if _, gateway := util.DefaultRoute(); gateway != "" {
		out = append(out, probe.Resolver{Label: "Modem / Gateway", Kind: "udp", Address: gateway + ":53"})
	}
	return append(out, c.Resolvers...)
}

// TrustedResolver returns the first resolver usable as ground truth.
func (c Config) TrustedResolver() probe.Resolver {
	for _, resolver := range c.Resolvers {
		if resolver.Trusted {
			return resolver
		}
	}
	return probe.Resolver{Label: "Cloudflare DoH", Kind: "doh",
		Address: "https://cloudflare-dns.com/dns-query", Trusted: true}
}

// Durations, kept as helpers so the rest of the code never juggles units.
func (c Config) PingInterval() time.Duration {
	return time.Duration(c.PingIntervalMs) * time.Millisecond
}
func (c Config) PingTimeout() time.Duration    { return time.Duration(c.PingTimeoutMs) * time.Millisecond }
func (c Config) ConnectTimeout() time.Duration { return time.Duration(c.ConnectTimeoutS) * time.Second }
func (c Config) DNSTimeout() time.Duration     { return time.Duration(c.DNSTimeoutS) * time.Second }
func (c Config) LoadDuration() time.Duration   { return time.Duration(c.LoadSeconds) * time.Second }
func (c Config) IdleDuration() time.Duration   { return time.Duration(c.IdleSeconds) * time.Second }
func (c Config) MonitorTick() time.Duration {
	return time.Duration(c.MonitorInterval) * time.Millisecond
}
