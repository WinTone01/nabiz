package report

import (
	"fmt"
	"strings"

	"github.com/WinTone01/nabiz/internal/suite"
)

// DiffRow is one metric measured under both conditions of an A/B run.
type DiffRow struct {
	Metric        string  `json:"metric"`
	Before        float64 `json:"before"`
	After         float64 `json:"after"`
	Delta         float64 `json:"delta"`
	Unit          string  `json:"unit"`
	LowerIsBetter bool    `json:"lower_is_better"`
}

// Better reports whether the change moved in the desired direction.
func (d DiffRow) Better() bool {
	if d.LowerIsBetter {
		return d.Delta < 0
	}
	return d.Delta > 0
}

// Significant filters out noise-level differences so the table shows decisions,
// not jitter.
func (d DiffRow) Significant() bool {
	scale := d.Before
	if scale < 0 {
		scale = -scale
	}
	if scale < 1 {
		return abs(d.Delta) > 0.05
	}
	return abs(d.Delta)/scale > 0.03
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// DiffRows compares two runs metric by metric.
func DiffRows(before, after suite.Result) []DiffRow {
	var rows []DiffRow
	add := func(metric string, a, b float64, unit string, lowerBetter bool) {
		rows = append(rows, DiffRow{Metric: metric, Before: a, After: b,
			Delta: b - a, Unit: unit, LowerIsBetter: lowerBetter})
	}

	latA, latB := before.InternetLatency(), after.InternetLatency()
	if latA != nil && latB != nil {
		add("gecikme ort", latA.Avg, latB.Avg, "ms", true)
		add("gecikme p95", latA.P95, latB.P95, "ms", true)
		add("jitter", latA.Jitter, latB.Jitter, "ms", true)
		add("paket kaybı", latA.LossPct, latB.LossPct, "%", true)
		add("MOS", latA.MOS, latB.MOS, "", false)
	}
	if before.Load != nil && after.Load != nil {
		if before.Load.Download != nil && after.Load.Download != nil {
			add("indirme", before.Load.Download.Bps/1e6, after.Load.Download.Bps/1e6, "Mbps", false)
			add("indirme retrans", before.Load.Download.Sockets.RetransPct,
				after.Load.Download.Sockets.RetransPct, "%", true)
		}
		if before.Load.Upload != nil && after.Load.Upload != nil {
			add("yükleme", before.Load.Upload.Bps/1e6, after.Load.Upload.Bps/1e6, "Mbps", false)
		}
		add("bufferbloat", before.Load.WorstDelta(), after.Load.WorstDelta(), "ms", true)
	}
	if len(before.DNSBench) > 0 && len(after.DNSBench) > 0 {
		if a, b := bestDNS(before), bestDNS(after); a > 0 && b > 0 {
			add("en hızlı DNS", a, b, "ms", true)
		}
	}
	if a, okA := reachable(before); okA {
		if b, okB := reachable(after); okB {
			add("doğrudan açılan site", a, b, "%", false)
		}
	}
	add("TCP retrans", before.Env.TCPHealth.RetransPct, after.Env.TCPHealth.RetransPct, "%", true)
	add("puan", before.Score, after.Score, "", false)
	return rows
}

func bestDNS(result suite.Result) float64 {
	best := -1.0
	for _, row := range result.DNSBench {
		if row.Failures > 0 || row.AvgMs <= 0 || row.Kind == "system" ||
			strings.HasPrefix(row.Address, "127.") {
			continue
		}
		if best < 0 || row.AvgMs < best {
			best = row.AvgMs
		}
	}
	return best
}

func reachable(result suite.Result) (float64, bool) {
	if len(result.DPI) == 0 {
		return 0, false
	}
	clean := 0
	for _, verdict := range result.DPI {
		if verdict.Verdict == "clean" {
			clean++
		}
	}
	return 100 * float64(clean) / float64(len(result.DPI)), true
}

// ABMarkdown renders the comparison table.
func ABMarkdown(labelA string, before suite.Result, labelB string, after suite.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Nabız A/B karşılaştırması\n\n")
	fmt.Fprintf(&b, "- A: **%s** (%s)\n", labelA, before.StartedAt.Format("15:04:05"))
	fmt.Fprintf(&b, "- B: **%s** (%s)\n\n", labelB, after.StartedAt.Format("15:04:05"))
	fmt.Fprintf(&b, "| Ölçüt | %s | %s | Fark |\n|---|---:|---:|---:|\n", labelA, labelB)
	for _, row := range DiffRows(before, after) {
		mark := "·"
		if row.Significant() {
			if row.Better() {
				mark = "🟢"
			} else {
				mark = "🔴"
			}
		}
		fmt.Fprintf(&b, "| %s | %.2f %s | %.2f %s | %+.2f %s |\n", row.Metric,
			row.Before, row.Unit, row.After, row.Unit, row.Delta, mark)
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}
