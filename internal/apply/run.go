package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/util"
)

// Result reports what happened, including whether the safety net fired.
type Result struct {
	Snapshot       *Snapshot
	Applied        []string
	Verified       bool
	RolledBack     bool
	BrokenServices []string
	Output         string
	Err            error
}

// Prepare writes the snapshot and both scripts without running anything, so a
// caller can show the user exactly what is about to happen.
func Prepare(changes []Change) (*Snapshot, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("apply.nothing"))
	}
	iface, _ := util.DefaultRoute()
	snapshot := &Snapshot{
		Stamp:     time.Now(),
		Changes:   changes,
		Kernel:    strings.TrimSpace(util.ReadText("/proc/sys/kernel/osrelease", "")),
		Interface: iface,
	}
	snapshot.Dir = filepath.Join(SnapshotsDir(), snapshot.Stamp.Format("20060102-150405"))
	if err := os.MkdirAll(filepath.Join(snapshot.Dir, "files"), 0o755); err != nil {
		return nil, err
	}

	// copy every file a change will touch, before anything runs
	for _, change := range changes {
		for _, path := range change.Files {
			data, err := os.ReadFile(path)
			if err != nil {
				continue // a missing file is restored by deleting it again
			}
			target := filepath.Join(snapshot.Dir, "files", sanitise(path))
			if err := os.WriteFile(target, data, 0o600); err != nil {
				return nil, err
			}
		}
	}
	if err := writeScript(filepath.Join(snapshot.Dir, "apply.sh"),
		applyLines(changes)); err != nil {
		return nil, err
	}
	if err := writeScript(filepath.Join(snapshot.Dir, "restore.sh"),
		restoreLines(snapshot)); err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(snapshot.Dir, "manifest.json"),
		manifest, 0o644); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func sanitise(path string) string {
	return strings.ReplaceAll(strings.TrimPrefix(path, "/"), "/", "_")
}

func applyLines(changes []Change) []string {
	lines := []string{"# nabiz apply", "set -u"}
	for _, change := range changes {
		lines = append(lines, "", "# "+change.ID+": "+change.Title)
		for _, command := range change.Apply {
			// a failing step must not abort the rest, or the batch ends
			// half-applied with no clear state to restore from
			lines = append(lines, command+" || echo \"nabiz: failed: "+change.ID+"\" >&2")
		}
	}
	return lines
}

func restoreLines(snapshot *Snapshot) []string {
	lines := []string{
		"# nabiz restore — undoes the batch applied at " +
			snapshot.Stamp.Format("2006-01-02 15:04:05"),
		"# safe to run by hand at any time, with or without nabiz installed",
		"set -u",
	}
	// restore files first: a service restart in a Restore step should see them
	for _, change := range snapshot.Changes {
		for _, path := range change.Files {
			source := filepath.Join(snapshot.Dir, "files", sanitise(path))
			if _, err := os.Stat(source); err != nil {
				continue
			}
			lines = append(lines, fmt.Sprintf("cp -a %q %q", source, path))
		}
	}
	// reverse order, so changes undo in the opposite sequence they applied
	for index := len(snapshot.Changes) - 1; index >= 0; index-- {
		change := snapshot.Changes[index]
		if len(change.Restore) == 0 {
			continue
		}
		lines = append(lines, "", "# undo "+change.ID)
		for _, command := range change.Restore {
			lines = append(lines, command+" || true")
		}
	}
	return lines
}

func writeScript(path string, lines []string) error {
	content := "#!/usr/bin/env bash\n" + strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o755)
}

// Apply runs the batch under one privileged invocation, then verifies that the
// connection still works. If it does not, the restore script runs immediately.
func Apply(ctx context.Context, snapshot *Snapshot, verify bool) Result {
	result := Result{Snapshot: snapshot}
	before := serviceStates(snapshot)
	out, err := runPrivileged(ctx, filepath.Join(snapshot.Dir, "apply.sh"))
	result.Output = out
	if err != nil {
		result.Err = err
		return result
	}
	for _, change := range snapshot.Changes {
		result.Applied = append(result.Applied, change.ID)
	}
	// mark the snapshot as actually applied. A dry run leaves the scripts on
	// disk without this marker, and rolling back something that was never
	// applied is a confusing no-op at best.
	_ = os.WriteFile(filepath.Join(snapshot.Dir, "applied"),
		[]byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
	if !verify {
		result.Verified = true
		return result
	}

	// give the stack a moment to settle before judging it
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
	}
	if Connectivity(ctx, 25*time.Second) {
		// The network is only half the promise. A change that leaves a service
		// it restarted in a failed state passed the connectivity check happily
		// once, and the broken unit stayed broken until someone noticed by eye.
		if broken := brokenServices(before); len(broken) == 0 {
			result.Verified = true
			return result
		} else {
			result.BrokenServices = broken
		}
	}
	rollbackOut, rollbackErr := runPrivileged(context.Background(),
		filepath.Join(snapshot.Dir, "restore.sh"))
	result.RolledBack = true
	result.Output += "\n" + rollbackOut
	if rollbackErr != nil {
		result.Err = rollbackErr
	}
	return result
}

// Rollback restores a snapshot by directory name, or the newest one.
func Rollback(ctx context.Context, dir string) (string, error) {
	if dir == "" {
		list, err := List()
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "", fmt.Errorf("%s", i18n.T("apply.nosnapshots"))
		}
		dir = list[len(list)-1].Dir
	}
	script := filepath.Join(dir, "restore.sh")
	if _, err := os.Stat(script); err != nil {
		return "", err
	}
	out, err := runPrivileged(ctx, script)
	if err == nil {
		// mark it undone, so nothing later reads this batch as still in force
		_ = os.WriteFile(filepath.Join(dir, "rolledback"),
			[]byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
	}
	return out, err
}

// List returns every stored snapshot, oldest first.
func List() ([]Snapshot, error) {
	entries, err := os.ReadDir(SnapshotsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Snapshot
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(SnapshotsDir(), entry.Name())
		data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			continue
		}
		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "applied")); err != nil {
			continue // prepared but never applied
		}
		snapshot.Dir = dir
		out = append(out, snapshot)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Stamp.Before(out[j].Stamp) })
	return out, nil
}

// Connectivity is the safety check: DNS resolves, a packet comes back, and TLS
// completes. All three must pass, because any one of them can survive a change
// that broke the other two.
func Connectivity(ctx context.Context, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}
		if connectivityOnce(ctx) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
	}
	return false
}

func connectivityOnce(ctx context.Context) bool {
	pinger, err := probe.NewPinger(1500 * time.Millisecond)
	if err == nil {
		defer pinger.Close()
		results := pinger.Sweep(ctx, []string{"1.1.1.1"}, 3, 200*time.Millisecond, nil)
		ok := false
		for _, samples := range results {
			for _, sample := range samples {
				if !sample.Lost {
					ok = true
				}
			}
		}
		if !ok {
			return false
		}
	}
	addrs := util.ResolveIPv4("cloudflare.com", 4*time.Second)
	if len(addrs) == 0 {
		return false
	}
	tls := probe.TLSHandshake(ctx, addrs[0], "cloudflare.com", 443, 5*time.Second, "none", false)
	return tls.OK
}

// runPrivileged executes a script with exactly one escalation prompt.
func runPrivileged(ctx context.Context, script string) (string, error) {
	var command *exec.Cmd
	switch {
	case util.IsRoot():
		command = exec.CommandContext(ctx, "bash", script)
	case util.Which("pkexec") != "":
		command = exec.CommandContext(ctx, "pkexec", "bash", script)
	case util.Which("sudo") != "":
		command = exec.CommandContext(ctx, "sudo", "bash", script)
	default:
		return "", fmt.Errorf("%s", i18n.T("ui.needs_root"))
	}
	command.Stdin = os.Stdin
	out, err := command.CombinedOutput()
	return string(out), err
}

// serviceStates records which units are running before a batch, so afterwards
// there is something to compare against. Units are taken both from what a
// change declares and from any `systemctl restart` it runs, because the second
// is easy to add and easy to forget to declare.
func serviceStates(snapshot *Snapshot) map[string]bool {
	states := map[string]bool{}
	for _, name := range snapshotServices(snapshot) {
		states[name] = unitActive(name)
	}
	return states
}

func snapshotServices(snapshot *Snapshot) []string {
	seen := map[string]bool{}
	var out []string
	note := func(name string) {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".service")
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, change := range snapshot.Changes {
		for _, name := range change.Services {
			note(name)
		}
		for _, command := range change.Apply {
			fields := strings.Fields(command)
			for index := 0; index+2 < len(fields); index++ {
				if fields[index] != "systemctl" {
					continue
				}
				switch fields[index+1] {
				case "restart", "start", "reload", "try-restart", "reload-or-restart":
					note(fields[index+2])
				}
			}
		}
	}
	return out
}

func unitActive(name string) bool {
	out, _ := util.Run(5*time.Second, "systemctl", "is-active", name)
	return strings.TrimSpace(out) == "active"
}

func unitFailed(name string) bool {
	out, _ := util.Run(5*time.Second, "systemctl", "is-failed", name)
	return strings.TrimSpace(out) == "failed"
}

// brokenServices lists units that were running before the batch and are not now.
func brokenServices(before map[string]bool) []string {
	var out []string
	for name, wasActive := range before {
		if !wasActive {
			continue
		}
		if unitFailed(name) || !unitActive(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
