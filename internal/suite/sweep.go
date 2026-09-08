package suite

import (
	"context"
	"fmt"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/util"
)

// Picking a shaping rate is the step where SQM is usually abandoned. The right
// number is a property of the line, not of the modem's advertised speed, and it
// is found by trying rates and watching latency - shape too high and the queue
// still builds in the modem, too low and throughput is given away for nothing.
// Asking a person to guess it, then to guess again, is why so many links end up
// with no shaping at all. This does the sweep.

// SweepPoint is one rate that was tried and what it cost.
type SweepPoint struct {
	DownMbit  int     `json:"down_mbit"`
	UpMbit    int     `json:"up_mbit"`
	DownDelta float64 `json:"down_delta"`
	UpDelta   float64 `json:"up_delta"`
	Grade     string  `json:"grade"`
	DownMbps  float64 `json:"down_mbps"`
	UpMbps    float64 `json:"up_mbps"`
	Unshaped  bool    `json:"unshaped,omitempty"`
}

// Bloat is the number the choice is made on.
func (p SweepPoint) Bloat() float64 {
	if p.UpDelta > p.DownDelta {
		return p.UpDelta
	}
	return p.DownDelta
}

// SweepResult is the whole sweep, with the rate worth keeping.
type SweepResult struct {
	Points   []SweepPoint `json:"points"`
	Best     *SweepPoint  `json:"best,omitempty"`
	Baseline *SweepPoint  `json:"baseline,omitempty"`
	Iface    string       `json:"iface"`
}

// Sweep measures the line unshaped, then at descending download rates, and
// returns the highest rate that kept latency under the threshold.
//
// Upload is not swept. Our own transmit queue is directly controlled, so a
// single margin under the measured rate is enough there; download is the half
// that needs searching, because its queue is inside the modem and only responds
// to us indirectly.
func Sweep(ctx context.Context, cfg config.Config, steps int, report func(SweepPoint)) (SweepResult, error) {
	state := probe.ReadSQM("")
	out := SweepResult{Iface: state.Iface}
	if !state.Possible() {
		return out, fmt.Errorf("cake or ifb is not available on this kernel")
	}
	if steps < 1 {
		steps = 3
	}

	// The unshaped line is the only honest starting point: a run measured
	// through an existing shaper measures the shaper.
	if _, err := util.Privileged(ctx, clearShaping(state)); err != nil {
		return out, err
	}
	base, err := measurePoint(ctx, cfg, 0, 0)
	if err != nil {
		return out, err
	}
	base.Unshaped = true
	out.Baseline = &base
	out.Points = append(out.Points, base)
	if report != nil {
		report(base)
	}

	up := int(base.UpMbps * 0.92)
	if up < 1 {
		up = 1
	}
	// 90% down to 60% of what the line delivered, which brackets every knee
	// seen in practice; below that the cost stops being worth the latency.
	for index := range steps {
		fraction := 0.90 - 0.30*float64(index)/float64(maxInt(steps-1, 1))
		down := int(base.DownMbps * fraction)
		if down < 1 {
			continue
		}
		if _, err := util.Privileged(ctx, applyShaping(state, up, down)); err != nil {
			return out, err
		}
		point, err := measurePoint(ctx, cfg, up, down)
		if err != nil {
			return out, err
		}
		out.Points = append(out.Points, point)
		if report != nil {
			report(point)
		}
	}

	out.Best = pickBest(out.Points, cfg.Thresholds.BloatWarn)
	return out, nil
}

// pickBest takes the fastest rate that stayed under the threshold, not simply
// the calmest. The calmest is always the slowest, and handing back a quarter of
// the line for latency nobody would have noticed is the other way people end up
// abandoning SQM. When nothing met the target it returns nil rather than the
// least bad: a queue that shaping cannot reach is worth saying out loud.
func pickBest(points []SweepPoint, threshold float64) *SweepPoint {
	var best *SweepPoint
	for index := range points {
		point := points[index]
		if point.Unshaped || point.Bloat() >= threshold {
			continue
		}
		if best == nil || point.DownMbit > best.DownMbit {
			best = &points[index]
		}
	}
	return best
}

// measurePoint runs the load suite and records what this rate cost.
func measurePoint(ctx context.Context, cfg config.Config, up, down int) (SweepPoint, error) {
	result := Run(ctx, cfg, "load", nil)
	if result.Load == nil {
		return SweepPoint{}, fmt.Errorf("the load measurement did not complete")
	}
	point := SweepPoint{
		DownMbit: down, UpMbit: up,
		DownDelta: result.Load.DownDelta, UpDelta: result.Load.UpDelta,
		Grade: result.Load.Grade,
	}
	if result.Load.Download != nil {
		point.DownMbps = result.Load.Download.Bps / 1e6
	}
	if result.Load.Upload != nil {
		point.UpMbps = result.Load.Upload.Bps / 1e6
	}
	return point, nil
}

func clearShaping(state probe.SQMState) []string {
	return []string{
		fmt.Sprintf("tc qdisc del dev %s root 2>/dev/null || true", state.Iface),
		fmt.Sprintf("tc qdisc del dev %s ingress 2>/dev/null || true", state.Iface),
		fmt.Sprintf("ip link del %s 2>/dev/null || true", state.IFBName),
		fmt.Sprintf("tc qdisc replace dev %s root fq_codel", state.Iface),
	}
}

func applyShaping(state probe.SQMState, up, down int) []string {
	return []string{
		"modprobe ifb numifbs=0 2>/dev/null || true",
		fmt.Sprintf("ip link show %s >/dev/null 2>&1 || ip link add %s type ifb",
			state.IFBName, state.IFBName),
		fmt.Sprintf("ip link set %s up", state.IFBName),
		fmt.Sprintf("tc qdisc replace dev %s root cake bandwidth %dmbit diffserv4 triple-isolate nat ack-filter",
			state.Iface, up),
		fmt.Sprintf("tc qdisc replace dev %s handle ffff: ingress", state.Iface),
		fmt.Sprintf("tc filter replace dev %s parent ffff: protocol all matchall action mirred egress redirect dev %s",
			state.Iface, state.IFBName),
		fmt.Sprintf("tc qdisc replace dev %s root cake bandwidth %dmbit besteffort triple-isolate nat wash ingress",
			state.IFBName, down),
	}
}

// RestoreShaping puts the chosen rate back, or clears shaping when the sweep
// found nothing worth keeping. A sweep that exits leaving the last, slowest
// rate in force would be worse than not running it.
func RestoreShaping(ctx context.Context, result SweepResult) error {
	state := probe.ReadSQM(result.Iface)
	lines := clearShaping(state)
	if result.Best != nil {
		lines = applyShaping(state, result.Best.UpMbit, result.Best.DownMbit)
	}
	_, err := util.Privileged(ctx, lines)
	return err
}
