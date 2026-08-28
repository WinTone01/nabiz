package suite

import (
	"context"
	"os"
	"testing"

	"github.com/WinTone01/nabiz/internal/config"
)

func TestLiveQuickSuite(t *testing.T) {
	if os.Getenv("NABIZ_LIVE") != "1" {
		t.Skip("set NABIZ_LIVE=1")
	}
	cfg := config.Load()
	cfg.QuickProbes = 20
	result := Run(context.Background(), cfg, "quick", nil)
	t.Logf("score %.1f (%s) in %.1fs", result.Score, result.Grade, result.Duration)
	for _, finding := range result.Findings {
		t.Logf("[%-4s] %-24s %s", finding.Level, finding.Key, finding.Title)
	}
	t.Logf("--- advice (%d) ---", len(result.Advice))
	for _, advice := range result.Advice {
		t.Logf("P%d [%s] %s", advice.Priority, advice.Category, advice.Title)
	}
	if len(result.Latency) == 0 {
		t.Error("no latency measured")
	}
}
