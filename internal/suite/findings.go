package suite

import (
	"sort"
	"strconv"
	"strings"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/sysinfo"
)

// Finalize derives findings, advice and the score from a completed run. It is
// the single place where "is this a problem" is decided, so the CLI and the TUI
// can never disagree - and it is cheap enough to re-run when the interface
// language changes.
func Finalize(result *Result, cfg config.Config) {
	result.Lang = string(i18n.Current())
	result.Findings = deriveFindings(*result, cfg)
	result.Advice = GenerateAdvice(*result, cfg)
	result.Score, result.Grade = scoreRun(*result)
}

// Refresh regenerates the derived text without re-measuring, which is what a
// language switch needs.
func Refresh(result *Result, cfg config.Config) { Finalize(result, cfg) }

// RefreshEnv re-reads the machine and re-derives, keeping the measurements.
//
// Applying a recommendation changes the environment, never the packets that
// were already sent: the latency, DPI and load figures stay valid while the
// sysctls, service states and hostlists they were judged against do not. Without
// this, advice is derived from a stored snapshot and an applied change never
// leaves the list, however many times it is applied.
func RefreshEnv(result *Result, cfg config.Config) {
	measured := result.Env
	result.Env = SnapshotEnv(cfg)
	// the counters below describe the run, not the machine, so they are carried
	// forward rather than replaced with the current totals
	result.Env.TCPHealth = measured.TCPHealth
	Finalize(result, cfg)
}

type findingList struct {
	items []Finding
}

// add appends a finding whose title and hint come from the catalog. The hint is
// looked up automatically as "<key>.hint" and skipped when absent.
func (f *findingList) add(level, key, source string, args ...any) {
	f.items = append(f.items, Finding{
		Level: level, Key: key, Source: source,
		Title: i18n.T("fnd."+key+".title", args...),
		Hint:  optional("fnd." + key + ".hint"),
	})
}

// addText is for findings whose title is already composed (regressions, notes).
func (f *findingList) addText(level, key, source, title, hint string) {
	f.items = append(f.items, Finding{Level: level, Key: key, Source: source,
		Title: title, Hint: hint})
}

func optional(key string) string {
	if !i18n.Has(key) {
		return ""
	}
	return i18n.T(key)
}

// causes says which finding explains which. The left side is a symptom, the
// right side the thing to actually fix; a symptom whose cause is not in this
// run stands on its own.
var causes = map[string]string{
	"link-downshift":  "link-drops",
	"link-speed":      "link-downshift",
	"eee-active":      "link-drops",
	"eee-unknown":     "link-drops",
	"aspm-blocked":    "link-drops",
	"aspm-unverified": "link-drops",
	"carrier-flaps":   "link-drops",
	"link-regression": "link-drops",
	"loss-burst":      "link-drops",
	"bpftune-buffers": "bufferbloat-bad",
	"qdisc-drops":     "bufferbloat-bad",
}

// linkCauses attaches each symptom to the finding that explains it, but only
// when that finding is actually present: on a link that never dropped, a 100
// Mbit negotiation is its own problem and should be reported as one.
func linkCauses(items []Finding) []Finding {
	present := make(map[string]bool, len(items))
	for _, finding := range items {
		present[finding.Key] = true
	}
	for index, finding := range items {
		if cause, ok := causes[finding.Key]; ok && present[cause] && cause != finding.Key {
			items[index].Because = cause
		}
	}
	return items
}

func deriveFindings(result Result, cfg config.Config) []Finding {
	var f findingList
	limits := cfg.Thresholds
	link := result.Env.Link
	history := result.Env.LinkLog

	// --- physical layer ------------------------------------------------
	if link.SpeedMbit > 0 && link.SpeedMbit <= 100 && !link.Wireless {
		f.add("warn", "link-speed", "link", link.Iface, link.SpeedMbit)
	}
	if link.Duplex != "" && link.Duplex != "full" {
		f.add("bad", "duplex", "link", link.Duplex)
	}

	if history.Available && history.Drops > 0 {
		// the kernel log turns a counter into evidence: when, how long, and why
		title := i18n.T("fnd.link-drops.title", history.Drops)
		if span := history.SpanMinutes(); span > 0 {
			title += i18n.T("fnd.link-drops.span", span, history.MeanGapMin)
		}
		if history.DownSeconds > 0 && history.SpanMinutes() > 0 {
			title += i18n.T("fnd.link-drops.down", history.DownSeconds,
				history.DownPct(), history.LongestDown)
		}
		level := "warn"
		if history.Drops > 5 || history.DownPct() > 1 {
			level = "bad"
		}
		hint := i18n.T("fnd.link-drops.hint")
		// A change made mid-boot cannot lower a count that only goes up, so say
		// how long it has been quiet and stop calling a settled link a fault.
		if history.Settled() {
			level = "info"
			title += i18n.T("fnd.link-drops.quiet", history.QuietMinutes())
			hint = i18n.T("fnd.link-drops.quiet-hint")
		}
		f.addText(level, "link-drops", "link", title, hint)
	} else if link.CarrierUps > 3 {
		// no kernel log to date it against, so the counter is all there is
		f.add("bad", "carrier-flaps", "link", link.CarrierUps)
	}

	// Both regressions describe drops that have already happened. Once the link
	// has settled they are history, not a fault to act on - and acting on them
	// means rolling back the kernel that is currently behaving.
	if detail := kernelRegressionText(result.Env); detail != "" && !history.Settled() {
		f.addText("bad", "kernel-regression", "kernel",
			i18n.T("fnd.kernel-regression.title", detail),
			i18n.T("fnd.kernel-regression.hint"))
	}
	if detail := linkRegressionText(result.Env); detail != "" && !history.Settled() {
		f.addText("bad", "link-regression", "link",
			i18n.T("fnd.link-regression.title", detail),
			i18n.T("fnd.link-regression.hint"))
	}
	if history.Downshifts > 0 {
		// same cumulative-counter problem as the drops above
		level := "bad"
		if history.Settled() {
			level = "info"
		}
		f.add(level, "link-downshift", "link", history.Downshifts, history.DownshiftNote)
	}
	flapping := history.Drops > 3 || link.CarrierUps > 3
	if result.Env.EEE.Active && flapping {
		f.add("warn", "eee-active", "link")
	} else if !result.Env.EEE.Checked && flapping {
		// EEE is a leading suspect for a flapping link, so not being able to
		// read it has to be said out loud rather than passing for "not a factor"
		f.add("warn", "eee-unknown", "link", result.Env.EEE.Reason)
	}
	// The driver tried to switch PCIe power saving off for this card and the
	// firmware would not let it. On its own that proves nothing: firmware that
	// never had ASPM on refuses the same call, and the card is then already in
	// the state the driver wanted. Only the Link Control register settles it,
	// and reading it needs root - so an unprivileged run says so instead of
	// picking whichever answer is more alarming.
	if aspm := result.Env.ASPM; aspm.Refused() {
		level := "warn"
		if history.Drops > 3 || link.CarrierUps > 3 {
			level = "bad"
		}
		f.add(level, "aspm-blocked", "link", aspm.Driver, aspm.Slot)
	} else if aspm.Blocked && !aspm.StateKnown {
		f.add("info", "aspm-unverified", "link", aspm.Driver)
	}
	// Say out loud when the kernel question cannot be answered yet, rather than
	// letting an unlogged kernel pass for a quiet one.
	if journal := result.Env.Journal; !journal.Persistent &&
		(history.Drops > 0 || link.CarrierUps > 1) {
		f.add("warn", "journal-volatile", "kernel", journal.Boots)
	}
	if link.Wireless && link.SignalDBm < -70 && link.SignalDBm != 0 {
		f.add("warn", "wifi-signal", "link", link.SignalDBm)
	}
	if worst, count := worstErrorCounter(link.Stats); count > 0 {
		total := link.Stats["rx_packets"] + link.Stats["tx_packets"]
		if total < 1 {
			total = 1
		}
		ratio := 100 * float64(count) / float64(total)
		level := "info"
		if ratio > 0.01 {
			level = "warn"
		}
		if worst == "rx_crc_errors" {
			level = "bad"
		}
		f.items = append(f.items, Finding{
			Level: level, Key: "nic-errors", Source: "link",
			Title: i18n.T("fnd.nic-errors.title", worst, count, ratio),
			Hint:  counterHint(worst),
		})
	}
	if link.QdiscStats.Drops > 0 {
		f.add("info", "qdisc-drops", "link", link.Qdisc, link.QdiscStats.Drops)
	}

	// --- kernel counters ------------------------------------------------
	health := result.Env.TCPHealth
	if health.RetransPct >= limits.RetransPct && health.OutSegs > 5000 {
		level := "warn"
		if health.RetransPct >= 3 {
			level = "bad"
		}
		f.add(level, "tcp-retrans", "kernel", health.RetransPct,
			health.RetransSegs, health.OutSegs)
	}
	if health.Timeouts > 100 {
		f.add("info", "tcp-timeouts", "kernel", health.Timeouts)
	}
	if health.OFOQueue > 1000 {
		f.add("info", "tcp-ofo", "kernel", health.OFOQueue)
	}
	if conntrack := result.Env.Conntrack; conntrack.Max > 0 && conntrack.Count > 0 {
		if ratio := float64(conntrack.Count) / float64(conntrack.Max); ratio > 0.8 {
			f.add("bad", "conntrack", "kernel", ratio*100)
		}
	}

	// --- firewall / ipv6 --------------------------------------------------
	if result.Env.Firewall.BlockedFrag > 0 {
		f.add("bad", "fw-icmp", "kernel", result.Env.Firewall.BlockedFrag)
	}
	switch {
	case result.Env.IPv6.Broken():
		f.add("warn", "ipv6-broken", "net")
	case !result.Env.IPv6.HasAddress && result.Env.IPv6.DNSHasAAAA:
		f.add("info", "ipv6-missing", "net")
	}

	// --- unwall / bpftune notes -------------------------------------------
	var nfqDrops int64
	for _, queue := range result.Env.NFQueue.Queues {
		nfqDrops += queue.QueueDropped + queue.UserDropped
	}
	for _, note := range sysinfo.AssessUnwall(result.Env.Unwall, nfqDrops,
		result.Env.NFQueue.Available) {
		f.addText(note.Level, note.Key, note.Source, note.Text, note.Hint)
	}
	if leak := sysinfo.UnwallDNSLeak(result.Env.Unwall, result.Env.DNSPaths); leak != nil {
		f.addText(leak.Level, leak.Key, leak.Source, leak.Text, leak.Hint)
	}
	if !result.Env.NFQueue.Available && result.Env.NFQueue.Reason == "root" &&
		result.Env.Unwall.Running {
		f.add("info", "nfqueue-root", "unwall")
	}
	rtt := 0.0
	if internet := result.InternetLatency(); internet != nil {
		rtt = internet.Avg
	}
	for _, note := range sysinfo.AssessBpftune(result.Env.Bpftune, link.SpeedMbit, rtt,
		health.RetransPct) {
		f.addText(note.Level, note.Key, note.Source, note.Text, note.Hint)
	}
	for _, note := range sysctlFindings(result.Env.Sysctls, link) {
		f.addText(note.Level, note.Key, "kernel", note.Text, "")
	}

	// --- latency ------------------------------------------------------------
	internet := result.InternetLatency()
	gateway := result.GatewayLatency()
	if internet != nil {
		switch {
		case internet.LossPct >= limits.LossBad:
			f.add("bad", "loss", "latency", internet.LossPct, internet.Target)
		case internet.LossPct >= limits.LossWarn:
			f.add("warn", "loss-small", "latency", internet.LossPct, internet.Target)
		}
		switch {
		case internet.Jitter >= limits.JitterBad:
			f.add("bad", "jitter", "latency", internet.Jitter)
		case internet.Jitter >= limits.JitterWarn:
			f.add("warn", "jitter-small", "latency", internet.Jitter)
		}
		if internet.WorstBurst >= 5 {
			f.add("bad", "loss-burst", "latency", internet.WorstBurst)
		}
		if internet.P95 > internet.Avg*3 && internet.P95-internet.Avg > 40 {
			f.add("warn", "spikes", "latency", internet.Avg, internet.P95)
		}
		if internet.Avg >= limits.RTTBad {
			f.add("warn", "rtt", "latency", internet.Avg)
		}
	}
	if gateway != nil {
		if gateway.LossPct >= limits.LossWarn {
			f.add("bad", "lan-loss", "latency", gateway.LossPct)
		}
		if gateway.Jitter >= limits.JitterWarn {
			f.add("warn", "lan-jitter", "latency", gateway.Jitter)
		}
	}

	// --- path ----------------------------------------------------------------
	if hop, ok := firstLossyHop(result); ok {
		f.add("bad", "path-loss", "path", hop.TTL, hop.IP)
	}
	if result.MTU != nil && result.MTU.Blackhole {
		f.addText("bad", "pmtu", "path", result.MTU.Detail, i18n.T("fnd.pmtu.hint"))
	}

	// --- dns -------------------------------------------------------------------
	for _, check := range result.DNSChecks {
		if check.Verdict == "bad" || check.Verdict == "warn" {
			f.addText(check.Verdict, "dns-"+check.Name, "dns", check.Detail, "")
		}
	}
	var plainBest, encBest float64 = -1, -1
	for _, row := range result.DNSBench {
		if row.Failures == row.Queries && row.Queries > 0 {
			// a router that runs no resolver is not a fault
			if strings.Contains(row.Label, "Gateway") || strings.Contains(row.Label, "Modem") {
				continue
			}
			level := "info"
			if row.Encrypted {
				level = "warn"
			}
			f.add(level, "dns-fail", "dns", row.Label)
			continue
		}
		if row.AvgMs >= limits.DNSBadMs {
			f.add("warn", "dns-slow", "dns", row.Label, row.AvgMs)
		}
		if row.AvgMs <= 0 || row.Failures > 0 || row.Kind == "system" ||
			strings.HasPrefix(row.Address, "127.") {
			// loopback stubs answer from cache in microseconds; comparing
			// against that would measure the cache, not the transport
			continue
		}
		if row.Encrypted {
			if encBest < 0 || row.AvgMs < encBest {
				encBest = row.AvgMs
			}
		} else if plainBest < 0 || row.AvgMs < plainBest {
			plainBest = row.AvgMs
		}
	}
	if plainBest > 0 && encBest > 0 {
		delta := encBest - plainBest
		level := "info"
		if delta > 120 {
			level = "warn"
		}
		f.add(level, "dns-encryption-cost", "dns", delta, plainBest, encBest)
	}
	for _, comparison := range result.DNSCompare {
		if len(comparison.System) > 0 && len(comparison.Trusted) > 0 && !comparison.Overlap {
			f.add("info", "dns-mismatch", "dns", comparison.Domain)
		}
	}

	// --- dpi --------------------------------------------------------------------
	if len(result.DPI) > 0 {
		var blocked, splitHelps, dnsFail, clean, quicBad, quicTotal []string
		for _, verdict := range result.DPI {
			switch verdict.Verdict {
			case "dpi-hard", "unreachable", "ip-block":
				blocked = append(blocked, verdict.Domain)
			case "dpi-split-helps":
				splitHelps = append(splitHelps, verdict.Domain)
			case "dns-fail":
				dnsFail = append(dnsFail, verdict.Domain)
			case "clean":
				clean = append(clean, verdict.Domain)
			}
			if verdict.QUIC != "" {
				quicTotal = append(quicTotal, verdict.Domain)
				if !strings.HasPrefix(verdict.QUIC, "ok") {
					quicBad = append(quicBad, verdict.Domain)
				}
			}
		}
		if len(splitHelps) > 0 {
			f.add("warn", "dpi-split", "dpi", len(splitHelps), joinN(splitHelps, 3))
		}
		if len(blocked) > 0 {
			f.add("bad", "dpi-blocked", "dpi", len(blocked), joinN(blocked, 3))
		}
		if len(dnsFail) > 0 {
			f.add("info", "dpi-dns", "dpi", len(dnsFail), joinN(dnsFail, 3))
		}
		if len(quicBad) > 0 && len(quicBad) == len(quicTotal) {
			f.add("warn", "quic-blocked", "dpi")
		}
		if len(clean) > 0 && len(blocked) == 0 && len(splitHelps) == 0 {
			f.add("ok", "dpi-clean", "dpi")
		}
	}

	// --- load ---------------------------------------------------------------------
	if result.Load != nil {
		bloat := result.Load.WorstDelta()
		switch {
		case bloat >= limits.BloatBad:
			f.add("bad", "bufferbloat-bad", "load", bloat, result.Load.Grade)
		case bloat >= limits.BloatWarn:
			f.add("warn", "bufferbloat-bad", "load", bloat, result.Load.Grade)
		default:
			f.add("ok", "bufferbloat-ok", "load", bloat, result.Load.Grade)
		}
		if download := result.Load.Download; download != nil {
			if download.Err != "" {
				f.add("info", "speed-error", "load", download.Err)
			}
			if link.SpeedMbit > 0 && download.Bps > 0 {
				if achieved := download.Bps / 1e6; achieved < float64(link.SpeedMbit)*0.5 {
					f.add("info", "speed-gap", "load", achieved, link.SpeedMbit)
				}
			}
			if download.Sockets.RetransPct > 3 {
				f.add("warn", "load-retrans", "load", download.Sockets.RetransPct)
			}
			if len(download.Sockets.CCAlgorithms) > 1 {
				f.add("info", "cc-mixed", "kernel",
					strings.Join(download.Sockets.CCAlgorithms, ", "))
			}
		}
	}

	if len(f.items) == 0 {
		f.add("ok", "clean", "")
	}
	return orderFindings(linkCauses(f.items))
}

var levelOrder = map[string]int{"bad": 0, "warn": 1, "info": 2, "ok": 3}

// orderFindings groups each symptom under the fault it belongs to and ranks the
// groups by their most serious member. Sorting by level alone splits a group
// apart - a warning symptom jumps above the red fault that causes it - and the
// reader has to reassemble the story from a list that no longer tells it.
func orderFindings(items []Finding) []Finding {
	byKey := make(map[string]Finding, len(items))
	for _, finding := range items {
		byKey[finding.Key] = finding
	}
	rank := map[string]int{}   // root -> best (lowest) level among its group
	seen := map[string]int{}   // root -> first appearance, to keep this stable
	for index, finding := range items {
		root := RootCause(finding.Key, byKey)
		if level, ok := rank[root]; !ok || levelOrder[finding.Level] < level {
			rank[root] = levelOrder[finding.Level]
		}
		if _, ok := seen[root]; !ok {
			seen[root] = index
		}
	}
	depth := func(key string) int {
		steps := 0
		for finding, ok := byKey[key]; ok && finding.Because != "" && steps < 8; finding, ok = byKey[key] {
			key = finding.Because
			steps++
		}
		return steps
	}
	sort.SliceStable(items, func(i, j int) bool {
		rootI, rootJ := RootCause(items[i].Key, byKey), RootCause(items[j].Key, byKey)
		if rootI != rootJ {
			if rank[rootI] != rank[rootJ] {
				return rank[rootI] < rank[rootJ]
			}
			return seen[rootI] < seen[rootJ]
		}
		// inside a group: the fault first, then its symptoms by severity
		if di, dj := depth(items[i].Key), depth(items[j].Key); di != dj {
			return di < dj
		}
		return levelOrder[items[i].Level] < levelOrder[items[j].Level]
	})
	return items
}

// kernelRegressionText re-renders the stored numbers in the active language,
// falling back to the sentence saved with older runs.
func kernelRegressionText(env Env) string {
	if data := env.KernRegData; data != nil {
		return i18n.T("fnd.kernel-regression.detail", data.GoodKernel, data.GoodHours,
			data.Running, data.CurrentRate)
	}
	return env.KernelRegr
}

func linkRegressionText(env Env) string {
	if data := env.LinkRegData; data != nil {
		if data.Kind == "worse" {
			return i18n.T("fnd.link-regression.worse", data.QuietRate, data.CurrentRate)
		}
		return i18n.T("fnd.link-regression.window", data.QuietMinutes, data.CurrentRate)
	}
	return env.LinkRegress
}

func counterHint(key string) string {
	switch key {
	case "rx_crc_errors":
		return i18n.T("fnd.nic-errors.crc")
	case "rx_missed_errors", "rx_fifo_errors":
		return i18n.T("fnd.nic-errors.fifo")
	case "tx_dropped", "rx_dropped":
		return i18n.T("fnd.nic-errors.drop")
	default:
		return ""
	}
}

type sysctlNote struct {
	Level string
	Key   string
	Text  string
}

// sysctlFindings is deliberately conservative: modern kernels are well tuned
// and blindly raising buffers usually makes latency worse.
func sysctlFindings(values map[string]string, link probe.LinkInfo) []sysctlNote {
	var notes []sysctlNote
	add := func(level, key, text string) {
		notes = append(notes, sysctlNote{Level: level, Key: key, Text: text})
	}
	switch values["net.core.default_qdisc"] {
	case "pfifo_fast", "noqueue", "":
		add("warn", "sysctl:default_qdisc", i18n.T("fnd.sysctl.qdisc"))
	}
	if values["net.ipv4.tcp_congestion_control"] == "cubic" {
		add("info", "sysctl:tcp_congestion_control", i18n.T("fnd.sysctl.cubic"))
	}
	if values["net.ipv4.tcp_window_scaling"] == "0" {
		add("bad", "sysctl:tcp_window_scaling", i18n.T("fnd.sysctl.wscale"))
	}
	if values["net.ipv4.tcp_sack"] == "0" {
		add("bad", "sysctl:tcp_sack", i18n.T("fnd.sysctl.sack"))
	}
	if values["net.ipv4.ip_no_pmtu_disc"] == "1" {
		add("warn", "sysctl:ip_no_pmtu_disc", i18n.T("fnd.sysctl.nopmtu"))
	}
	if values["net.ipv4.tcp_mtu_probing"] == "0" && link.MTU > 0 && link.MTU < 1500 {
		add("warn", "sysctl:tcp_mtu_probing", i18n.T("fnd.sysctl.mtuprobe", link.MTU))
	}
	if rmem, err := strconv.ParseInt(values["net.core.rmem_max"], 10, 64); err == nil &&
		rmem > 64*1024*1024 {
		add("warn", "sysctl:rmem_max", i18n.T("fnd.sysctl.rmem", rmem))
	}
	return notes
}

func worstErrorCounter(counters map[string]int64) (string, int64) {
	var worstKey string
	var worst int64
	for key, value := range counters {
		if !probe.ErrorKeys[key] || value <= 0 {
			continue
		}
		if value > worst {
			worstKey, worst = key, value
		}
	}
	return worstKey, worst
}

func joinN(items []string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	return strings.Join(items, ", ")
}

// maxFindingPenalty bounds how far findings can move the score. Past this point
// the measured numbers should be doing the talking: a run cannot be made
// arbitrarily bad by the tool happening to know about more things to check.
const maxFindingPenalty = 40

// findingPenalty charges once per distinct problem rather than once per finding.
//
// Summing every finding made the score a function of how many symptoms the tool
// emits instead of how bad the connection is. One bad pair in a cable produces
// link-drops, link-downshift and link-speed, so it was charged three times,
// while an unrelated single fault cost a third as much - and adding any new
// finding to the code quietly lowered the score of every machine that had the
// condition already, which made saved runs incomparable across versions.
func findingPenalty(findings []Finding) float64 {
	byKey := make(map[string]Finding, len(findings))
	for _, finding := range findings {
		byKey[finding.Key] = finding
	}
	worst := map[string]float64{}
	for _, finding := range findings {
		weight := 0.0
		switch finding.Level {
		case "bad":
			weight = 8
		case "warn":
			weight = 3
		default:
			continue
		}
		root := RootCause(finding.Key, byKey)
		if weight > worst[root] {
			worst[root] = weight
		}
	}
	total := 0.0
	for _, weight := range worst {
		total += weight
	}
	if total > maxFindingPenalty {
		total = maxFindingPenalty
	}
	return total
}

func scoreRun(result Result) (float64, string) {
	internet := result.InternetLatency()
	var score float64 = 100
	if internet != nil && internet.Received > 0 {
		bloat, haveBloat := 0.0, false
		if result.Load != nil {
			bloat, haveBloat = result.Load.WorstDelta(), true
		}
		score = stats.StabilityScore(internet.LossPct, internet.Jitter, internet.P95,
			internet.Avg, bloat, haveBloat)
	}
	score -= findingPenalty(result.Findings)
	if score < 0 {
		score = 0
	}
	score = float64(int(score*10+0.5)) / 10
	return score, stats.LetterFromScore(score)
}

// SysctlInt reads an integer field out of a whitespace-separated sysctl value.
func SysctlInt(value string, index int) int64 {
	fields := strings.Fields(value)
	if index >= len(fields) {
		return 0
	}
	number, err := strconv.ParseInt(fields[index], 10, 64)
	if err != nil {
		return 0
	}
	return number
}
