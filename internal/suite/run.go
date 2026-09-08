package suite

import (
	"context"
	"runtime"
	"sort"
	"time"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// Progress is called as a run advances; fraction is 0..1.
type Progress func(phase string, fraction float64)

// Names lists the runnable suites in menu order.
var Names = []string{"quick", "full", "dns", "dpi", "path", "load", "deep"}

// Describe returns the (tr, en) one-liner for a suite.
func Describe(name string) (string, string) {
	switch name {
	case "doctor":
		// environment only: no packets, no load, answers in under a second
	case "quick":
		return "Modem ve internet çıpalarına ping, DNS çözücü kıyaslaması, temel müdahale kontrolleri ve birkaç alan adında TLS testi. ~45 saniye.",
			"Pings the modem and internet anchors, benchmarks resolvers, runs the basic interference checks and TLS-probes a few domains. ~45 s."
	case "full":
		return "Hızlı testin tamamı artı yol analizi, yol MTU'su, indirme/yükleme hızı ve yük altında gecikme. 2-3 dakika sürer ve hattı doldurur.",
			"Everything in quick plus traceroute, path MTU, throughput and latency-under-load. 2-3 minutes, saturates the line."
	case "dns":
		return "Aynı sorular düz UDP, TCP, DoT ve DoH üzerinden; şifrelemenin gecikme maliyeti, kaçırma ve şeffaf yönlendirme kontrolleri.",
			"The same questions over plain UDP, TCP, DoT and DoH; the latency cost of encryption plus hijack and interception checks."
	case "dpi":
		return "Her alan adı için TCP, bütün ClientHello, iki bölünmüş varyant, farklı SNI ve QUIC denemesi; engelin türünü ayırır.",
			"Per domain: TCP, whole ClientHello, two split variants, a different SNI and a QUIC probe - enough to tell block types apart."
	case "path":
		return "Atlama atlama kayıp ve gecikme, ardından gerçek yol MTU'su.",
			"Per-hop loss and latency, then the real path MTU."
	case "load":
		return "Boşta gecikme, sonra hattı doldururken gecikme, tıkanıklık penceresi ve yeniden gönderim oranı.",
			"Idle latency, then latency, congestion window and retransmission rate while the line is saturated."
	case "deep":
		return "Tam test artı çekirdek sayaçları, soket başına TCP durumu ve bpftune etkisi - her şeyi köküne kadar ölçer. 3-4 dakika.",
			"The full test plus kernel counters, per-socket TCP state and bpftune impact - measures everything down to the root. 3-4 minutes."
	}
	return "", ""
}

// SnapshotEnv gathers the machine-side picture.
func SnapshotEnv(cfg config.Config) Env {
	host, _ := runtimeHostname()
	nfq := probe.ReadNFQueues()
	env := Env{
		Host:        host,
		Kernel:      kernelRelease(),
		Link:        probe.ReadLink(""),
		LinkLog:     probe.ReadLinkHistory(""),
		NFQueue:     nfq,
		Conntrack:   probe.ReadConntrack(),
		Sysctls:     probe.ReadSysctls(),
		TCPHealth:   probe.ReadSNMP().Health(),
		Unwall:      sysinfo.ReadUnwall(),
		Bpftune:     sysinfo.ReadBpftune().WithAppliedChanges(config.ChangesInForce()),
		SystemDNS:   probe.SystemResolvers(),
		UpstreamDNS: probe.UpstreamResolvers(),
		DNSPaths:    probe.UpstreamDetail(),
	}
	env.EEE = probe.ReadEEE(env.Link.Iface)
	env.ASPM = probe.ReadASPM(env.Link.Iface)
	env.Journal = probe.ReadJournalStorage()
	env.IPv6 = probe.CheckIPv6(context.Background(), 3*time.Second)
	env.Firewall = probe.CheckFirewallICMP()
	// comparing boots is the only way to tell "this always happened" from
	// "this started recently", which decides where to look next
	env.LinkBoots = probe.ReadBootHistory(env.Link.Iface, 6)
	if data, recent := probe.RegressionOf(env.LinkBoots); recent {
		env.LinkRegData = &data
	}
	// wtmp survives journal rotation and records the kernel of every boot, so
	// it is the only source that can say "this started with a kernel update"
	env.Reboots = probe.ReadReboots(40)
	env.KernelStats = probe.StabilityByKernel(env.Reboots, env.LinkBoots)
	if data, regressed := probe.KernelRegressionOf(env.KernelStats, env.Kernel); regressed {
		env.KernRegData = &data
		env.GoodKernel = data.GoodKernel
	}
	if sockets, err := probe.TCPSockets(""); err == nil {
		env.Sockets = probe.Summarize(sockets)
	}
	return env
}

func runtimeHostname() (string, error) {
	name, err := osHostname()
	return name, err
}

func kernelRelease() string {
	release := util.ReadText("/proc/sys/kernel/osrelease", "")
	if release == "" {
		return runtime.GOOS
	}
	return trimNewline(release)
}

func trimNewline(value string) string {
	for len(value) > 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == '\r') {
		value = value[:len(value)-1]
	}
	return value
}

// MeasureLatency pings every anchor and returns one summary per anchor.
func MeasureLatency(ctx context.Context, cfg config.Config, count int,
	progress Progress, base, span float64,
) []stats.Summary {
	anchors := cfg.LiveAnchors()
	hosts := make([]string, len(anchors))
	for i, anchor := range anchors {
		hosts[i] = anchor.Host
	}
	pinger, err := probe.NewPinger(cfg.PingTimeout())
	if err != nil {
		return nil
	}
	defer pinger.Close()

	total := float64(count * len(hosts))
	done := 0
	raw := pinger.Sweep(ctx, hosts, count, cfg.PingInterval(),
		func(string, int, stats.Sample) {
			done++
			if progress != nil && done%5 == 0 {
				progress("ping", base+span*float64(done)/total)
			}
		})
	out := make([]stats.Summary, 0, len(anchors))
	for _, anchor := range anchors {
		out = append(out, stats.Summarize(anchor.Label, anchor.Host, raw[anchor.Host]))
	}
	return out
}

// BenchmarkDNS times every reachable resolver over the same question set.
func BenchmarkDNS(ctx context.Context, cfg config.Config, domains []string,
	progress Progress, base, span float64,
) []DNSRow {
	if len(domains) == 0 {
		domains = append(append([]string{}, cfg.ControlDomains[:2]...), cfg.SensitiveDomains[:2]...)
	}
	resolvers := cfg.LiveResolvers()
	rows := make([]DNSRow, 0, len(resolvers))
	for index, resolver := range resolvers {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress("dns:"+resolver.Label, base+span*float64(index)/float64(len(resolvers)))
		}
		row := DNSRow{
			Label: resolver.Label, Kind: resolver.Kind, Encrypted: resolver.Encrypted(),
			Address: resolver.Address, Queries: len(domains),
		}
		if row.Address == "" {
			row.Address = "-"
		}
		var times []float64
		for _, domain := range domains {
			result := probe.Query(ctx, resolver, domain, "A", cfg.DNSTimeout(), false)
			if result.OK && len(result.Addrs) > 0 {
				times = append(times, result.Ms)
			} else {
				row.Failures++
			}
		}
		if len(times) > 0 {
			row.AvgMs = round1(stats.Mean(times))
			row.P95Ms = round1(stats.Percentile(times, 95))
			row.MinMs = round1(minOf(times))
		}
		rows = append(rows, row)
	}
	return rows
}

// DNSIntegrity runs the tamper checks and the system-versus-trusted comparison.
func DNSIntegrity(ctx context.Context, cfg config.Config, progress Progress,
	base, span float64,
) ([]probe.Check, []probe.AnswerComparison) {
	trusted := cfg.TrustedResolver()
	system := "1.1.1.1"
	if servers := probe.SystemResolvers(); len(servers) > 0 {
		system = servers[0]
	}
	steps := []struct {
		name string
		run  func() probe.Check
	}{
		{"dns:redirect", func() probe.Check { return probe.CheckTransparentRedirect(ctx, cfg.DNSTimeout()) }},
		{"dns:nxdomain", func() probe.Check { return probe.CheckNXDOMAINHijack(ctx, cfg.DNSTimeout()) }},
		{"dns:udp-tcp", func() probe.Check { return probe.CheckUDPvsTCP(ctx, system, "cloudflare.com", cfg.DNSTimeout()) }},
		{"dns:injection", func() probe.Check { return probe.CheckInjection(ctx, system, "discord.com", cfg.DNSTimeout()) }},
		{"dns:edns", func() probe.Check { return probe.CheckEDNS(ctx, trusted, cfg.DNSTimeout()) }},
		{"dns:dnssec", func() probe.Check { return probe.CheckDNSSEC(ctx, trusted, cfg.DNSTimeout()+time.Second) }},
	}
	checks := make([]probe.Check, 0, len(steps))
	for index, step := range steps {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress(step.name, base+span*0.6*float64(index)/float64(len(steps)))
		}
		checks = append(checks, step.run())
	}

	var comparisons []probe.AnswerComparison
	limit := 4
	if len(cfg.SensitiveDomains) < limit {
		limit = len(cfg.SensitiveDomains)
	}
	for index, domain := range cfg.SensitiveDomains[:limit] {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress("dns:compare", base+span*(0.6+0.4*float64(index)/float64(limit)))
		}
		comparisons = append(comparisons, probe.CompareAnswers(ctx, domain, trusted, cfg.DNSTimeout()))
	}
	return checks, comparisons
}

// ProbeDPI walks the domain list through the full block-classification ladder.
func ProbeDPI(ctx context.Context, cfg config.Config, domains []string, withHTTP bool,
	progress Progress, base, span float64,
) []probe.DomainVerdict {
	if len(domains) == 0 {
		domains = append(append([]string{}, cfg.SensitiveDomains...), cfg.ControlDomains[:2]...)
	}
	out := make([]probe.DomainVerdict, 0, len(domains))
	for index, domain := range domains {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress("dpi:"+domain, base+span*float64(index)/float64(len(domains)))
		}
		out = append(out, probe.ProbeDomain(ctx, domain, "", "system",
			cfg.ConnectTimeout(), true, withHTTP))
	}
	return out
}

// RunLoad performs the latency-under-load measurement.
func RunLoad(ctx context.Context, cfg config.Config, progress Progress,
	base, span float64,
) *probe.BloatResult {
	phases := map[string]float64{"idle": 0.15, "download": 0.55, "upload": 0.9}
	result := probe.Bufferbloat(ctx, probe.BloatOptions{
		DownURL:      cfg.DownURL,
		UpURL:        cfg.UpURL,
		Anchor:       cfg.PrimaryAnchor(),
		SocketFilter: ":443",
		Duration:     cfg.LoadDuration(),
		IdleDuration: cfg.IdleDuration(),
		Streams:      cfg.Streams,
		OnPhase: func(phase string, _ float64) {
			if progress != nil {
				progress("load:"+phase, base+span*phases[phase])
			}
		},
	})
	return &result
}

// Run executes one named suite end to end.
func Run(ctx context.Context, cfg config.Config, name string, progress Progress) Result {
	start := time.Now()
	result := Result{Name: name, StartedAt: start}
	step := func(phase string, fraction float64) {
		if progress != nil {
			progress(phase, fraction)
		}
	}
	step("env", 0)
	result.Env = SnapshotEnv(cfg)

	switch name {
	case "doctor":
		// environment only: no packets, no load, answers in under a second
	case "quick":
		result.Latency = MeasureLatency(ctx, cfg, cfg.QuickProbes, progress, 0.05, 0.45)
		result.DNSBench = BenchmarkDNS(ctx, cfg, cfg.ControlDomains[:2], progress, 0.5, 0.2)
		result.DNSChecks, result.DNSCompare = DNSIntegrity(ctx, cfg, progress, 0.7, 0.1)
		result.DPI = ProbeDPI(ctx, cfg, append(firstN(cfg.SensitiveDomains, 3),
			firstN(cfg.ControlDomains, 1)...), false, progress, 0.8, 0.2)
	case "dns":
		result.DNSBench = BenchmarkDNS(ctx, cfg, nil, progress, 0, 0.6)
		result.DNSChecks, result.DNSCompare = DNSIntegrity(ctx, cfg, progress, 0.6, 0.4)
	case "dpi":
		result.DPI = ProbeDPI(ctx, cfg, nil, true, progress, 0, 1)
	case "path":
		step("path", 0.05)
		result.Hops = probe.Traceroute(ctx, cfg.PrimaryAnchor(), 20, 3, 1200*time.Millisecond, false, nil)
		step("mtu", 0.8)
		mtu := probe.ProbePMTU(ctx, cfg.PrimaryAnchor(), 1200, 1500, time.Second)
		result.MTU = &mtu
	case "load":
		result.Latency = MeasureLatency(ctx, cfg, 40, progress, 0, 0.1)
		result.Load = RunLoad(ctx, cfg, progress, 0.1, 0.9)
	case "full", "deep":
		result.Latency = MeasureLatency(ctx, cfg, cfg.FullProbes, progress, 0.02, 0.25)
		if ctx.Err() == nil {
			result.DNSBench = BenchmarkDNS(ctx, cfg, nil, progress, 0.27, 0.12)
		}
		if ctx.Err() == nil {
			result.DNSChecks, result.DNSCompare = DNSIntegrity(ctx, cfg, progress, 0.39, 0.06)
		}
		if ctx.Err() == nil {
			result.DPI = ProbeDPI(ctx, cfg, nil, true, progress, 0.45, 0.2)
		}
		if ctx.Err() == nil {
			step("path", 0.65)
			result.Hops = probe.Traceroute(ctx, cfg.PrimaryAnchor(), 18, 3, 1200*time.Millisecond, false, nil)
			step("mtu", 0.72)
			mtu := probe.ProbePMTU(ctx, cfg.PrimaryAnchor(), 1200, 1500, time.Second)
			result.MTU = &mtu
		}
		if ctx.Err() == nil {
			result.Load = RunLoad(ctx, cfg, progress, 0.75, 0.24)
		}
	}

	// re-read the counters so the deltas cover the whole run
	result.Env.TCPHealth = probe.ReadSNMP().Health()
	result.Env.NFQueue = probe.ReadNFQueues()
	if sockets, err := probe.TCPSockets(""); err == nil {
		result.Env.Sockets = probe.Summarize(sockets)
	}
	result.Duration = time.Since(start).Seconds()
	result.Cancelled = ctx.Err() != nil
	Finalize(&result, cfg)
	step("done", 1)
	return result
}

func firstN(items []string, n int) []string {
	if len(items) < n {
		n = len(items)
	}
	return append([]string{}, items[:n]...)
}

func minOf(values []float64) float64 {
	out := values[0]
	for _, value := range values {
		if value < out {
			out = value
		}
	}
	return out
}

func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

// SortAdvice orders recommendations by priority then category.
func SortAdvice(items []Advice) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].Category < items[j].Category
	})
}
