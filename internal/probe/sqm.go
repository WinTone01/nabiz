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

// Shaping the interface only fixes half of bufferbloat. Upload queues in our own
// transmit path, so a qdisc on the interface controls it directly. Download
// queues one hop upstream - inside the modem, which receives from the WAN faster
// than it can hand over to us - and no local qdisc sits in front of that queue.
// The way round it is an IFB device: incoming packets are redirected through it
// and shaped there, which drops and ECN-marks early enough that the senders slow
// down and the modem's queue drains. Without the ingress half, the direction that
// hurts most on an asymmetric line is left untouched.

// SQMState describes the queue discipline actually in force on a link.
type SQMState struct {
	Iface string `json:"iface"`
	// CakeAvailable and IFBAvailable say whether the recipe can be offered at all.
	CakeAvailable bool `json:"cake_available"`
	IFBAvailable  bool `json:"ifb_available"`
	// Egress is the root qdisc kind on the interface, Ingress the one on its IFB.
	Egress  string `json:"egress,omitempty"`
	Ingress string `json:"ingress,omitempty"`
	// Shaped rates in Mbit; zero means unlimited, which is not shaping at all.
	EgressMbit  int `json:"egress_mbit"`
	IngressMbit int `json:"ingress_mbit"`
	// Persistent is true when something re-applies the shaping after a relink.
	Persistent bool   `json:"persistent"`
	IFBName    string `json:"ifb_name,omitempty"`
}

// EgressShaped reports a rate-limited qdisc on the interface itself.
func (s SQMState) EgressShaped() bool { return s.Egress == "cake" && s.EgressMbit > 0 }

// IngressShaped reports that download is being shaped through an IFB device.
func (s SQMState) IngressShaped() bool { return s.Ingress == "cake" && s.IngressMbit > 0 }

// Possible reports whether both halves of the recipe could be set up here.
func (s SQMState) Possible() bool { return s.CakeAvailable && s.IFBAvailable }

// IFBFor names the IFB device for an interface. Interface names are capped at
// 15 characters by the kernel, so the prefix has to give way on long names
// rather than produce a name that cannot be created.
func IFBFor(iface string) string {
	name := "ifb-" + iface
	if len(name) > 15 {
		name = name[:15]
	}
	return name
}

var (
	qdiscKindRe = regexp.MustCompile(`^qdisc (\S+)`)
	bandwidthRe = regexp.MustCompile(`bandwidth (\d+)([KMG])bit`)
)

// ReadSQM reports the shaping in force for iface, and whether more is possible.
func ReadSQM(iface string) SQMState {
	if iface == "" {
		iface, _ = util.DefaultRoute()
	}
	state := SQMState{Iface: iface}
	if iface == "" {
		return state
	}
	state.IFBName = IFBFor(iface)
	state.CakeAvailable = moduleAvailable("sch_cake")
	state.IFBAvailable = moduleAvailable("ifb")

	state.Egress, state.EgressMbit = rootQdisc(iface)
	state.Ingress, state.IngressMbit = rootQdisc(state.IFBName)
	state.Persistent = shapingIsReapplied()
	return state
}

// rootQdisc returns the kind and shaped rate of a device's root qdisc.
func rootQdisc(device string) (string, int) {
	out, ok := util.Run(4*time.Second, "tc", "qdisc", "show", "dev", device)
	if !ok {
		return "", 0
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, " root ") {
			continue
		}
		match := qdiscKindRe.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		return match[1], parseBandwidthMbit(line)
	}
	return "", 0
}

// parseBandwidthMbit reads cake's "bandwidth 78Mbit" back as a number. A qdisc
// with no bandwidth is running unlimited, which shapes nothing.
func parseBandwidthMbit(line string) int {
	match := bandwidthRe.FindStringSubmatch(line)
	if match == nil {
		return 0
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	switch match[2] {
	case "K":
		return value / 1000
	case "G":
		return value * 1000
	}
	return value
}

// moduleAvailable reports whether a kernel module can be loaded, either because
// it is built in and already present or because its object file is installed.
func moduleAvailable(name string) bool {
	if _, err := os.Stat(filepath.Join("/sys/module", name)); err == nil {
		return true
	}
	if util.Which("modinfo") == "" {
		return false
	}
	_, ok := util.Run(4*time.Second, "modinfo", "-n", name)
	return ok
}

// shapingIsReapplied looks for something that puts the qdisc back after a
// relink. `tc` alone does not survive one, and this link may renegotiate often.
func shapingIsReapplied() bool {
	for _, path := range []string{
		"/etc/NetworkManager/dispatcher.d",
		"/etc/networkd-dispatcher/routable.d",
	} {
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			body, err := os.ReadFile(filepath.Join(path, entry.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(string(body), "cake") && strings.Contains(string(body), "tc qdisc") {
				return true
			}
		}
	}
	if body, err := os.ReadFile("/etc/systemd/system/nabiz-sqm.service"); err == nil {
		return strings.Contains(string(body), "cake")
	}
	return false
}
