package probe

import (
	"context"
	"net"
	"strings"
	"time"
)

// Half-working IPv6 is worse than no IPv6: the resolver hands back an AAAA
// record, the application tries it first, waits for a timeout and only then
// falls back to IPv4. That wait is invisible in a speed test and lands on every
// new connection. So we check three things separately - do we have an address,
// does anything answer, and does DNS hand out AAAA records - because the
// interesting failure is the combination.

// IPv6Status is the result of the IPv6 reachability check.
type IPv6Status struct {
	HasAddress bool    `json:"has_address"`
	GlobalAddr string  `json:"global_addr,omitempty"`
	PingOK     bool    `json:"ping_ok"`
	PingMs     float64 `json:"ping_ms,omitempty"`
	TCPOK      bool    `json:"tcp_ok"`
	TCPMs      float64 `json:"tcp_ms,omitempty"`
	DNSHasAAAA bool    `json:"dns_has_aaaa"`
	Err        string  `json:"err,omitempty"`
}

// Working reports full IPv6 connectivity.
func (s IPv6Status) Working() bool { return s.HasAddress && (s.PingOK || s.TCPOK) }

// Broken is the expensive case: configured but dead.
func (s IPv6Status) Broken() bool { return s.HasAddress && !s.PingOK && !s.TCPOK }

// CheckIPv6 looks for a global address, then tries to reach the internet over it.
func CheckIPv6(ctx context.Context, timeout time.Duration) IPv6Status {
	var status IPv6Status
	interfaces, err := net.Interfaces()
	if err != nil {
		status.Err = err.Error()
		return status
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() != nil {
				continue
			}
			// link-local and unique-local do not reach the internet
			if ipnet.IP.IsLinkLocalUnicast() || ipnet.IP.IsPrivate() {
				continue
			}
			status.HasAddress = true
			status.GlobalAddr = ipnet.IP.String()
			break
		}
		if status.HasAddress {
			break
		}
	}

	// TCP is the honest test: ICMPv6 is filtered in more places than TCP/443.
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	if conn, err := dialer.DialContext(ctx, "tcp6", "[2606:4700:4700::1111]:443"); err == nil {
		_ = conn.Close()
		status.TCPOK = true
		status.TCPMs = float64(time.Since(start).Microseconds()) / 1000
	} else if status.Err == "" {
		status.Err = shortErr(err)
	}

	if pinger, err := NewPinger(timeout); err == nil {
		defer pinger.Close()
		// the v6 socket is opened lazily; a failure here is not fatal
		results := pinger.Sweep(ctx, []string{"2606:4700:4700::1111"}, 3,
			200*time.Millisecond, nil)
		for _, samples := range results {
			for _, sample := range samples {
				if !sample.Lost {
					status.PingOK = true
					status.PingMs = sample.RTTms
					break
				}
			}
		}
	}

	if addrs, err := net.DefaultResolver.LookupIP(ctx, "ip6", "cloudflare.com"); err == nil &&
		len(addrs) > 0 {
		status.DNSHasAAAA = true
	}
	return status
}

// FirewallICMP counts ICMP messages the local firewall dropped.
//
// Blocking ICMP type 3 code 4 (fragmentation needed) is the classic way to
// break path MTU discovery: connections open, then stall on the first large
// packet. The symptom looks exactly like an ISP fault, so it is worth naming.
type FirewallICMP struct {
	Available    bool `json:"available"`
	BlockedTotal int  `json:"blocked_total"`
	BlockedFrag  int  `json:"blocked_frag_needed"`
	BlockedTTL   int  `json:"blocked_time_exceeded"`
}

// CheckFirewallICMP scans the kernel log for firewall drops of ICMP.
func CheckFirewallICMP() FirewallICMP {
	var out FirewallICMP
	text, ok := KernelLog()
	if !ok {
		return out
	}
	out.Available = true
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "PROTO=ICMP") {
			continue
		}
		if !strings.Contains(line, "BLOCK") && !strings.Contains(line, "DROP") {
			continue
		}
		out.BlockedTotal++
		switch {
		case strings.Contains(line, "TYPE=3"):
			out.BlockedFrag++
		case strings.Contains(line, "TYPE=11"):
			out.BlockedTTL++
		}
	}
	return out
}
