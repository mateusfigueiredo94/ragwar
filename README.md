# RagWar

```
  ____          _      ____             _
 / ___|   _ ___| |__  | __ )  ___  __ _| |_
 \___ \  | '_/ _ \ '_ \ |  _ \ / _ \/ _` | __|
  ___) | | | |  __/ |_) || |_) |  __/ (_| | |_
 |____/  |_|  \___|____/ |____/ \___|\__,_|\__|

  Distributed DDoS attack system for Ragnarok Online private servers
```

**Strike at their peak hours with surgical precision and minimal cost.**

| | |
|---|---|
| **License** | [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE) |
| **Go Version** | [![Go Report Card](https://goreportcard.com/badge/github.com/mateusfigueiredo94/ragwar)](https://goreportcard.com/report/github.com/mateusfigueiredo94/ragwar) |

---

## Overview

Ragnarok private servers run on modest VPS (2–4 vCPU, 1–5 Gbps) using Hercules/RagServer emulators that are **CPU-bound per connection**. Thousands of fake connections can degrade a rival server without needing massive bandwidth.

RagWar automates this with a modular, pluggable attack engine — from application-layer fake clients to transport floods and amplification — all orchestrated from a central controller.

## Quick Start

```bash
# Clone and build
git clone https://github.com/mateusfigueiredo94/ragwar.git
cd ragwar
make build

# Run a UDP flood
./bin/worker attack --target 192.168.1.100:6900 --type udp --pps 50000 --duration 60s

# Run the interactive panel
./bin/worker panel
```

## How It Works

Ragnarok emulators (Hercules, RagServer, eAthena) handle each connected player in a single thread. Every packet sent, every status update, every mob AI tick — all CPU work per connection. This means:

1. **Few connections, high CPU impact** — a few thousand held connections can bring a 2-vCPU server to its knees.
2. **Protocol awareness multiplies the effect** — sending a status update packet forces the emulator to recalculate position, check for items, update buffs. Each packet costs real CPU cycles.
3. **Bandwidth is cheap; CPU is the bottleneck** — a 100Mbps line with a well-tuned attack module can outperform a 1Gbps line with a dumb flood.

RagWar exploits all three principles.

## Attack Modules

### Ready

| Module | Layer | Description |
|--------|-------|-------------|
| **UDP Flood** | L3 | High-rate UDP datagrams with randomized payloads. Each worker gets its own socket for better dedup resistance. |
| **SYN Flood** | L3 | Raw TCP SYN packets filling the backlog queue. Needs root/CAP_NET_RAW. Builds full IP+TCP headers with correct checksums. |
| **Connection Hold** | L4 | Opens and holds thousands of TCP connections (slowloris-style). Forces the emulator to spend CPU per accepted connection. |

### Planned

| Module | Layer | Description |
|--------|-------|-------------|
| **Fake Ragnarok Client** | L7 | Connects to login/char/game ports, sends processing-heavy packets (status updates, movement, item use). The niche differentiator. |
| **NTP Amplification** | L3/4 | NTP monlist amplification. Massive bandwidth from small workers. |
| **DNS Amplification** | L3/4 | DNS recursion amplification. Requires open-resolver targets. |
| **Memcached SSRF** | L3/4 | UDP memcached amplification via SSRF. The king of amplification ratios. |

## Components

### Worker

The attack node. A single Go binary that runs attacks against a target with configurable type, intensity, and duration.

```bash
# List available attack types
./bin/worker types

# Run a UDP flood (50k pps for 60 seconds)
./bin/worker attack --target 192.168.1.100:6900 --type udp --pps 50000 --duration 60s

# Run a SYN flood (needs root)
sudo ./bin/worker attack --target 192.168.1.100:6900 --type syn --pps 100000

# Run a connection hold attack
./bin/worker attack --target 192.168.1.100:6900 --type hold --conns 2000 --duration 120s

# Quiet mode (no live stats)
./bin/worker attack --target 192.168.1.100:6900 --type udp --pps 50000 --duration 60s --quiet

# Custom worker goroutines
./bin/worker attack --target 192.168.1.100:6900 --type udp --pps 50000 --workers 8
```

### Panel

Interactive ASCII terminal UI for managing multiple attacks simultaneously. Built with [tview](https://github.com/rivo/tview).

```bash
./bin/worker panel
```

**Keyboard controls:**

| Key | Action |
|-----|--------|
| `a` | Add a new attack (form) |
| `x` | Stop selected attack |
| `d` | Delete selected history row |
| `q` / `Ctrl+C` | Quit (stops all attacks) |

**What you see:**
- Active attacks with live pps/bps/conns stats
- Total packets, bytes, and errors per attack
- Attack state (running / stopped)
- Running count in the status bar

### Attack Engine

Modular, pluggable attack types registered at runtime via `init()`. Adding a new module is a one-line registration.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        RAGWAR SYSTEM                             │
│                                                                  │
│  ┌──────────────┐    ┌────────────────────┐    ┌──────────────┐  │
│  │   CLI Worker  │    │   Control Panel    │    │  Controller   │  │
│  │  (manual CLI) │    │   (keyboard UI)    │    │  (future)     │  │
│  │              │    │                    │    │               │  │
│  │ • attack cmd │    │ • Live stats       │    │ • REST API    │  │
│  │ • types      │    │ • Keyboard ctrl    │    │ • Dashboard   │  │
│  │ • version    │    │ • Multi-attack     │    │ • Scheduling  │  │
│  └──────┬───────┘    └────────┬───────────┘    └───────┬───────┘  │
│         │                     │                        │           │
│         └─────────────────────┼────────────────────────┘           │
│                               │                                    │
│                    ┌──────────▼──────────┐                         │
│                    │   ATTACK ENGINE     │                         │
│                    │                     │                         │
│                    │  ┌───────────────┐  │                         │
│                    │  │ Fake Ragnarok │  │  ─ L7 App Layer        │
│                    │  │ Client        │  │  ─ login/char/game     │
│                    │  └───────────────┘  │                         │
│                    │  ┌───────────────┐  │                         │
│                    │  │ Connection    │  │  ─ L4 Transport        │
│                    │  │ Hold          │  │  ─ TCP exhaustion      │
│                    │  └───────────────┘  │                         │
│                    │  ┌───────────────┐  │                         │
│                    │  │ UDP / SYN     │  │  ─ L3 Network          │
│                    │  │ Flood         │  │  ─ Transport floods    │
│                    │  └───────────────┘  │                         │
│                    │  ┌───────────────┐  │                         │
│                    │  │ Amplification │  │  ─ NTP/DNS/Memcached   │
│                    │  │               │  │  ─ (future)            │
│                    │  └───────────────┘  │                         │
│                    └─────────────────────┘                         │
│                                                                    │
│              ┌─────────────────────────────────────────┐           │
│              │         WORKER POOL (VPS)                │           │
│              │   Region A  │  Region B  │  Other ASNs   │           │
│              │   (5-10 VPS │   ($5-10/ea)│ IP rotation)  │           │
│              └─────────────────────────────────────────┘           │
└──────────────────────────────────────────────────────────────────┘
```

## Project Structure

```
├── cmd/
│   ├── worker/       # Attack node binary (CLI)
│   └── panel/        # Interactive ASCII control panel
├── internal/
│   └── attack/       # Attack engine (udp, syn, hold)
├── .github/          # GitHub templates & CI config
├── .gitignore        # Ignored files
├── Makefile          # Build targets (build, linux, test, clean)
├── go.mod            # Go module definition
├── go.sum            # Dependency checksums
├── PLAN.md           # Detailed project plan
└── README.md         # This file
```

## Default Ports

| Service | Port |
|---------|------|
| Login   | 5121 |
| Char    | 6121 |
| Game    | 6900 |

> Each server may use different ports — recon is essential.

## Roadmap

| Phase | Feature | Status |
|-------|---------|--------|
| 1 | MVP Worker + Panel | 🟡 In progress |
| 2 | Calibrate on own server | ⏳ Planned |
| 3 | Fake Ragnarok Client + Amplification | ⏳ Planned |
| 4 | Controller + REST API + WebSocket | ⏳ Planned |
| 5 | Web Dashboard | ⏳ Planned |
| 6 | Worker Pool with IP rotation | ⏳ Planned |
| 7 | Docker + CI/CD | ⏳ Planned |
| 8 | Production operation | ⏳ Planned |

## Cost Model

| Item | Cost |
|------|------|
| Per worker VPS | ~$5–10/mo |
| Recommended pool (5–10 workers) | ~$30–100/mo |

## Dependencies

| Package | Purpose |
|---------|---------|
| [rivo/tview](https://github.com/rivo/tview) | Terminal UI framework (panel) |

## License

MIT
