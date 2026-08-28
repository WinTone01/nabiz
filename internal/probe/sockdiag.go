package probe

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Per-socket TCP state straight from the kernel via netlink INET_DIAG.
//
// This is the deepest look available without root: for every TCP connection we
// get the same struct tcp_info that `ss -ti` prints, which means we can see the
// congestion window, the retransmission count, the smoothed RTT and - the
// interesting one on a bpftune box - which congestion control algorithm the
// kernel actually chose for that individual socket.

const (
	sockDiagByFamily = 20
	inetDiagInfo     = 2
	inetDiagCong     = 4
	inetDiagBBRInfo  = 16
	tcpEstablished   = 1
)

// SocketInfo is one live TCP connection with its kernel-side metrics.
type SocketInfo struct {
	Local        string  `json:"local"`
	Remote       string  `json:"remote"`
	State        string  `json:"state"`
	CC           string  `json:"cc"`
	RTTms        float64 `json:"rtt_ms"`
	RTTVarms     float64 `json:"rttvar_ms"`
	MinRTTms     float64 `json:"min_rtt_ms"`
	CWnd         uint32  `json:"cwnd"`
	SndMSS       uint32  `json:"snd_mss"`
	SndSSThresh  uint32  `json:"snd_ssthresh"`
	Retrans      uint32  `json:"retrans"`
	TotalRetrans uint32  `json:"total_retrans"`
	Lost         uint32  `json:"lost"`
	Unacked      uint32  `json:"unacked"`
	BytesSent    uint64  `json:"bytes_sent"`
	BytesAcked   uint64  `json:"bytes_acked"`
	BytesRecv    uint64  `json:"bytes_received"`
	BytesRetrans uint64  `json:"bytes_retrans"`
	DeliveryRate uint64  `json:"delivery_rate"` // bytes/s
	SegsOut      uint32  `json:"segs_out"`
	SegsIn       uint32  `json:"segs_in"`
	NotSent      uint32  `json:"notsent_bytes"`
	RcvOOOPack   uint32  `json:"rcv_ooo_pack"`
	PMTU         uint32  `json:"pmtu"`
	SndWnd       uint32  `json:"snd_wnd"`
	RcvSpace     uint32  `json:"rcv_space"`
	RqQueue      uint32  `json:"rx_queue"`
	WqQueue      uint32  `json:"tx_queue"`
}

// RetransPct is the share of this connection's bytes that had to be resent.
func (s SocketInfo) RetransPct() float64 {
	if s.BytesSent == 0 {
		return 0
	}
	return float64(s.BytesRetrans) / float64(s.BytesSent) * 100
}

// DeliveryMbps is the kernel's own estimate of achieved throughput.
func (s SocketInfo) DeliveryMbps() float64 {
	return float64(s.DeliveryRate) * 8 / 1e6
}

var stateNames = map[uint8]string{
	1: "ESTAB", 2: "SYN-SENT", 3: "SYN-RECV", 4: "FIN-WAIT1", 5: "FIN-WAIT2",
	6: "TIME-WAIT", 7: "CLOSE", 8: "CLOSE-WAIT", 9: "LAST-ACK", 10: "LISTEN",
	11: "CLOSING",
}

// TCPSockets lists established IPv4 TCP connections with full tcp_info.
// Passing a non-empty remoteFilter keeps only sockets whose peer address
// contains that string, which is how the load test isolates its own streams.
func TCPSockets(remoteFilter string) ([]SocketInfo, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM, unix.NETLINK_INET_DIAG)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	if err := unix.SetNonblock(fd, false); err != nil {
		return nil, err
	}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO,
		&unix.Timeval{Sec: 2})

	request := buildDiagRequest()
	addr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK}
	if err := unix.Sendto(fd, request, 0, addr); err != nil {
		return nil, err
	}

	var out []SocketInfo
	buf := make([]byte, 1<<20)
done:
	for {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			break
		}
		for _, message := range parseNetlink(buf[:n]) {
			switch message.kind {
			case unix.NLMSG_DONE:
				break done
			case unix.NLMSG_ERROR:
				return out, fmt.Errorf("netlink error")
			}
			info, ok := parseDiagMessage(message.body)
			if !ok {
				continue
			}
			if remoteFilter != "" && !strings.Contains(info.Remote, remoteFilter) {
				continue
			}
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BytesRecv > out[j].BytesRecv })
	return out, nil
}

// netlinkMsg is one message peeled off a netlink datagram. x/sys/unix does not
// export a parser in every release, and the header is 16 fixed bytes, so we do
// it here rather than pulling in the legacy syscall package.
type netlinkMsg struct {
	kind uint16
	body []byte
}

func parseNetlink(buf []byte) []netlinkMsg {
	var out []netlinkMsg
	for len(buf) >= unix.SizeofNlMsghdr {
		length := int(binary.LittleEndian.Uint32(buf[0:4]))
		kind := binary.LittleEndian.Uint16(buf[4:6])
		if length < unix.SizeofNlMsghdr || length > len(buf) {
			break
		}
		out = append(out, netlinkMsg{kind: kind, body: buf[unix.SizeofNlMsghdr:length]})
		aligned := (length + 3) &^ 3
		if aligned > len(buf) {
			break
		}
		buf = buf[aligned:]
	}
	return out
}

func buildDiagRequest() []byte {
	const reqLen = unix.SizeofNlMsghdr + 56 // inet_diag_req_v2 is 56 bytes
	buf := make([]byte, reqLen)
	header := (*unix.NlMsghdr)(unsafe.Pointer(&buf[0]))
	header.Len = uint32(reqLen)
	header.Type = sockDiagByFamily
	header.Flags = unix.NLM_F_REQUEST | unix.NLM_F_DUMP
	header.Seq = 1
	header.Pid = uint32(os.Getpid())

	body := buf[unix.SizeofNlMsghdr:]
	body[0] = unix.AF_INET     // sdiag_family
	body[1] = unix.IPPROTO_TCP // sdiag_protocol
	// idiag_ext: ask for TCP_INFO and the congestion control name
	body[2] = 1<<(inetDiagInfo-1) | 1<<(inetDiagCong-1)
	body[3] = 0
	binary.LittleEndian.PutUint32(body[4:], 1<<tcpEstablished) // idiag_states
	return buf
}

func parseDiagMessage(data []byte) (SocketInfo, bool) {
	// struct inet_diag_msg: 4 bytes + sockid(48) + 5*u32 = 72 bytes
	if len(data) < 72 {
		return SocketInfo{}, false
	}
	var info SocketInfo
	info.State = stateNames[data[1]]
	sport := binary.BigEndian.Uint16(data[4:6])
	dport := binary.BigEndian.Uint16(data[6:8])
	src := net.IP(data[8:12]).String()
	dst := net.IP(data[24:28]).String()
	info.Local = net.JoinHostPort(src, fmt.Sprint(sport))
	info.Remote = net.JoinHostPort(dst, fmt.Sprint(dport))
	info.RqQueue = binary.LittleEndian.Uint32(data[56:60])
	info.WqQueue = binary.LittleEndian.Uint32(data[60:64])

	// attributes follow, rtattr-aligned
	offset := 72
	for offset+4 <= len(data) {
		length := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		kind := binary.LittleEndian.Uint16(data[offset+2 : offset+4])
		if length < 4 || offset+length > len(data) {
			break
		}
		payload := data[offset+4 : offset+length]
		switch kind {
		case inetDiagCong:
			info.CC = strings.TrimRight(string(payload), "\x00")
		case inetDiagInfo:
			applyTCPInfo(&info, payload)
		}
		offset += (length + 3) &^ 3
	}
	return info, true
}

// applyTCPInfo decodes the stable prefix of struct tcp_info. Every field is
// length-guarded because the struct grows with each kernel release.
func applyTCPInfo(info *SocketInfo, data []byte) {
	u32 := func(offset int) uint32 {
		if offset+4 > len(data) {
			return 0
		}
		return binary.LittleEndian.Uint32(data[offset : offset+4])
	}
	u64 := func(offset int) uint64 {
		if offset+8 > len(data) {
			return 0
		}
		return binary.LittleEndian.Uint64(data[offset : offset+8])
	}
	info.SndMSS = u32(16)
	info.Unacked = u32(24)
	info.Lost = u32(32)
	info.Retrans = u32(36)
	info.PMTU = u32(60)
	info.RTTms = float64(u32(68)) / 1000
	info.RTTVarms = float64(u32(72)) / 1000
	info.SndSSThresh = u32(76)
	info.CWnd = u32(80)
	info.RcvSpace = u32(96)
	info.TotalRetrans = u32(100)
	info.BytesAcked = u64(120)
	info.BytesRecv = u64(128)
	info.SegsOut = u32(136)
	info.SegsIn = u32(140)
	info.NotSent = u32(144)
	info.MinRTTms = float64(u32(148)) / 1000
	info.DeliveryRate = u64(160)
	info.BytesSent = u64(200)
	info.BytesRetrans = u64(208)
	info.RcvOOOPack = u32(224)
	info.SndWnd = u32(228)
}

// SocketSummary aggregates a set of sockets - what the load screen shows.
type SocketSummary struct {
	Count         int      `json:"count"`
	CCAlgorithms  []string `json:"cc_algorithms"`
	AvgRTTms      float64  `json:"avg_rtt_ms"`
	MinRTTms      float64  `json:"min_rtt_ms"`
	AvgCWnd       float64  `json:"avg_cwnd"`
	TotalRetrans  uint32   `json:"total_retrans"`
	RetransPct    float64  `json:"retrans_pct"`
	DeliveryMbps  float64  `json:"delivery_mbps"`
	BytesRecv     uint64   `json:"bytes_received"`
	InflightBytes uint64   `json:"inflight_bytes"`
	WindowLimited bool     `json:"window_limited"`
}

// Summarize folds a socket list into the aggregate the UI shows.
func Summarize(sockets []SocketInfo) SocketSummary {
	out := SocketSummary{Count: len(sockets)}
	if len(sockets) == 0 {
		return out
	}
	seen := map[string]bool{}
	var rttTotal, cwndTotal float64
	var sent, retrans uint64
	minRTT := 1e9
	for _, socket := range sockets {
		if socket.CC != "" && !seen[socket.CC] {
			seen[socket.CC] = true
			out.CCAlgorithms = append(out.CCAlgorithms, socket.CC)
		}
		rttTotal += socket.RTTms
		cwndTotal += float64(socket.CWnd)
		out.TotalRetrans += socket.TotalRetrans
		out.BytesRecv += socket.BytesRecv
		out.DeliveryMbps += socket.DeliveryMbps()
		out.InflightBytes += uint64(socket.Unacked) * uint64(socket.SndMSS)
		sent += socket.BytesSent
		retrans += socket.BytesRetrans
		if socket.MinRTTms > 0 && socket.MinRTTms < minRTT {
			minRTT = socket.MinRTTms
		}
	}
	sort.Strings(out.CCAlgorithms)
	out.AvgRTTms = rttTotal / float64(len(sockets))
	out.AvgCWnd = cwndTotal / float64(len(sockets))
	if minRTT < 1e9 {
		out.MinRTTms = minRTT
	}
	if sent > 0 {
		out.RetransPct = float64(retrans) / float64(sent) * 100
	}
	return out
}

// SampleSockets polls repeatedly while a transfer runs and keeps the busiest
// snapshot, so the numbers describe the peak of the load rather than its tail.
func SampleSockets(stop <-chan struct{}, filter string, every time.Duration) SocketSummary {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	var best SocketSummary
	for {
		select {
		case <-stop:
			return best
		case <-ticker.C:
			sockets, err := TCPSockets(filter)
			if err != nil {
				continue
			}
			summary := Summarize(sockets)
			if summary.BytesRecv > best.BytesRecv {
				best = summary
			}
		}
	}
}
