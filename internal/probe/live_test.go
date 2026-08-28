package probe

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/util"
)

// Live tests talk to the real internet. They are skipped unless NABIZ_LIVE=1
// so `go test ./...` stays hermetic.
func requireLive(t *testing.T) {
	t.Helper()
	if os.Getenv("NABIZ_LIVE") != "1" {
		t.Skip("set NABIZ_LIVE=1 to run live network tests")
	}
}

func TestLivePingSweep(t *testing.T) {
	requireLive(t)
	pinger, err := NewPinger(1500 * time.Millisecond)
	if err != nil {
		t.Fatalf("icmp socket: %v", err)
	}
	defer pinger.Close()
	results := pinger.Sweep(context.Background(),
		[]string{"1.1.1.1", "8.8.8.8", "192.168.0.1"}, 12, 150*time.Millisecond, nil)
	for target, samples := range results {
		summary := stats.Summarize(target, target, samples)
		t.Logf("%-14s loss=%5.1f%% avg=%7.2f p95=%7.2f jit=%5.2f %s",
			target, summary.LossPct, summary.Avg, summary.P95, summary.Jitter,
			stats.Sparkline(samples, 12))
		if summary.Received == 0 {
			t.Errorf("%s: no replies at all", target)
		}
	}
}

func TestLiveDNS(t *testing.T) {
	requireLive(t)
	ctx := context.Background()
	resolvers := []Resolver{
		{Label: "system", Kind: "system"},
		{Label: "cloudflare-udp", Kind: "udp", Address: "1.1.1.1"},
		{Label: "quad9-udp", Kind: "udp", Address: "9.9.9.9"},
		{Label: "cloudflare-dot", Kind: "dot", Address: "1.1.1.1:853", Hostname: "cloudflare-dns.com", Trusted: true},
		{Label: "quad9-dot", Kind: "dot", Address: "9.9.9.9:853", Hostname: "dns.quad9.net", Trusted: true},
		{Label: "cloudflare-doh", Kind: "doh", Address: "https://cloudflare-dns.com/dns-query", Trusted: true},
		{Label: "google-doh", Kind: "doh", Address: "https://dns.google/dns-query", Trusted: true},
		{Label: "quad9-doh", Kind: "doh", Address: "https://dns.quad9.net/dns-query", Trusted: true},
	}
	for _, resolver := range resolvers {
		res := Query(ctx, resolver, "discord.com", "A", 3*time.Second, false)
		t.Logf("%-16s %-7s %7.1fms %s", resolver.Label, res.Transport, res.Ms, res.Summary())
		if !res.OK {
			t.Errorf("%s failed: %s", resolver.Label, res.Summary())
		}
	}
	t.Logf("system=%v upstream=%v", SystemResolvers(), UpstreamResolvers())
	for _, check := range []Check{
		CheckTransparentRedirect(ctx, 2*time.Second),
		CheckNXDOMAINHijack(ctx, 3*time.Second),
		CheckDNSSEC(ctx, resolvers[3], 4*time.Second),
		CheckUDPvsTCP(ctx, "1.1.1.1", "discord.com", 3*time.Second),
		CheckInjection(ctx, "1.1.1.1", "discord.com", 2*time.Second),
		CheckEDNS(ctx, resolvers[1], 3*time.Second),
	} {
		t.Logf("%-20s %-5s %s", check.Name, check.Verdict, check.Detail)
	}
}

func TestLiveDPI(t *testing.T) {
	requireLive(t)
	ctx := context.Background()
	for _, domain := range []string{"example.com", "discord.com", "media.discordapp.net", "www.roblox.com"} {
		v := ProbeDomain(ctx, domain, "", "system", 6*time.Second, true, true)
		t.Logf("%-24s %-16s ip=%-15s tls=%-6s %6.0fms alpn=%-8s cn=%-24s quic=%-20s http80=%s",
			domain, v.Verdict, v.IP, v.Whole.Kind, v.Whole.Ms, v.Whole.ALPN,
			v.Whole.PeerCN, v.QUIC, v.HTTP80)
	}
}

func TestSplitConnActuallySplits(t *testing.T) {
	requireLive(t)
	ctx := context.Background()
	addrs := []string{"1.1.1.1"}
	for _, mode := range []string{"none", "header", "sni"} {
		res := TLSHandshake(ctx, addrs[0], "cloudflare-dns.com", 443, 6*time.Second, mode, false)
		t.Logf("mode=%-7s ok=%-5v kind=%-6s hello=%d splitAt=%d %s",
			mode, res.OK, res.Kind, res.HelloSize, res.SplitAt, res.Version)
		if !res.OK {
			t.Errorf("mode %s should still complete against an unfiltered host: %s", mode, res.Err)
		}
		if mode != "none" && res.SplitAt == 0 {
			t.Errorf("mode %s did not split", mode)
		}
	}
}

func TestLivePath(t *testing.T) {
	requireLive(t)
	ctx := context.Background()
	hops := Traceroute(ctx, "1.1.1.1", 15, 3, 1200*time.Millisecond, false, nil)
	if len(hops) == 0 {
		t.Fatal("no hops returned")
	}
	for _, hop := range hops {
		ip := hop.IP
		if ip == "" {
			ip = "*"
		}
		t.Logf("%2d  %-18s loss=%5.1f%% best=%7.2f avg=%7.2f worst=%7.2f dest=%v",
			hop.TTL, ip, hop.LossPct(), hop.Best(), hop.Avg(), hop.Worst(), hop.IsDest)
	}
	if !hops[len(hops)-1].IsDest {
		t.Errorf("last hop is not the destination")
	}
	mtu := ProbePMTU(ctx, "1.1.1.1", 1200, 1500, time.Second)
	t.Logf("mtu iface=%d probed=%d kernel=%d blackhole=%v %s",
		mtu.IfaceMTU, mtu.ProbedMTU, mtu.KernelPMTU, mtu.Blackhole, mtu.Detail)
}

func TestLiveSockDiag(t *testing.T) {
	requireLive(t)
	// generate a socket to look at
	go func() {
		_ = HTTPProbe(context.Background(), "1.1.1.1", "cloudflare.com", 3*time.Second)
	}()
	time.Sleep(300 * time.Millisecond)
	sockets, err := TCPSockets("")
	if err != nil {
		t.Fatalf("netlink: %v", err)
	}
	t.Logf("%d established sockets", len(sockets))
	for i, socket := range sockets {
		if i >= 6 {
			break
		}
		t.Logf("%-24s cc=%-8s rtt=%6.2fms min=%6.2f cwnd=%-5d mss=%-5d retrans=%-4d rate=%7.2fMbps",
			socket.Remote, socket.CC, socket.RTTms, socket.MinRTTms, socket.CWnd,
			socket.SndMSS, socket.TotalRetrans, socket.DeliveryMbps())
	}
	summary := Summarize(sockets)
	t.Logf("summary: cc=%v avgRTT=%.2f avgCwnd=%.0f retrans=%.3f%%",
		summary.CCAlgorithms, summary.AvgRTTms, summary.AvgCWnd, summary.RetransPct)
	health := ReadSNMP().Health()
	t.Logf("tcp health: out=%d retrans=%d (%.2f%%) timeouts=%d ofo=%d prune=%d",
		health.OutSegs, health.RetransSegs, health.RetransPct, health.Timeouts,
		health.OFOQueue, health.PruneCalled)
}

func TestLiveBufferbloat(t *testing.T) {
	requireLive(t)
	result := Bufferbloat(context.Background(), BloatOptions{
		DownURL:      "https://speed.cloudflare.com/__down?bytes=50000000",
		UpURL:        "https://speed.cloudflare.com/__up",
		Anchor:       "1.1.1.1",
		SocketFilter: ":443",
		Duration:     6 * time.Second,
		IdleDuration: 3 * time.Second,
		Streams:      4,
	})
	t.Logf("idle p50=%.1f loss=%.1f%%", result.Idle.P50, result.Idle.LossPct)
	if result.Download != nil {
		t.Logf("down %.2f Mbps  p95=%.1f (+%.1f)  cc=%v cwnd=%.0f retrans=%.3f%% snmp_retrans=%d",
			result.Download.Bps/1e6, result.DownLatency.P95, result.DownDelta,
			result.Download.Sockets.CCAlgorithms, result.Download.Sockets.AvgCWnd,
			result.Download.Sockets.RetransPct, result.Download.SNMPDelta.RetransSegs)
	}
	if result.Upload != nil {
		t.Logf("up   %.2f Mbps  p95=%.1f (+%.1f)  err=%q",
			result.Upload.Bps/1e6, result.UpLatency.P95, result.UpDelta, result.Upload.Err)
	}
	t.Logf("grade %s", result.Grade)
}

func TestLiveLinkHistory(t *testing.T) {
	requireLive(t)
	history := ReadLinkHistory("")
	t.Logf("available=%v drops=%d downshifts=%d down=%.1fs longest=%.1fs gap=%.1fdk span=%.0fdk down%%=%.2f",
		history.Available, history.Drops, history.Downshifts, history.DownSeconds,
		history.LongestDown, history.MeanGapMin, history.SpanMinutes(), history.DownPct())
	t.Logf("downshift: %s", history.DownshiftNote)
	t.Logf("eee: %+v", ReadEEE("enp3s0"))
}

func TestLiveBootHistory(t *testing.T) {
	requireLive(t)
	boots := ReadBootHistory("", 6)
	for _, boot := range boots {
		t.Logf("boot %-3d kernel=%-18s %5.0f dk  düşme=%-3d (%.1f/saat) downshift=%-3d truncated=%v",
			boot.Index, boot.Kernel, boot.Minutes(), boot.Drops, boot.DropsPerHour(),
			boot.Downshifts, boot.Truncated)
	}
	text, recent := RegressionHint(boots)
	t.Logf("regression hint: recent=%v %s", recent, text)
}

func TestLiveKernelRegression(t *testing.T) {
	requireLive(t)
	reboots := ReadReboots(30)
	for i, reboot := range reboots {
		if i >= 8 {
			break
		}
		t.Logf("%-20s %s  uptime=%s current=%v", reboot.Kernel,
			reboot.At.Format("2006-01-02 15:04"), reboot.Uptime.Round(time.Minute), reboot.Current)
	}
	boots := ReadBootHistory("", 6)
	stats := StabilityByKernel(reboots, boots)
	for _, entry := range stats {
		t.Logf("%-20s boots=%-3d toplam=%6.1f saat  ölçülen=%5.1f saat  düşme=%-4d (%.1f/saat) measured=%v",
			entry.Kernel, entry.Boots, entry.Hours, entry.CoveredHrs, entry.Drops,
			entry.DropsPerHr, entry.Measured)
	}
	running := strings.TrimSpace(util.ReadText("/proc/sys/kernel/osrelease", ""))
	good, detail, regressed := KernelRegression(stats, running)
	t.Logf("REGRESSION: %v  iyi=%s  %s", regressed, good, detail)
}

func TestLiveIPv6AndFirewall(t *testing.T) {
	requireLive(t)
	status := CheckIPv6(context.Background(), 3*time.Second)
	t.Logf("ipv6: addr=%v (%s) ping=%v tcp=%v aaaa=%v err=%s working=%v broken=%v",
		status.HasAddress, status.GlobalAddr, status.PingOK, status.TCPOK,
		status.DNSHasAAAA, status.Err, status.Working(), status.Broken())
	fw := CheckFirewallICMP()
	t.Logf("firewall icmp: available=%v total=%d frag-needed=%d time-exceeded=%d",
		fw.Available, fw.BlockedTotal, fw.BlockedFrag, fw.BlockedTTL)
}
