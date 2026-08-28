// Package monitor watches the connection continuously and records anomalies.
//
// A 60-second speed test cannot see the thing people actually complain about:
// the connection that dies for eight seconds twice an hour. This keeps a
// low-rate ping running against several anchors, watches the kernel counters
// and the resolver on the side, and writes every anomaly to a timestamped log
// with a per-minute timeline, so patterns become visible.
package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/stats"
	"github.com/WinTone01/nabiz/internal/util"
)

// Event is one recorded anomaly.
type Event struct {
	At       time.Time `json:"at"`
	Kind     string    `json:"kind"`
	Severity string    `json:"severity"` // critical | bad | warn | info
	Target   string    `json:"target,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	Duration float64   `json:"duration,omitempty"`
}

// Stamp renders the event time for a log line.
func (e Event) Stamp() string { return e.At.Format("15:04:05") }

// Outage is a window where every internet anchor was unreachable.
type Outage struct {
	Start    time.Time `json:"start"`
	Duration float64   `json:"duration"`
}

type targetState struct {
	label       string
	host        string
	samples     []stats.Sample
	sent        int
	lost        int
	consecutive int
	baseline    float64
	last        *float64
}

// TargetSnapshot is one anchor's live state for the UI.
type TargetSnapshot struct {
	Label   string         `json:"label"`
	Host    string         `json:"host"`
	Sent    int            `json:"sent"`
	Lost    int            `json:"lost"`
	LossPct float64        `json:"loss_pct"`
	Last    *float64       `json:"last"`
	Avg     float64        `json:"avg"`
	P95     float64        `json:"p95"`
	Jitter  float64        `json:"jitter"`
	Recent  []stats.Sample `json:"-"`
}

// Minute is one bucket of the timeline.
type Minute struct {
	LossPct float64 `json:"loss"`
	Avg     float64 `json:"avg"`
	Max     float64 `json:"max"`
}

// Snapshot is everything the monitor knows right now.
type Snapshot struct {
	Uptime        time.Duration    `json:"uptime"`
	Targets       []TargetSnapshot `json:"targets"`
	Timeline      []Minute         `json:"timeline"`
	Outages       []Outage         `json:"outages"`
	OutageActive  bool             `json:"outage_active"`
	DNSFailures   int              `json:"dns_failures"`
	DNSLastMs     float64          `json:"dns_last_ms"`
	ReachFailures int              `json:"reach_failures"`
	ReachLast     string           `json:"reach_last"`
	CarrierFlaps  int              `json:"carrier_flaps"`
	NFQDrops      int64            `json:"nfq_drops"`
	RetransPct    float64          `json:"retrans_pct"`
	Events        []Event          `json:"events"`
	Availability  float64          `json:"availability"`
}

// Monitor is the long-running watcher.
type Monitor struct {
	cfg     config.Config
	onEvent func(Event)
	logPath string

	mu        sync.Mutex
	started   time.Time
	targets   []*targetState
	byHost    map[string]*targetState
	events    []Event
	minutes   map[int64]*minuteBucket
	outageAt  *time.Time
	outages   []Outage
	dnsFail   int
	dnsLastMs float64
	reachFail int
	reachLast string
	flaps     int
	nfqDrops  int64
	retrans   float64

	baseLink probe.LinkInfo
	baseSNMP probe.SNMP
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
}

type minuteBucket struct {
	sent, lost int
	rttSum     float64
	rttN       int
	maxRTT     float64
}

// New creates a monitor. onEvent may be nil.
func New(cfg config.Config, onEvent func(Event)) *Monitor {
	return &Monitor{
		cfg:     cfg,
		onEvent: onEvent,
		logPath: config.MonitorLog(),
		byHost:  map[string]*targetState{},
		minutes: map[int64]*minuteBucket{},
	}
}

// Start begins watching; it returns immediately.
func (m *Monitor) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.started = time.Now()
	m.running = true
	for _, anchor := range m.cfg.LiveAnchors() {
		state := &targetState{label: anchor.Label, host: anchor.Host}
		m.targets = append(m.targets, state)
		m.byHost[anchor.Host] = state
	}
	m.baseLink = probe.ReadLink("")
	m.baseSNMP = probe.ReadSNMP()
	m.mu.Unlock()

	pinger, err := probe.NewPinger(m.cfg.PingTimeout())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	samples := make(chan probe.StreamSample, 128)
	hosts := m.cfg.AnchorHosts()
	m.wg.Add(3)
	go func() {
		defer m.wg.Done()
		defer pinger.Close()
		pinger.Stream(ctx, hosts, m.cfg.MonitorTick(), samples)
	}()
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case sample := <-samples:
				m.onSample(sample)
			}
		}
	}()
	go func() {
		defer m.wg.Done()
		m.sideLoop(ctx)
	}()
	m.emit(Event{At: time.Now(), Kind: "start", Severity: "info",
		Detail: i18n.T("mon.started")})
	return nil
}

// Stop ends the monitor and closes any open outage.
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	m.closeOutage(time.Now())
}

// Running reports whether the monitor is active.
func (m *Monitor) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Monitor) onSample(sample probe.StreamSample) {
	var pending []Event
	m.mu.Lock()
	state := m.byHost[sample.Target]
	if state == nil {
		m.mu.Unlock()
		return
	}
	state.samples = append(state.samples, sample.Sample)
	if len(state.samples) > 900 {
		state.samples = state.samples[len(state.samples)-900:]
	}
	state.sent++
	minute := sample.At.Unix() / 60
	bucket := m.minutes[minute]
	if bucket == nil {
		bucket = &minuteBucket{}
		m.minutes[minute] = bucket
	}
	bucket.sent++

	if sample.Sample.Lost {
		state.lost++
		state.consecutive++
		state.last = nil
		bucket.lost++
	} else {
		if state.consecutive >= 3 {
			pending = append(pending, Event{At: sample.At, Kind: "loss-burst", Severity: "bad",
				Target: state.label,
				Detail: i18n.T("mon.lossburst", state.consecutive)})
		}
		state.consecutive = 0
		rtt := sample.Sample.RTTms
		state.last = &rtt
		bucket.rttSum += rtt
		bucket.rttN++
		if rtt > bucket.maxRTT {
			bucket.maxRTT = rtt
		}
		if got := received(state.samples); len(got) >= 20 {
			state.baseline = stats.Percentile(tail(got, 300), 50)
		}
		if state.baseline > 0 && rtt > maxFloat(150, state.baseline*4) {
			pending = append(pending, Event{At: sample.At, Kind: "spike", Severity: "warn",
				Target: state.label,
				Detail: i18n.T("mon.spike", rtt, state.baseline)})
		}
	}
	if len(m.minutes) > 1500 {
		m.trimMinutes()
	}
	pending = append(pending, m.updateOutageLocked(sample.At)...)
	m.mu.Unlock()

	for _, event := range pending {
		m.emit(event)
	}
}

// updateOutageLocked must be called with the lock held.
//
// An outage is every internet anchor failing at once - one dead anchor is that
// anchor's problem, all of them is yours.
func (m *Monitor) updateOutageLocked(now time.Time) []Event {
	var internet []*targetState
	for _, state := range m.targets {
		if state.label != "Modem / Gateway" {
			internet = append(internet, state)
		}
	}
	if len(internet) == 0 {
		return nil
	}
	allDown := true
	for _, state := range internet {
		if state.consecutive < 2 {
			allDown = false
			break
		}
	}
	switch {
	case allDown && m.outageAt == nil:
		at := now
		m.outageAt = &at
		return []Event{{At: now, Kind: "outage-start", Severity: "critical",
			Detail: i18n.T("mon.outage.start")}}
	case !allDown && m.outageAt != nil:
		start := *m.outageAt
		duration := now.Sub(start).Seconds()
		m.outageAt = nil
		m.outages = append(m.outages, Outage{Start: start, Duration: duration})
		return []Event{{At: now, Kind: "outage-end", Severity: "critical",
			Duration: duration,
			Detail:   i18n.T("mon.outage.end", duration)}}
	}
	return nil
}

func (m *Monitor) closeOutage(now time.Time) {
	m.mu.Lock()
	if m.outageAt == nil {
		m.mu.Unlock()
		return
	}
	start := *m.outageAt
	duration := now.Sub(start).Seconds()
	m.outageAt = nil
	m.outages = append(m.outages, Outage{Start: start, Duration: duration})
	m.mu.Unlock()
	m.emit(Event{At: now, Kind: "outage-end", Severity: "critical", Duration: duration,
		Detail: i18n.T("mon.outage.end", duration)})
}

func (m *Monitor) trimMinutes() {
	var keys []int64
	for key := range m.minutes {
		keys = append(keys, key)
	}
	if len(keys) <= 1440 {
		return
	}
	// keep the newest 1440 buckets (24h)
	var newest int64
	for _, key := range keys {
		if key > newest {
			newest = key
		}
	}
	for _, key := range keys {
		if newest-key > 1440 {
			delete(m.minutes, key)
		}
	}
}

func (m *Monitor) sideLoop(ctx context.Context) {
	control := "example.com"
	if len(m.cfg.ControlDomains) > 0 {
		control = m.cfg.ControlDomains[0]
	}
	sensitive := "discord.com"
	if len(m.cfg.SensitiveDomains) > 0 {
		sensitive = m.cfg.SensitiveDomains[0]
	}
	dnsTicker := time.NewTicker(time.Duration(m.cfg.MonitorDNSEvery) * time.Second)
	reachTicker := time.NewTicker(time.Duration(m.cfg.MonitorReachEvery) * time.Second)
	linkTicker := time.NewTicker(5 * time.Second)
	defer dnsTicker.Stop()
	defer reachTicker.Stop()
	defer linkTicker.Stop()
	toggle := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-dnsTicker.C:
			m.checkDNS(ctx, control)
		case <-reachTicker.C:
			toggle = !toggle
			domain := control
			if toggle {
				domain = sensitive
			}
			m.checkReach(ctx, domain)
		case <-linkTicker.C:
			m.checkLink()
		}
	}
}

func (m *Monitor) checkDNS(ctx context.Context, domain string) {
	servers := probe.SystemResolvers()
	if len(servers) == 0 {
		return
	}
	result := probe.QueryUDP(ctx, servers[0], domain, "A", m.cfg.DNSTimeout(), false)
	m.mu.Lock()
	m.dnsLastMs = result.Ms
	if !result.OK {
		m.dnsFail++
	}
	m.mu.Unlock()
	switch {
	case !result.OK:
		m.emit(Event{At: time.Now(), Kind: "dns-fail", Severity: "bad",
			Target: servers[0], Detail: result.Summary()})
	case result.Ms > m.cfg.Thresholds.DNSBadMs:
		m.emit(Event{At: time.Now(), Kind: "dns-slow", Severity: "warn",
			Target: servers[0], Detail: fmt.Sprintf("%.0f ms", result.Ms)})
	}
}

func (m *Monitor) checkReach(ctx context.Context, domain string) {
	addrs := util.ResolveIPv4(domain, m.cfg.DNSTimeout())
	if len(addrs) == 0 {
		m.mu.Lock()
		m.reachFail++
		m.reachLast = domain + ": " + i18n.T("mon.noresolve")
		m.mu.Unlock()
		m.emit(Event{At: time.Now(), Kind: "reach-dns", Severity: "bad",
			Target: domain, Detail: i18n.T("mon.noresolve")})
		return
	}
	result := probe.TLSHandshake(ctx, addrs[0], domain, 443, m.cfg.ConnectTimeout(), "none", false)
	m.mu.Lock()
	m.reachLast = fmt.Sprintf("%s: %s (%.0f ms)", domain, result.Kind, result.Ms)
	if !result.OK {
		m.reachFail++
	}
	m.mu.Unlock()
	if !result.OK {
		m.emit(Event{At: time.Now(), Kind: "reach-fail", Severity: "bad", Target: domain,
			Detail: result.Kind + " - " + result.Err})
	}
}

func (m *Monitor) checkLink() {
	info := probe.ReadLink(m.baseLink.Iface)
	health := probe.ReadSNMP().Health()
	var events []Event
	m.mu.Lock()
	m.retrans = health.RetransPct
	if m.baseLink.CarrierUps >= 0 && info.CarrierUps > m.baseLink.CarrierUps {
		m.flaps += int(info.CarrierUps - m.baseLink.CarrierUps)
		events = append(events, Event{At: time.Now(), Kind: "carrier-flap", Severity: "critical",
			Target: info.Iface, Detail: i18n.T("mon.flap")})
		m.baseLink = info
	}
	nfq := probe.ReadNFQueues()
	var drops int64
	for _, queue := range nfq.Queues {
		drops += queue.QueueDropped + queue.UserDropped
	}
	if drops > m.nfqDrops {
		delta := drops - m.nfqDrops
		m.nfqDrops = drops
		events = append(events, Event{At: time.Now(), Kind: "nfqueue-drop", Severity: "bad",
			Detail: i18n.T("mon.nfqdrop", delta)})
	}
	m.mu.Unlock()
	for _, event := range events {
		m.emit(event)
	}
}

func (m *Monitor) emit(event Event) {
	m.mu.Lock()
	m.events = append(m.events, event)
	if len(m.events) > 500 {
		m.events = m.events[len(m.events)-500:]
	}
	m.mu.Unlock()
	if m.onEvent != nil {
		m.onEvent(event)
	}
	if m.logPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.logPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(m.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	if data, err := json.Marshal(event); err == nil {
		_, _ = file.Write(append(data, '\n'))
	}
}

// Snapshot returns a consistent view for rendering.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := Snapshot{
		DNSFailures:   m.dnsFail,
		DNSLastMs:     m.dnsLastMs,
		ReachFailures: m.reachFail,
		ReachLast:     m.reachLast,
		CarrierFlaps:  m.flaps,
		NFQDrops:      m.nfqDrops,
		RetransPct:    m.retrans,
		OutageActive:  m.outageAt != nil,
		Outages:       append([]Outage(nil), m.outages...),
		Events:        append([]Event(nil), m.events...),
	}
	if !m.started.IsZero() {
		snapshot.Uptime = time.Since(m.started)
	}
	for _, state := range m.targets {
		recent := tailSamples(state.samples, 300)
		summary := stats.Summarize(state.label, state.host, recent)
		lossPct := 0.0
		if state.sent > 0 {
			lossPct = 100 * float64(state.lost) / float64(state.sent)
		}
		snapshot.Targets = append(snapshot.Targets, TargetSnapshot{
			Label: state.label, Host: state.host, Sent: state.sent, Lost: state.lost,
			LossPct: lossPct, Last: state.last, Avg: summary.Avg, P95: summary.P95,
			Jitter: summary.Jitter, Recent: tailSamples(state.samples, 160),
		})
	}
	var keys []int64
	for key := range m.minutes {
		keys = append(keys, key)
	}
	sortInt64(keys)
	if len(keys) > 120 {
		keys = keys[len(keys)-120:]
	}
	for _, key := range keys {
		bucket := m.minutes[key]
		minute := Minute{}
		if bucket.sent > 0 {
			minute.LossPct = 100 * float64(bucket.lost) / float64(bucket.sent)
		}
		if bucket.rttN > 0 {
			minute.Avg = bucket.rttSum / float64(bucket.rttN)
		}
		minute.Max = bucket.maxRTT
		snapshot.Timeline = append(snapshot.Timeline, minute)
	}
	total := 0.0
	for _, outage := range snapshot.Outages {
		total += outage.Duration
	}
	snapshot.Availability = 100
	if snapshot.Uptime > 0 {
		snapshot.Availability = maxFloat(0, 100-100*total/snapshot.Uptime.Seconds())
	}
	return snapshot
}

// HourlyHistogram counts outages per hour of day, revealing scheduled misery.
func (m *Monitor) HourlyHistogram() [24]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	var hours [24]int
	for _, outage := range m.outages {
		hours[outage.Start.Hour()]++
	}
	return hours
}

// LoadEvents reads the persisted event log.
func LoadEvents(path string, limit int) []Event {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var events []Event
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			events = append(events, event)
		}
	}
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events
}

func splitLines(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			out = append(out, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

func received(samples []stats.Sample) []float64 {
	var out []float64
	for _, sample := range samples {
		if !sample.Lost {
			out = append(out, sample.RTTms)
		}
	}
	return out
}

func tail(values []float64, n int) []float64 {
	if len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func tailSamples(values []stats.Sample, n int) []stats.Sample {
	if len(values) <= n {
		return append([]stats.Sample(nil), values...)
	}
	return append([]stats.Sample(nil), values[len(values)-n:]...)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sortInt64(values []int64) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
