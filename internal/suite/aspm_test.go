package suite

import (
	"testing"

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
