package suite

import (
	"fmt"
	"strings"
	"time"

	"github.com/WinTone01/nabiz/internal/config"
	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/probe"
	"github.com/WinTone01/nabiz/internal/sysinfo"
	"github.com/WinTone01/nabiz/internal/util"
)

// The advice engine. Every recommendation must be traceable to a number this
// run actually measured - no generic "optimisation tips". Ordering matters more
// than volume: a cable fault makes every kernel tunable irrelevant, and a
// proven kernel regression outranks the cable, so priorities shift with the
// evidence rather than being fixed per category.
//
// Priority 1 = fix this first, 5 = optional polish.

const (
	catPhysical = "physical"
	catQueue    = "queue"
	catKernel   = "kernel"
	catBpftune  = "bpftune"
	catDNS      = "dns"
	catDPI      = "dpi"
	catISP      = "isp"
	catApp      = "app"
	catSecurity = "security"
	catMethod   = "method"
)

// CategoryLabel translates a category id for display.
func CategoryLabel(category string) string {
	switch category {
	case catPhysical:
		return i18n.T("cat.physical")
	case catQueue:
		return i18n.T("cat.queue")
	case catKernel:
		return i18n.T("cat.kernel")
	case catBpftune:
		return i18n.T("cat.bpftune")
	case catDNS:
		return i18n.T("cat.dns")
	case catDPI:
		return i18n.T("cat.dpi")
	case catISP:
		return i18n.T("cat.isp")
	case catSecurity:
		return i18n.T("cat.security")
	case catMethod:
		return i18n.T("cat.method")
	default:
		return i18n.T("cat.app")
	}
}

type adviceList struct{ items []Advice }

func (a *adviceList) push(item Advice) { a.items = append(a.items, item) }

func t(key string, args ...any) string { return i18n.T(key, args...) }

// GenerateAdvice turns a finished run into a ranked, actionable to-do list.
func GenerateAdvice(result Result, cfg config.Config) []Advice {
	var out adviceList
	link := result.Env.Link
	health := result.Env.TCPHealth
	history := result.Env.LinkLog
	internet := result.InternetLatency()
	gateway := result.GatewayLatency()
	iface := link.Iface
	rtt := 0.0
	if internet != nil {
		rtt = internet.Avg
	}
	kernelDetail := kernelRegressionText(result.Env)
	linkDetail := linkRegressionText(result.Env)
	kernelRegression := kernelDetail != ""

	// --- 1. proven kernel regression ------------------------------------
	if kernelRegression && result.Env.GoodKernel != "" {
		steps := []string{}
		if packages := probe.CachedKernelPackages(result.Env.GoodKernel); len(packages) > 0 {
			steps = append(steps, "sudo pacman -U "+strings.Join(packages, " \\\n              "))
		} else {
			steps = append(steps, t("adv.kernel-downgrade.nocache"))
		}
		steps = append(steps,
			t("adv.kernel-downgrade.reboot"),
			"journalctl -k -b | grep -c 'Link is Down'",
			t("adv.kernel-downgrade.verify"),
			t("adv.kernel-downgrade.retry"))
		out.push(Advice{
			ID: "kernel-downgrade", Priority: 1, Category: catKernel,
			Title:  t("adv.kernel-downgrade.title", result.Env.GoodKernel),
			Why:    t("adv.kernel-downgrade.why", kernelDetail),
			How:    steps,
			Gain:   t("adv.kernel-downgrade.gain"),
			Risk:   t("adv.kernel-downgrade.risk"),
			Revert: []string{"sudo pacman -Syu"},
		})
		out.push(Advice{
			ID: "kernel-report", Priority: 3, Category: catKernel,
			Title: t("adv.kernel-report.title"),
			Why:   t("adv.kernel-report.why", kernelDetail),
			How: []string{
				t("adv.kernel-report.s1"),
				t("adv.kernel-report.s2"),
				t("adv.kernel-report.s3"),
			},
			Gain: t("adv.kernel-report.gain"),
		})
	} else if history.Drops > 3 && linkDetail != "" {
		out.push(Advice{
			ID: "kernel-bisect", Priority: 1, Category: catKernel,
			Title: t("adv.kernel-bisect.title"),
			Why:   t("adv.kernel-bisect.why", linkDetail),
			How: []string{
				t("adv.kernel-bisect.s1"),
				t("adv.kernel-bisect.s2"),
				"journalctl -k -b | grep -c 'Link is Down'",
				t("adv.kernel-bisect.s3"),
			},
			Gain: t("adv.kernel-bisect.gain"),
			Risk: t("adv.kernel-bisect.risk"),
		})
	}

	// --- 2. physical layer -------------------------------------------------
	if history.Drops > 3 || link.CarrierUps > 3 {
		priority := 1
		if kernelRegression {
			// the same cable was quiet for hundreds of hours on the old kernel
			priority = 3
		}
		drops := history.Drops
		if int(link.CarrierUps) > drops {
			drops = int(link.CarrierUps)
		}
		why := t("adv.cable-flap.why", iface, drops)
		if history.SpanMinutes() > 0 {
			why += t("adv.cable-flap.span", history.SpanMinutes(), history.MeanGapMin,
				history.DownSeconds, history.DownPct())
		}
		why += "."
		if history.Downshifts > 0 && !kernelRegression {
			why += t("adv.cable-flap.downshift", history.Downshifts, history.DownshiftNote)
		}
		out.push(Advice{
			ID: "cable-flap", Priority: priority, Category: catPhysical,
			Title: t("adv.cable-flap.title"),
			Why:   why,
			How: []string{
				t("adv.cable-flap.s1"), t("adv.cable-flap.s2"), t("adv.cable-flap.s3"),
				"journalctl -k -b | grep -E 'Link is|Downshift'",
				t("adv.cable-flap.s4"),
			},
			Gain: t("adv.cable-flap.gain"),
		})
		out.push(Advice{
			ID: "modem-port", Priority: priority + 1, Category: catPhysical,
			Title: t("adv.modem-port.title"),
			Why:   t("adv.modem-port.why"),
			How: []string{t("adv.modem-port.s1"), t("adv.modem-port.s2"),
				t("adv.modem-port.s3"), "journalctl -k -b | grep -c 'Link is Down'"},
			Gain: t("adv.modem-port.gain"),
		})
		out.push(Advice{
			ID: "switch-bypass", Priority: 3, Category: catPhysical,
			Title: t("adv.switch-bypass.title"),
			Why:   t("adv.switch-bypass.why"),
			How:   []string{t("adv.switch-bypass.s1"), t("adv.switch-bypass.s2")},
			Gain:  t("adv.switch-bypass.gain"),
		})
	}
	if history.Downshifts > 3 && !kernelRegression {
		out.push(Advice{
			ID: "pin-100full", Priority: 2, Category: catPhysical,
			Title: t("adv.pin-100full.title"),
			Why:   t("adv.pin-100full.why", history.Downshifts),
			How: []string{
				"sudo ethtool -s " + iface + " autoneg on advertise 0x008",
				t("adv.pin-100full.s1"),
				t("adv.pin-100full.s2"),
			},
			Gain:   t("adv.pin-100full.gain"),
			Risk:   t("adv.pin-100full.risk"),
			Revert: []string{"sudo ethtool -s " + iface + " autoneg on advertise 0x03f"},
		})
	}
	if result.Env.EEE.Active && (history.Drops > 3 || link.CarrierUps > 3) {
		out.push(Advice{
			ID: "eee-off", Priority: 3, Category: catPhysical,
			Title: t("adv.eee-off.title"),
			Why:   t("adv.eee-off.why"),
			How: []string{
				"sudo ethtool --set-eee " + iface + " eee off",
				t("adv.eee-off.s1"), t("adv.eee-off.s2"),
			},
			Gain:   t("adv.eee-off.gain"),
			Risk:   t("adv.eee-off.risk"),
			Revert: []string{"sudo ethtool --set-eee " + iface + " eee on"},
		})
		if !result.Env.ASPM.Blocked {
			out.push(Advice{
				ID: "aspm", Priority: 4, Category: catPhysical,
				Title: t("adv.aspm.title"),
				Why:   t("adv.aspm.why"),
				How:   []string{t("adv.aspm.s1"), t("adv.aspm.s2")},
				Gain:  t("adv.aspm.gain"),
				Risk:  t("adv.aspm.risk"),
			})
		}
	}
	// Nothing else on this list can be judged across a reboot while the journal
	// dies with the boot, so this comes before the experiments it makes readable.
	if journal := result.Env.Journal; !journal.Persistent &&
		(history.Drops > 0 || link.CarrierUps > 1) {
		out.push(Advice{
			ID: "journal-persist", Priority: 1, Category: catPhysical,
			Title: t("adv.journal-persist.title"),
			Why:   t("adv.journal-persist.why", journal.Boots),
			How: []string{
				"sudo mkdir -p /var/log/journal",
				"sudo systemd-tmpfiles --create --prefix /var/log/journal",
				"sudo systemctl restart systemd-journald",
				t("adv.journal-persist.s1"),
			},
			Gain:   t("adv.journal-persist.gain"),
			Risk:   t("adv.journal-persist.risk"),
			Revert: []string{"sudo rm -rf /var/log/journal && sudo systemctl restart systemd-journald"},
		})
	}
	// When the kernel itself reported that it could not disable ASPM, this stops
	// being a guess and outranks the other power-saving advice. Note that
	// pcie_aspm=off is the wrong answer here: it tells the kernel to keep its
	// hands off ASPM, which leaves the firmware's setting - the one the driver
	// objected to - in place. Only pcie_aspm=force hands control over so the
	// driver's own disable call can succeed.
	if aspm := result.Env.ASPM; aspm.Blocked && !aspm.Forced() {
		out.push(Advice{
			ID: "aspm-force", Priority: 1, Category: catPhysical,
			Title: t("adv.aspm-force.title"),
			Why:   t("adv.aspm-force.why", aspm.Driver, aspm.Slot),
			How: []string{
				t("adv.aspm-force.s1"),
				"sudo sed -i \"s/^GRUB_CMDLINE_LINUX_DEFAULT='/&pcie_aspm=force /\" /etc/default/grub",
				"sudo grub-mkconfig -o /boot/grub/grub.cfg",
				t("adv.aspm-force.s2"),
				"journalctl -k -b | grep -c \"can't disable ASPM\"",
			},
			Gain:   t("adv.aspm-force.gain"),
			Risk:   t("adv.aspm-force.risk"),
			Revert: []string{t("adv.aspm-force.revert")},
		})
	}
	if link.SpeedMbit > 0 && link.SpeedMbit <= 100 && !link.Wireless {
		out.push(Advice{
			ID: "link-speed", Priority: 3, Category: catPhysical,
			Title: t("adv.link-speed.title", link.SpeedMbit),
			Why:   t("adv.link-speed.why"),
			How: []string{
				"ethtool " + iface, t("adv.link-speed.s1"),
				t("adv.link-speed.s2"), t("adv.link-speed.s3"),
			},
			Gain: t("adv.link-speed.gain"),
		})
	}
	if link.Duplex != "" && link.Duplex != "full" {
		out.push(Advice{
			ID: "duplex", Priority: 1, Category: catPhysical,
			Title: t("adv.duplex.title"), Why: t("adv.duplex.why"),
			How:  []string{"sudo ethtool -s " + iface + " autoneg on", t("adv.duplex.s1")},
			Gain: t("adv.duplex.gain"),
		})
	}
	if count := link.Stats["rx_crc_errors"]; count > 0 {
		out.push(Advice{
			ID: "crc", Priority: 1, Category: catPhysical,
			Title: t("adv.crc.title"), Why: t("adv.crc.why", count),
			How:  []string{t("adv.crc.s1"), t("adv.crc.s2"), t("adv.crc.s3")},
			Gain: t("adv.crc.gain"),
		})
	}
	if link.Wireless && link.SignalDBm < -70 && link.SignalDBm != 0 {
		out.push(Advice{
			ID: "wifi", Priority: 2, Category: catPhysical,
			Title: t("adv.wifi.title", link.SignalDBm), Why: t("adv.wifi.why"),
			How: []string{t("adv.wifi.s1"), t("adv.wifi.s2"),
				"iw dev " + iface + " scan | grep -E 'SSID|signal|freq'", t("adv.wifi.s3")},
			Gain: t("adv.wifi.gain"),
		})
	}
	if link.Wireless && internet != nil && internet.Jitter > cfg.Thresholds.JitterWarn {
		out.push(Advice{
			ID: "wifi-channel", Priority: 3, Category: catPhysical,
			Title: t("adv.wifi-channel.title"),
			Why:   t("adv.wifi-channel.why", internet.Jitter),
			How: []string{"iw dev " + iface + " scan | grep -E 'freq|signal|SSID'",
				t("adv.wifi-channel.s1")},
			Gain: t("adv.wifi-channel.gain"),
		})
	}
	if gateway != nil && gateway.LossPct >= cfg.Thresholds.LossWarn {
		out.push(Advice{
			ID: "lan-loss", Priority: 1, Category: catPhysical,
			Title: t("adv.lan-loss.title"), Why: t("adv.lan-loss.why", gateway.LossPct),
			How:  []string{t("adv.lan-loss.s1"), t("adv.lan-loss.s2"), "nabiz quick"},
			Gain: t("adv.lan-loss.gain"),
		})
	}

	// --- 3. queue management ------------------------------------------------
	if result.Load != nil {
		bloat := result.Load.WorstDelta()
		upMbps, downMbps := 0.0, 0.0
		if result.Load.Upload != nil {
			upMbps = result.Load.Upload.Bps / 1e6
		}
		if result.Load.Download != nil {
			downMbps = result.Load.Download.Bps / 1e6
		}
		if bloat >= cfg.Thresholds.BloatWarn {
			shapeUp, shapeDown := int(upMbps*0.92), int(downMbps*0.92)
			out.push(Advice{
				ID: "sqm", Priority: 2, Category: catQueue,
				Title: t("adv.sqm.title"),
				Why:   t("adv.sqm.why", bloat, result.Load.Grade),
				How: []string{
					t("adv.sqm.s1"),
					t("adv.sqm.s2", shapeDown, shapeUp),
					t("adv.sqm.s3"),
					fmt.Sprintf("sudo tc qdisc replace dev %s root cake bandwidth %dmbit besteffort",
						iface, maxInt(shapeUp, 1)),
					t("adv.sqm.s4"), "nabiz load",
				},
				Gain:   t("adv.sqm.gain"),
				Risk:   t("adv.sqm.risk"),
				Revert: []string{"sudo tc qdisc replace dev " + iface + " root fq_codel"},
			})
			out.push(Advice{
				ID: "cake-gaming", Priority: 4, Category: catQueue,
				Title: t("adv.cake-gaming.title"),
				Why:   t("adv.cake-gaming.why", result.Load.UpDelta),
				How: []string{
					fmt.Sprintf("sudo tc qdisc replace dev %s root cake bandwidth %dmbit diffserv4",
						iface, maxInt(int(upMbps*0.92), 1)),
					t("adv.cake-gaming.s1"), "nabiz load",
				},
				Gain:   t("adv.cake-gaming.gain"),
				Risk:   t("adv.cake-gaming.risk"),
				Revert: []string{"sudo tc qdisc replace dev " + iface + " root fq_codel"},
			})
		}
		if result.Load.UpDelta >= cfg.Thresholds.BloatWarn &&
			result.Env.Sysctls["net.ipv4.tcp_notsent_lowat"] == "-1" {
			out.push(Advice{
				ID: "notsent-lowat", Priority: 4, Category: catKernel,
				Title: t("adv.notsent-lowat.title"),
				Why:   t("adv.notsent-lowat.why", result.Load.UpDelta),
				How: []string{"sudo sysctl -w net.ipv4.tcp_notsent_lowat=131072",
					t("adv.notsent-lowat.s1"), "nabiz load"},
				Gain:   t("adv.notsent-lowat.gain"),
				Revert: []string{"sudo sysctl -w net.ipv4.tcp_notsent_lowat=-1"},
			})
		}
		if qdisc := result.Env.Sysctls["net.core.default_qdisc"]; qdisc != "" &&
			qdisc != "fq_codel" && qdisc != "cake" && qdisc != "fq" {
			out.push(Advice{
				ID: "default-qdisc", Priority: 3, Category: catQueue,
				Title: t("adv.default-qdisc.title"),
				Why:   t("adv.default-qdisc.why", qdisc),
				How: []string{
					"echo 'net.core.default_qdisc = fq_codel' | sudo tee /etc/sysctl.d/99-nabiz.conf",
					"sudo sysctl --system",
				},
				Gain:   t("adv.default-qdisc.gain"),
				Revert: []string{"sudo rm /etc/sysctl.d/99-nabiz.conf && sudo sysctl --system"},
			})
		}
	}

	// --- 4. kernel tunables ---------------------------------------------------
	cc := result.Env.Sysctls["net.ipv4.tcp_congestion_control"]
	available := result.Env.Sysctls["net.ipv4.tcp_available_congestion_control"]
	if cc == "cubic" && strings.Contains(available, "bbr") &&
		(health.RetransPct > 1 || (internet != nil && internet.LossPct > 0.5)) {
		out.push(Advice{
			ID: "bbr", Priority: 3, Category: catKernel,
			Title: t("adv.bbr.title"), Why: t("adv.bbr.why", health.RetransPct),
			How: []string{
				"sudo sysctl -w net.ipv4.tcp_congestion_control=bbr",
				"nabiz load", t("adv.bbr.s1"), t("adv.bbr.s2"),
			},
			Gain:   t("adv.bbr.gain"),
			Risk:   t("adv.bbr.risk"),
			Revert: []string{"sudo sysctl -w net.ipv4.tcp_congestion_control=cubic"},
		})
	}
	if result.MTU != nil && result.MTU.Blackhole {
		out.push(Advice{
			ID: "mtu-probe", Priority: 2, Category: catKernel,
			Title: t("adv.mtu-probe.title"),
			Why:   t("adv.mtu-probe.why", result.MTU.IfaceMTU, result.MTU.ProbedMTU),
			How: []string{
				"sudo sysctl -w net.ipv4.tcp_mtu_probing=1",
				t("adv.mtu-probe.s1"),
				fmt.Sprintf("sudo ip link set %s mtu %d", iface, result.MTU.ProbedMTU),
			},
			Gain:   t("adv.mtu-probe.gain"),
			Revert: []string{"sudo sysctl -w net.ipv4.tcp_mtu_probing=0"},
		})
	}
	if result.Env.Sysctls["net.ipv4.tcp_slow_start_after_idle"] == "1" {
		out.push(Advice{
			ID: "ssaio", Priority: 5, Category: catKernel,
			Title: t("adv.ssaio.title"), Why: t("adv.ssaio.why"),
			How:    []string{"sudo sysctl -w net.ipv4.tcp_slow_start_after_idle=0"},
			Gain:   t("adv.ssaio.gain"),
			Revert: []string{"sudo sysctl -w net.ipv4.tcp_slow_start_after_idle=1"},
		})
	}
	if conntrack := result.Env.Conntrack; conntrack.Max > 0 &&
		float64(conntrack.Count)/float64(conntrack.Max) > 0.8 {
		ratio := float64(conntrack.Count) / float64(conntrack.Max) * 100
		out.push(Advice{
			ID: "conntrack", Priority: 2, Category: catKernel,
			Title: t("adv.conntrack.title"), Why: t("adv.conntrack.why", ratio),
			How: []string{fmt.Sprintf("sudo sysctl -w net.netfilter.nf_conntrack_max=%d",
				conntrack.Max*2)},
			Gain: t("adv.conntrack.gain"),
		})
	}
	// Gated on what ethtool reports right now. The retransmission rate is
	// cumulative since boot and never falls, so judging by that alone left this
	// recommendation standing for the rest of the uptime after it was applied.
	if health.RetransPct > 2 && gateway != nil && gateway.LossPct < 0.5 &&
		!link.Wireless && link.Offloads.AnyOn() {
		out.push(Advice{
			ID: "nic-offload", Priority: 4, Category: catKernel,
			Title: t("adv.nic-offload.title"), Why: t("adv.nic-offload.why", health.RetransPct),
			How: []string{
				"sudo ethtool -K " + iface + " gro off gso off tso off",
				t("adv.nic-offload.s1"), "nabiz load",
			},
			Gain:   t("adv.nic-offload.gain"),
			Risk:   t("adv.nic-offload.risk"),
			Revert: []string{"sudo ethtool -K " + iface + " gro on gso on tso on"},
		})
	}

	// --- 5. bpftune ---------------------------------------------------------------
	bpftune := result.Env.Bpftune
	if bpftune.Failed {
		steps := []string{}
		if bpftune.Overridden {
			steps = append(steps,
				t("adv.bpftune-repair.s1"),
				"sudo rm -f /etc/systemd/system/bpftune.service.d/99-nabiz-tuners.conf",
				"sudo systemctl daemon-reload")
		}
		steps = append(steps,
			"sudo systemctl reset-failed bpftune && sudo systemctl restart bpftune",
			"systemctl status bpftune")
		out.push(Advice{
			ID: "bpftune-repair", Priority: 1, Category: catBpftune,
			Title: t("adv.bpftune-repair.title"),
			Why:   t("adv.bpftune-repair.why", bpftune.FailDetail),
			How:   steps,
			Gain:  t("adv.bpftune-repair.gain"),
		})
	}
	if bpftune.Installed && bpftune.Running {
		bdp := sysinfo.BDPBytes(link.SpeedMbit, rtt)
		for _, tunable := range bpftune.Tunables {
			if tunable.Key != "net.ipv4.tcp_rmem" || bdp <= 0 {
				continue
			}
			maxBuf := float64(SysctlInt(tunable.Current, 2))
			if maxBuf <= 0 || maxBuf/bdp <= 50 {
				continue
			}
			sane := int64(bdp * 4)
			out.push(Advice{
				ID: "bpftune-buffers", Priority: 3, Category: catBpftune,
				Title: t("adv.bpftune-buffers.title"),
				Why:   t("adv.bpftune-buffers.why", maxBuf/1e6, link.SpeedMbit, rtt, bdp/1024),
				How: []string{
					fmt.Sprintf("sudo sysctl -w net.ipv4.tcp_rmem=\"4096 131072 %d\"", sane),
					t("adv.bpftune-buffers.s1"), "nabiz load",
					t("adv.bpftune-buffers.s2"), "sudo bpftune -R",
				},
				Gain:   t("adv.bpftune-buffers.gain"),
				Risk:   t("adv.bpftune-buffers.risk"),
				Revert: []string{"sudo systemctl restart bpftune"},
			})
		}
		// One growth step is tuning; a dozen is a feedback loop with the loss on
		// this line, and pinning the value only holds until the next restart.
		if change, ok := bpftune.ChangedByBpftune("net.ipv4.tcp_rmem"); ok && change.Count >= 5 {
			out.push(Advice{
				ID: "bpftune-tuner-off", Priority: 3, Category: catBpftune,
				Title: t("adv.bpftune-tuner-off.title"),
				Why:   t("adv.bpftune-tuner-off.why", change.Count, change.To),
				How: []string{
					t("adv.bpftune-tuner-off.s1"),
					"sudo mkdir -p /etc/systemd/system/bpftune.service.d",
					"sudo printf '[Service]\\nExecStart=\\nExecStart=/usr/sbin/bpftune -a tcp_conn_tuner\\n' > /etc/systemd/system/bpftune.service.d/99-nabiz-tuners.conf",
					"sudo systemctl daemon-reload && sudo systemctl restart bpftune",
					"nabiz ab --target bpftune",
				},
				Gain:   t("adv.bpftune-tuner-off.gain"),
				Revert: []string{"sudo systemctl revert bpftune.service"},
			})
		}
		if len(bpftune.CCVotes) > 1 {
			var parts []string
			for name, count := range bpftune.CCVotes {
				parts = append(parts, fmt.Sprintf("%s=%d", name, count))
			}
			out.push(Advice{
				ID: "bpftune-cc", Priority: 4, Category: catBpftune,
				Title: t("adv.bpftune-cc.title"),
				Why:   t("adv.bpftune-cc.why", strings.Join(parts, ", ")),
				How: []string{"nabiz ab --target bpftune", t("adv.bpftune-cc.s1"),
					t("adv.bpftune-cc.s2"),
					"sudo sysctl -w net.ipv4.tcp_allowed_congestion_control=\"reno cubic bbr htcp\""},
				Gain: t("adv.bpftune-cc.gain"),
			})
		}
		if health.RetransPct > 2 {
			priority := 2
			if kernelRegression {
				priority = 4
			}
			out.push(Advice{
				ID: "bpftune-vs-physical", Priority: priority, Category: catBpftune,
				Title: t("adv.bpftune-vs-physical.title"),
				Why:   t("adv.bpftune-vs-physical.why", health.RetransPct),
				How: []string{t("adv.bpftune-vs-physical.s1"),
					"sudo bpftune -R && sudo systemctl restart bpftune",
					t("adv.bpftune-vs-physical.s2"), "nabiz deep"},
				Gain: t("adv.bpftune-vs-physical.gain"),
			})
		}
	} else if !bpftune.Installed && result.Load != nil &&
		result.Load.WorstDelta() < 30 && health.RetransPct < 0.5 {
		out.push(Advice{
			ID: "bpftune-not-needed", Priority: 5, Category: catBpftune,
			Title: t("adv.bpftune-not-needed.title"), Why: t("adv.bpftune-not-needed.why"),
			How:  []string{t("adv.bpftune-not-needed.s1")},
			Gain: t("adv.bpftune-not-needed.gain"),
		})
	}

	// --- 6. DNS -----------------------------------------------------------------------
	unwall := result.Env.Unwall
	leaks := probe.PlaintextLeaks(result.Env.DNSPaths)
	if unwall.DNSEncrypted && len(leaks) > 0 {
		// The steps have to name the scope that is actually leaking. Telling
		// someone to clear the global fallback when the addresses came from a
		// DHCP lease on one interface is advice that cannot work, however many
		// times it is followed.
		var described []string
		steps := []string{}
		for _, leak := range leaks {
			if leak.Global() {
				described = append(described, i18n.T("dns.scope.global")+": "+
					strings.Join(leak.Servers, ", "))
				steps = append(steps, "sudo systemctl edit systemd-resolved",
					t("adv.dns-leak.s1"))
				continue
			}
			described = append(described, leak.Link+": "+strings.Join(leak.Servers, ", "))
			steps = append(steps,
				t("adv.dns-leak.s3", leak.Link),
				fmt.Sprintf("sudo nmcli connection modify \"$(nmcli -g GENERAL.CONNECTION device show %s)\" ipv4.ignore-auto-dns yes ipv6.ignore-auto-dns yes", leak.Link),
				fmt.Sprintf("sudo nmcli device reapply %s", leak.Link))
		}
		steps = append(steps, "nabiz dns")
		out.push(Advice{
			ID: "dns-leak", Priority: 2, Category: catDNS,
			Title: t("adv.dns-leak.title"),
			Why:   t("adv.dns-leak.why", strings.Join(described, " · ")),
			How:   steps,
			Gain:  t("adv.dns-leak.gain"),
		})
	}
	if !unwall.DNSEncrypted {
		out.push(Advice{
			ID: "dns-encrypt", Priority: 3, Category: catDNS,
			Title: t("adv.dns-encrypt.title"), Why: t("adv.dns-encrypt.why"),
			How:  []string{t("adv.dns-encrypt.s1"), t("adv.dns-encrypt.s2")},
			Gain: t("adv.dns-encrypt.gain"),
			Risk: t("adv.dns-encrypt.risk"),
		})
	}
	if fastest, fastestMs := fastestResolver(result.DNSBench); fastest != "" {
		out.push(Advice{
			ID: "dns-fastest", Priority: 5, Category: catDNS,
			Title: t("adv.dns-fastest.title", fmt.Sprintf("%s (%.0f ms)", fastest, fastestMs)),
			Why:   t("adv.dns-fastest.why"),
			How:   []string{t("adv.dns-fastest.s1"), t("adv.dns-fastest.s2"), "nabiz dns"},
			Gain:  t("adv.dns-fastest.gain"),
		})
		if fastestMs > 20 && !hasLocalCache(result.DNSBench) {
			out.push(Advice{
				ID: "dns-cache", Priority: 4, Category: catDNS,
				Title: t("adv.dns-cache.title"), Why: t("adv.dns-cache.why", fastestMs),
				How:  []string{t("adv.dns-cache.s1"), t("adv.dns-cache.s2")},
				Gain: t("adv.dns-cache.gain"),
			})
		}
	}
	for _, check := range result.DNSChecks {
		switch {
		case check.Name == "nxdomain-hijack" && check.Verdict == "bad":
			out.push(Advice{
				ID: "dns-hijack", Priority: 2, Category: catDNS,
				Title: t("adv.dns-hijack.title"), Why: check.Detail,
				How:  []string{t("adv.dns-hijack.s1"), t("adv.dns-hijack.s2")},
				Gain: t("adv.dns-hijack.gain"),
			})
		case check.Name == "transparent-dns" && check.Verdict == "bad":
			out.push(Advice{
				ID: "dns-transparent", Priority: 1, Category: catDNS,
				Title: t("adv.dns-transparent.title"), Why: check.Detail,
				How: []string{t("adv.dns-transparent.s1"),
					"unwallctl dns enable quad9 dnscrypt",
					t("adv.dns-transparent.s2"), "nabiz dns"},
				Gain: t("adv.dns-transparent.gain"),
			})
		}
	}

	// --- 7. DPI / zapret -------------------------------------------------------------
	var splitHelps, blocked []string
	for _, verdict := range result.DPI {
		switch verdict.Verdict {
		case "dpi-split-helps":
			splitHelps = append(splitHelps, verdict.Domain)
		case "dpi-hard", "ip-block", "unreachable":
			blocked = append(blocked, verdict.Domain)
		}
	}
	if len(splitHelps) > 0 {
		out.push(Advice{
			ID: "zapret-split", Priority: 2, Category: catDPI,
			Title: t("adv.zapret-split.title"),
			Why:   t("adv.zapret-split.why", joinN(splitHelps, 3)),
			How: []string{t("adv.zapret-split.s1"),
				"unwallctl config set STRATEGY=analiz && unwallctl blockcheck",
				t("adv.zapret-split.s2"), "nabiz dpi"},
			Gain: t("adv.zapret-split.gain"),
		})
	}
	if len(blocked) > 0 && unwall.Running {
		out.push(Advice{
			ID: "zapret-strategy", Priority: 2, Category: catDPI,
			Title: t("adv.zapret-strategy.title"),
			Why:   t("adv.zapret-strategy.why", joinN(blocked, 3)),
			How:   []string{"unwallctl blockcheck", t("adv.zapret-strategy.s1"), "nabiz dpi"},
			Gain:  t("adv.zapret-strategy.gain"),
		})
	}
	notNeeded := stillListed(result.DesyncNotNeeded)
	if len(notNeeded) > 0 {
		out.push(Advice{
			ID: "hostlist-false-positives", Priority: 2, Category: catDPI,
			Title: t("adv.hostlist-fp.title", len(notNeeded)),
			Why:   t("adv.hostlist-fp.why", joinN(notNeeded, 4), len(notNeeded)),
			How: []string{
				t("adv.hostlist-fp.s1"),
				"nabiz apply hostlist-false-positives",
				t("adv.hostlist-fp.s2"),
			},
			Gain:   t("adv.hostlist-fp.gain"),
			Revert: []string{"nabiz rollback"},
		})
	}
	// Entries that no longer resolve cost a lookup on every match and will never
	// be blocked again; they are pure sediment in a list that only ever grows.
	if dead := deadHostlistEntries(result); len(dead) > 0 {
		out.push(Advice{
			ID: "hostlist-dead", Priority: 4, Category: catDPI,
			Title:  t("adv.hostlist-dead.title", len(dead)),
			Why:    t("adv.hostlist-dead.why", joinN(dead, 4)),
			How:    []string{t("adv.hostlist-dead.s1"), "nabiz apply hostlist-dead"},
			Gain:   t("adv.hostlist-dead.gain"),
			Revert: []string{"nabiz rollback"},
		})
	}
	if unwall.Running && len(result.DPI) > 0 && len(result.DesyncNotNeeded) == 0 &&
		comparisonIsStale(cfg) &&
		len(splitHelps) == 0 && len(blocked) == 0 && unwall.HostlistN+unwall.AutoHostlistN > 0 {
		out.push(Advice{
			ID: "unwall-verify", Priority: 4, Category: catDPI,
			Title: t("adv.unwall-verify.title"),
			Why:   t("adv.unwall-verify.why", len(result.DPI)),
			How:   []string{"nabiz ab --target unwall", t("adv.unwall-verify.s1")},
			Gain:  t("adv.unwall-verify.gain"),
		})
	}
	if nfqDropCount(result) > 0 {
		out.push(Advice{
			ID: "nfqueue-qlen", Priority: 2, Category: catDPI,
			Title: t("adv.nfqueue-qlen.title"),
			Why:   t("adv.nfqueue-qlen.why", nfqDropCount(result)),
			How: []string{
				"sudo sysctl -w net.core.rmem_max=8388608",
				t("adv.nfqueue-qlen.s1"),
				t("adv.nfqueue-qlen.s2"),
			},
			Gain: t("adv.nfqueue-qlen.gain"),
		})
	}
	if unwall.AutoHostlistN > 300 {
		out.push(Advice{
			ID: "hostlist-prune", Priority: 3, Category: catDPI,
			Title: t("adv.hostlist-prune.title", unwall.AutoHostlistN),
			Why:   t("adv.hostlist-prune.why"),
			How: []string{
				"sudo cp /etc/unwall/autohostlist.txt /etc/unwall/autohostlist.bak",
				"sudo truncate -s0 /etc/unwall/autohostlist.txt",
				"sudo systemctl restart unwall",
				t("adv.hostlist-prune.s1"),
			},
			Gain:   t("adv.hostlist-prune.gain"),
			Revert: []string{"sudo cp /etc/unwall/autohostlist.bak /etc/unwall/autohostlist.txt"},
		})
		if unwall.AutoHostlistN > 800 {
			out.push(Advice{
				ID: "hostlist-manual", Priority: 4, Category: catDPI,
				Title: t("adv.hostlist-manual.title"),
				Why:   t("adv.hostlist-manual.why", unwall.AutoHostlistN),
				How: []string{t("adv.hostlist-manual.s1"),
					"unwallctl config set HOSTLIST_MODE=manual",
					"sudo systemctl restart unwall"},
				Gain:   t("adv.hostlist-manual.gain"),
				Revert: []string{"unwallctl config set HOSTLIST_MODE=auto"},
			})
		}
	}
	if unwall.GatewayMode {
		out.push(Advice{
			ID: "gateway-off", Priority: 4, Category: catDPI,
			Title: t("adv.gateway-off.title"), Why: t("adv.gateway-off.why"),
			How:  []string{"unwallctl config set GATEWAY_MODE=0", "sudo systemctl restart unwall"},
			Gain: t("adv.gateway-off.gain"),
		})
	}
	var nfqDrops int64
	for _, queue := range result.Env.NFQueue.Queues {
		nfqDrops += queue.QueueDropped + queue.UserDropped
	}
	if nfqDrops > 0 {
		out.push(Advice{
			ID: "nfqueue-drops", Priority: 1, Category: catDPI,
			Title: t("adv.nfqueue-drops.title"), Why: t("adv.nfqueue-drops.why", nfqDrops),
			How: []string{t("adv.nfqueue-drops.s1"), t("adv.nfqueue-drops.s2"),
				t("adv.nfqueue-drops.s3")},
			Gain: t("adv.nfqueue-drops.gain"),
		})
	}
	quicBlocked, quicTotal := 0, 0
	for _, verdict := range result.DPI {
		if verdict.QUIC == "" {
			continue
		}
		quicTotal++
		if !strings.HasPrefix(verdict.QUIC, "ok") {
			quicBlocked++
		}
	}
	if quicTotal > 0 && quicBlocked == quicTotal {
		out.push(Advice{
			ID: "quic", Priority: 3, Category: catApp,
			Title: t("adv.quic.title"), Why: t("adv.quic.why"),
			How: []string{t("adv.quic.s1"),
				"unwallctl config set PORTS_UDP=50000-50100",
				"sudo systemctl restart unwall && nabiz dpi", t("adv.quic.s2")},
			Gain: t("adv.quic.gain"),
		})
	}

	// --- 8. firewall / ipv6 --------------------------------------------------------------
	if result.Env.Firewall.BlockedFrag > 0 {
		out.push(Advice{
			ID: "ufw-icmp", Priority: 2, Category: catSecurity,
			Title: t("adv.ufw-icmp.title"),
			Why:   t("adv.ufw-icmp.why", result.Env.Firewall.BlockedFrag),
			How:   []string{t("adv.ufw-icmp.s1"), t("adv.ufw-icmp.s2")},
			Gain:  t("adv.ufw-icmp.gain"),
		})
	}
	switch {
	case result.Env.IPv6.Broken():
		out.push(Advice{
			ID: "ipv6-broken", Priority: 2, Category: catApp,
			Title: t("adv.ipv6-broken.title"), Why: t("adv.ipv6-broken.why"),
			How:  []string{t("adv.ipv6-broken.s1"), t("adv.ipv6-broken.s2")},
			Gain: t("adv.ipv6-broken.gain"),
		})
	case !result.Env.IPv6.HasAddress && result.Env.IPv6.DNSHasAAAA:
		out.push(Advice{
			ID: "ipv6-missing", Priority: 5, Category: catApp,
			Title: t("adv.ipv6-missing.title"), Why: t("adv.ipv6-missing.why"),
			How: []string{t("adv.ipv6-missing.s1")},
		})
	}

	// --- 9. ISP ------------------------------------------------------------------------------
	if hop, ok := firstLossyHop(result); ok {
		out.push(Advice{
			ID: "isp-evidence", Priority: 2, Category: catISP,
			Title: t("adv.isp-evidence.title"), Why: t("adv.isp-evidence.why", hop.TTL, hop.IP),
			How: []string{"nabiz full --md report.md", t("adv.isp-evidence.s1"),
				t("adv.isp-evidence.s2"), "nabiz monitor -d 24h"},
			Gain: t("adv.isp-evidence.gain"),
		})
	}
	if internet != nil && internet.Avg > cfg.Thresholds.RTTBad {
		out.push(Advice{
			ID: "high-rtt", Priority: 4, Category: catISP,
			Title: t("adv.high-rtt.title", internet.Avg), Why: t("adv.high-rtt.why"),
			How: []string{"nabiz path", t("adv.high-rtt.s1"), t("adv.high-rtt.s2")},
		})
	}

	// --- 10. method ----------------------------------------------------------------------------
	if result.Score >= 80 && result.Baseline == nil {
		out.push(Advice{
			ID: "baseline", Priority: 5, Category: catMethod,
			Title: t("adv.baseline.title"), Why: t("adv.baseline.why", result.Score),
			How:  []string{t("adv.baseline.s1")},
			Gain: t("adv.baseline.gain"),
		})
	}
	if history.Drops > 0 || (internet != nil && internet.LossPct > 0) {
		out.push(Advice{
			ID: "monitor-long", Priority: 4, Category: catMethod,
			Title: t("adv.monitor-long.title"), Why: t("adv.monitor-long.why"),
			How:  []string{"nabiz monitor -d 4h", t("adv.monitor-long.s1")},
			Gain: t("adv.monitor-long.gain"),
		})
	}
	// Once a baseline exists this is no longer advice, it is the workflow the
	// tool is already running: every later result carries a delta against it.
	if result.Baseline == nil {
		out.push(Advice{
			ID: "measure-first", Priority: 5, Category: catMethod,
			Title: t("adv.measure-first.title"), Why: t("adv.measure-first.why"),
			How: []string{
				t("adv.measure-first.s0"),
				"nabiz load --baseline",
				t("adv.measure-first.s2"),
				t("adv.measure-first.s3"),
				"nabiz load",
			},
			Gain: t("adv.measure-first.gain"),
		})
	}

	SortAdvice(out.items)
	return out.items
}

// deadHostlistEntries are hostlist names the DNS probe could not resolve.
//
// The scan is measurement data and survives a re-derivation, so the list has to
// be intersected with the hostlist as it stands now. Otherwise removing the
// entries leaves the recommendation in place, describing names that are no
// longer there.
func deadHostlistEntries(result Result) []string {
	var out []string
	for _, verdict := range result.DPI {
		if verdict.Verdict == "dns-fail" {
			out = append(out, strings.ToLower(verdict.Domain))
		}
	}
	return stillListed(out)
}

// stillListed keeps only the names the hostlists actually contain.
func stillListed(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	listed := map[string]bool{}
	for _, domain := range sysinfo.UnwallDomains(8192) {
		listed[strings.ToLower(strings.TrimPrefix(domain, "."))] = true
	}
	var out []string
	for _, name := range names {
		if listed[name] || listed[strings.TrimPrefix(name, "www.")] {
			out = append(out, name)
		}
	}
	return out
}

// comparisonIsStale reports whether it has been long enough since the last A/B
// for asking again to be useful rather than noise.
func comparisonIsStale(cfg config.Config) bool {
	if cfg.LastComparison == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, cfg.LastComparison)
	if err != nil {
		return true
	}
	return time.Since(last) > 7*24*time.Hour
}

func nfqDropCount(result Result) int64 {
	var drops int64
	for _, queue := range result.Env.NFQueue.Queues {
		drops += queue.QueueDropped + queue.UserDropped
	}
	return drops
}

func fastestResolver(rows []DNSRow) (string, float64) {
	best, bestMs := "", -1.0
	for _, row := range rows {
		// loopback stubs and the system resolver answer from cache in
		// microseconds; ranking them would rank the cache, not the resolver
		if row.Failures > 0 || row.AvgMs <= 0 || row.Kind == "system" ||
			strings.HasPrefix(row.Address, "127.") {
			continue
		}
		if bestMs < 0 || row.AvgMs < bestMs {
			bestMs, best = row.AvgMs, row.Label
		}
	}
	return best, bestMs
}

func hasLocalCache(rows []DNSRow) bool {
	for _, row := range rows {
		if strings.HasPrefix(row.Address, "127.") && row.Failures < row.Queries {
			return true
		}
	}
	return false
}

type hopLike struct {
	TTL int
	IP  string
}

func firstLossyHop(result Result) (hopLike, bool) {
	if len(result.Hops) < 2 {
		return hopLike{}, false
	}
	if result.Hops[len(result.Hops)-1].LossPct() < 20 {
		return hopLike{}, false
	}
	for index, candidate := range result.Hops[:len(result.Hops)-1] {
		if candidate.LossPct() < 20 || candidate.IP == "" {
			continue
		}
		allBad := true
		for _, later := range result.Hops[index+1:] {
			if later.LossPct() < 20 {
				allBad = false
				break
			}
		}
		if allBad {
			return hopLike{TTL: candidate.TTL, IP: candidate.IP}, true
		}
	}
	return hopLike{}, false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// DomainsWithoutDesync returns the hostlist domains that opened cleanly in a
// run made with the bypass engine stopped.
//
// A hostlist grows by guessing: the engine adds a name the first time a
// connection to it fails, and never revisits that guess. Comparing a scan taken
// with the engine off against the list is the only evidence that separates a
// name that needs desync from one that was added on a bad day.
func DomainsWithoutDesync(withoutEngine Result) []string {
	if withoutEngine.Env.Unwall.Running {
		return nil // the engine was up, so this run proves nothing
	}
	listed := map[string]bool{}
	for _, domain := range sysinfo.UnwallDomains(4096) {
		listed[strings.ToLower(strings.TrimPrefix(domain, "."))] = true
	}
	if len(listed) == 0 {
		return nil
	}
	var out []string
	for _, verdict := range withoutEngine.DPI {
		if verdict.Verdict != "clean" {
			continue
		}
		name := strings.ToLower(verdict.Domain)
		switch {
		case listed[name]:
			out = append(out, name)
		default:
			// hostlists hold registrable names while probes use hostnames
			if trimmed := strings.TrimPrefix(name, "www."); listed[trimmed] {
				out = append(out, trimmed)
			}
		}
	}
	return util.Uniq(out)
}
