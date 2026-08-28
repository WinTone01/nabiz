package probe

import (
	"net"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/util"
)

// Where a resolver comes from decides both whether it is a problem and how to
// remove it. "systemd-resolved lists a plaintext address" is not actionable;
// "enp3s0 is still using the two addresses its DHCP lease handed out" is.
type ResolverPath struct {
	Link    string   `json:"link"` // interface name, or "" for the global scope
	Servers []string `json:"servers"`
}

// Global reports whether this is the system-wide scope rather than a link.
func (r ResolverPath) Global() bool { return r.Link == "" }

// UpstreamDetail parses resolvectl into per-scope resolver lists.
func UpstreamDetail() []ResolverPath {
	out, ok := util.Run(6*time.Second, "resolvectl", "status")
	if !ok || strings.TrimSpace(out) == "" {
		var fallback []ResolverPath
		if servers := SystemResolvers(); len(servers) > 0 {
			fallback = append(fallback, ResolverPath{Servers: servers})
		}
		return fallback
	}
	var paths []ResolverPath
	current := ResolverPath{}
	flush := func() {
		if len(current.Servers) > 0 {
			current.Servers = util.Uniq(current.Servers)
			paths = append(paths, current)
		}
		current = ResolverPath{}
	}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Link "):
			flush()
			// "Link 2 (enp3s0)" -> enp3s0
			if open := strings.Index(trimmed, "("); open >= 0 {
				if close := strings.Index(trimmed[open:], ")"); close > 0 {
					current.Link = trimmed[open+1 : open+close]
				}
			}
		case strings.HasPrefix(trimmed, "Global"):
			flush()
		case strings.HasPrefix(trimmed, "DNS Servers:"),
			strings.HasPrefix(trimmed, "Current DNS Server:"),
			strings.HasPrefix(trimmed, "Fallback DNS Servers:"):
			_, list, _ := strings.Cut(trimmed, ":")
			for _, token := range strings.Fields(list) {
				if server := normaliseServer(token); server != "" {
					current.Servers = append(current.Servers, server)
				}
			}
		case trimmed == "":
			// resolvectl wraps long server lists onto continuation lines, so a
			// blank line rather than an unindented one ends a scope
		}
	}
	flush()
	return paths
}

// normaliseServer strips the "#hostname" suffix and the port, keeping the address.
func normaliseServer(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if hash := strings.Index(token, "#"); hash >= 0 {
		token = token[:hash]
	}
	if host, _, err := net.SplitHostPort(token); err == nil {
		token = host
	}
	if net.ParseIP(token) == nil {
		return ""
	}
	return token
}

// LocalResolver reports whether an address is a resolver that lives on this
// machine or on a private tunnel of its own.
//
// The loopback stub and a MagicDNS address are not leaks: the first is the
// encrypted proxy itself, and the second answers only for a tailnet over an
// encrypted link. Flagging them trains people to ignore the finding that
// matters, which is the ISP's resolver arriving over DHCP.
func LocalResolver(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	// Tailscale MagicDNS: 100.100.100.100 out of the CGNAT range, and fd7a:115c:a1e0::53
	if ip.Equal(net.ParseIP("100.100.100.100")) {
		return true
	}
	if tailscaleV6 := net.ParseIP("fd7a:115c:a1e0::"); tailscaleV6 != nil {
		mask := net.CIDRMask(48, 128)
		if ip.To4() == nil && ip.Mask(mask).Equal(tailscaleV6.Mask(mask)) {
			return true
		}
	}
	return false
}

// PlaintextLeaks returns the scopes still offering a resolver that is neither
// local nor encrypted, which is what defeats encrypted DNS in practice.
func PlaintextLeaks(paths []ResolverPath) []ResolverPath {
	var out []ResolverPath
	for _, path := range paths {
		var leaking []string
		for _, server := range path.Servers {
			if !LocalResolver(server) {
				leaking = append(leaking, server)
			}
		}
		if len(leaking) > 0 {
			out = append(out, ResolverPath{Link: path.Link, Servers: leaking})
		}
	}
	return out
}

// Offloads is the state of the segmentation offloads a NIC can do in hardware.
type Offloads struct {
	Known bool `json:"known"`
	GRO   bool `json:"gro"`
	GSO   bool `json:"gso"`
	TSO   bool `json:"tso"`
}

// AnyOn reports whether there is anything left to turn off.
func (o Offloads) AnyOn() bool { return o.Known && (o.GRO || o.GSO || o.TSO) }

// ReadOffloads asks ethtool what is currently enabled.
//
// Without this, advice to disable offloads is judged against the kernel's
// cumulative retransmission counter, which never falls: the recommendation
// would survive being applied, for the rest of the uptime.
func ReadOffloads(iface string) Offloads {
	var out Offloads
	if iface == "" || util.Which("ethtool") == "" {
		return out
	}
	text, ok := util.Run(4*time.Second, "ethtool", "-k", iface)
	if !ok {
		return out
	}
	for _, line := range strings.Split(text, "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		on := strings.HasPrefix(strings.TrimSpace(value), "on")
		switch strings.TrimSpace(name) {
		case "generic-receive-offload":
			out.Known, out.GRO = true, on
		case "generic-segmentation-offload":
			out.Known, out.GSO = true, on
		case "tcp-segmentation-offload":
			out.Known, out.TSO = true, on
		}
	}
	return out
}
