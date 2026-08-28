package probe

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/unix"

	"github.com/WinTone01/nabiz/internal/util"
)

// Hop is one TTL step of the path.
type Hop struct {
	TTL     int       `json:"ttl"`
	IP      string    `json:"ip"`
	Name    string    `json:"name,omitempty"`
	RTTs    []float64 `json:"rtts"`
	Lost    int       `json:"lost"`
	IsDest  bool      `json:"is_dest"`
	ASNHint string    `json:"asn_hint,omitempty"`
}

// LossPct is the share of probes this hop did not answer. A router that
// deprioritises ICMP looks lossy while forwarding fine, so only loss that
// continues to the destination actually matters.
func (h Hop) LossPct() float64 {
	total := len(h.RTTs) + h.Lost
	if total == 0 {
		return 100
	}
	return float64(h.Lost) / float64(total) * 100
}

// Avg of the answered probes.
func (h Hop) Avg() float64 {
	if len(h.RTTs) == 0 {
		return 0
	}
	total := 0.0
	for _, rtt := range h.RTTs {
		total += rtt
	}
	return total / float64(len(h.RTTs))
}

// Best is the minimum RTT, the one least polluted by router queueing.
func (h Hop) Best() float64 {
	if len(h.RTTs) == 0 {
		return 0
	}
	best := h.RTTs[0]
	for _, rtt := range h.RTTs {
		if rtt < best {
			best = rtt
		}
	}
	return best
}

// Worst is the maximum RTT.
func (h Hop) Worst() float64 {
	worst := 0.0
	for _, rtt := range h.RTTs {
		if rtt > worst {
			worst = rtt
		}
	}
	return worst
}

// icmpSocket is a raw-ish handle on an unprivileged ICMP datagram socket. We
// drive it through x/sys/unix rather than net.PacketConn because traceroute
// needs the error queue (MSG_ERRQUEUE), which the high level API hides.
type icmpSocket struct {
	fd int
}

func newICMPSocket() (*icmpSocket, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_ICMP)
	if err != nil {
		return nil, err
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_RECVERR, 1); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &icmpSocket{fd: fd}, nil
}

func (s *icmpSocket) Close() { _ = unix.Close(s.fd) }

func (s *icmpSocket) setTTL(ttl int) error {
	return unix.SetsockoptInt(s.fd, unix.IPPROTO_IP, unix.IP_TTL, ttl)
}

func (s *icmpSocket) setDontFragment() error {
	return unix.SetsockoptInt(s.fd, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO)
}

func (s *icmpSocket) kernelMTU() int {
	value, err := unix.GetsockoptInt(s.fd, unix.IPPROTO_IP, unix.IP_MTU)
	if err != nil {
		return 0
	}
	return value
}

// send writes one echo request. The kernel rewrites the id and fills in the
// checksum for ping sockets, so we leave both zero.
func (s *icmpSocket) send(ip net.IP, seq, payload int) error {
	packet := make([]byte, 8+payload)
	packet[0] = 8 // echo request
	binary.BigEndian.PutUint16(packet[6:8], uint16(seq))
	copy(packet[8:], "nabiz")
	var addr unix.SockaddrInet4
	copy(addr.Addr[:], ip.To4())
	return unix.Sendto(s.fd, packet, 0, &addr)
}

// hopReply is what one probe produced: either a reply from the destination or
// an ICMP error from a router along the way.
type hopReply struct {
	from   string
	seq    int
	isDest bool
	got    bool
}

// wait polls for either a normal echo reply or a queued ICMP error.
func (s *icmpSocket) wait(deadline time.Time) hopReply {
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return hopReply{}
		}
		fds := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN | unix.POLLERR}}
		n, err := unix.Poll(fds, int(remaining.Milliseconds()))
		if err == unix.EINTR {
			continue
		}
		if err != nil || n == 0 {
			return hopReply{}
		}
		if fds[0].Revents&unix.POLLERR != 0 {
			if reply, ok := s.readErrQueue(); ok {
				return reply
			}
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			buf := make([]byte, 1500)
			n, from, err := unix.Recvfrom(s.fd, buf, unix.MSG_DONTWAIT)
			if err != nil || n < 8 {
				continue
			}
			ip := ""
			if sa, ok := from.(*unix.SockaddrInet4); ok {
				ip = net.IP(sa.Addr[:]).String()
			}
			return hopReply{from: ip, seq: int(binary.BigEndian.Uint16(buf[6:8])),
				isDest: true, got: true}
		}
	}
}

// readErrQueue pulls one ICMP error and extracts the offending router address
// from the SO_EE_OFFENDER trailer of the sock_extended_err control message.
func (s *icmpSocket) readErrQueue() (hopReply, bool) {
	buf := make([]byte, 1500)
	oob := make([]byte, 1024)
	n, oobn, _, _, err := unix.Recvmsg(s.fd, buf, oob, unix.MSG_ERRQUEUE|unix.MSG_DONTWAIT)
	if err != nil {
		return hopReply{}, false
	}
	seq := 0
	if n >= 8 {
		seq = int(binary.BigEndian.Uint16(buf[6:8]))
	}
	messages, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return hopReply{}, false
	}
	for _, message := range messages {
		if message.Header.Level != unix.IPPROTO_IP || message.Header.Type != unix.IP_RECVERR {
			continue
		}
		data := message.Data
		if len(data) < 16+8 {
			continue
		}
		offender := data[16:]
		family := binary.LittleEndian.Uint16(offender[0:2])
		if family != unix.AF_INET || len(offender) < 8 {
			continue
		}
		return hopReply{from: net.IP(offender[4:8]).String(), seq: seq, got: true}, true
	}
	return hopReply{}, false
}

// Traceroute walks the TTL, measuring per-hop loss and latency without root.
func Traceroute(ctx context.Context, dest string, maxHops, probes int,
	timeout time.Duration, resolveNames bool, onHop func(Hop),
) []Hop {
	addrs := util.ResolveIPv4(dest, 5*time.Second)
	if len(addrs) == 0 {
		return nil
	}
	target := net.ParseIP(addrs[0])
	socket, err := newICMPSocket()
	if err != nil {
		return nil
	}
	defer socket.Close()

	var hops []Hop
	seq := 2000
	for ttl := 1; ttl <= maxHops; ttl++ {
		select {
		case <-ctx.Done():
			return hops
		default:
		}
		if err := socket.setTTL(ttl); err != nil {
			break
		}
		hop := Hop{TTL: ttl}
		for i := 0; i < probes; i++ {
			seq = (seq + 1) & 0x7fff
			start := time.Now()
			if err := socket.send(target, seq, 32); err != nil {
				hop.Lost++
				continue
			}
			reply := socket.wait(start.Add(timeout))
			if !reply.got {
				hop.Lost++
				continue
			}
			hop.RTTs = append(hop.RTTs, float64(time.Since(start).Microseconds())/1000)
			if hop.IP == "" {
				hop.IP = reply.from
			}
			if reply.isDest {
				hop.IsDest = true
			}
		}
		if resolveNames && hop.IP != "" {
			if names, err := net.LookupAddr(hop.IP); err == nil && len(names) > 0 {
				hop.Name = names[0]
			}
		}
		hops = append(hops, hop)
		if onHop != nil {
			onHop(hop)
		}
		if hop.IsDest || hop.IP == target.String() {
			break
		}
	}
	return hops
}

// --- path MTU -----------------------------------------------------------

// MTUResult compares what the interface claims with what actually gets through.
type MTUResult struct {
	IfaceMTU   int    `json:"iface_mtu"`
	KernelPMTU int    `json:"kernel_pmtu"`
	ProbedMTU  int    `json:"probed_mtu"`
	Blackhole  bool   `json:"blackhole"`
	Detail     string `json:"detail"`
}

// IfaceMTU reads the MTU of an interface (default route's when empty).
func IfaceMTU(iface string) int {
	if iface == "" {
		iface, _ = util.DefaultRoute()
	}
	if iface == "" {
		return 0
	}
	return int(util.ReadInt("/sys/class/net/"+iface+"/mtu", 0))
}

// ProbePMTU binary-searches the largest DF echo that still comes back. A gap
// between the interface MTU and the measured one is a PMTU black hole - the
// classic "page starts loading then hangs", and something that fragmentation
// based bypass tricks can make worse.
func ProbePMTU(ctx context.Context, dest string, low, high int, timeout time.Duration) MTUResult {
	result := MTUResult{IfaceMTU: IfaceMTU("")}
	addrs := util.ResolveIPv4(dest, 5*time.Second)
	if len(addrs) == 0 {
		result.Detail = "hedef çözümlenemedi / target did not resolve"
		return result
	}
	target := net.ParseIP(addrs[0])
	socket, err := newICMPSocket()
	if err != nil {
		result.Detail = err.Error()
		return result
	}
	defer socket.Close()
	if err := socket.setDontFragment(); err != nil {
		result.Detail = err.Error()
		return result
	}

	seq := 5000
	reaches := func(mtu int) bool {
		payload := mtu - 28 // IPv4 header + ICMP echo header
		if payload < 0 {
			return false
		}
		seq = (seq + 1) & 0x7fff
		start := time.Now()
		if err := socket.send(target, seq, payload); err != nil {
			return false
		}
		reply := socket.wait(start.Add(timeout))
		return reply.got && reply.isDest
	}

	if reaches(high) {
		result.ProbedMTU = high
	} else {
		best := 0
		lo, hi := low, high
		for lo <= hi {
			select {
			case <-ctx.Done():
				lo = hi + 1
				continue
			default:
			}
			mid := (lo + hi) / 2
			if reaches(mid) {
				best = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		result.ProbedMTU = best
	}
	result.KernelPMTU = socket.kernelMTU()

	switch {
	case result.ProbedMTU == 0:
		result.Detail = "hiçbir boyut geri dönmedi / no size returned"
	case result.IfaceMTU > 0 && result.ProbedMTU < result.IfaceMTU:
		result.Blackhole = true
		result.Detail = fmt.Sprintf("yol MTU'su %d, arayüz %d - büyük paketler düşüyor",
			result.ProbedMTU, result.IfaceMTU)
	default:
		result.Detail = "sorun yok / clean"
	}
	return result
}
