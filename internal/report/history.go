package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/suite"
)

// A baseline is one saved run that later runs are compared against. It answers
// the question a single measurement never can: "is this better or worse than
// before I changed something?"

// BaselinePath is where the pinned reference run lives.
func BaselinePath() string { return filepath.Join(config.DataDir(), "baseline.json") }

// SaveBaseline pins a run as the reference.
func SaveBaseline(result suite.Result) error {
	return SaveJSON(result, BaselinePath())
}

// LoadBaseline reads the pinned reference, if any.
func LoadBaseline() (*suite.Result, error) {
	result, err := LoadJSON(BaselinePath())
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ClearBaseline removes the pinned reference.
func ClearBaseline() error {
	err := os.Remove(BaselinePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// RunSummary is the cheap header of a saved run, for trend views that must not
// deserialise every full result.
type RunSummary struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"started_at"`
	Duration  float64   `json:"duration"`
	Score     float64   `json:"score"`
	Grade     string    `json:"grade"`
	Bad       int       `json:"bad"`
	Warn      int       `json:"warn"`
}

// History lists saved runs oldest-first so a trend line reads left to right.
func History(limit int) []RunSummary {
	paths := ListRuns(limit)
	out := make([]RunSummary, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var partial struct {
			Name      string    `json:"name"`
			StartedAt time.Time `json:"started_at"`
			Duration  float64   `json:"duration"`
			Score     float64   `json:"score"`
			Grade     string    `json:"grade"`
			Findings  []struct {
				Level string `json:"level"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(data, &partial); err != nil {
			continue
		}
		summary := RunSummary{
			Path: path, Name: partial.Name, StartedAt: partial.StartedAt,
			Duration: partial.Duration, Score: partial.Score, Grade: partial.Grade,
		}
		for _, finding := range partial.Findings {
			switch finding.Level {
			case "bad":
				summary.Bad++
			case "warn":
				summary.Warn++
			}
		}
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

// ScoreSeries extracts the score trend for a sparkline.
func ScoreSeries(history []RunSummary) []float64 {
	out := make([]float64, 0, len(history))
	for _, entry := range history {
		out = append(out, entry.Score)
	}
	return out
}
