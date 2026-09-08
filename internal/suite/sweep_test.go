package suite

import (
	"strings"
	"testing"

	"github.com/WinTone01/nabiz/internal/probe"
)

// The sweep must pick the fastest rate that met the target, not the calmest.
// The calmest is always the slowest, and handing back a quarter of the line for
// latency nobody would have noticed is the other way people end up abandoning
// SQM.
func TestSweepPrefersTheFastestRateThatMetTheTarget(t *testing.T) {
	points := []SweepPoint{
		{Unshaped: true, DownDelta: 207, UpDelta: 37},
		{DownMbit: 84, UpMbit: 18, DownDelta: 87, UpDelta: 2},
		{DownMbit: 78, UpMbit: 18, DownDelta: 4, UpDelta: 3},
		{DownMbit: 65, UpMbit: 18, DownDelta: 3, UpDelta: 3},
	}
	best := pickBest(points, 30)
	if best == nil {
		t.Fatal("nothing chosen")
	}
	if best.DownMbit != 78 {
		t.Errorf("chose %d Mbit, want 78 - the fastest that came in under the threshold",
			best.DownMbit)
	}
}

// When no rate reached the target the queue is probably not where shaping can
// reach it, and claiming a winner would be worse than saying so.
func TestSweepReportsNoWinnerRatherThanTheLeastBad(t *testing.T) {
	points := []SweepPoint{
		{Unshaped: true, DownDelta: 300},
		{DownMbit: 80, DownDelta: 190},
		{DownMbit: 60, DownDelta: 140},
	}
	if best := pickBest(points, 30); best != nil {
		t.Errorf("declared %d Mbit a winner at +%.0f ms", best.DownMbit, best.Bloat())
	}
}

// The commands have to build the ingress path, not just the egress one: that is
// the whole reason the sweep exists on an asymmetric line.
func TestSweepShapesBothDirections(t *testing.T) {
	state := probe.SQMState{Iface: "enp3s0", IFBName: "ifb-enp3s0"}
	script := strings.Join(applyShaping(state, 19, 78), "\n")
	for _, want := range []string{"ip link add ifb-enp3s0 type ifb", "mirred egress redirect",
		"dev enp3s0 root cake bandwidth 19mbit", "dev ifb-enp3s0 root cake bandwidth 78mbit",
		"ingress"} {
		if !strings.Contains(script, want) {
			t.Errorf("missing %q in:\n%s", want, script)
		}
	}
	clear := strings.Join(clearShaping(state), "\n")
	for _, want := range []string{"qdisc del dev enp3s0 root", "qdisc del dev enp3s0 ingress",
		"ip link del ifb-enp3s0", "root fq_codel"} {
		if !strings.Contains(clear, want) {
			t.Errorf("missing %q in:\n%s", want, clear)
		}
	}
}
