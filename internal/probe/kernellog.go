package probe

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/WinTone01/nabiz/internal/util"
)

// The current boot's kernel log is wanted by several probes at once - link
// events, the ASPM message, the IPv6 checks - and each used to shell out for its
// own copy. That was free while the journal lived in memory and died at
// shutdown. Persistent storage changes the arithmetic: journald will grow to a
// tenth of the filesystem, a long uptime keeps every line of it, and on a
// machine that logs firewall drops to the kernel ring buffer most of those lines
// are noise nobody here reads. So the text is fetched once and shared.

var (
	kernelLogMu   sync.Mutex
	kernelLogText string
	kernelLogAt   time.Time
	kernelLogOK   bool
)

// kernelLogTTL is long enough that one run shares a single read, short enough
// that a monitor looping every few seconds still sees new events.
const kernelLogTTL = 3 * time.Second

// KernelLog returns this boot's kernel log, reusing a recent read.
func KernelLog() (string, bool) {
	kernelLogMu.Lock()
	defer kernelLogMu.Unlock()
	if kernelLogOK && time.Since(kernelLogAt) < kernelLogTTL {
		return kernelLogText, true
	}
	text, ok := util.Run(10*time.Second, "journalctl", "-k", "-b", "--no-pager",
		"--output=short-iso", "-n", "20000")
	kernelLogText, kernelLogOK, kernelLogAt = text, ok, time.Now()
	return text, ok
}

// ResetKernelLog drops the cached copy. Tests and long-running monitors that
// want a guaranteed-fresh read call this first.
func ResetKernelLog() {
	kernelLogMu.Lock()
	kernelLogOK = false
	kernelLogMu.Unlock()
}

// bootWindow is one entry of `journalctl --list-boots -o json`.
type bootWindow struct {
	Index      int   `json:"index"`
	FirstEntry int64 `json:"first_entry"` // microseconds since the epoch
	LastEntry  int64 `json:"last_entry"`
}

// bootWindows asks journald directly how far each retained boot's log reaches.
//
// The alternative is to read every line of every boot and look at the first and
// last timestamps, which is how this used to work: six boots at up to 200000
// lines each, to learn two numbers per boot that journald already knows.
func bootWindows(maxBoots int) ([]bootWindow, bool) {
	out, ok := util.Run(8*time.Second, "journalctl", "--list-boots", "-o", "json", "--no-pager")
	if !ok {
		return nil, false // older systemd: the caller falls back to reading the log
	}
	var windows []bootWindow
	if err := json.Unmarshal([]byte(out), &windows); err != nil {
		return nil, false
	}
	kept := windows[:0]
	for _, window := range windows {
		if window.Index > -maxBoots && window.FirstEntry > 0 && window.LastEntry > 0 {
			kept = append(kept, window)
		}
	}
	return kept, true
}

func (b bootWindow) from() time.Time { return time.UnixMicro(b.FirstEntry) }
func (b bootWindow) to() time.Time   { return time.UnixMicro(b.LastEntry) }
