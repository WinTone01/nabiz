<div align="center">

# Nabız

**Internet stability, measured all the way down.**

A terminal tool that finds *where* your connection breaks, *how long* it lasts,
*whose fault* it is — and *which change actually helped*.

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Bubble Tea](https://img.shields.io/badge/TUI-Bubble%20Tea-FF62B6)](https://github.com/charmbracelet/bubbletea)
[![Platform](https://img.shields.io/badge/platform-Linux-informational)](#)
[![No root](https://img.shields.io/badge/root-not%20required-success)](#no-root-needed)
[![Language](https://img.shields.io/badge/i18n-EN%20%C2%B7%20TR-blue)](#language)
[![License](https://img.shields.io/badge/license-GPLv3-blue)](LICENSE)

</div>

---

Speed-test sites say your connection is fine while Discord still drops, the game
still rubber-bands and pages still hang halfway. Nabız fills that gap.

It is one static binary with no runtime dependencies, it needs no root, and it
knows about the two things that quietly rewrite your networking behind your back:
**[bpftune](https://github.com/oracle/bpftune)**, the kernel auto-tuner, and
**[Unwall](https://github.com/WinTone01/Unwall)** / zapret, the DPI-bypass stack.

## The thing it is actually good at

A counter says the link dropped. Nabız says **whether that is new**:

```
kernel                     boots   hours   measured   drops  per hour
 7.2.0-1-cachyos               3     8.3      8.1 h     105      13.0
 7.1.8-1-cachyos              13   214.5      3.0 h       0       0.0
```

Boot times and kernel versions come from `wtmp`, which survives journal rotation;
the drop counts come from the kernel log. A release that ran 214 hours with zero
drops, next to one dropping every four minutes, is a **driver regression — not a
cable** — even when the kernel prints `check cabling!` on every relink.

That distinction is the difference between buying a cable and rolling back a package.
It is also a real bug this tool found, on the machine it was written on.

## Screens

```
1 Overview  2 Test  3 Layers  4 Kernel  5 bpftune  6 Unwall  7 DNS  8 Monitor  9 Advice  0 History  p Reports  ? Help
```

| Screen | What it shows |
|---|---|
| **Overview** | Live ping sparklines, link and kernel counters, tool status, and the verdict from the last run |
| **Test** | Seven suites — `quick` `full` `deep` `dns` `dpi` `path` `load` — with progress, findings and a score |
| **Layers** | Physical → TCP counters → **per-socket `tcp_info`** → netfilter → sysctl |
| **Kernel** | Link drops per kernel release; the regression detector |
| **bpftune** | What it changed, how often, why; per-connection congestion control; a BDP verdict |
| **Unwall** | zapret state, DPI scan, block-type classification, NFQUEUE drops |
| **DNS** | Plain / DoT / DoH side by side, interference checks, system-vs-encrypted diff |
| **Monitor** | Leave it open for hours; every outage timestamped, with an hour-of-day histogram |
| **Advice** | A ranked, measurement-backed to-do list with the exact commands |
| **History** | Score trend across saved runs, against a baseline you pin |
| **Reports** | Browse and export saved runs |

`r` run · `s` stop · `e` export · `b` baseline · `a` A/B · `l` language · `?` help · `q` quit

## Install

```bash
go install github.com/WinTone01/nabiz/cmd/nabiz@latest
```

or build it:

```bash
git clone https://github.com/WinTone01/nabiz && cd nabiz
go build -o nabiz ./cmd/nabiz
```

### No root needed

ICMP uses Linux's unprivileged `SOCK_DGRAM`/`IPPROTO_ICMP` socket, traceroute uses
`IP_RECVERR`, and socket statistics come from netlink `INET_DIAG`. It works with
neither `ping`, `traceroute`, `mtr` nor `ss` installed. Only the **NFQUEUE counters**
need root; when they are unreadable the tool says so instead of reporting zero.

## Usage

```bash
nabiz                      # interactive interface
nabiz doctor               # instant diagnosis, no packets sent, under a second
nabiz quick                # ~45 s general sweep
nabiz full                 # + path, MTU, throughput, bufferbloat
nabiz deep                 # + kernel counters, per-socket TCP state
nabiz monitor -d 2h        # long-running stability monitor
nabiz ab --target bpftune  # same suite with a component on and off
nabiz advice               # the full advice list from the last run
nabiz history              # score trend
nabiz apply --list         # what can be applied automatically
nabiz rollback             # undo the last applied batch
```

Every run is saved to `~/.local/share/nabiz/runs/`.

## Language

English is the default **and the fallback**. Turkish is selected automatically when
the locale asks for it; anything that is neither resolves to English.

```bash
nabiz --lang tr    # force Turkish
nabiz --lang en    # force English
```

`l` switches language instantly inside the interface and **remembers the choice**.
Findings and advice are re-derived, not re-measured — the numbers stay, only the
wording changes. A test asserts that every catalog key exists in both languages and
that their format placeholders match, so a language switch can never crash a render.

## What it measures

<details>
<summary><b>Physical layer</b> — where most complaints actually end</summary>

Interface speed and duplex, MTU, `carrier_up_count`, `rx_crc_errors` and the drop
counters, EEE state, Wi-Fi signal, `ethtool` driver and advertised modes, qdisc
statistics. Then the kernel log: every link-down/up pair with timestamps, durations
and the PHY's own downshift messages. No packets sent.
</details>

<details>
<summary><b>Latency and loss</b> — separating your house from your ISP</summary>

Simultaneous ICMP to the modem and several internet anchors. Loss percentage,
consecutive loss bursts, RFC 3550 jitter, p50/p95/p99 and MOS. A clean ping to the
modem with loss beyond it means the problem is upstream; loss on the way to the modem
means it is inside your home. The tool writes that conclusion itself.
</details>

<details>
<summary><b>Kernel</b> — what TCP is really doing</summary>

`/proc/net/snmp` and `netstat`: retransmission rate, RTO timeouts, SYN retransmits,
out-of-order packets, spurious RTOs, prune events. Then **netlink INET_DIAG** for
per-socket `tcp_info`: RTT, min_rtt, cwnd, ssthresh, retransmits, delivery rate and
the congestion control algorithm chosen *for that socket*. Everything `ss -ti` shows,
tabled and interpreted.
</details>

<details>
<summary><b>Latency under load</b> — the number that actually matters</summary>

Idle latency, then the same latency while saturating each direction with multiple
streams, graded A+…F. Socket counters are sampled during the transfer: a large cwnd
with inflating RTT is queueing, a small cwnd with steady min_rtt is loss. A 200 Mbps
line with +300 ms under load still breaks calls and games.
</details>

<details>
<summary><b>DNS</b> — including the checks nobody runs</summary>

The same questions over plain UDP, TCP, DoT (853) and DoH (443), with average and p95
per resolver. Then:

- **Transparent redirection** — a documentation address that runs no resolver is asked
  a question; an answer proves UDP/53 is intercepted and changing resolvers cannot help
- **Injection** — more than one answer racing for the same query
- **NXDOMAIN hijack**, **DNSSEC validation**, **UDP↔TCP consistency**, **EDNS0 passage**
- **Leak** — encrypted DNS on while plaintext resolvers stay configured as fallback
</details>

<details>
<summary><b>DPI</b> — telling block types apart</summary>

Per domain: DNS → TCP → **whole ClientHello** → **split at the record header** →
**split across the SNI** → a different SNI to the same IP → QUIC.

| Observation | Meaning |
|---|---|
| RST after the ClientHello | the DPI injects RST |
| Silence after the ClientHello | a middlebox is dropping |
| TCP never connects | IP/port block |
| Whole fails, split passes | SNI matched in one packet → `multisplit`/`multidisorder` |
| Same IP opens with another SNI | the block is on the SNI, not the address |
| Certificate does not verify | interception / warning page |

QUIC is tested with a real **Version Negotiation** packet: no crypto needed to learn
whether UDP/443 survives the path.

> The split probe only manipulates TCP segment boundaries. zapret's fake packets and
> TTL tricks live in the kernel and cannot be reproduced from userspace, so "splitting
> helped" is a hint, not a replacement for `blockcheck`.
</details>

<details>
<summary><b>Path, MTU, IPv6, firewall</b></summary>

Per-hop loss and latency without `traceroute` installed. Real path MTU by DF binary
search, flagged as a **PMTU black hole** when it disagrees with the interface MTU.
IPv6 checked three ways — address, reachability, AAAA records — because *half-working*
IPv6 costs a timeout on every new connection. And a scan for firewall-dropped ICMP,
because blocking `fragmentation needed` breaks PMTU discovery in a way that looks
exactly like an ISP fault.
</details>

## Advice, not tips

Nabız does not hand out generic optimisation advice. Every recommendation is tied to a
number this run measured, and the ordering shifts with the evidence — a proven kernel
regression outranks the cable, and a cable fault outranks every kernel tunable.

Categories: `physical` `queue` `kernel` `bpftune` `dns` `dpi` `isp` `security`
`application` `method`. Each item carries **why** (with the measured number), **how**
(runnable commands), **expected gain**, **risk** and **how to revert**.

```
 P1 [kernel] Roll the kernel back to 7.1.8-1-cachyos
   7.1.8-1-cachyos ran for 214 h with no drops on record; 7.2.0-1-cachyos drops 13.0
   times an hour. The cable, the modem and the settings did not change — only the
   kernel did, and the drops start in the very first session booted on the new one.
     $ sudo pacman -U /var/cache/pacman/pkg/linux-cachyos-7.1.8-1-x86_64_v3.pkg.tar.zst
     • Reboot and use the machine for an hour
     $ journalctl -k -b | grep -c 'Link is Down'
   → The drops stop completely, without touching the cable
   ! An older kernel misses newer hardware support and security fixes
```

## Applying advice — with a safety net

Changing network settings over the connection you are reading this on deserves more
care than `sudo sysctl -w`. `nabiz apply` is built around that:

```bash
nabiz apply --list        # what can be automated, and the exact commands
nabiz apply --dry-run --safe
nabiz apply --safe        # apply everything that cannot interrupt the link
nabiz rollback            # undo the last batch
```

1. **A snapshot is written before anything runs** — every file that will be touched is
   copied, and every current value is captured.
2. **The undo is a plain shell script**, generated from the *actual* previous values,
   stored beside the snapshot. It works without nabiz installed.
3. **The whole batch runs under one privileged invocation** — one password prompt, no
   half-applied state from a cancelled `sudo`.
4. **Connectivity is re-verified afterwards** — ping, DNS and a TLS handshake all have
   to pass. If they do not, the restore script runs **automatically**.
5. Changes that can renegotiate the link are excluded unless you pass `--include-link`.

```
snapshot: ~/.local/share/nabiz/snapshots/20260828-042244
undo at any time with: nabiz rollback  (or run …/restore.sh)
```

## Baselines and trends

Fix something, then prove it:

```bash
nabiz load --baseline    # pin this as the reference
# ...change one thing...
nabiz load               # the result now carries a delta against the baseline
nabiz history            # the trend across every saved run
```

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea) ·
[Bubbles](https://github.com/charmbracelet/bubbles) ·
[Lip Gloss](https://github.com/charmbracelet/lipgloss) ·
[Glamour](https://github.com/charmbracelet/glamour) — and nothing else at runtime.

## Layout

```
cmd/nabiz/            CLI and entry point
internal/
  probe/              icmp · dns · dnschecks · dpi · path · linkevents · reboots
                      link · netstat · sockdiag · throughput · ipv6
  sysinfo/            bpftune and Unwall integration, service control
  suite/              suites, findings, scoring, advice engine
  apply/              reversible changes, snapshots, connectivity watchdog
  monitor/            long-running monitor, outage detection, event log
  report/             json + markdown, A/B diff, baselines, history
  ui/                 Bubble Tea interface — one file per screen
  i18n/               translation engine and catalogs (en/tr)
  stats/ config/ util/
```

## Tests

Network-touching tests are skipped by default:

```bash
go test ./...                 # hermetic, includes the i18n completeness checks
NABIZ_LIVE=1 go test ./...    # with real measurements
```

## License

GPLv3.
