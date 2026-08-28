// Package probe holds the individual network measurements. Every probe is
// cancellable through a context and returns plain data - no logging, no exits.
package probe

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/WinTone01/nabiz/internal/stats"
)

// Pinger sends ICMP echoes over an unprivileged datagram socket.
//
// Linux hands SOCK_DGRAM/IPPROTO_ICMP to any uid inside
// net.ipv4.ping_group_range (systemd opens it to everyone by default), so we
// get per-packet timing without root and without shelling out to /bin/ping.
// The kernel rewrites the echo id, but the sequence number survives, which is
// all we need to match replies to probes.
type Pinger struct {
	conn    *icmp.PacketConn
	v4      *ipv4.PacketConn
	timeout time.Duration

	mu      sync.Mutex
	pending map[int]*inflight
	closed  bool
}

type inflight struct {
	target string
	index  int
	sentAt time.Time
	done   chan stats.Sample
}

// StreamSample is one live measurement handed to the dashboard or monitor.
type StreamSample struct {
	Target string
	Sample stats.Sample
	At     time.Time
}

// NewPinger opens the socket. The error tells the caller ICMP is unavailable
// (an unusual sysctl or a container without the capability).
func NewPinger(timeout time.Duration) (*Pinger, error) {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	pinger := &Pinger{
		conn:    conn,
		v4:      conn.IPv4PacketConn(),
		timeout: timeout,
		pending: make(map[int]*inflight),
	}
	go pinger.readLoop()
	return pinger, nil
}

// Available reports whether unprivileged ICMP works on this machine.
func Available() bool {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Close releases the socket; in-flight probes resolve as lost.
func (p *Pinger) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	_ = p.conn.Close()
}

// SetTTL limits the hop count - used by the traceroute probe.
func (p *Pinger) SetTTL(ttl int) error {
	if p.v4 == nil {
		return errors.New("no IPv4 socket")
	}
	return p.v4.SetTTL(ttl)
}

func (p *Pinger) readLoop() {
	buf := make([]byte, 1500)
	for {
		n, _, err := p.conn.ReadFrom(buf)
		if err != nil {
			p.mu.Lock()
			closed := p.closed
			p.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		msg, err := icmp.ParseMessage(1, buf[:n]) // 1 = IPv4 ICMP
		if err != nil {
			continue
		}
		echo, ok := msg.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		now := time.Now()
		p.mu.Lock()
		entry := p.pending[echo.Seq]
		if entry != nil {
			delete(p.pending, echo.Seq)
		}
		p.mu.Unlock()
		if entry == nil {
			continue
		}
		entry.done <- stats.Sample{RTTms: float64(now.Sub(entry.sentAt).Microseconds()) / 1000}
	}
}

var seqCounter struct {
	sync.Mutex
	value int
}

func nextSeq() int {
	seqCounter.Lock()
	defer seqCounter.Unlock()
	seqCounter.value = (seqCounter.value + 1) & 0x7fff
	if seqCounter.value == 0 {
		seqCounter.value = 1
	}
	return seqCounter.value
}

// send fires one echo and returns a channel that yields exactly one sample.
func (p *Pinger) send(target string, index, payloadSize int) (<-chan stats.Sample, error) {
	ip := net.ParseIP(target)
	if ip == nil {
		return nil, errors.New("not an IP: " + target)
	}
	seq := nextSeq()
	entry := &inflight{target: target, index: index, sentAt: time.Now(),
		done: make(chan stats.Sample, 1)}
	body := make([]byte, payloadSize)
	copy(body, "nabiz")
	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: 0, Seq: seq, Data: body},
	}
	wire, err := message.Marshal(nil)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.pending[seq] = entry
	p.mu.Unlock()
	if _, err := p.conn.WriteTo(wire, &net.UDPAddr{IP: ip}); err != nil {
		p.mu.Lock()
		delete(p.pending, seq)
		p.mu.Unlock()
		return nil, err
	}
	timeout := p.timeout
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		<-timer.C
		p.mu.Lock()
		still := p.pending[seq]
		if still == entry {
			delete(p.pending, seq)
		}
		p.mu.Unlock()
		if still == entry {
			entry.done <- stats.Sample{Lost: true}
		}
	}()
	return entry.done, nil
}

// Sweep pings every target count times at a fixed interval and returns the
// full series per target. onSample, when non-nil, fires as results arrive.
func (p *Pinger) Sweep(ctx context.Context, targets []string, count int,
	interval time.Duration, onSample func(target string, index int, sample stats.Sample),
) map[string][]stats.Sample {
	results := make(map[string][]stats.Sample, len(targets))
	for _, target := range targets {
		results[target] = make([]stats.Sample, count)
		for i := range results[target] {
			results[target][i] = stats.Sample{Lost: true}
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sent := 0
loop:
	for index := 0; index < count; index++ {
		for _, target := range targets {
			channel, err := p.send(target, index, 56)
			if err != nil {
				continue
			}
			sent++
			wg.Add(1)
			go func(target string, index int, channel <-chan stats.Sample) {
				defer wg.Done()
				sample := <-channel
				mu.Lock()
				if index < len(results[target]) {
					results[target][index] = sample
				}
				mu.Unlock()
				if onSample != nil {
					onSample(target, index, sample)
				}
			}(target, index, channel)
		}
		if index == count-1 {
			break
		}
		select {
		case <-ctx.Done():
			// trim the untouched tail so a cancelled run is not all-loss
			mu.Lock()
			for target := range results {
				if index+1 < len(results[target]) {
					results[target] = results[target][:index+1]
				}
			}
			mu.Unlock()
			break loop
		case <-ticker.C:
		}
	}
	wg.Wait()
	_ = sent
	return results
}

// Stream pings forever until the context is cancelled, publishing every result.
func (p *Pinger) Stream(ctx context.Context, targets []string, interval time.Duration,
	out chan<- StreamSample,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for _, target := range targets {
			channel, err := p.send(target, 0, 56)
			if err != nil {
				continue
			}
			go func(target string, channel <-chan stats.Sample) {
				sample := <-channel
				select {
				case out <- StreamSample{Target: target, Sample: sample, At: time.Now()}:
				case <-ctx.Done():
				}
			}(target, channel)
		}
	}
}
