// Command nabiz measures internet stability and the real impact of the tools
// that sit between this machine and the network.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/WinTone01/nabiz/internal/apply"
	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/monitor"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/report"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/ui"
	"github.com/WinTone01/nabiz/internal/util"
)

// Version is stamped at build time with -ldflags.
var Version = "0.3.1"

var (
	sTitle = lipgloss.NewStyle().Bold(true)
	sDim   = lipgloss.NewStyle().Faint(true)
	sBad   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	sWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	sOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	sInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	sAcc   = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
)

func levelStyle(level string) lipgloss.Style {
	switch level {
	case "bad", "critical":
		return sBad
	case "warn":
		return sWarn
	case "ok":
		return sOK
	default:
		return sInfo
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, sBad.Render(i18n.T("ui.error", err.Error())))
		os.Exit(1)
	}
}

// extractLang pulls a global --lang/-l flag out of the argument list before the
// subcommand parser sees it, so it works in any position.
var langFlagSeen bool

func extractLang(args []string) []string {
	out := args[:0:0]
	for index := 0; index < len(args); index++ {
		switch {
		case args[index] == "--lang" || args[index] == "-lang":
			if index+1 < len(args) {
				i18n.SetFromString(args[index+1])
				langFlagSeen = true
				index++
			}
		case strings.HasPrefix(args[index], "--lang="):
			i18n.SetFromString(strings.TrimPrefix(args[index], "--lang="))
			langFlagSeen = true
		default:
			out = append(out, args[index])
		}
	}
	return out
}

func usage() {
	if i18n.Current() == i18n.TR {
		fmt.Print(`Nabız ` + Version + ` — internet kararlılık ve ağ katmanı laboratuvarı

KULLANIM
  nabiz [komut] [seçenekler]

KOMUTLAR
  (yok)        etkileşimli arayüzü açar
  doctor       paket göndermeden anlık teşhis (bir saniyeden kısa)
  quick        ~45 sn genel tarama
  full         yol, MTU, hız ve bufferbloat dahil tam tarama
  deep         tam tarama + çekirdek sayaçları + soket başına TCP durumu
  dns          yalnızca DNS testleri (düz / DoT / DoH + müdahale kontrolleri)
  dpi          yalnızca engel/DPI sınıflandırması
  path         traceroute + yol MTU'su
  load         hız + yük altında gecikme (bufferbloat)
  monitor      uzun süreli kararlılık izleme
  ab           bir bileşeni açık/kapalı karşılaştır (--target unwall|bpftune)
  advice       son çalışmanın önerilerini göster
  history      kayıtlı çalışmaların puan eğilimi
  apply        önerileri yedek alarak ve güvenlik ağıyla uygula
  sweep        şekillendirme hızını deneyerek bul (cake/SQM)
  rollback     son uygulanan değişiklikleri geri al
  baseline     referans çalışmayı ayarla/göster/temizle
  env          ortam dökümü (arayüz, unwall, bpftune, sysctl, netfilter)
  report       kayıtlı çalışmalar
  tui          etkileşimli arayüz

SEÇENEKLER
  --lang tr|en   arayüz dili (varsayılan: ortamdan algılanır, yoksa İngilizce)
  --json PATH    sonucu JSON olarak yaz
  --md PATH      sonucu Markdown rapor olarak yaz
  --baseline     bu çalışmayı referans olarak sabitle
  --no-save      otomatik kaydetme
  -q             ilerleme çubuğunu gizle

ÖRNEKLER
  nabiz doctor
  nabiz deep --md rapor.md
  nabiz monitor -d 2h
  nabiz ab --target bpftune
`)
		return
	}
	fmt.Print(`Nabız ` + Version + ` — internet stability and network-layer lab

USAGE
  nabiz [command] [options]

COMMANDS
  (none)       open the interactive interface
  doctor       instant diagnosis without sending a packet (under a second)
  quick        ~45 s general sweep
  full         full sweep including path, MTU, throughput and bufferbloat
  deep         full sweep + kernel counters + per-socket TCP state
  dns          DNS only (plain / DoT / DoH plus interference checks)
  dpi          block-type classification only
  path         traceroute + path MTU
  load         throughput + latency under load (bufferbloat)
  monitor      long-running stability monitor
  ab           compare a component on and off (--target unwall|bpftune)
  advice       show the advice from the last run
  history      score trend across saved runs
  apply        apply advice with a snapshot and an automatic safety net
  sweep        find the shaping rate by trying rates and measuring (cake/SQM)
  rollback     undo the last applied batch
  baseline     set / show / clear the reference run
  env          environment dump (link, unwall, bpftune, sysctl, netfilter)
  report       saved runs
  tui          interactive interface

OPTIONS
  --lang tr|en   interface language (default: detected, English otherwise)
  --json PATH    write the result as JSON
  --md PATH      write the result as a Markdown report
  --baseline     pin this run as the reference
  --no-save      do not auto-save the run
  -q             hide the progress bar

EXAMPLES
  nabiz doctor
  nabiz deep --md report.md
  nabiz monitor -d 2h
  nabiz ab --target bpftune
`)
}

// applyLanguage resolves the interface language once: an explicit --lang wins,
// then the stored preference, then the environment. Doing it in one place keeps
// the CLI and the TUI from disagreeing about which language they are in.
func applyLanguage(cfg config.Config, explicit bool) {
	if explicit {
		return
	}
	if cfg.Lang != "" && cfg.Lang != "auto" {
		i18n.SetFromString(cfg.Lang)
	}
}

func run(args []string) error {
	before := i18n.Current()
	args = extractLang(args)
	explicitLang := i18n.Current() != before || langFlagSeen
	if len(args) == 0 {
		cfg := config.Load()
		applyLanguage(cfg, explicitLang)
		return ui.Run(cfg, Version)
	}
	command, rest := args[0], args[1:]
	switch command {
	case "-h", "--help", "help":
		usage()
		return nil
	case "-V", "--version", "version":
		fmt.Println("nabiz " + Version)
		return nil
	case "tui":
		cfg := config.Load()
		applyLanguage(cfg, explicitLang)
		return ui.Run(cfg, Version)
	case "doctor", "quick", "full", "deep", "dns", "dpi", "path", "load":
		return cmdRun(command, rest)
	case "monitor":
		return cmdMonitor(rest)
	case "ab":
		return cmdAB(rest)
	case "env":
		return cmdEnv(rest)
	case "advice":
		return cmdAdvice(rest)
	case "history":
		return cmdHistory(rest)
	case "baseline":
		return cmdBaseline(rest)
	case "apply":
		return cmdApply(rest)
	case "sweep":
		return cmdSweep(rest)
	case "rollback":
		return cmdRollback(rest)
	case "report":
		return cmdReport(rest)
	default:
		usage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

// --- run ------------------------------------------------------------------

func cmdRun(name string, args []string) error {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	jsonPath := flags.String("json", "", "write JSON")
	mdPath := flags.String("md", "", "write Markdown")
	noSave := flags.Bool("no-save", false, "skip autosave")
	setBaseline := flags.Bool("baseline", false, "pin this run as the baseline")
	quiet := flags.Bool("q", false, "hide progress")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := config.Load()
	applyLanguage(cfg, langFlagSeen)

	fmt.Printf("%s\n", sTitle.Render(fmt.Sprintf("Nabız %s · %s", Version, name)))
	printEnvLine()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	progress := newProgressBar(!*quiet && name != "doctor")
	result := suite.Run(ctx, cfg, name, progress.update)
	progress.clear()

	if baseline, err := report.LoadBaseline(); err == nil {
		result.Baseline = &suite.BaselineRef{
			Score: baseline.Score, Grade: baseline.Grade,
			StartedAt: baseline.StartedAt.Format(time.RFC3339),
		}
	}
	printResult(result)

	if !*noSave && name != "doctor" {
		if path, err := report.Autosave(result); err == nil {
			fmt.Println(sDim.Render(i18n.T("ui.saved", path)))
		}
	}
	if *setBaseline {
		if err := report.SaveBaseline(result); err != nil {
			return err
		}
		fmt.Println(sAcc.Render(i18n.T("misc.baseline_set")))
	}
	if *jsonPath != "" {
		if err := report.SaveJSON(result, *jsonPath); err != nil {
			return err
		}
		fmt.Println("JSON: " + *jsonPath)
	}
	if *mdPath != "" {
		if err := report.SaveMarkdown(result, *mdPath); err != nil {
			return err
		}
		fmt.Println("Markdown: " + *mdPath)
	}
	return nil
}

func printEnvLine() {
	unwall := sysinfo.ReadUnwall()
	bpftune := sysinfo.ReadBpftune().WithAppliedChanges(config.ChangesInForce())
	var parts []string
	if iface, _ := util.DefaultRoute(); iface != "" {
		parts = append(parts, iface)
	}
	parts = append(parts, strings.TrimSpace(util.ReadText("/proc/sys/kernel/osrelease", "")))
	if unwall.Installed {
		parts = append(parts, "unwall: "+unwall.Label())
	}
	if bpftune.Installed {
		parts = append(parts, "bpftune: "+bpftune.Label())
	}
	fmt.Println(sDim.Render("  " + strings.Join(parts, " · ")))
}

type progressBar struct {
	enabled bool
	last    time.Time
}

func newProgressBar(enabled bool) *progressBar {
	if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice == 0 {
		enabled = false
	}
	return &progressBar{enabled: enabled}
}

func (p *progressBar) update(phase string, fraction float64) {
	if !p.enabled || (time.Since(p.last) < 100*time.Millisecond && fraction < 1) {
		return
	}
	p.last = time.Now()
	const width = 28
	filled := int(fraction * width)
	fmt.Printf("\r  [%s%s] %3d%%  %-30s", strings.Repeat("━", filled),
		strings.Repeat("·", width-filled), int(fraction*100), util.Truncate(phase, 30))
}

func (p *progressBar) clear() {
	if p.enabled {
		fmt.Printf("\r%s\r", strings.Repeat(" ", 72))
	}
}

func printResult(result suite.Result) {
	fmt.Println()
	style := sOK
	switch {
	case result.Score < 60:
		style = sBad
	case result.Score < 80:
		style = sWarn
	}
	line := fmt.Sprintf("%s  %s   %s", sTitle.Render(i18n.T("f.score")+":"),
		style.Render(fmt.Sprintf("%.1f / 100  (%s)", result.Score, result.Grade)),
		sDim.Render(fmt.Sprintf("%.0f s", result.Duration)))
	if result.Baseline != nil {
		delta := result.Score - result.Baseline.Score
		deltaStyle := sOK
		if delta < 0 {
			deltaStyle = sBad
		}
		line += "   " + sDim.Render(i18n.T("misc.vs_baseline")+" ") +
			deltaStyle.Render(fmt.Sprintf("%+.1f", delta))
	}
	fmt.Println(line)

	if len(result.Latency) > 0 {
		fmt.Println()
		fmt.Println(sDim.Render(fmt.Sprintf("%-22s %8s %8s %8s %8s %6s",
			i18n.T("col.target"), i18n.T("col.loss"), i18n.T("col.avg"),
			i18n.T("col.p95"), i18n.T("col.jitter"), i18n.T("col.mos"))))
		for _, summary := range result.Latency {
			fmt.Printf("%-22s %8.1f %8.1f %8.1f %8.1f %6.2f  %s\n",
				util.Truncate(summary.Label, 22), summary.LossPct, summary.Avg,
				summary.P95, summary.Jitter, summary.MOS,
				stats.Sparkline(summary.Samples, 24))
		}
	}
	if load := result.Load; load != nil {
		fmt.Println()
		gradeStyle := sOK
		if load.Grade != "A+" && load.Grade != "A" {
			gradeStyle = sWarn
		}
		down, up := 0.0, 0.0
		if load.Download != nil {
			down = load.Download.Bps
		}
		if load.Upload != nil {
			up = load.Upload.Bps
		}
		fmt.Printf("%s  %s ↓  %s ↑   %s %s  (+%.0f / +%.0f ms)\n",
			sTitle.Render(i18n.T("sec.load")+":"), util.HumanRate(down), util.HumanRate(up),
			i18n.T("load.bloat"), gradeStyle.Render(load.Grade), load.DownDelta, load.UpDelta)
		if load.Download != nil && load.Download.Sockets.Count > 0 {
			sockets := load.Download.Sockets
			fmt.Println(sDim.Render("   " + i18n.T("load.kernel",
				strings.Join(sockets.CCAlgorithms, ","), sockets.AvgCWnd,
				sockets.RetransPct, sockets.MinRTTms, sockets.Count)))
		}
	}
	if len(result.DNSBench) > 0 {
		fmt.Println()
		fmt.Println(sDim.Render(fmt.Sprintf("%-26s %-7s %9s %9s %7s",
			i18n.T("col.resolver"), i18n.T("col.kind"), i18n.T("col.avgms"),
			i18n.T("col.p95"), i18n.T("col.errors"))))
		for _, row := range result.DNSBench {
			fmt.Printf("%-26s %-7s %9.1f %9.1f %6d/%d\n", util.Truncate(row.Label, 26),
				row.Kind, row.AvgMs, row.P95Ms, row.Failures, row.Queries)
		}
	}
	for _, check := range result.DNSChecks {
		fmt.Printf("  %-20s %s %s\n", check.Name,
			levelStyle(check.Verdict).Render(fmt.Sprintf("%-5s", check.Verdict)), check.Detail)
	}
	if len(result.DPI) > 0 {
		fmt.Println()
		fmt.Println(sDim.Render(fmt.Sprintf("%-28s %-18s %-10s %-13s %s",
			i18n.T("col.domain"), i18n.T("col.verdict"), i18n.T("col.tls"),
			i18n.T("col.split"), i18n.T("col.quic"))))
		for _, verdict := range result.DPI {
			style := sInfo
			switch verdict.Verdict {
			case "clean":
				style = sOK
			case "dpi-split-helps":
				style = sWarn
			case "dpi-hard", "ip-block", "unreachable":
				style = sBad
			}
			split := "-"
			if verdict.SplitHdr.Kind != "" || verdict.SplitSNI.Kind != "" {
				split = orDash(verdict.SplitHdr.Kind) + "/" + orDash(verdict.SplitSNI.Kind)
			}
			fmt.Printf("%-28s %s %-10s %-13s %s\n", util.Truncate(verdict.Domain, 28),
				style.Render(fmt.Sprintf("%-18s", verdict.Verdict)),
				orDash(verdict.Whole.Kind), split, util.Truncate(verdict.QUIC, 22))
		}
	}
	if len(result.Hops) > 0 {
		fmt.Println()
		fmt.Println(sDim.Render(fmt.Sprintf("%3s %-18s %8s %9s %9s",
			i18n.T("col.hop"), i18n.T("col.ip"), i18n.T("col.loss"),
			i18n.T("col.avg"), i18n.T("col.worst"))))
		for _, hop := range result.Hops {
			ip := hop.IP
			if ip == "" {
				ip = "*"
			}
			fmt.Printf("%3d %-18s %8.0f %9.1f %9.1f\n", hop.TTL, ip, hop.LossPct(),
				hop.Avg(), hop.Worst())
		}
	}
	if result.MTU != nil {
		fmt.Printf("\n%s\n", i18n.T("misc.mtu_line", result.MTU.IfaceMTU,
			result.MTU.ProbedMTU, result.MTU.Detail))
	}

	fmt.Println()
	fmt.Println(sTitle.Render(i18n.T("sec.findings")))
	for _, finding := range result.Findings {
		mark := map[string]string{"bad": "!!", "warn": " !", "info": " ·", "ok": " +"}[finding.Level]
		// a symptom is printed under the fault it belongs to, so the list reads
		// as the handful of problems it is rather than a wall of red
		indent, key := "", sTitle.Render(finding.Key)
		if finding.Because != "" {
			indent, key = "   ", sDim.Render("└ "+finding.Key)
		}
		fmt.Printf("%s %s %s  %s\n", indent, levelStyle(finding.Level).Render(mark),
			key, finding.Title)
		if finding.Hint != "" {
			fmt.Println(indent + "      " + sDim.Render(finding.Hint))
		}
	}
	printAdvice(result, 3)
}

func printAdvice(result suite.Result, limit int) {
	if len(result.Advice) == 0 {
		return
	}
	fmt.Println()
	extra := ""
	if limit > 0 && len(result.Advice) > limit {
		extra = sDim.Render(fmt.Sprintf("  (%d · nabiz advice)", len(result.Advice)))
	}
	fmt.Println(sTitle.Render(i18n.T("sec.advice")) + extra)
	shown := 0
	for _, advice := range result.Advice {
		if limit > 0 && shown >= limit {
			break
		}
		shown++
		fmt.Printf("\n %s %s\n",
			sAcc.Render(fmt.Sprintf("P%d [%s]", advice.Priority,
				suite.CategoryLabel(advice.Category))),
			sTitle.Render(advice.Title))
		fmt.Println("   " + wrapText(advice.Why, 92, "   "))
		for _, step := range advice.How {
			marker := sDim.Render("• ")
			if suite.IsCommand(step) {
				marker = sDim.Render("$ ")
			}
			fmt.Println("     " + marker + step)
		}
		if advice.Gain != "" {
			fmt.Println("   " + sOK.Render("→ ") + advice.Gain)
		}
		if advice.Risk != "" {
			fmt.Println("   " + sWarn.Render("! ") + advice.Risk)
		}
	}
}

func wrapText(text string, width int, indent string) string {
	words := strings.Fields(text)
	var lines []string
	current := ""
	for _, word := range words {
		switch {
		case current == "":
			current = word
		case len(current)+1+len(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n"+indent)
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// --- monitor ---------------------------------------------------------------

var durationRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)([smhd]?)$`)

func parseDuration(value string) (time.Duration, error) {
	match := durationRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if match == nil {
		return 0, errors.New("duration format: 90s, 30m, 2h")
	}
	amount, _ := strconv.ParseFloat(match[1], 64)
	unit := map[string]time.Duration{"s": time.Second, "m": time.Minute,
		"h": time.Hour, "d": 24 * time.Hour, "": time.Minute}[match[2]]
	return time.Duration(amount * float64(unit)), nil
}

func cmdMonitor(args []string) error {
	flags := flag.NewFlagSet("monitor", flag.ContinueOnError)
	durationText := flags.String("d", "", "duration, e.g. 30m or 2h")
	jsonPath := flags.String("json", "", "write the summary as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var duration time.Duration
	if *durationText != "" {
		parsed, err := parseDuration(*durationText)
		if err != nil {
			return err
		}
		duration = parsed
	}

	watcher := monitor.New(config.Load(), func(event monitor.Event) {
		fmt.Printf("%s %s %s %s\n", sDim.Render(event.Stamp()),
			levelStyle(event.Severity).Render(fmt.Sprintf("%-14s", event.Kind)),
			sAcc.Render(event.Target), event.Detail)
	})
	label := i18n.T("ui.none")
	if duration > 0 {
		label = duration.String()
	}
	fmt.Println(sTitle.Render("Nabız " + i18n.T("nav.monitor") + " — " + label + " · Ctrl-C"))
	if err := watcher.Start(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	} else {
		<-ctx.Done()
	}
	watcher.Stop()

	snapshot := watcher.Snapshot()
	fmt.Println("\n" + sTitle.Render(i18n.T("panel.stability")))
	fmt.Printf("  %-16s %s\n", i18n.T("f.uptime"), util.ShortDuration(snapshot.Uptime))
	fmt.Printf("  %-16s %.3f%%\n", i18n.T("f.avail"), snapshot.Availability)
	fmt.Printf("  %-16s %d\n", i18n.T("f.outages"), len(snapshot.Outages))
	fmt.Printf("  %-16s %d\n", i18n.T("f.dnsfail"), snapshot.DNSFailures)
	fmt.Printf("  %-16s %d\n", i18n.T("f.reachfail"), snapshot.ReachFailures)
	fmt.Printf("  %-16s %d\n", i18n.T("f.flaps"), snapshot.CarrierFlaps)
	if snapshot.NFQDrops > 0 {
		fmt.Printf("  %-16s %d\n", i18n.T("f.nfqueue"), snapshot.NFQDrops)
	}
	fmt.Println()
	for _, target := range snapshot.Targets {
		fmt.Printf("  %-20s %6.2f%%  %6.1f  %6.1f  %s\n", util.Truncate(target.Label, 20),
			target.LossPct, target.Avg, target.P95, stats.Sparkline(target.Recent, 30))
	}
	if *jsonPath != "" {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*jsonPath, data, 0o644); err != nil {
			return err
		}
		fmt.Println("\nJSON: " + *jsonPath)
	}
	fmt.Println(sDim.Render("\n" + config.MonitorLog()))
	return nil
}

// --- A/B --------------------------------------------------------------------

func cmdAB(args []string) error {
	flags := flag.NewFlagSet("ab", flag.ContinueOnError)
	target := flags.String("target", "unwall", "component: unwall | bpftune")
	suiteName := flags.String("suite", "quick", "suite to run")
	mdPath := flags.String("md", "", "write the comparison as Markdown")
	assumeYes := flags.Bool("y", false, "skip the confirmation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := config.Load()

	var (
		labelOn, labelOff string
		wasRunning        bool
		setState          func(bool) (bool, string)
		canControl        bool
	)
	switch *target {
	case "unwall":
		state := sysinfo.ReadUnwall()
		if !state.Installed {
			return errors.New("Unwall " + i18n.T("ui.notinstalled"))
		}
		labelOn, labelOff = "zapret ON", "zapret OFF"
		wasRunning, setState, canControl = state.Running, sysinfo.SetUnwall, sysinfo.CanControlUnwall()
	case "bpftune":
		state := sysinfo.ReadBpftune().WithAppliedChanges(config.ChangesInForce())
		if !state.Installed {
			return errors.New("bpftune " + i18n.T("ui.notinstalled"))
		}
		labelOn, labelOff = "bpftune ON", "bpftune OFF"
		wasRunning, setState, canControl = state.Running, sysinfo.SetBpftune, sysinfo.CanControlBpftune()
	default:
		return fmt.Errorf("unknown target: %s", *target)
	}
	if !canControl {
		return errors.New(i18n.T("ui.needs_root") + " (pkexec/sudo)")
	}

	fmt.Println(sWarn.Render(i18n.T("ui.confirm_ab", *target)))
	if !*assumeYes {
		var answer string
		_, _ = fmt.Scanln(&answer)
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "e", "evet", "y", "yes":
		default:
			return errors.New(i18n.T("ui.cancelled"))
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer func() {
		if ok, message := setState(wasRunning); !ok {
			fmt.Fprintln(os.Stderr, sBad.Render(message))
		}
	}()

	var results [2]suite.Result
	for index, want := range []bool{true, false} {
		if ctx.Err() != nil {
			return errors.New(i18n.T("ui.cancelled"))
		}
		if ok, message := setState(want); !ok {
			return errors.New(message)
		}
		time.Sleep(3 * time.Second)
		phase := labelOn
		if !want {
			phase = labelOff
		}
		fmt.Println("\n" + sTitle.Render("["+string(rune('A'+index))+"] "+phase))
		progress := newProgressBar(true)
		results[index] = suite.Run(ctx, cfg, *suiteName, progress.update)
		progress.clear()
		printResult(results[index])
	}

	if saved := config.Load(); true {
		saved.LastComparison = time.Now().Format(time.RFC3339)
		_ = config.Save(saved)
	}
	text := report.ABMarkdown(labelOn, results[0], labelOff, results[1])
	fmt.Println("\n" + text)
	if *mdPath != "" {
		if err := os.WriteFile(*mdPath, []byte(text), 0o644); err != nil {
			return err
		}
		fmt.Println("Markdown: " + *mdPath)
	}
	return nil
}

// --- env / advice / history / baseline / report -----------------------------

func cmdEnv(args []string) error {
	flags := flag.NewFlagSet("env", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "write JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	env := suite.SnapshotEnv(config.Load())
	if *asJSON {
		data, err := json.MarshalIndent(env, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	link := env.Link
	fmt.Println(sTitle.Render(i18n.T("sec.physical")))
	fmt.Printf("  %-16s %s (%s)  %s\n", i18n.T("f.iface"), link.Iface, link.Address,
		orDash(link.Driver))
	fmt.Printf("  %-16s %s\n", i18n.T("f.gateway"), link.Gateway)
	fmt.Printf("  %-16s %d Mbit/s · MTU %d · %s\n", i18n.T("f.speed"), link.SpeedMbit,
		link.MTU, link.Duplex)
	fmt.Printf("  %-16s %s\n", i18n.T("f.qdisc"), orDash(link.Qdisc))
	fmt.Printf("  %-16s %d\n", i18n.T("f.flaps"), env.LinkLog.Drops)
	if link.Wireless {
		fmt.Printf("  %-16s %s · %.0f dBm\n", i18n.T("f.wifi"), link.SSID, link.SignalDBm)
	}

	fmt.Println("\n" + sTitle.Render(i18n.T("sec.tcpcounters")))
	health := env.TCPHealth
	fmt.Printf("  %-16s %d / %d (%.2f%%)\n", i18n.T("f.retransmit"), health.RetransSegs,
		health.OutSegs, health.RetransPct)
	fmt.Printf("  %-16s %d\n", i18n.T("f.timeouts"), health.Timeouts)
	fmt.Printf("  %-16s %d\n", i18n.T("f.ooo"), health.OFOQueue)
	if env.Sockets.Count > 0 {
		fmt.Printf("  %-16s %d · cc=%s · rtt %.1f ms · cwnd %.0f\n", i18n.T("f.sockets"),
			env.Sockets.Count, strings.Join(env.Sockets.CCAlgorithms, ","),
			env.Sockets.AvgRTTms, env.Sockets.AvgCWnd)
	}

	fmt.Println("\n" + sTitle.Render(i18n.T("nav.kernel")))
	fmt.Printf("  %-16s %s\n", i18n.T("col.kernelv"), env.Kernel)
	for _, entry := range env.KernelStats {
		measured := i18n.T("misc.unmeasured")
		if entry.Measured {
			measured = fmt.Sprintf("%.1f h", entry.CoveredHrs)
		}
		fmt.Printf("  %-22s boots %-3d  %7.1f h  %-12s drops %-4d (%.1f/h)\n",
			entry.Kernel, entry.Boots, entry.Hours, measured, entry.Drops, entry.DropsPerHr)
	}

	fmt.Println("\n" + sTitle.Render("bpftune"))
	if !env.Bpftune.Installed {
		fmt.Println("  " + i18n.T("ui.notinstalled"))
	} else {
		fmt.Printf("  %-16s %s\n", i18n.T("f.status"), env.Bpftune.Label())
		for _, change := range env.Bpftune.Changes {
			fmt.Printf("  %-40s %s → %s  (×%d)\n", change.Tunable,
				util.Truncate(change.From, 22), util.Truncate(change.To, 22), change.Count)
		}
	}

	fmt.Println("\n" + sTitle.Render("Unwall"))
	if !env.Unwall.Installed {
		fmt.Println("  " + i18n.T("ui.notinstalled"))
	} else {
		fmt.Printf("  %-16s %s\n", i18n.T("f.status"), env.Unwall.Label())
		fmt.Printf("  %-16s %s (%d / %d)\n", i18n.T("f.hostlist"), env.Unwall.HostlistMode,
			env.Unwall.HostlistN, env.Unwall.AutoHostlistN)
		fmt.Printf("  %-16s %s\n", i18n.T("f.dns"), env.Unwall.DNSBackend+" / "+env.Unwall.DNSProvider)
	}

	fmt.Println("\n" + sTitle.Render("DNS"))
	fmt.Printf("  %-16s %s\n", i18n.T("f.system"), strings.Join(env.SystemDNS, ", "))
	fmt.Printf("  %-16s %s\n", i18n.T("f.upstream"), strings.Join(env.UpstreamDNS, ", "))
	fmt.Printf("  %-16s ipv6 %v · aaaa %v\n", "ipv6", env.IPv6.Working(), env.IPv6.DNSHasAAAA)

	fmt.Println("\n" + sTitle.Render(i18n.T("panel.netfilter")))
	if env.NFQueue.Available {
		for _, queue := range env.NFQueue.Queues {
			fmt.Printf("  queue %-4d dropped %d/%d\n", queue.QNum,
				queue.QueueDropped, queue.UserDropped)
		}
		if len(env.NFQueue.Queues) == 0 {
			fmt.Println("  " + i18n.T("ui.none"))
		}
	} else {
		fmt.Printf("  %s (%s)\n", i18n.T("ui.needs_root"), env.NFQueue.Reason)
	}
	fmt.Printf("  %-16s %d / %d\n", i18n.T("f.conntrack"), env.Conntrack.Count, env.Conntrack.Max)

	fmt.Println("\n" + sTitle.Render(i18n.T("panel.sysctl")))
	for _, key := range probe.SysctlKeys {
		if value, ok := env.Sysctls[key]; ok {
			fmt.Printf("  %-44s %s\n", key, value)
		}
	}
	return nil
}

func cmdAdvice(args []string) error {
	flags := flag.NewFlagSet("advice", flag.ContinueOnError)
	path := flags.String("run", "", "a specific saved run")
	if err := flags.Parse(args); err != nil {
		return err
	}
	target := *path
	var result suite.Result
	var err error
	if target == "" {
		result, target, err = report.LatestRun()
	} else {
		result, err = report.LoadJSON(target)
	}
	if err != nil {
		return err
	}
	// re-read the machine as well as the language: advice that has already been
	// acted on should not still be listed
	suite.RefreshEnv(&result, config.Load())
	fmt.Println(sTitle.Render(i18n.T("sec.advice") + " — " + filepath.Base(target)))
	fmt.Println(sDim.Render(fmt.Sprintf("%.1f (%s) · %s", result.Score, result.Grade,
		result.StartedAt.Format("2006-01-02 15:04"))))
	printAdvice(result, 0)
	return nil
}

func cmdHistory(args []string) error {
	flags := flag.NewFlagSet("history", flag.ContinueOnError)
	limit := flags.Int("n", 40, "how many runs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	history := report.History(*limit)
	if len(history) == 0 {
		fmt.Println(i18n.T("misc.no_runs"))
		return nil
	}
	fmt.Println(sTitle.Render(i18n.T("sec.trend")))
	series := report.ScoreSeries(history)
	fmt.Println("  " + sOK.Render(stats.SparklineF(series, len(series))))
	fmt.Println()
	fmt.Println(sDim.Render(fmt.Sprintf("%-18s %-8s %8s %6s %6s %6s",
		i18n.T("col.date"), i18n.T("col.suite"), i18n.T("f.score"),
		i18n.T("col.grade"), "bad", "warn")))
	for index := len(history) - 1; index >= 0; index-- {
		entry := history[index]
		fmt.Printf("%-18s %-8s %8.1f %6s %6d %6d\n",
			entry.StartedAt.Format("2006-01-02 15:04"), entry.Name, entry.Score,
			entry.Grade, entry.Bad, entry.Warn)
	}
	if baseline, err := report.LoadBaseline(); err == nil {
		fmt.Printf("\n%-18s %.1f (%s) · %s\n", i18n.T("f.baseline"), baseline.Score,
			baseline.Grade, baseline.StartedAt.Format("2006-01-02 15:04"))
	}
	return nil
}

func cmdBaseline(args []string) error {
	action := "show"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "clear":
		if err := report.ClearBaseline(); err != nil {
			return err
		}
		fmt.Println(i18n.T("ui.done"))
		return nil
	case "set":
		runs := report.ListRuns(1)
		if len(runs) == 0 {
			return errors.New(i18n.T("misc.no_runs"))
		}
		result, err := report.LoadJSON(runs[0])
		if err != nil {
			return err
		}
		if err := report.SaveBaseline(result); err != nil {
			return err
		}
		fmt.Println(i18n.T("misc.baseline_set"))
		return nil
	default:
		baseline, err := report.LoadBaseline()
		if err != nil {
			fmt.Println(i18n.T("ui.none"))
			return nil
		}
		fmt.Printf("%s: %.1f (%s) · %s · %s\n", i18n.T("f.baseline"), baseline.Score,
			baseline.Grade, baseline.Name,
			baseline.StartedAt.Format("2006-01-02 15:04"))
		return nil
	}
}

func cmdReport(args []string) error {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	list := flags.Bool("list", false, "list saved runs")
	mdPath := flags.String("md", "", "export as Markdown")
	if err := flags.Parse(args); err != nil {
		return err
	}
	runs := report.ListRuns(30)
	if len(runs) == 0 {
		fmt.Println(i18n.T("misc.no_runs"))
		return nil
	}
	if *list {
		for _, path := range runs {
			fmt.Println(path)
		}
		return nil
	}
	target := runs[0]
	if flags.NArg() > 0 {
		target = flags.Arg(0)
	}
	result, err := report.LoadJSON(target)
	if err != nil {
		return err
	}
	suite.Refresh(&result, config.Load())
	if *mdPath != "" {
		if err := report.SaveMarkdown(result, *mdPath); err != nil {
			return err
		}
		fmt.Println("Markdown: " + *mdPath)
		return nil
	}
	fmt.Print(report.Markdown(result))
	return nil
}

// measureEffect repeats the measurement a change promised to move.
//
// The connectivity check only asks whether the internet still works, which a
// change can pass while making the very thing it was meant to fix worse - a
// shaper set too low keeps every packet flowing and halves the throughput. This
// puts the line back under load and reads the same number again.
func measureEffect(ctx context.Context, snapshot *apply.Snapshot, changes []apply.Change) error {
	var promised []apply.Change
	for _, change := range changes {
		if change.Effect != nil {
			promised = append(promised, change)
		}
	}
	if len(promised) == 0 {
		return nil
	}
	fmt.Println(sDim.Render(i18n.T("apply.measuring")))
	after := suite.Run(ctx, config.Load(), "load", nil)
	if after.Load == nil {
		fmt.Println(sWarn.Render(i18n.T("apply.measurefailed")))
		return nil
	}

	regressed := false
	for _, change := range promised {
		value, ok := metricValue(after, change.Effect.Metric)
		if !ok {
			continue
		}
		line := i18n.T("apply.effect", change.ID, change.Effect.Before, value)
		if change.Effect.Worse(value) {
			regressed = true
			fmt.Println(sBad.Render(line))
			continue
		}
		if change.Effect.Improvement(value) > 0 {
			fmt.Println(sOK.Render(line))
		} else {
			fmt.Println(sDim.Render(line))
		}
	}
	if !regressed {
		return nil
	}
	fmt.Println(sBad.Render(i18n.T("apply.effectworse")))
	out, err := apply.Rollback(ctx, snapshot.Dir)
	if strings.TrimSpace(out) != "" {
		fmt.Println(sDim.Render(strings.TrimSpace(out)))
	}
	return err
}

// metricValue reads one promised number out of a finished load run.
func metricValue(result suite.Result, metric string) (float64, bool) {
	if result.Load == nil {
		return 0, false
	}
	switch metric {
	case apply.MetricBufferbloat:
		return result.Load.WorstDelta(), true
	case apply.MetricUploadRetrans:
		if result.Load.Upload == nil {
			return 0, false
		}
		return result.Load.Upload.SNMPDelta.RetransPct, true
	}
	return 0, false
}

// cmdSweep finds the shaping rate for this line by trying rates and measuring.
//
// Picking that number is the step where SQM is usually abandoned: it belongs to
// the line rather than to anything the modem advertises, and the only way to
// find it is to shape, saturate, and watch the latency. Guessing it once is
// hard; guessing it repeatedly is what makes people give up and leave the link
// unshaped.
func cmdSweep(args []string) error {
	flags := flag.NewFlagSet("sweep", flag.ContinueOnError)
	steps := flags.Int("steps", 3, "how many rates to try below the measured line speed")
	keep := flags.Bool("keep", false, "leave the winning rate in force when the sweep ends")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := config.Load()

	fmt.Println(sTitle.Render(i18n.T("sweep.title")))
	fmt.Println(sDim.Render(i18n.T("sweep.intro", *steps+1)))
	fmt.Println()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	printed := false
	report := func(point suite.SweepPoint) {
		if !printed {
			fmt.Printf("  %-12s %9s %9s %7s %9s\n",
				i18n.T("sweep.col.rate"), i18n.T("sweep.col.down"),
				i18n.T("sweep.col.up"), i18n.T("sweep.col.grade"), i18n.T("sweep.col.bloat"))
			printed = true
		}
		label := i18n.T("sweep.unshaped")
		if !point.Unshaped {
			label = fmt.Sprintf("%d/%d", point.DownMbit, point.UpMbit)
		}
		style := sOK
		if point.Bloat() >= cfg.Thresholds.BloatWarn {
			style = sWarn
		}
		fmt.Printf("  %-12s %8.1f %8.1f %7s %8.1f\n", label,
			point.DownMbps, point.UpMbps, style.Render(point.Grade), point.Bloat())
	}

	result, err := suite.Sweep(ctx, cfg, *steps, report)
	fmt.Println()
	if err != nil {
		// leave nothing half-applied behind, whatever went wrong
		_ = suite.RestoreShaping(ctx, suite.SweepResult{Iface: result.Iface})
		return err
	}

	if result.Best == nil {
		fmt.Println(sWarn.Render(i18n.T("sweep.none")))
		_ = suite.RestoreShaping(ctx, suite.SweepResult{Iface: result.Iface})
		return nil
	}
	best := *result.Best
	cost := 0.0
	if result.Baseline != nil && result.Baseline.DownMbps > 0 {
		cost = (result.Baseline.DownMbps - best.DownMbps) / result.Baseline.DownMbps * 100
	}
	fmt.Println(sOK.Render(i18n.T("sweep.best", best.DownMbit, best.UpMbit, best.Bloat(), cost)))

	if !*keep {
		_ = suite.RestoreShaping(ctx, suite.SweepResult{Iface: result.Iface})
		fmt.Println(sDim.Render(i18n.T("sweep.notkept")))
		return nil
	}
	if err := suite.RestoreShaping(ctx, result); err != nil {
		return err
	}
	fmt.Println(sDim.Render(i18n.T("sweep.kept")))
	return nil
}

// --- apply / rollback --------------------------------------------------------

func cmdApply(args []string) error {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	list := flags.Bool("list", false, "list applicable changes and exit")
	dryRun := flags.Bool("dry-run", false, "write the snapshot and scripts without running them")
	safe := flags.Bool("safe", false, "apply every change that cannot interrupt the link")
	includeLink := flags.Bool("include-link", false, "also allow changes that renegotiate the link")
	noVerify := flags.Bool("no-verify", false, "skip the connectivity check and auto-rollback")
	measure := flags.Bool("measure", false, "re-run the load test afterwards and roll back a change that made things worse")
	assumeYes := flags.Bool("y", false, "skip the confirmation")
	if err := flags.Parse(args); err != nil {
		return err
	}

	result, _, err := report.LatestRun()
	if err != nil {
		return errors.New(i18n.T("misc.no_runs"))
	}
	// the run may be hours old and the machine may have moved on; offering a
	// change that is already in place is the whole complaint this fixes
	suite.RefreshEnv(&result, config.Load())

	available := apply.Available(result)
	if len(available) == 0 {
		fmt.Println(i18n.T("apply.listempty"))
		return nil
	}

	if *list || (!*safe && flags.NArg() == 0) {
		fmt.Println(sTitle.Render(i18n.T("apply.available")))
		for _, change := range available {
			style := sOK
			switch change.Risk {
			case apply.RiskMedium:
				style = sWarn
			case apply.RiskLink:
				style = sBad
			}
			fmt.Printf("  %-20s %s  %s\n", sAcc.Render(change.ID),
				style.Render(fmt.Sprintf("%-6s", change.Risk)), change.Title)
			for _, line := range change.Apply {
				fmt.Println("      " + sDim.Render("$ "+line))
			}
		}
		fmt.Println()
		fmt.Println(sDim.Render(i18n.T("apply.risklink")))
		fmt.Println(sDim.Render("nabiz apply --safe   |   nabiz apply <id> [<id>...]"))
		return nil
	}

	var selected []apply.Change
	if *safe {
		selected = apply.Filter(available, *includeLink)
	} else {
		selected, err = apply.Select(available, flags.Args())
		if err != nil {
			return err
		}
	}
	if len(selected) == 0 {
		fmt.Println(i18n.T("apply.nothing"))
		return nil
	}

	snapshot, err := apply.Prepare(selected)
	if err != nil {
		return err
	}
	restoreScript := filepath.Join(snapshot.Dir, "restore.sh")
	fmt.Println(sTitle.Render(i18n.T("apply.available")))
	for _, change := range selected {
		fmt.Printf("  %s  %s\n", sAcc.Render(change.ID), change.Title)
	}
	fmt.Println()
	fmt.Println(sDim.Render(i18n.T("apply.snapshot", snapshot.Dir)))
	fmt.Println(sDim.Render(i18n.T("apply.restorehint", restoreScript)))

	if *dryRun {
		return nil
	}
	if !*assumeYes {
		fmt.Print(i18n.T("apply.confirm", len(selected)))
		var answer string
		_, _ = fmt.Scanln(&answer)
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes", "e", "evet":
		default:
			return errors.New(i18n.T("ui.cancelled"))
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !*noVerify {
		fmt.Println(sDim.Render(i18n.T("apply.verifying")))
	}
	outcome := apply.Apply(ctx, snapshot, !*noVerify)
	if strings.TrimSpace(outcome.Output) != "" {
		fmt.Println(sDim.Render(strings.TrimSpace(outcome.Output)))
	}
	switch {
	case outcome.RolledBack:
		fmt.Println(sBad.Render(i18n.T("apply.rolledback")))
	case outcome.Err != nil:
		return outcome.Err
	default:
		fmt.Println(sOK.Render(i18n.T("apply.ok")))
		if *measure {
			if err := measureEffect(ctx, snapshot, selected); err != nil {
				return err
			}
		}
		// re-read the machine into the stored run, otherwise the next
		// `nabiz apply --list` still offers what was just applied
		suite.RefreshEnv(&result, config.Load())
		if _, err := report.Autosave(result); err == nil {
			remaining := apply.Available(result)
			fmt.Println()
			if len(remaining) == 0 {
				fmt.Println(sDim.Render(i18n.T("apply.listempty")))
			} else {
				fmt.Println(sDim.Render(i18n.T("apply.remaining", len(remaining))))
				for _, change := range remaining {
					fmt.Printf("  %s  %s\n", sAcc.Render(change.ID), change.Title)
				}
			}
		}
	}
	fmt.Println(sDim.Render(i18n.T("apply.restorehint", restoreScript)))
	return nil
}

func cmdRollback(args []string) error {
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	list := flags.Bool("list", false, "list stored snapshots")
	if err := flags.Parse(args); err != nil {
		return err
	}
	snapshots, err := apply.List()
	if err != nil {
		return err
	}
	if len(snapshots) == 0 {
		fmt.Println(i18n.T("apply.nosnapshots"))
		return nil
	}
	if *list {
		for _, snapshot := range snapshots {
			var ids []string
			for _, change := range snapshot.Changes {
				ids = append(ids, change.ID)
			}
			fmt.Printf("%-40s %s\n", snapshot.Dir, strings.Join(ids, ", "))
		}
		return nil
	}
	target := ""
	if flags.NArg() > 0 {
		target = flags.Arg(0)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	out, err := apply.Rollback(ctx, target)
	if strings.TrimSpace(out) != "" {
		fmt.Println(sDim.Render(strings.TrimSpace(out)))
	}
	if err != nil {
		return err
	}
	name := target
	if name == "" {
		name = snapshots[len(snapshots)-1].Dir
	}
	fmt.Println(sOK.Render(i18n.T("apply.rollbackok", name)))
	return nil
}
