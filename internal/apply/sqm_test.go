package apply

import (
	"strings"
	"testing"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/suite"
)

func bloatedRun() suite.Result {
	result := suite.Result{
		Env: suite.Env{
			Link: probe.LinkInfo{Iface: "enp3s0", SpeedMbit: 100, Duplex: "full", MTU: 1500},
			SQM: probe.SQMState{
				Iface: "enp3s0", CakeAvailable: true, IFBAvailable: true,
				Egress: "fq_codel", IFBName: "ifb-enp3s0",
			},
			Sysctls: map[string]string{"net.core.default_qdisc": "fq_codel"},
		},
		Load: &probe.BloatResult{
			DownDelta: 207, UpDelta: 37, Grade: "F",
			Download: &probe.TransferResult{Bps: 93e6},
			Upload:   &probe.TransferResult{Bps: 19.6e6},
		},
	}
	suite.Finalize(&result, config.Default())
	return result
}

// The shaping has to reach the download direction. Egress-only leaves the queue
// that actually hurts on an asymmetric line - the one inside the modem -
// completely untouched, which is what the advice used to do.
func TestSQMShapesBothDirections(t *testing.T) {
	changes := Available(bloatedRun())
	var sqm *Change
	for i := range changes {
		if changes[i].ID == "sqm" {
			sqm = &changes[i]
		}
	}
	if sqm == nil {
		t.Fatalf("no sqm change offered; got %v", ids(changes))
	}
	script := strings.Join(sqm.Apply, "\n")
	for _, want := range []string{
		"ifb-enp3s0",                 // the ingress device exists
		"mirred egress redirect",     // traffic actually reaches it
		"ingress",                    // and is shaped there
		"dev enp3s0 root cake",       // upload shaped too
		"dispatcher.d/60-nabiz-sqm",  // and it survives a relink
	} {
		if !strings.Contains(script, want) {
			t.Errorf("apply script is missing %q:\n%s", want, script)
		}
	}
	// rates come from the measurement, under it in both directions
	if !strings.Contains(script, "18mbit") { // 19.6 * 0.92
		t.Errorf("upload rate not derived from the measurement:\n%s", script)
	}
	if !strings.Contains(script, "79mbit") { // 93 * 0.85
		t.Errorf("download rate not derived from the measurement:\n%s", script)
	}

	restore := strings.Join(sqm.Restore, "\n")
	for _, want := range []string{"rm -f /etc/NetworkManager/dispatcher.d/60-nabiz-sqm",
		"ip link del ifb-enp3s0", "root fq_codel"} {
		if !strings.Contains(restore, want) {
			t.Errorf("restore is missing %q:\n%s", want, restore)
		}
	}
}

// The batch runner appends "|| echo failed" to every apply line, so a line that
// spans more than one line of shell would be corrupted. A heredoc is the easy
// way to get that wrong.
func TestSQMApplyLinesAreSingleShellLines(t *testing.T) {
	changes := Available(bloatedRun())
	for _, change := range changes {
		for _, line := range change.Apply {
			if strings.Contains(line, "\n") {
				t.Errorf("%s: apply line spans multiple lines: %q", change.ID, line)
			}
		}
	}
}

// Without cake or an IFB device the recipe cannot be installed, and offering it
// would produce a batch that half-fails.
func TestSQMNotOfferedWithoutKernelSupport(t *testing.T) {
	result := bloatedRun()
	result.Env.SQM.IFBAvailable = false
	suite.Finalize(&result, config.Default())
	for _, change := range Available(result) {
		if change.ID == "sqm" {
			t.Error("offered SQM on a machine with no IFB support")
		}
	}
}

func ids(changes []Change) []string {
	out := make([]string, 0, len(changes))
	for _, change := range changes {
		out = append(out, change.ID)
	}
	return out
}

// bpftune is only worth turning off when the run actually caught it costing
// something, and the evidence is the retransmission during the transfer, not
// the kernel-wide average.
func TestBpftuneOfferedOnMeasuredRetransmission(t *testing.T) {
	result := bloatedRun()
	result.Env.Bpftune.Installed = true
	result.Env.Bpftune.Running = true
	result.Load.Upload.SNMPDelta.OutSegs = 10570
	result.Load.Upload.SNMPDelta.RetransSegs = 1067
	result.Load.Upload.SNMPDelta.RetransPct = 10.09
	suite.Finalize(&result, config.Default())

	found := false
	for _, change := range Available(result) {
		if change.ID == "bpftune-retrans" {
			found = true
			if strings.Join(change.Apply, " ") != "systemctl disable --now bpftune" {
				t.Errorf("unexpected apply: %v", change.Apply)
			}
			if len(change.Services) == 0 {
				t.Error("the service is not tracked, so a broken unit would survive the batch")
			}
		}
	}
	if !found {
		t.Error("no bpftune change offered despite 10% retransmission under load")
	}

	// a quiet upload must not produce the same recommendation
	calm := bloatedRun()
	calm.Env.Bpftune.Installed = true
	calm.Env.Bpftune.Running = true
	calm.Load.Upload.SNMPDelta.OutSegs = 10570
	calm.Load.Upload.SNMPDelta.RetransPct = 0.3
	suite.Finalize(&calm, config.Default())
	for _, change := range Available(calm) {
		if change.ID == "bpftune-retrans" {
			t.Error("recommended disabling bpftune on a clean upload")
		}
	}
}

// Deriving the rate from a run that was already shaped measures the shaper
// rather than the line. Taking 85% of that on every apply walks the limit down
// until the link crawls, so a rate already in force has to be preserved.
func TestSQMDoesNotRatchetItsOwnRateDown(t *testing.T) {
	result := bloatedRun()
	// as measured *through* an existing 78/19 shaper
	result.Load.Download.Bps = 70e6
	result.Load.Upload.Bps = 17.5e6
	result.Env.SQM.Egress, result.Env.SQM.EgressMbit = "cake", 19
	result.Env.SQM.Ingress, result.Env.SQM.IngressMbit = "cake", 78
	result.Env.SQM.Persistent = false
	suite.Finalize(&result, config.Default())

	for _, change := range Available(result) {
		if change.ID != "sqm" {
			continue
		}
		script := strings.Join(change.Apply, "\n")
		if !strings.Contains(script, "78mbit") || !strings.Contains(script, "19mbit") {
			t.Errorf("rates were recomputed from a shaped run instead of kept:\n%s", script)
		}
		if strings.Contains(script, "59mbit") {
			t.Error("the download limit ratcheted down")
		}
	}
}
