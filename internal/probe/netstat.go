package probe

import (
	"os"
	"strconv"
	"strings"
)

// SNMP holds the kernel's own protocol counters, flattened to "Tcp.RetransSegs"
// style keys. These are cumulative since boot; the deltas are what matter.
type SNMP map[string]int64

// ReadSNMP parses /proc/net/snmp and /proc/net/netstat together.
func ReadSNMP() SNMP {
	out := SNMP{}
	for _, path := range []string{"/proc/net/snmp", "/proc/net/netstat"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i := 0; i+1 < len(lines); i += 2 {
			headerPrefix, headerRest, ok := strings.Cut(lines[i], ":")
			if !ok {
				continue
			}
			valuePrefix, valueRest, ok := strings.Cut(lines[i+1], ":")
			if !ok || headerPrefix != valuePrefix {
				continue
			}
			names := strings.Fields(headerRest)
			values := strings.Fields(valueRest)
			for j := range names {
				if j >= len(values) {
					break
				}
				number, err := strconv.ParseInt(values[j], 10, 64)
				if err != nil {
					continue
				}
				out[headerPrefix+"."+names[j]] = number
			}
		}
	}
	return out
}

// Delta returns after-before for every key that grew.
func (s SNMP) Delta(before SNMP) SNMP {
	out := SNMP{}
	for key, value := range s {
		if diff := value - before[key]; diff > 0 {
			out[key] = diff
		}
	}
	return out
}

// TCPHealth is the human-meaningful summary of the TCP counters.
type TCPHealth struct {
	OutSegs        int64   `json:"out_segs"`
	InSegs         int64   `json:"in_segs"`
	RetransSegs    int64   `json:"retrans_segs"`
	RetransPct     float64 `json:"retrans_pct"`
	Timeouts       int64   `json:"timeouts"`
	LostRetransmit int64   `json:"lost_retransmit"`
	SynRetrans     int64   `json:"syn_retrans"`
	OFOQueue       int64   `json:"ofo_queue"`
	PruneCalled    int64   `json:"prune_called"`
	RcvPruned      int64   `json:"rcv_pruned"`
	ListenDrops    int64   `json:"listen_drops"`
	InErrs         int64   `json:"in_errs"`
	CsumErrors     int64   `json:"csum_errors"`
	AttemptFails   int64   `json:"attempt_fails"`
	EstabResets    int64   `json:"estab_resets"`
	CurrEstab      int64   `json:"curr_estab"`
	SpuriousRTOs   int64   `json:"spurious_rtos"`
	SackRecovery   int64   `json:"sack_recovery"`
}

// Health folds the raw counters into TCPHealth.
func (s SNMP) Health() TCPHealth {
	health := TCPHealth{
		OutSegs:        s["Tcp.OutSegs"],
		InSegs:         s["Tcp.InSegs"],
		RetransSegs:    s["Tcp.RetransSegs"],
		InErrs:         s["Tcp.InErrs"],
		AttemptFails:   s["Tcp.AttemptFails"],
		EstabResets:    s["Tcp.EstabResets"],
		CurrEstab:      s["Tcp.CurrEstab"],
		CsumErrors:     s["Tcp.InCsumErrors"],
		Timeouts:       s["TcpExt.TCPTimeouts"],
		LostRetransmit: s["TcpExt.TCPLostRetransmit"],
		SynRetrans:     s["TcpExt.TCPSynRetrans"],
		OFOQueue:       s["TcpExt.TCPOFOQueue"],
		PruneCalled:    s["TcpExt.PruneCalled"],
		RcvPruned:      s["TcpExt.RcvPruned"],
		ListenDrops:    s["TcpExt.ListenDrops"],
		SpuriousRTOs:   s["TcpExt.TCPSpuriousRTOs"],
		SackRecovery:   s["TcpExt.TCPSackRecovery"],
	}
	if health.OutSegs > 0 {
		health.RetransPct = float64(health.RetransSegs) / float64(health.OutSegs) * 100
	}
	return health
}
