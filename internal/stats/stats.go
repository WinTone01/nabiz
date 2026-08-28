// Package stats turns raw latency samples into the numbers people argue about:
// percentiles, jitter, MOS, and letter grades.
package stats

import (
	"math"
	"sort"
	"strings"
)

// Spark is the ramp used by every inline chart in the UI.
var Spark = []rune("▁▂▃▄▅▆▇█")

// Sample is one probe result; Lost distinguishes "no reply" from "0 ms".
type Sample struct {
	RTTms float64
	Lost  bool
}

// Percentile returns the linear-interpolated percentile of unsorted values.
func Percentile(values []float64, pct float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	if len(ordered) == 1 {
		return ordered[0]
	}
	pos := float64(len(ordered)-1) * pct / 100
	low := int(math.Floor(pos))
	high := int(math.Ceil(pos))
	if low == high {
		return ordered[low]
	}
	return ordered[low] + (ordered[high]-ordered[low])*(pos-float64(low))
}

// Mean of a slice, 0 for empty.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

// StdDev is the sample standard deviation.
func StdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	avg := Mean(values)
	total := 0.0
	for _, value := range values {
		total += (value - avg) * (value - avg)
	}
	return math.Sqrt(total / float64(len(values)-1))
}

// RFC3550Jitter is the smoothed inter-arrival jitter VoIP equipment reports.
func RFC3550Jitter(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	jitter := 0.0
	for i := 1; i < len(values); i++ {
		jitter += (math.Abs(values[i]-values[i-1]) - jitter) / 16
	}
	return jitter
}

// MeanAbsDelta is mdev, the average step between consecutive samples.
func MeanAbsDelta(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(values); i++ {
		total += math.Abs(values[i] - values[i-1])
	}
	return total / float64(len(values)-1)
}

// MOS is a simplified ITU E-model score in [1.0, 4.5].
func MOS(avgRTT, jitter, lossPct float64) float64 {
	effective := avgRTT + jitter*2 + 10
	var r float64
	if effective < 160 {
		r = 93.2 - effective/40
	} else {
		r = 93.2 - (effective-120)/10
	}
	r -= lossPct * 2.5
	r = math.Max(0, math.Min(100, r))
	mos := 1 + 0.035*r + 7e-6*r*(r-60)*(100-r)
	return round2(math.Max(1, math.Min(4.5, mos)))
}

// Summary is the aggregate of one target's probe series.
type Summary struct {
	Label      string    `json:"label"`
	Target     string    `json:"target"`
	Sent       int       `json:"sent"`
	Received   int       `json:"received"`
	LossPct    float64   `json:"loss_pct"`
	Min        float64   `json:"min"`
	Avg        float64   `json:"avg"`
	Max        float64   `json:"max"`
	P50        float64   `json:"p50"`
	P95        float64   `json:"p95"`
	P99        float64   `json:"p99"`
	StdDev     float64   `json:"stdev"`
	Jitter     float64   `json:"jitter"`
	MDev       float64   `json:"mdev"`
	MOS        float64   `json:"mos"`
	WorstBurst int       `json:"worst_loss_burst"`
	Samples    []Sample  `json:"-"`
	RTTs       []float64 `json:"rtts"`
}

// Summarize folds a probe series into a Summary.
func Summarize(label, target string, samples []Sample) Summary {
	out := Summary{Label: label, Target: target, Sent: len(samples), Samples: samples}
	if len(samples) == 0 {
		return out
	}
	got := make([]float64, 0, len(samples))
	burst, worst := 0, 0
	for _, sample := range samples {
		if sample.Lost {
			burst++
			if burst > worst {
				worst = burst
			}
			continue
		}
		burst = 0
		got = append(got, sample.RTTms)
	}
	out.Received = len(got)
	out.WorstBurst = worst
	out.LossPct = round2(100 * float64(len(samples)-len(got)) / float64(len(samples)))
	if len(got) == 0 {
		return out
	}
	out.RTTs = got
	out.Min, out.Max = got[0], got[0]
	for _, value := range got {
		if value < out.Min {
			out.Min = value
		}
		if value > out.Max {
			out.Max = value
		}
	}
	out.Avg = round2(Mean(got))
	out.P50 = round2(Percentile(got, 50))
	out.P95 = round2(Percentile(got, 95))
	out.P99 = round2(Percentile(got, 99))
	out.StdDev = round2(StdDev(got))
	out.Jitter = round2(RFC3550Jitter(got))
	out.MDev = round2(MeanAbsDelta(got))
	out.Min, out.Max = round2(out.Min), round2(out.Max)
	out.MOS = MOS(out.Avg, out.Jitter, out.LossPct)
	return out
}

// --- grading ------------------------------------------------------------

// BufferbloatGrade maps the latency increase under load to a letter.
func BufferbloatGrade(deltaMs float64) string {
	switch {
	case deltaMs < 5:
		return "A+"
	case deltaMs < 30:
		return "A"
	case deltaMs < 60:
		return "B"
	case deltaMs < 100:
		return "C"
	case deltaMs < 200:
		return "D"
	default:
		return "F"
	}
}

// LetterFromScore converts a 0-100 score into a letter grade.
func LetterFromScore(score float64) string {
	switch {
	case score >= 95:
		return "A+"
	case score >= 88:
		return "A"
	case score >= 80:
		return "B+"
	case score >= 72:
		return "B"
	case score >= 62:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

// StabilityScore weighs loss hardest, then jitter, then the p95/avg spread.
func StabilityScore(lossPct, jitter, p95, avg float64, bloatMs float64, haveBloat bool) float64 {
	score := 100.0
	score -= math.Min(45, lossPct*9)
	score -= math.Min(20, jitter*1.6)
	score -= math.Min(15, math.Max(0, p95-avg)*0.35)
	score -= math.Min(10, math.Max(0, avg-40)*0.08)
	if haveBloat {
		score -= math.Min(20, bloatMs*0.09)
	}
	return round1(math.Max(0, score))
}

// --- text charts --------------------------------------------------------

// Sparkline renders samples as block characters; losses become '!'.
func Sparkline(samples []Sample, width int) string {
	if width <= 0 || len(samples) == 0 {
		return strings.Repeat(" ", max(0, width))
	}
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}
	low, high := math.MaxFloat64, -math.MaxFloat64
	any := false
	for _, sample := range samples {
		if sample.Lost {
			continue
		}
		any = true
		low = math.Min(low, sample.RTTms)
		high = math.Max(high, sample.RTTms)
	}
	if !any {
		return strings.Repeat("!", len(samples))
	}
	if high-low < 1e-9 {
		high = low + 1
	}
	var builder strings.Builder
	for _, sample := range samples {
		if sample.Lost {
			builder.WriteRune('!')
			continue
		}
		index := int((sample.RTTms-low)/(high-low)*float64(len(Spark)-1) + 0.5)
		if index < 0 {
			index = 0
		}
		if index >= len(Spark) {
			index = len(Spark) - 1
		}
		builder.WriteRune(Spark[index])
	}
	return builder.String()
}

// SparklineF is Sparkline for plain float series (throughput, latency traces).
func SparklineF(values []float64, width int) string {
	samples := make([]Sample, len(values))
	for i, value := range values {
		samples[i] = Sample{RTTms: value}
	}
	return Sparkline(samples, width)
}

// Bar draws a proportional bar with sub-character resolution.
func Bar(fraction float64, width int) string {
	blocks := []rune(" ▏▎▍▌▋▊▉█")
	if width <= 0 {
		return ""
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	total := fraction * float64(width)
	full := int(total)
	rest := total - float64(full)
	var builder strings.Builder
	for i := 0; i < full && i < width; i++ {
		builder.WriteRune('█')
	}
	if full < width {
		builder.WriteRune(blocks[int(rest*8)])
	}
	out := []rune(builder.String())
	for len(out) < width {
		out = append(out, ' ')
	}
	return string(out[:width])
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }
func round2(value float64) float64 { return math.Round(value*100) / 100 }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
