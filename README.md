# 🎮 RagWar — DDoS Attack System for Ragnarok Private Servers

**RagWar** is a distributed DDoS attack system designed to target competitor Ragnarok Online private servers at low operational cost. It uses a combination of application-layer attacks (fake Ragnarok clients), connection exhaustion, and transport-layer floods to degrade or take down rival game servers.

## 📖 Overview

Ragnarok emulators (Hercules/RagServer) are **CPU-bound per connection** — thousands of fake connections can severely degrade a server without needing massive bandwidth. RagWar automates this with a modular attack system.

## 🏗️ Architecture

```
[CLI Panel / Worker Binary]  ←→  [Attack Engine]
                                    │
                              ┌─────┼─────┐
                        [UDP] [SYN] [HOLD]
```

- **Worker** — single binary that runs attacks against a target (Phase 1 MVP).
- **Panel** — ASCII terminal UI for live attack control (keyboard-driven).
- **Attack Engine** — modular, pluggable attack types.

## ⚔️ Attack Modules

| # | Module | Description | Priority |
|---|--------|-------------|----------|
| 1 | **Fake Ragnarok Client** | Connects to login/char/game ports, keeps connections alive with processing-heavy packets. Most efficient. | 🔥 Highest |
| 2 | **Connection Hold** | Opens and holds thousands of TCP connections (slowloris-style) on game ports. | 🔥 High |
| 3 | **UDP / SYN Flood** | Transport-layer floods on game ports (5121, 6121, 6900). | Medium |
| 4 | **Amplification** | NTP monlist, DNS, Memcached, SSDP for high-bandwidth attacks. | Future |

## 🚀 Quick Start

### Prerequisites
- Go 1.24+
- Linux or macOS (for building)

### Build

```bash
# Build the worker binary
make build

# Cross-compile for Linux VPS workers
make linux

# Run tests
make test
```

### Usage

```bash
# Run the worker
./bin/worker attack --target 192.168.1.100:6900 --type udp --pps 50000 --duration 60s

# Run the interactive panel
./bin/worker panel
```

### CLI Help

```bash
./bin/worker --help
./bin/worker attack --help
./bin/worker types
./bin/worker version
```

## 📋 Roadmap

| Phase | Status | Description |
|-------|--------|-------------|
| 0 | ✅ | Recon — identify competitors, IPs, ports, peak hours |
| 1 | 🟡 | MVP Worker — Go binary with UDP, SYN, HOLD modules + CLI |
| 2 | ⏳ | Calibrate on own server — find the "lethal dose" |
| 3 | ⏳ | Fake Ragnarok Client module + amplification |
| 4 | ⏳ | Controller + web dashboard |
| 5 | ⏳ | Worker pool across multiple VPS with IP rotation |
| 6 | ⏳ | Production operation |

## 💰 Cost Estimate

| Item | Cost |
|------|------|
| Per worker VPS | ~$5-10/mo |
| Recommended pool (5-10 workers) | ~$30-100/mo |

## 📁 Project Structure

```
├── cmd/
│   ├── worker/    # Attack node binary
│   └── panel/     # Interactive ASCII control panel
├── internal/
│   └── attack/    # Attack engine (udp, syn, hold)
├── bin/           # Built binaries (gitignored)
├── Makefile       # Build targets
├── PLAN.md        # Detailed project plan
└── go.mod         # Go module definition
```

## 🛡️ Default Ports

| Service | Port |
|---------|------|
| Login   | 5121 |
| Char    | 6121 |
| Game    | 6900 |

> ⚠️ Each server may use different ports — recon is essential.

## 📜 License

Private project.
