package probe

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/WinTone01/nabiz/internal/util"
)

// Resolver describes one place to ask a question, and how to get there.
type Resolver struct {
	Label    string `json:"label"`
	Kind     string `json:"kind"`     // system | udp | tcp | dot | doh
	Address  string `json:"address"`  // ip[:port], or URL for doh
	Hostname string `json:"hostname"` // TLS server name for dot
	Trusted  bool   `json:"trusted"`  // usable as ground truth for tamper checks
}

// Encrypted reports whether the transport hides the query from the path.
func (r Resolver) Encrypted() bool { return r.Kind == "dot" || r.Kind == "doh" }

// DNSResult is one answered (or unanswered) question.
type DNSResult struct {
	OK        bool     `json:"ok"`
	Ms        float64  `json:"ms"`
	RCode     string   `json:"rcode"`
	Addrs     []string `json:"addrs"`
	CNAMEs    []string `json:"cnames"`
	TTL       uint32   `json:"ttl"`
	Truncated bool     `json:"truncated"`
	AD        bool     `json:"ad"`
	Err       string   `json:"err"`
	Transport string   `json:"transport"`
	Server    string   `json:"server"`
	OffPath   string   `json:"off_path,omitempty"`
}

// Summary renders the result for a table cell.
func (r DNSResult) Summary() string {
	if r.OK && len(r.Addrs) > 0 {
		return strings.Join(r.Addrs, ", ")
	}
	if r.Err != "" {
		return r.Err
	}
	if r.RCode != "" {
		return r.RCode
	}
	return "fail"
}

// rcodeName uses the names operators actually type, not Go's verbose ones.
func rcodeName(code dnsmessage.RCode) string {
	switch code {
	case dnsmessage.RCodeSuccess:
		return "NOERROR"
	case dnsmessage.RCodeFormatError:
		return "FORMERR"
	case dnsmessage.RCodeServerFailure:
		return "SERVFAIL"
	case dnsmessage.RCodeNameError:
		return "NXDOMAIN"
	case dnsmessage.RCodeNotImplemented:
		return "NOTIMP"
	case dnsmessage.RCodeRefused:
		return "REFUSED"
	default:
		return strings.ToUpper(strings.TrimPrefix(code.String(), "RCode"))
	}
}

func buildQuery(name, qtype string, wantDNSSEC bool) ([]byte, uint16, error) {
	dnsName, err := dnsmessage.NewName(strings.TrimSuffix(name, ".") + ".")
	if err != nil {
		return nil, 0, err
	}
	id := uint16(rand.Intn(65535))
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{ID: id, RecursionDesired: true},
		Questions: []dnsmessage.Question{{
			Name:  dnsName,
			Type:  qtypeOf(qtype),
			Class: dnsmessage.ClassINET,
		}},
	}
	if wantDNSSEC {
		var header dnsmessage.ResourceHeader
		if err := header.SetEDNS0(4096, dnsmessage.RCodeSuccess, true); err != nil {
			return nil, 0, err
		}
		msg.Additionals = append(msg.Additionals, dnsmessage.Resource{
			Header: header, Body: &dnsmessage.OPTResource{},
		})
	}
	packed, err := msg.Pack()
	return packed, id, err
}

func qtypeOf(name string) dnsmessage.Type {
	switch strings.ToUpper(name) {
	case "AAAA":
		return dnsmessage.TypeAAAA
	case "CNAME":
		return dnsmessage.TypeCNAME
	case "TXT":
		return dnsmessage.TypeTXT
	case "NS":
		return dnsmessage.TypeNS
	case "SOA":
		return dnsmessage.TypeSOA
	default:
		return dnsmessage.TypeA
	}
}

func parseResponse(data []byte, expectID uint16, checkID bool) DNSResult {
	var out DNSResult
	var msg dnsmessage.Message
	if err := msg.Unpack(data); err != nil {
		out.Err = "parse: " + err.Error()
		return out
	}
	if checkID && msg.Header.ID != expectID {
		out.Err = "id mismatch"
		return out
	}
	out.RCode = rcodeName(msg.Header.RCode)
	out.Truncated = msg.Header.Truncated
	out.AD = msg.Header.AuthenticData
	for _, answer := range msg.Answers {
		switch body := answer.Body.(type) {
		case *dnsmessage.AResource:
			out.Addrs = append(out.Addrs, net.IP(body.A[:]).String())
			if out.TTL == 0 {
				out.TTL = answer.Header.TTL
			}
		case *dnsmessage.AAAAResource:
			out.Addrs = append(out.Addrs, net.IP(body.AAAA[:]).String())
			if out.TTL == 0 {
				out.TTL = answer.Header.TTL
			}
		case *dnsmessage.CNAMEResource:
			out.CNAMEs = append(out.CNAMEs, body.CNAME.String())
		}
	}
	out.OK = msg.Header.RCode == dnsmessage.RCodeSuccess
	return out
}

func hostPort(address string, defaultPort string) string {
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	return net.JoinHostPort(address, defaultPort)
}

// QueryUDP is the plain, interceptable path - and therefore the interesting one.
func QueryUDP(ctx context.Context, server, name, qtype string, timeout time.Duration,
	wantDNSSEC bool,
) DNSResult {
	target := hostPort(server, "53")
	packed, id, err := buildQuery(name, qtype, wantDNSSEC)
	if err != nil {
		return DNSResult{Err: err.Error(), Transport: "udp", Server: server}
	}
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", target)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "udp", Server: server}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(packed); err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "udp", Server: server}
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "udp", Server: server}
	}
	out := parseResponse(buf[:n], id, true)
	out.Ms = msSince(start)
	out.Transport = "udp"
	out.Server = server
	return out
}

func tcpExchange(conn net.Conn, packed []byte, timeout time.Duration) ([]byte, error) {
	_ = conn.SetDeadline(time.Now().Add(timeout))
	framed := make([]byte, 2+len(packed))
	binary.BigEndian.PutUint16(framed, uint16(len(packed)))
	copy(framed[2:], packed)
	if _, err := conn.Write(framed); err != nil {
		return nil, err
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	body := make([]byte, binary.BigEndian.Uint16(header))
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	return body, nil
}

// QueryTCP is also how we detect UDP-only interception.
func QueryTCP(ctx context.Context, server, name, qtype string, timeout time.Duration,
	wantDNSSEC bool,
) DNSResult {
	target := hostPort(server, "53")
	packed, id, err := buildQuery(name, qtype, wantDNSSEC)
	if err != nil {
		return DNSResult{Err: err.Error(), Transport: "tcp", Server: server}
	}
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "tcp", Server: server}
	}
	defer conn.Close()
	body, err := tcpExchange(conn, packed, timeout)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "tcp", Server: server}
	}
	out := parseResponse(body, id, true)
	out.Ms = msSince(start)
	out.Transport = "tcp"
	out.Server = server
	return out
}

// QueryDoT speaks DNS over TLS on 853.
func QueryDoT(ctx context.Context, server, hostname, name, qtype string,
	timeout time.Duration, wantDNSSEC bool,
) DNSResult {
	target := hostPort(server, "853")
	packed, id, err := buildQuery(name, qtype, wantDNSSEC)
	if err != nil {
		return DNSResult{Err: err.Error(), Transport: "dot", Server: server}
	}
	start := time.Now()
	dialer := &net.Dialer{Timeout: timeout}
	config := &tls.Config{ServerName: hostname}
	if hostname == "" {
		config.InsecureSkipVerify = true
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", target, config)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "dot", Server: server}
	}
	defer conn.Close()
	body, err := tcpExchange(conn, packed, timeout)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "dot", Server: server}
	}
	out := parseResponse(body, id, true)
	out.Ms = msSince(start)
	out.Transport = "dot"
	out.Server = server
	return out
}

var dohClient = &http.Client{
	Transport: &http.Transport{
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
	},
}

// QueryDoH speaks RFC 8484 over HTTPS; Go negotiates HTTP/2 for us, which
// matters because several public resolvers refuse HTTP/1.1.
func QueryDoH(ctx context.Context, url, name, qtype string, timeout time.Duration,
	wantDNSSEC bool,
) DNSResult {
	packed, _, err := buildQuery(name, qtype, wantDNSSEC)
	if err != nil {
		return DNSResult{Err: err.Error(), Transport: "doh", Server: url}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(packed))
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "doh", Server: url}
	}
	req.Header.Set("content-type", "application/dns-message")
	req.Header.Set("accept", "application/dns-message")
	req.Header.Set("user-agent", "nabiz")
	resp, err := dohClient.Do(req)
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "doh", Server: url}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return DNSResult{Ms: msSince(start), Err: shortErr(err), Transport: "doh", Server: url}
	}
	if resp.StatusCode != http.StatusOK {
		return DNSResult{Ms: msSince(start), Err: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Transport: "doh", Server: url}
	}
	out := parseResponse(body, 0, false)
	out.Ms = msSince(start)
	out.Transport = "doh"
	out.Server = url
	return out
}

// Query dispatches on the resolver kind.
func Query(ctx context.Context, resolver Resolver, name, qtype string,
	timeout time.Duration, wantDNSSEC bool,
) DNSResult {
	switch resolver.Kind {
	case "system":
		servers := SystemResolvers()
		if len(servers) == 0 {
			return DNSResult{Err: "no system resolver", Transport: "system"}
		}
		out := QueryUDP(ctx, servers[0], name, qtype, timeout, wantDNSSEC)
		out.Transport = "system"
		return out
	case "tcp":
		return QueryTCP(ctx, resolver.Address, name, qtype, timeout, wantDNSSEC)
	case "dot":
		return QueryDoT(ctx, resolver.Address, resolver.Hostname, name, qtype,
			timeout+time.Second, wantDNSSEC)
	case "doh":
		return QueryDoH(ctx, resolver.Address, name, qtype, timeout+2*time.Second, wantDNSSEC)
	default:
		return QueryUDP(ctx, resolver.Address, name, qtype, timeout, wantDNSSEC)
	}
}

// --- environment --------------------------------------------------------

// SystemResolvers reads /etc/resolv.conf.
func SystemResolvers() []string {
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer file.Close()
	var out []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "nameserver" {
			out = append(out, fields[1])
		}
	}
	return out
}

// UpstreamResolvers digs past a local stub to what it really forwards to -
// the place a "my DNS is encrypted" claim usually falls apart.
func UpstreamResolvers() []string {
	servers := SystemResolvers()
	local := false
	for _, server := range servers {
		if strings.HasPrefix(server, "127.") || server == "::1" {
			local = true
		}
	}
	if !local {
		return servers
	}
	out, ok := util.Run(5*time.Second, "resolvectl", "status")
	if !ok {
		return servers
	}
	var found []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Current DNS Server:") && !strings.HasPrefix(line, "DNS Servers:") {
			continue
		}
		_, value, _ := strings.Cut(line, ":")
		for _, token := range strings.Fields(value) {
			token, _, _ = strings.Cut(token, "#")
			host := token
			if h, _, err := net.SplitHostPort(token); err == nil {
				host = h
			}
			if net.ParseIP(host) != nil {
				found = append(found, token)
			}
		}
	}
	found = util.Uniq(found)
	if len(found) == 0 {
		return servers
	}
	return found
}

func msSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}

func shortErr(err error) string {
	text := err.Error()
	if len(text) > 90 {
		text = text[:90]
	}
	return text
}
