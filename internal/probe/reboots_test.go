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
