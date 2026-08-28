package probe

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/util"
)

// StatKeys are the sysfs counters worth watching; the *_errors and *_dropped
// ones answer more "my internet is broken" tickets than any speed test.
var StatKeys = []string{
	"rx_bytes", "tx_bytes", "rx_packets", "tx_packets",
	"rx_errors", "tx_errors", "rx_dropped", "tx_dropped",
	"rx_crc_errors", "rx_frame_errors", "rx_missed_errors", "rx_over_errors",
	"rx_fifo_errors", "tx_fifo_errors", "tx_carrier_errors", "collisions",
}

// ErrorKeys is the subset that indicates a fault rather than volume.
var ErrorKeys = map[string]bool{
	"rx_errors": true, "tx_errors": true, "rx_dropped": true, "tx_dropped": true,
	"rx_crc_errors": true, "rx_frame_errors": true, "rx_missed_errors": true,
	"rx_over_errors": true, "rx_fifo_errors": true, "tx_fifo_errors": true,
	"tx_carrier_errors": true, "collisions": true,
}

// LinkInfo is everything the kernel already knows about the local interface.
type LinkInfo struct {
	Iface        string           `json:"iface"`
	Gateway      string           `json:"gateway"`
	Address      string           `json:"address"`
	MTU          int              `json:"mtu"`
	SpeedMbit    int              `json:"speed_mbit"`
	Duplex       string           `json:"duplex"`
	Carrier      bool             `json:"carrier"`
	CarrierUps   int64            `json:"carrier_up_count"`
	CarrierDowns int64            `json:"carrier_down_count"`
	Wireless     bool             `json:"wireless"`
	SSID         string           `json:"ssid,omitempty"`
	SignalDBm    float64          `json:"signal_dbm,omitempty"`
	TxBitrate    string           `json:"tx_bitrate,omitempty"`
	Qdisc        string           `json:"qdisc"`
	QdiscStats   QdiscStats       `json:"qdisc_stats"`
	Driver       string           `json:"driver,omitempty"`
	Advertised   string           `json:"advertised,omitempty"`
	Stats        map[string]int64 `json:"stats"`
}

// QdiscStats is where bufferbloat stops being a theory: a queue that reports
// drops and overlimits is actively shaping, one with a growing backlog is not.
type QdiscStats struct {
	Kind       string `json:"kind"`
	Bytes      int64  `json:"bytes"`
	Packets    int64  `json:"packets"`
	Drops      int64  `json:"drops"`
	Overlimits int64  `json:"overlimits"`
	Requeues   int64  `json:"requeues"`
	Backlog    int64  `json:"backlog"`
	QueueLen   int64  `json:"qlen"`
}

func sysnet(iface, name string) string {
	return strings.TrimSpace(util.ReadText(filepath.Join("/sys/class/net", iface, name), ""))
}

// ReadLink gathers the local interface picture. iface may be empty to use the
// interface carrying the default route.
func ReadLink(iface string) LinkInfo {
	detected, gateway := util.DefaultRoute()
	if iface == "" {
		iface = detected
	}
	info := LinkInfo{Iface: iface, Gateway: gateway, Stats: map[string]int64{}}
	if iface == "" {
		return info
	}
	target := gateway
	if target == "" {
		target = "1.1.1.1"
	}
	info.Address = util.LocalIPFor(target)
	info.MTU = int(util.ReadInt(filepath.Join("/sys/class/net", iface, "mtu"), 0))
	if speed, err := strconv.Atoi(sysnet(iface, "speed")); err == nil {
		info.SpeedMbit = speed
	}
	info.Duplex = sysnet(iface, "duplex")
	info.Carrier = sysnet(iface, "carrier") == "1"
	info.CarrierUps = util.ReadInt(filepath.Join("/sys/class/net", iface, "carrier_up_count"), -1)
	info.CarrierDowns = util.ReadInt(filepath.Join("/sys/class/net", iface, "carrier_down_count"), -1)
	if _, err := os.Stat(filepath.Join("/sys/class/net", iface, "wireless")); err == nil {
		info.Wireless = true
	}
	if _, err := os.Stat(filepath.Join("/sys/class/net", iface, "phy80211")); err == nil {
		info.Wireless = true
	}
	info.Stats = ReadCounters(iface)
	if info.Wireless {
		readWiFi(&info)
	}
	readQdisc(&info)
	readEthtool(&info)
	return info
}

// ReadCounters snapshots the sysfs statistics directory.
func ReadCounters(iface string) map[string]int64 {
	base := filepath.Join("/sys/class/net", iface, "statistics")
	out := make(map[string]int64, len(StatKeys))
	for _, key := range StatKeys {
		if value := util.ReadInt(filepath.Join(base, key), -1); value >= 0 {
			out[key] = value
		}
	}
	return out
}

// CounterDelta returns only the counters that actually moved.
func CounterDelta(before, after map[string]int64) map[string]int64 {
	out := map[string]int64{}
	for key, value := range after {
		if delta := value - before[key]; delta > 0 {
			out[key] = delta
		}
	}
	return out
}

var (
	signalRe     = regexp.MustCompile(`(-?\d+)\s*dBm`)
	qdiscNumRe   = regexp.MustCompile(`(\w+)\s+(\d+)`)
	ethtoolAdvRe = regexp.MustCompile(`(?s)Advertised link modes:(.*?)Advertised pause`)
)

func readWiFi(info *LinkInfo) {
	if util.Which("iw") == "" {
		return
	}
	out, ok := util.Run(4*time.Second, "iw", "dev", info.Iface, "link")
	if !ok {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SSID:"):
			info.SSID = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
		case strings.HasPrefix(line, "signal:"):
			if match := signalRe.FindStringSubmatch(line); match != nil {
				info.SignalDBm, _ = strconv.ParseFloat(match[1], 64)
			}
		case strings.HasPrefix(line, "tx bitrate:"):
			info.TxBitrate = strings.TrimSpace(strings.TrimPrefix(line, "tx bitrate:"))
		}
	}
}

func readQdisc(info *LinkInfo) {
	if util.Which("tc") == "" {
		return
	}
	out, ok := util.Run(4*time.Second, "tc", "-s", "qdisc", "show", "dev", info.Iface)
	if !ok || strings.TrimSpace(out) == "" {
		return
	}
	lines := strings.Split(out, "\n")
	fields := strings.Fields(lines[0])
	if len(fields) > 1 {
		info.Qdisc = fields[1]
		info.QdiscStats.Kind = fields[1]
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Sent ") {
			// Sent 123 bytes 45 pkt (dropped 0, overlimits 0 requeues 0)
			for _, pair := range qdiscNumRe.FindAllStringSubmatch(line, -1) {
				value, _ := strconv.ParseInt(pair[2], 10, 64)
				switch pair[1] {
				case "Sent":
					info.QdiscStats.Bytes = value
				case "dropped":
					info.QdiscStats.Drops = value
				case "overlimits":
					info.QdiscStats.Overlimits = value
				case "requeues":
					info.QdiscStats.Requeues = value
				}
			}
			if match := regexp.MustCompile(`(\d+) pkt`).FindStringSubmatch(line); match != nil {
				info.QdiscStats.Packets, _ = strconv.ParseInt(match[1], 10, 64)
			}
		}
		if strings.HasPrefix(line, "backlog") {
			if match := regexp.MustCompile(`backlog (\d+)b (\d+)p`).FindStringSubmatch(line); match != nil {
				info.QdiscStats.Backlog, _ = strconv.ParseInt(match[1], 10, 64)
				info.QdiscStats.QueueLen, _ = strconv.ParseInt(match[2], 10, 64)
			}
		}
	}
}

func readEthtool(info *LinkInfo) {
	if util.Which("ethtool") == "" {
		return
	}
	if out, ok := util.Run(4*time.Second, "ethtool", "-i", info.Iface); ok {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "driver:") {
				info.Driver = strings.TrimSpace(strings.TrimPrefix(line, "driver:"))
			}
		}
	}
	if out, ok := util.Run(4*time.Second, "ethtool", info.Iface); ok {
		if match := ethtoolAdvRe.FindStringSubmatch(out); match != nil {
			info.Advertised = strings.Join(strings.Fields(match[1]), " ")
		}
	}
}

// --- netfilter ----------------------------------------------------------

// NFQueue mirrors one row of /proc/net/netfilter/nfnetlink_queue.
type NFQueue struct {
	QNum         int   `json:"qnum"`
	PeerPID      int   `json:"peer_pid"`
	Queued       int64 `json:"queued"`
	CopyMode     int   `json:"copy_mode"`
	CopyRange    int64 `json:"copy_range"`
	QueueDropped int64 `json:"queue_dropped"`
	UserDropped  int64 `json:"user_dropped"`
	IDSequence   int64 `json:"id_sequence"`
}

// NFQueueInfo carries the reason when the file is unreadable rather than
// pretending the queues are empty.
type NFQueueInfo struct {
	Available bool      `json:"available"`
	Reason    string    `json:"reason,omitempty"`
	Queues    []NFQueue `json:"queues"`
}

// ReadNFQueues reports the NFQUEUE counters. Dropped packets here mean a
// userspace desync engine (zapret) cannot keep up with the traffic - a local
// fault that looks exactly like an ISP problem.
func ReadNFQueues() NFQueueInfo {
	const path = "/proc/net/netfilter/nfnetlink_queue"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NFQueueInfo{Reason: "no NFQUEUE subsystem"}
		}
		if os.IsPermission(err) {
			return NFQueueInfo{Reason: "root"}
		}
		return NFQueueInfo{Reason: err.Error()}
	}
	info := NFQueueInfo{Available: true}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		var queue NFQueue
		queue.QNum, _ = strconv.Atoi(fields[0])
		queue.PeerPID, _ = strconv.Atoi(fields[1])
		queue.Queued, _ = strconv.ParseInt(fields[2], 10, 64)
		queue.CopyMode, _ = strconv.Atoi(fields[3])
		queue.CopyRange, _ = strconv.ParseInt(fields[4], 10, 64)
		queue.QueueDropped, _ = strconv.ParseInt(fields[5], 10, 64)
		queue.UserDropped, _ = strconv.ParseInt(fields[6], 10, 64)
		queue.IDSequence, _ = strconv.ParseInt(fields[7], 10, 64)
		info.Queues = append(info.Queues, queue)
	}
	return info
}

// Conntrack reports table pressure; a full table drops new connections.
type Conntrack struct {
	Count int64 `json:"count"`
	Max   int64 `json:"max"`
}

// ReadConntrack reads the netfilter connection tracking counters.
func ReadConntrack() Conntrack {
	return Conntrack{
		Count: util.ReadInt("/proc/sys/net/netfilter/nf_conntrack_count", -1),
		Max:   util.ReadInt("/proc/sys/net/netfilter/nf_conntrack_max", -1),
	}
}

// SysctlKeys are the tunables that actually change behaviour on a home link.
var SysctlKeys = []string{
	"net.ipv4.tcp_congestion_control",
	"net.ipv4.tcp_available_congestion_control",
	"net.ipv4.tcp_allowed_congestion_control",
	"net.core.default_qdisc",
	"net.ipv4.tcp_mtu_probing",
	"net.core.rmem_max",
	"net.core.wmem_max",
	"net.ipv4.tcp_rmem",
	"net.ipv4.tcp_wmem",
	"net.ipv4.tcp_mem",
	"net.core.netdev_max_backlog",
	"net.ipv4.tcp_max_syn_backlog",
	"net.ipv4.tcp_slow_start_after_idle",
	"net.ipv4.tcp_ecn",
	"net.ipv4.tcp_fastopen",
	"net.ipv4.tcp_window_scaling",
	"net.ipv4.tcp_sack",
	"net.ipv4.tcp_timestamps",
	"net.ipv4.tcp_no_metrics_save",
	"net.ipv4.tcp_moderate_rcvbuf",
	"net.ipv4.ip_no_pmtu_disc",
	"net.ipv4.tcp_notsent_lowat",
}

// ReadSysctls snapshots the tunables above.
func ReadSysctls() map[string]string {
	out := make(map[string]string, len(SysctlKeys))
	for _, key := range SysctlKeys {
		if value := util.Sysctl(key); value != "" {
			out[key] = value
		}
	}
	return out
}
