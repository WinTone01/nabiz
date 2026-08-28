package probe

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WinTone01/nabiz/internal/stats"
)

// Raw speed is the least interesting number on a modern line. What breaks
// calls, games and page loads is the queue that builds in the modem the moment
// the uplink saturates. So every transfer here runs with a ping in parallel and
// with the kernel's own per-socket counters sampled underneath it: the verdict
// is the latency delta and the retransmission rate, not the megabits.

const transferChunk = 64 * 1024

// TransferResult is one direction of a throughput test.
type TransferResult struct {
	Direction string        `json:"direction"`
	Bytes     int64         `json:"bytes"`
	Seconds   float64       `json:"seconds"`
	Bps       float64       `json:"bps"`
	PeakBps   float64       `json:"peak_bps"`
	Streams   int           `json:"streams"`
	Samples   []float64     `json:"samples"`
	Sockets   SocketSummary `json:"sockets"`
	SNMPDelta TCPHealth     `json:"snmp_delta"`
	Err       string        `json:"err,omitempty"`
}

var transferClient = &http.Client{
	Transport: &http.Transport{
		DisableCompression:  true,
		MaxIdleConnsPerHost: 16,
		ForceAttemptHTTP2:   false, // one TCP stream per goroutine, not one h2 mux
	},
}

func downloadWorker(ctx context.Context, url string, counter *atomic.Int64, errs chan<- string) {
	for ctx.Err() == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		req.Header.Set("user-agent", "nabiz")
		resp, err := transferClient.Do(req)
		if err != nil {
			if ctx.Err() == nil {
				select {
				case errs <- shortErr(err):
				default:
				}
			}
			return
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			select {
			case errs <- "HTTP " + resp.Status:
			default:
			}
			return
		}
		buf := make([]byte, transferChunk)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				counter.Add(int64(n))
			}
			if err != nil || ctx.Err() != nil {
				break
			}
		}
		resp.Body.Close()
	}
}

// countingReader feeds random bytes to the uploader and counts what the
// transport actually consumed.
type countingReader struct {
	ctx     context.Context
	block   []byte
	counter *atomic.Int64
	limit   int64
	sent    int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.ctx.Err() != nil {
		return 0, io.EOF
	}
	if c.sent >= c.limit {
		return 0, io.EOF
	}
	n := copy(p, c.block)
	if remaining := c.limit - c.sent; int64(n) > remaining {
		n = int(remaining)
	}
	c.sent += int64(n)
	c.counter.Add(int64(n))
	return n, nil
}

func uploadWorker(ctx context.Context, url string, counter *atomic.Int64, errs chan<- string) {
	block := make([]byte, 256*1024)
	_, _ = rand.Read(block)
	const blockTotal = 8 * 1024 * 1024
	for ctx.Err() == nil {
		body := &countingReader{ctx: ctx, block: block, counter: counter, limit: blockTotal}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			return
		}
		req.ContentLength = blockTotal
		req.Header.Set("content-type", "application/octet-stream")
		resp, err := transferClient.Do(req)
		if err != nil {
			if ctx.Err() == nil {
				select {
				case errs <- shortErr(err):
				default:
				}
			}
			return
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
	}
}

// Transfer saturates the link in one direction for the given duration.
func Transfer(ctx context.Context, url, direction string, duration time.Duration,
	streams int, socketFilter string, onProgress func(elapsed time.Duration, bps float64),
) TransferResult {
	if streams < 1 {
		streams = 1
	}
	result := TransferResult{Direction: direction, Streams: streams}
	before := ReadSNMP()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var counter atomic.Int64
	errs := make(chan string, streams)
	var wg sync.WaitGroup
	worker := downloadWorker
	if direction == "up" {
		worker = uploadWorker
	}
	for i := 0; i < streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(runCtx, url, &counter, errs)
		}()
	}

	socketStop := make(chan struct{})
	socketDone := make(chan SocketSummary, 1)
	go func() {
		socketDone <- SampleSockets(socketStop, socketFilter, 400*time.Millisecond)
	}()

	start := time.Now()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastValue, lastTime := int64(0), start
	deadline := time.After(duration)
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-deadline:
			break loop
		case now := <-ticker.C:
			value := counter.Load()
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed <= 0 {
				continue
			}
			bps := float64(value-lastValue) * 8 / elapsed
			result.Samples = append(result.Samples, bps)
			lastValue, lastTime = value, now
			if onProgress != nil {
				onProgress(now.Sub(start), bps)
			}
		}
	}
	cancel()
	close(socketStop)
	result.Sockets = <-socketDone
	wg.Wait()

	result.Seconds = time.Since(start).Seconds()
	result.Bytes = counter.Load()
	// discard the first second: TCP slow start is not the line's fault
	warm := result.Samples
	if len(warm) > 4 {
		warm = warm[4:]
	}
	if len(warm) > 0 {
		total := 0.0
		for _, sample := range warm {
			total += sample
			if sample > result.PeakBps {
				result.PeakBps = sample
			}
		}
		result.Bps = total / float64(len(warm))
	}
	result.SNMPDelta = ReadSNMP().Delta(before).Health()
	if result.Bytes == 0 {
		select {
		case message := <-errs:
			result.Err = message
		default:
		}
	}
	return result
}

// --- bufferbloat --------------------------------------------------------

// BloatResult is the idle-versus-loaded comparison plus both transfers.
type BloatResult struct {
	Idle        stats.Summary   `json:"idle"`
	DownLatency stats.Summary   `json:"down_latency"`
	UpLatency   stats.Summary   `json:"up_latency"`
	Download    *TransferResult `json:"download,omitempty"`
	Upload      *TransferResult `json:"upload,omitempty"`
	DownDelta   float64         `json:"down_delta"`
	UpDelta     float64         `json:"up_delta"`
	Grade       string          `json:"grade"`
	Err         string          `json:"err,omitempty"`
}

// WorstDelta is the number the grade is based on.
func (b BloatResult) WorstDelta() float64 {
	if b.UpDelta > b.DownDelta {
		return b.UpDelta
	}
	return b.DownDelta
}

// latencyRecorder pings an anchor continuously so any window of the run can be
// summarised after the fact.
type latencyRecorder struct {
	mu      sync.Mutex
	samples []stats.Sample
	cancel  context.CancelFunc
	done    chan struct{}
	pinger  *Pinger
}

func startRecorder(anchor string, interval time.Duration) (*latencyRecorder, error) {
	pinger, err := NewPinger(1500 * time.Millisecond)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &latencyRecorder{cancel: cancel, done: make(chan struct{}), pinger: pinger}
	channel := make(chan StreamSample, 64)
	go pinger.Stream(ctx, []string{anchor}, interval, channel)
	go func() {
		defer close(recorder.done)
		for {
			select {
			case <-ctx.Done():
				return
			case sample := <-channel:
				recorder.mu.Lock()
				recorder.samples = append(recorder.samples, sample.Sample)
				recorder.mu.Unlock()
			}
		}
	}()
	return recorder, nil
}

func (r *latencyRecorder) mark() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.samples)
}

func (r *latencyRecorder) slice(from int) []stats.Sample {
	r.mu.Lock()
	defer r.mu.Unlock()
	if from > len(r.samples) {
		return nil
	}
	out := make([]stats.Sample, len(r.samples)-from)
	copy(out, r.samples[from:])
	return out
}

func (r *latencyRecorder) stop() {
	r.cancel()
	<-r.done
	r.pinger.Close()
}

// BloatOptions configures a latency-under-load run.
type BloatOptions struct {
	DownURL      string
	UpURL        string
	Anchor       string
	SocketFilter string
	Duration     time.Duration
	IdleDuration time.Duration
	Streams      int
	OnPhase      func(phase string, bps float64)
}

// Bufferbloat measures idle latency, then latency while saturating each
// direction in turn.
func Bufferbloat(ctx context.Context, options BloatOptions) BloatResult {
	var result BloatResult
	recorder, err := startRecorder(options.Anchor, 150*time.Millisecond)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	defer recorder.stop()
	phase := func(name string, bps float64) {
		if options.OnPhase != nil {
			options.OnPhase(name, bps)
		}
	}

	phase("idle", 0)
	idleStart := recorder.mark()
	select {
	case <-ctx.Done():
	case <-time.After(options.IdleDuration):
	}
	result.Idle = stats.Summarize("idle", options.Anchor, recorder.slice(idleStart))

	if ctx.Err() == nil {
		phase("download", 0)
		downStart := recorder.mark()
		download := Transfer(ctx, options.DownURL, "down", options.Duration,
			options.Streams, options.SocketFilter,
			func(_ time.Duration, bps float64) { phase("download", bps) })
		result.Download = &download
		result.DownLatency = stats.Summarize("download", options.Anchor, recorder.slice(downStart))
	}

	if ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		phase("upload", 0)
		upStart := recorder.mark()
		upStreams := options.Streams / 2
		if upStreams < 2 {
			upStreams = 2
		}
		upload := Transfer(ctx, options.UpURL, "up", options.Duration*3/4, upStreams,
			options.SocketFilter, func(_ time.Duration, bps float64) { phase("upload", bps) })
		result.Upload = &upload
		result.UpLatency = stats.Summarize("upload", options.Anchor, recorder.slice(upStart))
	}

	base := result.Idle.P50
	if base > 0 {
		if result.DownLatency.Received > 0 {
			result.DownDelta = maxFloat(0, result.DownLatency.P95-base)
		}
		if result.UpLatency.Received > 0 {
			result.UpDelta = maxFloat(0, result.UpLatency.P95-base)
		}
	} else {
		result.Err = "boşta gecikme ölçülemedi / idle latency unavailable"
	}
	result.Grade = stats.BufferbloatGrade(result.WorstDelta())
	return result
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
