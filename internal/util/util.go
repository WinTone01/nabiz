// Package util holds the small shared helpers every probe needs: timing,
// reading /proc and /sys, shelling out safely and formatting numbers.
package util

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ReadText returns the file contents, or def when the file cannot be read.
func ReadText(path, def string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	return string(data)
}

// ReadInt reads a single integer out of a sysfs/procfs file.
func ReadInt(path string, def int64) int64 {
	raw := strings.TrimSpace(ReadText(path, ""))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return value
}

// Sysctl reads a kernel tunable through /proc/sys, whitespace normalised.
func Sysctl(key string) string {
	path := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	return strings.Join(strings.Fields(ReadText(path, "")), " ")
}

// Outcome says why a command produced nothing, which is the difference between
// "this machine does not have the feature" and "we were not allowed to look".
// Collapsing the two lets a probe report a comfortable zero for something it
// never managed to read - the failure mode this tool exists to avoid.
type Outcome int

const (
	OK       Outcome = iota
	NotFound         // the binary is not installed
	Denied           // ran, refused: permission, capability, netlink EPERM
	TimedOut         // still running when the budget ran out
	Failed           // ran and exited non-zero for some other reason
)

// String names the outcome for a finding that has to explain itself.
func (o Outcome) String() string {
	switch o {
	case OK:
		return "ok"
	case NotFound:
		return "not-installed"
	case Denied:
		return "permission-denied"
	case TimedOut:
		return "timed-out"
	}
	return "failed"
}

// Readable reports whether the command actually answered.
func (o Outcome) Readable() bool { return o == OK }

// Run executes a command with a timeout and never returns an error for a
// non-zero exit; probes care about the output, not the status.
func Run(timeout time.Duration, name string, args ...string) (stdout string, ok bool) {
	out, outcome := RunDetail(timeout, name, args...)
	return out, outcome == OK
}

// RunDetail is Run with the reason for a failure kept. Prefer it wherever the
// absence of an answer would otherwise be reported as a measurement.
func RunDetail(timeout time.Duration, name string, args ...string) (string, Outcome) {
	if _, err := exec.LookPath(name); err != nil {
		return "", NotFound
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	switch {
	case err == nil:
		return string(out), OK
	case ctx.Err() != nil:
		return string(out), TimedOut
	case deniedBy(errBuf.String(), err):
		return string(out), Denied
	}
	return string(out), Failed
}

// deniedBy recognises a refusal. Tools that talk to the kernel report it in
// their own words - ethtool prints a netlink EPERM, ip and tc say "Operation
// not permitted" - so the message has to be read as well as the exit status.
func deniedBy(stderr string, err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	lower := strings.ToLower(stderr)
	for _, phrase := range []string{
		"operation not permitted", "permission denied", "not permitted",
		"must be root", "are you root", "eperm", "access denied",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// Which reports whether a binary exists in PATH.
func Which(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// DefaultRoute parses /proc/net/route for the IPv4 default gateway.
func DefaultRoute() (iface, gateway string) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return "", ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan() // header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[1] != "00000000" {
			continue
		}
		flags, _ := strconv.ParseUint(fields[3], 16, 32)
		if flags&0x2 == 0 { // RTF_GATEWAY
			continue
		}
		raw, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			continue
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(raw))
		return fields[0], net.IP(buf).String()
	}
	return "", ""
}

// LocalIPFor asks the kernel which source address it would use for target,
// without sending anything.
func LocalIPFor(target string) string {
	conn, err := net.Dial("udp", net.JoinHostPort(target, "53"))
	if err != nil {
		return ""
	}
	defer conn.Close()
	host, _, _ := net.SplitHostPort(conn.LocalAddr().String())
	return host
}

// ResolveIPv4 resolves through the system resolver and returns dotted quads.
func ResolveIPv4(host string, timeout time.Duration) []string {
	if net.ParseIP(host) != nil {
		return []string{host}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}

// IsRoot reports whether we can read the root-only counters.
func IsRoot() bool { return os.Geteuid() == 0 }

// Uniq preserves order while removing duplicates.
func Uniq(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := items[:0:0]
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// Truncate shortens with an ellipsis, counting runes not bytes.
func Truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

// HumanRate formats bits per second.
func HumanRate(bps float64) string {
	units := []string{"bps", "Kbps", "Mbps", "Gbps"}
	index := 0
	for bps >= 1000 && index < len(units)-1 {
		bps /= 1000
		index++
	}
	return strconv.FormatFloat(bps, 'f', 2, 64) + " " + units[index]
}

// HumanBytes formats a byte count.
func HumanBytes(value float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[index]
}

// Clamp constrains a float to [low, high].
func Clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// ShortDuration renders a duration compactly and language-neutrally: 45s, 12m,
// 3h31m, 1d12h. Unit letters are deliberately the SI-ish English ones so the
// string means the same thing in every interface language.
func ShortDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	switch {
	case seconds < 60:
		return strconv.Itoa(seconds) + "s"
	case seconds < 3600:
		minutes, rest := seconds/60, seconds%60
		if rest == 0 {
			return strconv.Itoa(minutes) + "m"
		}
		return strconv.Itoa(minutes) + "m" + pad2(rest) + "s"
	case seconds < 86400:
		hours, minutes := seconds/3600, (seconds%3600)/60
		if minutes == 0 {
			return strconv.Itoa(hours) + "h"
		}
		return strconv.Itoa(hours) + "h" + pad2(minutes) + "m"
	default:
		days, hours := seconds/86400, (seconds%86400)/3600
		if hours == 0 {
			return strconv.Itoa(days) + "d"
		}
		return strconv.Itoa(days) + "d" + strconv.Itoa(hours) + "h"
	}
}

func pad2(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
