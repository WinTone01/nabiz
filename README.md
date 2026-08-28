<div align="center">

<img src="docs/logo.svg" alt="Nabız" width="700">

**Find *where* your connection breaks, *how long* it lasts, *whose fault* it is —
and *which change actually helped*.**

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Bubble Tea](https://img.shields.io/badge/TUI-Bubble%20Tea-FF62B6)](https://github.com/charmbracelet/bubbletea)
[![Mouse](https://img.shields.io/badge/mouse-clickable-b79cff)](#the-interface)
[![Platform](https://img.shields.io/badge/platform-Linux-informational)](#)
[![No root](https://img.shields.io/badge/root-not%20required-5fdf90)](#no-root-needed)
[![i18n](https://img.shields.io/badge/i18n-EN%20%C2%B7%20TR-61d4ec)](#language)
[![License](https://img.shields.io/badge/license-GPLv3-blue)](LICENSE)

</div>

---

Speed-test sites say your connection is fine while Discord still drops, the game
still rubber-bands and pages still hang halfway. Nabız fills that gap.

One static binary, no runtime dependencies, no root. It also knows about the two
things that quietly rewrite your networking behind your back:
**[bpftune](https://github.com/oracle/bpftune)**, the kernel auto-tuner, and
**[Unwall](https://github.com/WinTone01/Unwall)** / zapret, the DPI-bypass stack.

## The interface

A real application in the terminal: clickable tabs, buttons, checkboxes and
dialogs — with every action also on a key.

```
  Nabız  0.3.1                          enp3s0 · 6.18.42-lts · unwall ● · bpftune ● · EN
╭───────────────────────────────────────────────────────────────────────────────────────╮
│ 1 Overview  2 Test  3 Layers  4 Kernel  5 bpftune  6 Unwall  7 DNS  8 Monitor  9 Advice│
│───────────────────────────────────────────────────────────────────────────────────────│
│╭───────╮ ╭────────╮ ╭──────────╮ ╭────────────────╮ ╭───────╮ ╭────────╮ ╭────────╮   │
││ ▶ run │ │ ■ stop │ │ ⭳ export │ │ ◎ set baseline │ │ 🌐 EN │ │ ? help │ │ ✕ quit │   │
│╰───────╯ ╰────────╯ ╰──────────╯ ╰────────────────╯ ╰───────╯ ╰────────╯ ╰────────╯   │
│                                                                                       │
│╭────────────────────────────╮ ╭──────────────────────────────────────────────────────╮│
││ Link                       │ │ Live latency                                         ││
││ interface   enp3s0         │ │ target            last   avg   p95   loss            ││
││ speed/mtu   100 Mbit·1500  │ │ Modem / Gateway      1     1     1   0.0% ▅▁█▄▃▅▁    ││
││ link drops  0              │ │ Cloudflare          25    24    25   0.0% ▄▃▇▃▆▅▄    ││
│╰────────────────────────────╯ ╰──────────────────────────────────────────────────────╯│
╰───────────────────────────────────────────────────────────────────────────────────────╯
 r run • s stop • e export • l language • ? help • q quit               score 72.9 (B)
```

| Screen | What it shows |
|---|---|
| **Overview** | Live ping sparklines, link and kernel counters, tool status, verdict from the last run |
| **Test** | Seven suites — `quick` `full` `deep` `dns` `dpi` `path` `load` — with progress, findings, score |
| **Layers** | Physical → TCP counters → per-socket `tcp_info` → netfilter → sysctl |
| **Kernel** | Link drops per kernel release; the regression detector |
| **bpftune** | What it changed, how often, why; per-connection congestion control; a BDP verdict |
| **Unwall** | zapret state, DPI scan, block-type classification, NFQUEUE drops |
| **DNS** | Plain / DoT / DoH side by side, interference checks, system-vs-encrypted diff |
| **Monitor** | Leave it open for hours; every outage timestamped, with an hour-of-day histogram |
| **Advice** | Ranked recommendations — **tick the ones you want and apply them from here** |
| **History** | Score trend across saved runs, against a baseline you pin |
| **Reports** | Browse and export saved runs |

## What it measures

<img src="docs/layers.svg" alt="What Nabız measures, layer by layer" width="100%">

## The thing it is actually good at

<img src="docs/regression.svg" alt="Telling a driver regression from a broken cable" width="100%">

This is not a hypothetical. It is the bug this tool found on the machine it was
written on: a kernel upgrade broke the r8169 link, the kernel blamed the cable on
every relink, and the cable was fine.

## Applying advice — with a safety net

Recommendations are only useful if acting on them is safe. Tick the changes you
want in the **Advice** screen, press Apply, and confirm:

<img src="docs/apply-safety.svg" alt="How applying a change stays reversible" width="100%">

```
╭──────────────────────────────────────────────────────────────────╮
│  Apply 6 changes                                                 │
│                                                                  │
│  A snapshot is written first and an undo script is generated     │
│  from your current values.                                       │
│                                                                  │
│  After applying, connectivity is verified (ping + DNS + TLS).    │
│  If it fails, everything is rolled back automatically.           │
│                                                                  │
│  • bbr — Try the bbr congestion control on a lossy link          │
│  • dns-leak — plaintext resolvers are still in reserve           │
│  • hostlist-prune — autohostlist has grown to 964 domains        │
│                                                                  │
│  ~/.local/share/nabiz/snapshots/20260828-043651                  │
│                                                                  │
│  ╭───────╮ ╭────────╮                                            │
│  │ Apply │ │ Cancel │                                            │
│  ╰───────╯ ╰────────╯                                            │
╰──────────────────────────────────────────────────────────────────╯
```

The same flow exists on the command line:

```bash
nabiz apply --list          # what can be automated, and the exact commands
nabiz apply --dry-run --safe
nabiz apply --safe
nabiz rollback
```

## Telling block types apart

<img src="docs/dpi-split.svg" alt="Split ClientHello probing" width="100%">

| Observation | Meaning |
|---|---|
| RST after the ClientHello | the DPI injects RST |
| Silence after the ClientHello | a middlebox is dropping |
| TCP never connects | IP/port block |
| Whole fails, split passes | SNI matched in one packet → `multisplit` / `multidisorder` |
| Same IP opens with another SNI | the block is on the SNI, not the address |
| Certificate does not verify | interception / warning page |

QUIC is tested with a real **Version Negotiation** packet: no crypto needed to
learn whether UDP/443 survives the path.

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
nabiz                      # the interface
nabiz doctor               # instant diagnosis, no packets sent, under a second
nabiz quick                # ~45 s general sweep
nabiz full                 # + path, MTU, throughput, bufferbloat
nabiz deep                 # + kernel counters, per-socket TCP state
nabiz monitor -d 2h        # long-running stability monitor
nabiz ab --target bpftune  # the same suite with a component on and off
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
nabiz --lang tr
nabiz --lang en
```

`l` — or the 🌐 button — switches language instantly **and remembers the choice**.
Findings and advice are re-derived, not re-measured: the numbers stay, only the
wording changes. A test asserts that every catalog key exists in both languages and
that their format placeholders match, so a language switch can never crash a render.

## Advice, not tips

Every recommendation is tied to a number this run measured, and the ordering shifts
with the evidence — a proven kernel regression outranks the cable, and a cable fault
outranks every kernel tunable.

Categories: `physical` `queue` `kernel` `bpftune` `dns` `dpi` `isp` `security`
`application` `method`. Each item carries **why** (with the measured number), **how**
(runnable commands), **expected gain**, **risk** and **how to revert**.

```
 ☑ [low] [kernel] Try the bbr congestion control on a lossy link
     The retransmission rate is 2.99%. cubic reads loss as congestion and cuts the
     rate unnecessarily; bbr decides from measured latency and bandwidth instead.
       $ sudo sysctl -w net.ipv4.tcp_congestion_control=bbr
       $ nabiz load
     → On lossy links, upload speed and stability improve
     ! bbr can fill queues on some paths; do not make it permanent without measuring
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
[Glamour](https://github.com/charmbracelet/glamour) ·
[bubblezone](https://github.com/lrstanley/bubblezone) — and nothing else at runtime.

## Layout

```
cmd/nabiz/            CLI and entry point
docs/                 diagrams used by this README
internal/
  probe/              icmp · dns · dnschecks · dpi · path · linkevents · reboots
                      link · netstat · sockdiag · throughput · ipv6
  sysinfo/            bpftune and Unwall integration, service control
  suite/              suites, findings, scoring, advice engine
  apply/              reversible changes, snapshots, connectivity watchdog
  monitor/            long-running monitor, outage detection, event log
  report/             json + markdown, A/B diff, baselines, history
  ui/                 the interface — one file per screen, clickable widgets
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
