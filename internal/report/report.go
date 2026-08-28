// Package report serialises a run to JSON and renders it as Markdown.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// SaveJSON writes the full result, creating parent directories as needed.
func SaveJSON(result suite.Result, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Autosave stores every completed run under the data directory.
func Autosave(result suite.Result) (string, error) {
	name := fmt.Sprintf("%s-%s.json", result.StartedAt.Format("20060102-150405"), result.Name)
	path := filepath.Join(config.RunsDir(), name)
	return path, SaveJSON(result, path)
}

// LoadJSON reads a previously saved run.
func LoadJSON(path string) (suite.Result, error) {
	var result suite.Result
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	return result, json.Unmarshal(data, &result)
}

// ListRuns returns saved runs, newest first.
func ListRuns(limit int) []string {
	entries, err := os.ReadDir(config.RunsDir())
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = filepath.Join(config.RunsDir(), name)
	}
	return out
}

var levelMark = map[string]string{"bad": "🔴", "warn": "🟡", "info": "🔵", "ok": "🟢"}

// Markdown renders a run as a document you can paste into a ticket.
func Markdown(result suite.Result) string {
	var b strings.Builder
	write := func(format string, args ...any) {
		fmt.Fprintf(&b, format+"\n", args...)
	}

	write("# Nabız raporu — %s", result.Name)
	write("")
	write("- Tarih: %s", result.StartedAt.Format("2006-01-02 15:04:05"))
	write("- Süre: %.1f s", result.Duration)
	write("- Makine: %s (%s)", result.Env.Host, result.Env.Kernel)
	write("- Puan: **%.1f / 100 (%s)**", result.Score, result.Grade)
	write("")

	link := result.Env.Link
	write("## Ortam")
	write("")
	write("| | |")
	write("|---|---|")
	write("| Arayüz | %s (%s) |", link.Iface, link.Address)
	write("| Hız / MTU / duplex | %d Mbit/s · %d · %s |", link.SpeedMbit, link.MTU, link.Duplex)
	write("| Qdisc | %s |", orDash(link.Qdisc))
	write("| Link flap | %d |", link.CarrierUps)
	if link.Wireless {
		write("| Wi-Fi | %s · %.0f dBm · %s |", link.SSID, link.SignalDBm, link.TxBitrate)
	}
	if link.Driver != "" {
		write("| Sürücü | %s |", link.Driver)
	}
	health := result.Env.TCPHealth
	write("| TCP yeniden gönderim | %d / %d (%%%.2f) |", health.RetransSegs, health.OutSegs, health.RetransPct)
	write("| Conntrack | %d / %d |", result.Env.Conntrack.Count, result.Env.Conntrack.Max)
	if unwall := result.Env.Unwall; unwall.Installed {
		state := "durdurulmuş"
		if unwall.Running {
			state = "çalışıyor"
		}
		write("| Unwall | %s · %s · %s |", unwall.Engine, unwall.Strategy, state)
		write("| Şifreli DNS | %s |", dnsLabel(unwall))
		write("| Hostlist | %s (manuel %d / otomatik %d) |", unwall.HostlistMode,
			unwall.HostlistN, unwall.AutoHostlistN)
	}
	if bpftune := result.Env.Bpftune; bpftune.Installed {
		state := "durdurulmuş"
		if bpftune.Running {
			state = "çalışıyor"
		}
		write("| bpftune | %s · %d tunable değişikliği |", state, len(bpftune.Changes))
	}
	write("")

	if len(result.Findings) > 0 {
		write("## Bulgular")
		write("")
		for _, finding := range result.Findings {
			mark := levelMark[finding.Level]
			if mark == "" {
				mark = "·"
			}
			line := fmt.Sprintf("- %s **%s** — %s", mark, finding.Key, finding.Title)
			if finding.Hint != "" {
				line += "  \n  _" + finding.Hint + "_"
			}
			write("%s", line)
		}
		write("")
	}

	if len(result.Advice) > 0 {
		write("## Öneriler")
		write("")
		current := -1
		for _, advice := range result.Advice {
			if advice.Priority != current {
				current = advice.Priority
				write("### Öncelik %d", current)
				write("")
			}
			write("**[%s] %s**", advice.Category, advice.Title)
			write("")
			write("%s", advice.Why)
			write("")
			if len(advice.How) > 0 {
				write("```bash")
				for _, step := range advice.How {
					write("%s", step)
				}
				write("```")
				write("")
			}
			if advice.Gain != "" {
				write("- Beklenen kazanç: %s", advice.Gain)
			}
			if advice.Risk != "" {
				write("- Risk: %s", advice.Risk)
			}
			if len(advice.Revert) > 0 {
				write("- Geri alma: `%s`", strings.Join(advice.Revert, " ; "))
			}
			write("")
		}
	}

	if len(result.Latency) > 0 {
		write("## Gecikme")
		write("")
		write("| Hedef | Kayıp | Ort | p50 | p95 | Jitter | MOS |")
		write("|---|---:|---:|---:|---:|---:|---:|")
		for _, summary := range result.Latency {
			write("| %s (%s) | %%%.1f | %.1f | %.1f | %.1f | %.1f | %.2f |",
				summary.Label, summary.Target, summary.LossPct, summary.Avg,
				summary.P50, summary.P95, summary.Jitter, summary.MOS)
		}
		write("")
	}

	if load := result.Load; load != nil {
		write("## Yük altında davranış (bufferbloat)")
		write("")
		write("| | Hız | Gecikme p95 | Artış |")
		write("|---|---:|---:|---:|")
		write("| Boşta | — | %.1f ms | — |", load.Idle.P50)
		if load.Download != nil {
			write("| İndirme | %s | %.1f ms | +%.1f ms |",
				util.HumanRate(load.Download.Bps), load.DownLatency.P95, load.DownDelta)
		}
		if load.Upload != nil {
			write("| Yükleme | %s | %.1f ms | +%.1f ms |",
				util.HumanRate(load.Upload.Bps), load.UpLatency.P95, load.UpDelta)
		}
		write("")
		write("**Bufferbloat notu: %s**", load.Grade)
		write("")
		if load.Download != nil && load.Download.Sockets.Count > 0 {
			sockets := load.Download.Sockets
			write("İndirme sırasında çekirdek: tıkanıklık algoritması %s · ortalama cwnd %.0f · "+
				"soket yeniden gönderim %%%.2f · min RTT %.1f ms",
				strings.Join(sockets.CCAlgorithms, ", "), sockets.AvgCWnd,
				sockets.RetransPct, sockets.MinRTTms)
			write("")
		}
	}

	if len(result.DNSBench) > 0 {
		write("## DNS çözücüleri")
		write("")
		write("| Çözücü | Tip | Ort | p95 | Hata |")
		write("|---|---|---:|---:|---:|")
		for _, row := range result.DNSBench {
			write("| %s | %s | %.1f | %.1f | %d/%d |", row.Label, row.Kind,
				row.AvgMs, row.P95Ms, row.Failures, row.Queries)
		}
		write("")
	}
	if len(result.DNSChecks) > 0 {
		write("### DNS müdahale kontrolleri")
		write("")
		for _, check := range result.DNSChecks {
			write("- `%s`: **%s** — %s", check.Name, check.Verdict, check.Detail)
		}
		write("")
	}

	if len(result.DPI) > 0 {
		write("## DPI / erişilebilirlik")
		write("")
		write("| Alan adı | Sonuç | TLS | Bölünmüş | QUIC |")
		write("|---|---|---|---|---|")
		for _, verdict := range result.DPI {
			write("| %s | %s | %s | %s | %s |", verdict.Domain, verdict.Verdict,
				orDash(verdict.Whole.Kind), splitLabel(verdict), orDash(verdict.QUIC))
		}
		write("")
	}

	if len(result.Hops) > 0 {
		write("## Yol")
		write("")
		write("| # | IP | Kayıp | En iyi | Ort | En kötü |")
		write("|---:|---|---:|---:|---:|---:|")
		for _, hop := range result.Hops {
			write("| %d | %s | %%%.0f | %.1f | %.1f | %.1f |", hop.TTL, orStar(hop.IP),
				hop.LossPct(), hop.Best(), hop.Avg(), hop.Worst())
		}
		write("")
	}
	if result.MTU != nil {
		write("MTU: arayüz %d · ölçülen yol %d — %s", result.MTU.IfaceMTU,
			result.MTU.ProbedMTU, result.MTU.Detail)
		write("")
	}

	if bpftune := result.Env.Bpftune; bpftune.Installed && len(bpftune.Changes) > 0 {
		write("## bpftune'un yaptığı değişiklikler")
		write("")
		write("| Tunable | Başlangıç | Şu an | Kez | Gerekçe |")
		write("|---|---|---|---:|---|")
		for _, change := range bpftune.Changes {
			write("| `%s` | %s | %s | %d | %s |", change.Tunable, change.From,
				change.To, change.Count, change.Reason)
		}
		write("")
	}

	write("---")
	write("")
	write("_Nabız ile üretildi._")
	return b.String()
}

// SaveMarkdown renders and writes the report.
func SaveMarkdown(result suite.Result, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Markdown(result)), 0o644)
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func orStar(value string) string {
	if value == "" {
		return "*"
	}
	return value
}

func dnsLabel(unwall sysinfo.UnwallState) string {
	if !unwall.DNSEncrypted {
		return "kapalı"
	}
	return unwall.DNSBackend + " / " + unwall.DNSProvider
}

func splitLabel(verdict probe.DomainVerdict) string {
	if verdict.SplitHdr.Kind == "" && verdict.SplitSNI.Kind == "" {
		return "-"
	}
	return orDash(verdict.SplitHdr.Kind) + "/" + orDash(verdict.SplitSNI.Kind)
}
