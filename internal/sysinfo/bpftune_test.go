package sysinfo

import (
	"os"
	"testing"
)

func TestReadBpftuneLive(t *testing.T) {
	if os.Getenv("NABIZ_LIVE") != "1" {
		t.Skip("set NABIZ_LIVE=1")
	}
	state := ReadBpftune()
	t.Logf("installed=%v running=%v enabled=%v version=%s", state.Installed,
		state.Running, state.Enabled, state.Version)
	t.Logf("cc votes: %v", state.CCVotes)
	for _, change := range state.Changes {
		t.Logf("change %-42s %s -> %s   [%s]", change.Tunable, change.From, change.To, change.Reason)
	}
	for _, tunable := range state.Tunables {
		mark := " "
		if tunable.Modified {
			mark = "*"
		}
		t.Logf("%s %-42s %-26s (default %s)", mark, tunable.Key, tunable.Current, tunable.Default)
	}
	for _, note := range AssessBpftune(state, 100, 27, 5.5) {
		t.Logf("[%s] %s — %s", note.Level, note.Key, note.Text)
		if note.Hint != "" {
			t.Logf("      %s", note.Hint)
		}
	}
}
