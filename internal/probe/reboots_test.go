package probe

import (
	"testing"
	"time"
)

// The bug this guards against: with a volatile journal only the running kernel
// has any log, so every other kernel scored zero drops out of zero observed
// minutes and was recommended as the quiet one to go back to. On this machine
// that named the kernel the flapping had started on.
func TestUnwatchedKernelIsNotAQuietKernel(t *testing.T) {
	stats := []KernelStability{
		{Kernel: "7.2.3", Boots: 1, Hours: 1, Measured: true, CoveredHrs: 1, DropsPerHr: 16},
		{Kernel: "7.2.0", Boots: 13, Hours: 45}, // ran for days, never logged
	}
	if _, regressed := KernelRegressionOf(stats, "7.2.3"); regressed {
		t.Error("called an unlogged kernel quiet")
	}
	if _, _, regressed := KernelRegression(stats, "7.2.3"); regressed {
		t.Error("called an unlogged kernel quiet")
	}

	// the same comparison is valid once that kernel has actually been watched
	stats[1].Measured = true
	stats[1].CoveredHrs = 6
	data, regressed := KernelRegressionOf(stats, "7.2.3")
	if !regressed || data.GoodKernel != "7.2.0" {
		t.Errorf("expected a regression against a watched quiet kernel, got %+v", data)
	}
}

// A window too short to have caught anything is not evidence either.
func TestBrieflyWatchedKernelIsNotAQuietKernel(t *testing.T) {
	stats := []KernelStability{
		{Kernel: "7.2.3", Measured: true, CoveredHrs: 1, DropsPerHr: 16, Hours: 1},
		{Kernel: "6.18.48", Measured: true, CoveredHrs: 0.1, DropsPerHr: 0, Hours: 12},
	}
	if _, regressed := KernelRegressionOf(stats, "7.2.3"); regressed {
		t.Error("six minutes of silence was read as a clean kernel")
	}
}

func TestBootLinkStatsRate(t *testing.T) {
	now := time.Now()
	boot := BootLinkStats{From: now.Add(-2 * time.Hour), To: now, Drops: 30}
	if rate := boot.DropsPerHour(); rate < 14.9 || rate > 15.1 {
		t.Errorf("DropsPerHour = %.2f, want 15", rate)
	}
}

// The failure this guards against: one boot the journal never kept used to shift
// every older entry by one, filing each boot's drops under the kernel that ran
// the boot before it. The table's whole purpose is to say which kernel drops the
// link, so a silent off-by-one there is worse than having no table.
func TestBootsAreMatchedByTimeNotPosition(t *testing.T) {
	now := time.Now()
	hour := time.Hour
	// newest first, as both sources report
	reboots := []Reboot{
		{Kernel: "7.2.3", At: now.Add(-2 * hour), Uptime: 2 * hour, Current: true},
		{Kernel: "6.18.48", At: now.Add(-12 * hour), Uptime: 8 * hour}, // journal missing
		{Kernel: "7.2.3", At: now.Add(-30 * hour), Uptime: 10 * hour},
	}
	boots := []BootLinkStats{
		{Index: 0, From: now.Add(-2 * hour), To: now, Drops: 20},
		{Index: -1, From: now.Add(-30 * hour), To: now.Add(-20 * hour), Drops: 18},
	}
	stats := StabilityByKernel(reboots, boots)

	byKernel := map[string]KernelStability{}
	for _, entry := range stats {
		byKernel[entry.Kernel] = entry
	}
	if got := byKernel["7.2.3"].Drops; got != 38 {
		t.Errorf("7.2.3 got %d drops, want both of its boots (38)", got)
	}
	if lts := byKernel["6.18.48"]; lts.Measured || lts.Drops != 0 {
		t.Errorf("an unlogged kernel was credited with someone else's boot: %+v", lts)
	}
}

// A journal window that ends before its session started belongs to neither.
func TestBootOutsideEverySessionIsIgnored(t *testing.T) {
	now := time.Now()
	reboots := []Reboot{{Kernel: "7.2.3", At: now.Add(-time.Hour), Uptime: time.Hour, Current: true}}
	boots := []BootLinkStats{{From: now.Add(-100 * time.Hour), To: now.Add(-99 * time.Hour), Drops: 5}}
	stats := StabilityByKernel(reboots, boots)
	if len(stats) != 1 || stats[0].Measured {
		t.Errorf("matched a boot that predates every session: %+v", stats)
	}
}
