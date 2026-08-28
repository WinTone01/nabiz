package suite

import (
	"context"
	"os"
	"testing"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
)

func TestLiveBothLanguages(t *testing.T) {
	if os.Getenv("NABIZ_LIVE") != "1" {
		t.Skip("set NABIZ_LIVE=1")
	}
	cfg := config.Load()
	cfg.QuickProbes = 10
	result := Run(context.Background(), cfg, "quick", nil)
	for _, lang := range []i18n.Lang{i18n.EN, i18n.TR} {
		i18n.Set(lang)
		Refresh(&result, cfg)
		t.Logf("=== %s === score %.1f (%s), %d findings, %d advice",
			lang, result.Score, result.Grade, len(result.Findings), len(result.Advice))
		for i, finding := range result.Findings {
			if i >= 4 {
				break
			}
			t.Logf("  [%-4s] %-22s %s", finding.Level, finding.Key, finding.Title)
		}
		for i, advice := range result.Advice {
			if i >= 3 {
				break
			}
			t.Logf("  P%d [%s] %s", advice.Priority, CategoryLabel(advice.Category), advice.Title)
		}
	}
	if missing := i18n.MissingTranslations(); len(missing) > 0 {
		t.Logf("keys without Turkish text: %d", len(missing))
	}
}
