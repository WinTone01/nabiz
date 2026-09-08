package suite

import (
	"testing"
	"time"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/probe"
)

func findingKeys(result Result) map[string]bool {
	out := map[string]bool{}
	for _, finding := range result.Findings {
		out[finding.Key] = true
	}
	return out
}

// "can't disable ASPM" reports a refused request, not an enabled feature.
// Firmware that never turned ASPM on refuses the same call, and then there is
// nothing to fix - acting on the log line alone sends people to edit a kernel
// command line for no reason.
func TestASPMFindingNeedsTheRegisterNotTheLogLine(t *testing.T) {
	cfg := config.Default()
	base := Env{
		Link:    probe.LinkInfo{Iface: "eth0", SpeedMbit: 1000, Duplex: "full", MTU: 1500, CarrierUps: 20},
		LinkLog: probe.LinkHistory{Available: true, Drops: 15},
	}

	refusedButOff := Result{Env: base}
	refusedButOff.Env.ASPM = probe.ASPMStatus{
		Checked: true, Blocked: true, Driver: "r8169", Slot: "0000:03:00.0",
		StateKnown: true, Enabled: false, LnkCtl: "Disabled",
	}
	Finalize(&refusedButOff, cfg)
	if findingKeys(refusedButOff)["aspm-blocked"] {
		t.Error("raised an ASPM fault while the register says ASPM is disabled")
	}
	if adviceIDs(refusedButOff)["aspm-force"] {
		t.Error("recommended pcie_aspm=force with nothing to disable")
	}

	unreadable := Result{Env: base}
	unreadable.Env.ASPM = probe.ASPMStatus{
		Checked: true, Blocked: true, Driver: "r8169", Slot: "0000:03:00.0",
	}
	Finalize(&unreadable, cfg)
	if !findingKeys(unreadable)["aspm-unverified"] {
		t.Error("stayed silent instead of saying the state could not be read")
	}

	reallyOn := Result{Env: base}
	reallyOn.Env.ASPM = probe.ASPMStatus{
		Checked: true, Blocked: true, Driver: "r8169", Slot: "0000:03:00.0",
		StateKnown: true, Enabled: true, LnkCtl: "L1 Enabled",
	}
	Finalize(&reallyOn, cfg)
	if !findingKeys(reallyOn)["aspm-blocked"] {
		t.Error("missed a card genuinely running with ASPM on")
	}
	if !adviceIDs(reallyOn)["aspm-force"] {
		t.Error("no advice for a card genuinely running with ASPM on")
	}
}

// The journal advice exists to make the kernel comparison possible at all, so
// it must appear exactly when the log is not being kept.
func TestJournalAdviceTracksStorage(t *testing.T) {
	cfg := config.Default()
	base := Env{
		Link:    probe.LinkInfo{Iface: "eth0", SpeedMbit: 1000, Duplex: "full", MTU: 1500, CarrierUps: 20},
		LinkLog: probe.LinkHistory{Available: true, Drops: 15},
	}

	volatile := Result{Env: base}
	volatile.Env.Journal = probe.JournalStorage{Persistent: false, Boots: 1}
	Finalize(&volatile, cfg)
	if !adviceIDs(volatile)["journal-persist"] {
		t.Error("no advice to keep the log while it is being thrown away")
	}

	kept := Result{Env: base}
	kept.Env.Journal = probe.JournalStorage{Persistent: true, Boots: 9}
	Finalize(&kept, cfg)
	if adviceIDs(kept)["journal-persist"] {
		t.Error("still advising a change that is already in place")
	}
}

// Every judgement built on drops that have stopped has to stand down together.
// Leaving one live means recommending a kernel downgrade, or a new cable, on a
// link that is currently behaving - which undoes whatever settled it.
func TestSettledLinkSilencesEveryDropDerivedClaim(t *testing.T) {
	cfg := config.Default()
	now := time.Now()
	history := probe.LinkHistory{
		Available: true, Drops: 15, MeanGapMin: 7,
		WindowFrom: now.Add(-3 * time.Hour), WindowTo: now,
		Events: []probe.LinkEvent{
			{At: now.Add(-3 * time.Hour), Kind: "down"},
			{At: now.Add(-2 * time.Hour), Kind: "up"},
		},
		Downshifts: 14, DownshiftNote: "1Gbps → 100Mbps",
	}
	result := Result{Env: Env{
		Link: probe.LinkInfo{Iface: "enp3s0", Gateway: "192.168.0.1", Carrier: true,
			SpeedMbit: 100, Duplex: "full", CarrierUps: 15},
		LinkLog:     history,
		GoodKernel:  "7.2.0",
		KernRegData: &probe.KernelRegressionData{GoodKernel: "7.2.0", GoodHours: 45, Running: "7.2.3", CurrentRate: 16},
		LinkRegData: &probe.Regression{},
	}}
	Finalize(&result, cfg)

	if !history.Settled() {
		t.Fatal("test setup: the link should count as settled")
	}
	for _, finding := range result.Findings {
		if finding.Level == "bad" {
			t.Errorf("still a red finding on a settled link: %s", finding.Key)
		}
	}
	for _, advice := range result.Advice {
		switch advice.ID {
		case "kernel-downgrade", "cable-flap", "modem-port", "pin-100full":
			t.Errorf("still recommending %q on a settled link", advice.ID)
		}
	}
}
