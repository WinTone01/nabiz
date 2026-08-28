package probe

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/WinTone01/nabiz/internal/util"
)

// BenignSNI is a name no censor cares about, used to separate "this IP is
// blocked" from "this name is blocked".
const BenignSNI = "example.com"

// TCPResult records not just success but the shape of the failure.
type TCPResult struct {
	OK   bool    `json:"ok"`
	Ms   float64 `json:"ms"`
	Kind string  `json:"kind"` // ok | refused | timeout | unreachable | error
	Err  string  `json:"err,omitempty"`
}

// TLSResult is one handshake attempt with a specific segmentation strategy.
type TLSResult struct {
	OK        bool     `json:"ok"`
	Ms        float64  `json:"ms"`
	Kind      string   `json:"kind"` // ok | reset | timeout | eof | cert | error | skipped
	Err       string   `json:"err,omitempty"`
	Version   string   `json:"version,omitempty"`
	Cipher    string   `json:"cipher,omitempty"`
	ALPN      string   `json:"alpn,omitempty"`
	PeerCN    string   `json:"peer_cn,omitempty"`
	IssuerCN  string   `json:"issuer_cn,omitempty"`
	HelloSize int      `json:"hello_size,omitempty"`
	SplitAt   int      `json:"split_at,omitempty"`
	Chain     []string `json:"chain,omitempty"`
}

// DomainVerdict is the full picture for one hostname.
type DomainVerdict struct {
	Domain   string    `json:"domain"`
	IP       string    `json:"ip"`
	IPSource string    `json:"ip_source"`
	DNSOK    bool      `json:"dns_ok"`
	TCP      TCPResult `json:"tcp"`
	Whole    TLSResult `json:"tls_whole"`
	SplitHdr TLSResult `json:"tls_split_header"`
	SplitSNI TLSResult `json:"tls_split_sni"`
	Benign   TLSResult `json:"tls_benign_sni"`
	QUIC     string    `json:"quic"`
	HTTP80   string    `json:"http80"`
	Verdict  string    `json:"verdict"`
	Note     string    `json:"note"`
	TTFBms   float64   `json:"ttfb_ms,omitempty"`
}

// --- TCP ----------------------------------------------------------------

// TCPConnect distinguishes a refusal (an RST, usually injected) from a
// blackhole (a silent drop) from an unreachable route.
func TCPConnect(ctx context.Context, ip string, port int, timeout time.Duration) TCPResult {
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, fmt.Sprint(port)))
	if err == nil {
		_ = conn.Close()
		return TCPResult{OK: true, Ms: msSince(start), Kind: "ok"}
	}
	return TCPResult{Ms: msSince(start), Kind: classifyNetErr(err), Err: shortErr(err)}
}

func classifyNetErr(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, syscall.ECONNRESET):
		return "reset"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return "unreachable"
	case errors.Is(err, context.DeadlineExceeded), os_IsTimeout(err):
		return "timeout"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "eof"
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return "cert"
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return "cert"
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return "cert"
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "reset"):
		return "reset"
	case strings.Contains(text, "timeout"), strings.Contains(text, "timed out"):
		return "timeout"
	case strings.Contains(text, "eof"):
		return "eof"
	case strings.Contains(text, "certificate"):
		return "cert"
	}
	return "error"
}

func os_IsTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// --- TLS with a controlled ClientHello ----------------------------------

// splitConn cuts the very first Write into two segments with a pause between
// them. That is the userspace shadow of what zapret does inside the kernel: a
// DPI engine that matches SNI within a single packet never sees the full name.
type splitConn struct {
	net.Conn
	once  sync.Once
	mode  string // none | header | sni
	sni   string
	delay time.Duration

	firstSize int
	splitAt   int
}

func (c *splitConn) Write(payload []byte) (int, error) {
	var (
		written int
		err     error
		done    bool
	)
	c.once.Do(func() {
		done = true
		c.firstSize = len(payload)
		cut := splitOffset(payload, c.sni, c.mode)
		c.splitAt = cut
		if cut <= 0 || cut >= len(payload) {
			written, err = c.Conn.Write(payload)
			return
		}
		var n int
		n, err = c.Conn.Write(payload[:cut])
		written += n
		if err != nil {
			return
		}
		time.Sleep(c.delay)
		n, err = c.Conn.Write(payload[cut:])
		written += n
	})
	if done {
		return written, err
	}
	return c.Conn.Write(payload)
}

func splitOffset(hello []byte, sni, mode string) int {
	switch mode {
	case "header":
		return 2 // cuts the TLS record header itself
	case "sni":
		if index := bytes.Index(hello, []byte(sni)); index > 0 {
			return index + max(1, len(sni)/2)
		}
		return 2
	default:
		return 0
	}
}

// TLSHandshake performs one handshake and reports exactly how it ended.
// verify=false keeps certificate problems from masking DPI behaviour; the
// certificate is still inspected afterwards so interception is still visible.
func TLSHandshake(ctx context.Context, ip, sni string, port int, timeout time.Duration,
	splitMode string, verify bool,
) TLSResult {
	out := TLSResult{}
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	raw, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, fmt.Sprint(port)))
	if err != nil {
		out.Kind = classifyNetErr(err)
		out.Err = shortErr(err)
		out.Ms = msSince(start)
		return out
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(timeout))

	wrapped := &splitConn{Conn: raw, mode: splitMode, sni: sni, delay: 30 * time.Millisecond}
	config := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: !verify,
		NextProtos:         []string{"h2", "http/1.1"},
		MinVersion:         tls.VersionTLS12,
	}
	client := tls.Client(wrapped, config)
	handshakeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.HandshakeContext(handshakeCtx); err != nil {
		out.Kind = classifyNetErr(err)
		out.Err = shortErr(err)
		out.Ms = msSince(start)
		out.HelloSize = wrapped.firstSize
		out.SplitAt = wrapped.splitAt
		return out
	}
	state := client.ConnectionState()
	out.OK = true
	out.Kind = "ok"
	out.Ms = msSince(start)
	out.Version = tlsVersionName(state.Version)
	out.Cipher = tls.CipherSuiteName(state.CipherSuite)
	out.ALPN = state.NegotiatedProtocol
	out.HelloSize = wrapped.firstSize
	out.SplitAt = wrapped.splitAt
	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		out.PeerCN = leaf.Subject.CommonName
		out.IssuerCN = leaf.Issuer.CommonName
		for _, cert := range state.PeerCertificates {
			out.Chain = append(out.Chain, cert.Subject.CommonName)
		}
		// verify separately so we can report a MITM without failing the probe
		if _, err := leaf.Verify(x509.VerifyOptions{DNSName: sni,
			Intermediates: intermediates(state.PeerCertificates)}); err != nil {
			out.Kind = "cert"
			out.Err = shortErr(err)
		}
	}
	_ = client.Close()
	return out
}

func intermediates(certs []*x509.Certificate) *x509.CertPool {
	if len(certs) < 2 {
		return nil
	}
	pool := x509.NewCertPool()
	for _, cert := range certs[1:] {
		pool.AddCert(cert)
	}
	return pool
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS10:
		return "TLS1.0"
	default:
		return fmt.Sprintf("0x%04x", version)
	}
}

// --- QUIC ---------------------------------------------------------------

// QUICProbe forces a Version Negotiation reply by announcing a reserved
// version. No crypto is needed: RFC 9000 requires the server to answer, so one
// datagram tells us whether UDP/443 survives the path end to end.
func QUICProbe(ctx context.Context, ip string, port int, timeout time.Duration) string {
	packet := make([]byte, 1200)
	packet[0] = 0xc3
	copy(packet[1:5], []byte{0x0a, 0x0a, 0x0a, 0x0a}) // reserved version
	packet[5] = 8
	for i := 0; i < 8; i++ {
		packet[6+i] = byte(time.Now().UnixNano() >> (i * 8))
	}
	packet[14] = 8
	for i := 0; i < 8; i++ {
		packet[15+i] = byte(time.Now().UnixNano()>>(i*8)) ^ 0x5a
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(ip, fmt.Sprint(port)))
	if err != nil {
		return "error: " + shortErr(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(packet); err != nil {
		return "error: " + shortErr(err)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		if os_IsTimeout(err) {
			return "timeout"
		}
		return "error: " + shortErr(err)
	}
	data := buf[:n]
	if n >= 5 && data[0]&0x80 != 0 && bytes.Equal(data[1:5], []byte{0, 0, 0, 0}) {
		offset := 5
		if offset < n {
			offset += 1 + int(data[offset])
		}
		if offset < n {
			offset += 1 + int(data[offset])
		}
		var versions []string
		for offset+4 <= n && len(versions) < 3 {
			versions = append(versions,
				fmt.Sprintf("0x%02x%02x%02x%02x", data[offset], data[offset+1],
					data[offset+2], data[offset+3]))
			offset += 4
		}
		if len(versions) > 0 {
			return "ok (" + strings.Join(versions, ", ") + ")"
		}
		return "ok"
	}
	return "unexpected reply"
}

// --- plain HTTP ---------------------------------------------------------

var blockPageMarkers = []string{
	"bilgi teknolojileri", "iletisim baskanligi", "5651", "blocked",
	"engellen", "yasakl", "internet2.ttnet", "guvenlik duvari",
}

// HTTPProbe sends a Host header on port 80 - the oldest DPI trigger there is -
// and looks for a redirect to a block page.
func HTTPProbe(ctx context.Context, ip, host string, timeout time.Duration) string {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "80"))
	if err != nil {
		return classifyNetErr(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	request := "GET / HTTP/1.1\r\nHost: " + host + "\r\nUser-Agent: nabiz\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		return classifyNetErr(err)
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil && n == 0 {
		return classifyNetErr(err)
	}
	data := buf[:n]
	status, _, _ := strings.Cut(string(data), "\r\n")
	lower := strings.ToLower(string(data))
	for _, marker := range blockPageMarkers {
		if strings.Contains(lower, marker) {
			return util.Truncate(status, 40) + " [block page]"
		}
	}
	return util.Truncate(status, 40)
}

// --- orchestration ------------------------------------------------------

// ProbeDomain runs the whole ladder and classifies the outcome.
func ProbeDomain(ctx context.Context, domain, ip, ipSource string,
	timeout time.Duration, doQUIC, doHTTP bool,
) DomainVerdict {
	out := DomainVerdict{Domain: domain, IP: ip, IPSource: ipSource}
	if out.IP == "" {
		addrs := util.ResolveIPv4(domain, timeout)
		if len(addrs) == 0 {
			out.Verdict = "dns-fail"
			out.Note = "ad çözümlenemedi / name did not resolve"
			return out
		}
		out.IP = addrs[0]
	}
	out.DNSOK = true

	out.TCP = TCPConnect(ctx, out.IP, 443, timeout)
	if !out.TCP.OK {
		out.Verdict = "ip-block"
		out.Note = out.TCP.Err
		if doQUIC {
			out.QUIC = QUICProbe(ctx, out.IP, 443, minDuration(timeout, 3*time.Second))
		}
		return out
	}

	out.Whole = TLSHandshake(ctx, out.IP, domain, 443, timeout, "none", false)
	switch {
	case out.Whole.OK && out.Whole.Kind == "ok":
		out.Verdict = "clean"
	case out.Whole.Kind == "cert":
		out.Verdict = "mitm"
		out.Note = "sertifika doğrulanmadı: " + out.Whole.Err
	default:
		out.SplitHdr = TLSHandshake(ctx, out.IP, domain, 443, timeout, "header", false)
		out.SplitSNI = TLSHandshake(ctx, out.IP, domain, 443, timeout, "sni", false)
		out.Benign = TLSHandshake(ctx, out.IP, BenignSNI, 443, timeout, "none", false)
		switch {
		case out.SplitHdr.OK || out.SplitSNI.OK:
			out.Verdict = "dpi-split-helps"
			which := "sni-split"
			if out.SplitHdr.OK {
				which = "record-split"
			}
			out.Note = which + " ile geçti / passed with " + which
		case out.Benign.OK:
			out.Verdict = "dpi-hard"
			out.Note = "aynı IP başka SNI ile açılıyor / same IP opens with another SNI"
		default:
			out.Verdict = "unreachable"
			out.Note = out.Whole.Err
		}
	}

	if doQUIC {
		out.QUIC = QUICProbe(ctx, out.IP, 443, minDuration(timeout, 3*time.Second))
	}
	if doHTTP {
		out.HTTP80 = HTTPProbe(ctx, out.IP, domain, timeout)
	}
	return out
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
