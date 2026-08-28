package sysinfo

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/util"
)

// Unwall (github.com/WinTone01/Unwall) wraps the zapret / zapret2 DPI-bypass
// engines plus encrypted DNS. nabiz works fine without it - everything here
// degrades to "not installed" - but when it is present its state belongs in
// every report: a measurement taken with desync active is a different
// measurement.

const (
	unwallCtl  = "unwallctl"
	unwallUnit = "unwall.service"
	unwallEtc  = "/etc/unwall"
)

// UnwallState mirrors `unwallctl status` plus its DNS and hostlist state.
type UnwallState struct {
	Installed     bool     `json:"installed"`
	Version       string   `json:"version,omitempty"`
	Engine        string   `json:"engine,omitempty"`
	Strategy      string   `json:"strategy,omitempty"`
	HostlistMode  string   `json:"hostlist_mode,omitempty"`
	GatewayMode   bool     `json:"gateway_mode"`
	Running       bool     `json:"running"`
	Enabled       bool     `json:"enabled"`
	PID           int      `json:"pid"`
	NftOK         bool     `json:"nft_ok"`
	EngineReady   bool     `json:"engine_ready"`
	QNum          int      `json:"qnum"`
	PortsTCP      string   `json:"ports_tcp,omitempty"`
	PortsUDP      string   `json:"ports_udp,omitempty"`
	HostlistN     int      `json:"hostlist_count"`
	AutoHostlistN int      `json:"autohostlist_count"`
	DNSBackend    string   `json:"dns_backend,omitempty"`
	DNSProvider   string   `json:"dns_provider,omitempty"`
	DNSEncrypted  bool     `json:"dns_encrypted"`
	DNSServers    []string `json:"dns_servers,omitempty"`
}

// Label renders the state for a status bar.
func (s UnwallState) Label() string {
	switch {
	case !s.Installed:
		return i18n.T("ui.notinstalled")
	case !s.Running:
		return i18n.T("ui.stopped")
	default:
		return s.Engine + " · " + s.Strategy
	}
}

func parseKV(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return out
}

// ReadUnwall queries unwallctl and the config directory.
func ReadUnwall() UnwallState {
	var state UnwallState
	binary := util.Which(unwallCtl)
	if binary == "" {
		if _, err := os.Stat(unwallEtc); err != nil {
			return state
		}
	}
	state.Installed = true
	if binary != "" {
		if out, ok := util.Run(10*time.Second, binary, "status"); ok {
			data := parseKV(out)
			state.Version = data["version"]
			state.Engine = data["engine"]
			state.Strategy = data["strategy"]
			state.HostlistMode = data["hostlist"]
			state.GatewayMode = data["gateway"] == "1"
			state.Running = data["running"] == "1"
			state.Enabled = data["enabled"] == "1"
			state.PID, _ = strconv.Atoi(data["pid"])
			state.NftOK = data["nft"] == "1"
			state.EngineReady = data["engine_ready"] == "1"
		}
		if out, ok := util.Run(10*time.Second, binary, "dns", "status"); ok {
			data := parseKV(out)
			state.DNSBackend = data["backend"]
			state.DNSProvider = data["provider"]
			state.DNSEncrypted = data["encrypted"] == "1"
			state.DNSServers = strings.Fields(data["servers"])
		}
	}
	if !state.Running {
		if out, _ := util.Run(6*time.Second, "systemctl", "is-active", unwallUnit); strings.TrimSpace(out) == "active" {
			state.Running = true
		}
	}
	conf := parseKV(util.ReadText(filepath.Join(unwallEtc, "unwall.conf"), ""))
	state.QNum, _ = strconv.Atoi(conf["QNUM"])
	state.PortsTCP = conf["PORTS_TCP"]
	state.PortsUDP = conf["PORTS_UDP"]
	state.HostlistN = countEntries(filepath.Join(unwallEtc, "hostlist.txt"))
	state.AutoHostlistN = countEntries(filepath.Join(unwallEtc, "autohostlist.txt"))
	return state
}

func countEntries(path string) int {
	count := 0
	for _, line := range strings.Split(util.ReadText(path, ""), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}

// UnwallDomains pulls real hostnames out of the Unwall hostlists so the DPI
// probes test exactly what the user routes through zapret.
func UnwallDomains(limit int) []string {
	var found []string
	for _, name := range []string{"hostlist.txt", "autohostlist.txt"} {
		for _, line := range strings.Split(util.ReadText(filepath.Join(unwallEtc, name), ""), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
			if strings.ContainsAny(line, "/ ") {
				continue
			}
			host := strings.TrimPrefix(line, ".")
			if !strings.Contains(host, ".") {
				continue
			}
			found = append(found, host)
			if len(found) >= limit {
				return util.Uniq(found)
			}
		}
	}
	return util.Uniq(found)
}

// AssessUnwall judges the DPI-bypass setup itself.
func AssessUnwall(state UnwallState, nfqDrops int64, nfqReadable bool) []Note {
	var notes []Note
	if !state.Installed {
		return notes
	}
	note := func(level, key string, args ...any) {
		notes = append(notes, Note{Level: level, Key: key, Source: "unwall",
			Text: i18n.T("fnd."+key+".title", args...),
			Hint: hintOf("fnd." + key + ".hint")})
	}
	if state.Running && !state.EngineReady {
		note("bad", "unwall-engine")
	}
	if state.Running && !state.NftOK && util.IsRoot() {
		note("bad", "unwall-nft")
	}
	if nfqReadable && nfqDrops > 0 {
		note("bad", "nfqueue-drop", nfqDrops)
	}
	if state.AutoHostlistN > 300 {
		note("warn", "unwall-autohostlist", state.AutoHostlistN)
	}
	if state.GatewayMode {
		note("info", "unwall-gateway")
	}
	if strings.Contains(state.PortsUDP, "443") {
		note("info", "unwall-quic")
	}
	return notes
}

// UnwallDNSLeak reports plaintext resolvers still configured while encrypted
// DNS is on - systemd-resolved will happily fall back to them.
func UnwallDNSLeak(state UnwallState, upstream []string) *Note {
	if !state.DNSEncrypted {
		return nil
	}
	var plaintext []string
	for _, server := range upstream {
		if !strings.HasPrefix(server, "127.") && !strings.HasPrefix(server, "::1") {
			plaintext = append(plaintext, server)
		}
	}
	if len(plaintext) == 0 {
		return nil
	}
	if len(plaintext) > 3 {
		plaintext = plaintext[:3]
	}
	return &Note{Level: "warn", Key: "dns-leak", Source: "unwall",
		Text: i18n.T("fnd.dns-leak.title", strings.Join(plaintext, ", ")),
		Hint: i18n.T("fnd.dns-leak.hint")}
}

// CanControlUnwall reports whether the A/B screen can drive the service.
func CanControlUnwall() bool {
	return util.Which(unwallCtl) != "" && privileged([]string{"true"}) != nil
}

// SetUnwall starts or stops the zapret engine.
func SetUnwall(running bool) (bool, string) {
	binary := util.Which(unwallCtl)
	if binary == "" {
		return false, "unwallctl bulunamadı"
	}
	action := "stop"
	if running {
		action = "start"
	}
	command := privileged([]string{binary, action})
	if command == nil {
		return false, "yetki yok / no privilege escalation available"
	}
	out, ok := util.Run(40*time.Second, command[0], command[1:]...)
	if !ok {
		return false, strings.TrimSpace(out)
	}
	return true, "ok"
}
