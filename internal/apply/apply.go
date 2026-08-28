// Package apply turns advice into changes that can be undone.
//
// The contract is deliberately paranoid, because the thing being changed is the
// network connection the user is reading this over:
//
//  1. Nothing is applied without a snapshot written to disk first.
//  2. Every change is expressed as a shell command, and its inverse is written
//     into a restore script the user can read and run without this tool.
//  3. After applying, connectivity is re-verified. If it broke, the restore
//     script runs automatically.
//  4. The whole batch runs under one privileged invocation, so there is one
//     password prompt and no half-applied state from a cancelled sudo.
package apply

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
	"github.com/WinTone01/nabiz/internal/util"
)

// Risk levels. Anything that can interrupt the link is "link"; those are never
// part of a default batch.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskLink   = "link"
)

// Change is one applicable recommendation.
type Change struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Risk    string   `json:"risk"`
	Files   []string `json:"files,omitempty"` // snapshotted before applying
	Apply   []string `json:"apply"`           // shell lines
	Restore []string `json:"restore"`         // inverse shell lines
}

// Snapshot is one applied batch, kept so it can be rolled back later.
type Snapshot struct {
	Dir       string    `json:"-"`
	Stamp     time.Time `json:"stamp"`
	Changes   []Change  `json:"changes"`
	Kernel    string    `json:"kernel"`
	Interface string    `json:"interface"`
}

// SnapshotsDir holds every applied batch.
func SnapshotsDir() string { return filepath.Join(config.DataDir(), "snapshots") }

// Available maps a finished run's advice onto changes this package can perform.
// Advice with no safe automation is simply absent - it is better to leave a
// recommendation to the user than to half-automate something irreversible.
func Available(result suite.Result) []Change {
	iface := result.Env.Link.Iface
	if iface == "" {
		iface, _ = util.DefaultRoute()
	}
	sysctls := result.Env.Sysctls
	byID := map[string]Change{}

	for _, advice := range result.Advice {
		switch advice.ID {
		case "bbr":
			byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskLow,
				"net.ipv4.tcp_congestion_control", "bbr", sysctls)

		case "ssaio":
			byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskLow,
				"net.ipv4.tcp_slow_start_after_idle", "0", sysctls)

		case "mtu-probe":
			byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskLow,
				"net.ipv4.tcp_mtu_probing", "1", sysctls)

		case "notsent-lowat":
			byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskLow,
				"net.ipv4.tcp_notsent_lowat", "131072", sysctls)

		case "default-qdisc":
			byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskLow,
				"net.core.default_qdisc", "fq_codel", sysctls)

		case "bpftune-buffers":
			if target := saneRmem(result); target != "" {
				byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskMedium,
					"net.ipv4.tcp_rmem", target, sysctls)
			}

		case "conntrack":
			current := result.Env.Conntrack.Max
			if current > 0 {
				byID[advice.ID] = sysctlChange(advice.ID, advice.Title, RiskLow,
					"net.netfilter.nf_conntrack_max", fmt.Sprint(current*2), sysctls)
			}

		case "hostlist-prune":
			path := "/etc/unwall/autohostlist.txt"
			byID[advice.ID] = Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Files:   []string{path},
				Apply:   []string{": > " + path, "systemctl restart unwall"},
				Restore: []string{"systemctl restart unwall"},
			}

		case "dns-leak":
			// a drop-in is used rather than editing resolved.conf, so undoing it
			// is deleting one file rather than reconstructing an edit
			const dropin = "/etc/systemd/resolved.conf.d/99-nabiz-no-fallback.conf"
			byID[advice.ID] = Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Apply: []string{
					"mkdir -p /etc/systemd/resolved.conf.d",
					"printf '[Resolve]\\nFallbackDNS=\\n' > " + dropin,
					"systemctl restart systemd-resolved",
				},
				Restore: []string{
					"rm -f " + dropin,
					"systemctl restart systemd-resolved",
				},
			}

		case "eee-off":
			byID[advice.ID] = Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskLink,
				Apply:   []string{"ethtool --set-eee " + iface + " eee off"},
				Restore: []string{"ethtool --set-eee " + iface + " eee on"},
			}

		case "nic-offload":
			byID[advice.ID] = Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskLink,
				Apply:   []string{"ethtool -K " + iface + " gro off gso off tso off"},
				Restore: []string{"ethtool -K " + iface + " gro on gso on tso on"},
			}

		case "gateway-off":
			byID[advice.ID] = Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Files: []string{"/etc/unwall/unwall.conf"},
				Apply: []string{
					"unwallctl config set GATEWAY_MODE=0",
					"systemctl restart unwall",
				},
				Restore: []string{
					"unwallctl config set GATEWAY_MODE=1",
					"systemctl restart unwall",
				},
			}
		}
	}

	extraChanges(result, iface, sysctls, byID)

	out := make([]Change, 0, len(byID))
	for _, change := range byID {
		if len(change.Apply) > 0 {
			out = append(out, change)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// sysctlChange builds a reversible sysctl edit, capturing the current value as
// the restore value rather than assuming the documented default.
func sysctlChange(id, title, risk, key, value string, current map[string]string) Change {
	previous := current[key]
	if previous == "" {
		previous = util.Sysctl(key)
	}
	restore := []string{}
	if previous != "" {
		restore = append(restore, fmt.Sprintf("sysctl -w %s=%q", key, previous))
	}
	return Change{
		ID: id, Title: title, Risk: risk,
		Apply:   []string{fmt.Sprintf("sysctl -w %s=%q", key, value)},
		Restore: restore,
	}
}

// saneRmem derives a receive-buffer ceiling of four times the measured BDP.
func saneRmem(result suite.Result) string {
	link := result.Env.Link
	internet := result.InternetLatency()
	if internet == nil || link.SpeedMbit <= 0 || internet.Avg <= 0 {
		return ""
	}
	bdp := float64(link.SpeedMbit) * 1e6 / 8 * (internet.Avg / 1000)
	if bdp <= 0 {
		return ""
	}
	fields := strings.Fields(result.Env.Sysctls["net.ipv4.tcp_rmem"])
	low, mid := "4096", "131072"
	if len(fields) == 3 {
		low, mid = fields[0], fields[1]
	}
	return fmt.Sprintf("%s %s %d", low, mid, int64(bdp*4))
}

// Filter selects changes at or below a risk ceiling.
func Filter(changes []Change, includeLink bool) []Change {
	out := changes[:0:0]
	for _, change := range changes {
		if change.Risk == RiskLink && !includeLink {
			continue
		}
		out = append(out, change)
	}
	return out
}

// Select picks changes by id.
func Select(changes []Change, ids []string) ([]Change, error) {
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var out []Change
	for _, change := range changes {
		if wanted[change.ID] {
			out = append(out, change)
			delete(wanted, change.ID)
		}
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for id := range wanted {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("%s: %s", i18n.T("apply.unknown"), strings.Join(missing, ", "))
	}
	return out, nil
}

// extraChanges covers the recommendations added after the first pass. They live
// here rather than in the switch above only to keep that function readable.
func extraChanges(result suite.Result, iface string, sysctls map[string]string,
	byID map[string]Change,
) {
	add := func(change Change) {
		if len(change.Apply) > 0 {
			byID[change.ID] = change
		}
	}
	for _, advice := range result.Advice {
		switch advice.ID {
		case "hostlist-false-positives":
			if len(result.DesyncNotNeeded) == 0 {
				continue
			}
			const manual = "/etc/unwall/hostlist.txt"
			const auto = "/etc/unwall/autohostlist.txt"
			var commands []string
			for _, domain := range result.DesyncNotNeeded {
				// anchored to a whole line so example.com never takes
				// notexample.com with it
				pattern := "^" + regexpEscape(domain) + "$"
				commands = append(commands,
					fmt.Sprintf("sed -i -E '/%s/d' %s", pattern, manual),
					fmt.Sprintf("sed -i -E '/%s/d' %s", pattern, auto))
			}
			commands = append(commands, "systemctl restart unwall")
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Files:   []string{manual, auto},
				Apply:   commands,
				Restore: []string{"systemctl restart unwall"},
			})

		case "hostlist-dead":
			dead := deadEntries(result)
			if len(dead) == 0 {
				continue
			}
			const manual = "/etc/unwall/hostlist.txt"
			const auto = "/etc/unwall/autohostlist.txt"
			var commands []string
			for _, domain := range dead {
				pattern := "^" + regexpEscape(domain) + "$"
				commands = append(commands,
					fmt.Sprintf("sed -i -E '/%s/d' %s", pattern, manual),
					fmt.Sprintf("sed -i -E '/%s/d' %s", pattern, auto))
			}
			commands = append(commands, "systemctl restart unwall")
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskLow,
				Files:   []string{manual, auto},
				Apply:   commands,
				Restore: []string{"systemctl restart unwall"},
			})

		case "hostlist-manual":
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Files:   []string{"/etc/unwall/unwall.conf"},
				Apply:   []string{"unwallctl config set HOSTLIST_MODE=manual", "systemctl restart unwall"},
				Restore: []string{"unwallctl config set HOSTLIST_MODE=auto", "systemctl restart unwall"},
			})

		case "quic":
			previous := result.Env.Unwall.PortsUDP
			if previous == "" {
				continue
			}
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Files: []string{"/etc/unwall/unwall.conf"},
				Apply: []string{"unwallctl config set PORTS_UDP=50000-50100",
					"systemctl restart unwall"},
				Restore: []string{"unwallctl config set PORTS_UDP=" + previous,
					"systemctl restart unwall"},
			})

		case "dns-encrypt":
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Apply:   []string{"unwallctl dns enable quad9 dnscrypt"},
				Restore: []string{"unwallctl dns disable"},
			})

		case "ipv6-broken":
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskLow,
				Apply:   []string{"sysctl -w net.ipv6.conf.all.disable_ipv6=1"},
				Restore: []string{"sysctl -w net.ipv6.conf.all.disable_ipv6=0"},
			})

		case "sqm":
			rate := shapeRate(result)
			if rate <= 0 || iface == "" {
				continue
			}
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Apply: []string{fmt.Sprintf(
					"tc qdisc replace dev %s root cake bandwidth %dmbit besteffort", iface, rate)},
				Restore: []string{fmt.Sprintf(
					"tc qdisc replace dev %s root %s", iface, restoreQdisc(sysctls))},
			})

		case "cake-gaming":
			rate := shapeRate(result)
			if rate <= 0 || iface == "" {
				continue
			}
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Apply: []string{fmt.Sprintf(
					"tc qdisc replace dev %s root cake bandwidth %dmbit diffserv4", iface, rate)},
				Restore: []string{fmt.Sprintf(
					"tc qdisc replace dev %s root %s", iface, restoreQdisc(sysctls))},
			})

		case "bpftune-cc":
			// dctcp is the outlier: without end-to-end ECN it behaves like reno,
			// so the fix is to stop offering it rather than to stop bpftune
			current := sysctls["net.ipv4.tcp_allowed_congestion_control"]
			if current == "" || !strings.Contains(current, "dctcp") {
				continue
			}
			var kept []string
			for _, name := range strings.Fields(current) {
				if name != "dctcp" {
					kept = append(kept, name)
				}
			}
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskLow,
				Apply: []string{fmt.Sprintf(
					"sysctl -w net.ipv4.tcp_allowed_congestion_control=%q",
					strings.Join(kept, " "))},
				Restore: []string{fmt.Sprintf(
					"sysctl -w net.ipv4.tcp_allowed_congestion_control=%q", current)},
			})

		// bpftune-vs-physical is deliberately absent. Its command rolls back
		// everything bpftune has done, which would undo the buffer ceiling and
		// the tuner override applied alongside it; it is advice about the order
		// to do physical work in, not a change to make.

		case "bpftune-tuner-off":
			binary := util.Which("bpftune")
			if binary == "" {
				continue
			}
			const dropin = "/etc/systemd/system/bpftune.service.d/99-nabiz-tuners.conf"
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Apply: []string{
					"mkdir -p /etc/systemd/system/bpftune.service.d",
					fmt.Sprintf("printf '[Service]\\nExecStart=\\nExecStart=%s -a tcp_conn_tuner\\n' > %s",
						binary, dropin),
					"systemctl daemon-reload",
					"systemctl restart bpftune",
				},
				Restore: []string{
					"rm -f " + dropin,
					"systemctl daemon-reload",
					"systemctl restart bpftune",
				},
			})

		case "dns-transparent":
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskMedium,
				Apply:   []string{"unwallctl dns enable quad9 dnscrypt"},
				Restore: []string{"unwallctl dns disable"},
			})

		case "pin-100full":
			if iface == "" {
				continue
			}
			add(Change{
				ID: advice.ID, Title: advice.Title, Risk: RiskLink,
				Apply:   []string{"ethtool -s " + iface + " autoneg on advertise 0x008"},
				Restore: []string{"ethtool -s " + iface + " autoneg on advertise 0x03f"},
			})
		}
	}
}

// deadEntries mirrors the advice engine's view of names that no longer resolve.
func deadEntries(result suite.Result) []string {
	var out []string
	for _, verdict := range result.DPI {
		if verdict.Verdict == "dns-fail" {
			out = append(out, strings.ToLower(verdict.Domain))
		}
	}
	return out
}

// shapeRate is the upload rate to shape at: a little under what was measured,
// because a shaper only controls the queue while it stays the bottleneck.
func shapeRate(result suite.Result) int {
	if result.Load == nil || result.Load.Upload == nil {
		return 0
	}
	rate := int(result.Load.Upload.Bps / 1e6 * 0.92)
	if rate < 1 {
		return 0
	}
	return rate
}

func restoreQdisc(sysctls map[string]string) string {
	if value := sysctls["net.core.default_qdisc"]; value != "" {
		return value
	}
	return "fq_codel"
}

// regexpEscape quotes the characters that matter inside a sed address.
func regexpEscape(value string) string {
	replacer := strings.NewReplacer(".", `\.`, "*", `\*`, "[", `\[`, "]", `\]`,
		"^", `\^`, "$", `\$`, "/", `\/`, "+", `\+`, "?", `\?`, "(", `\(`, ")", `\)`,
		"{", `\{`, "}", `\}`, "|", `\|`)
	return replacer.Replace(value)
}
