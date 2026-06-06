# Tusk 🔧

**Tusk: Hardware emulation for Termux, because sometimes working is better than fast.**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

## The Problem

Docker needs Linux namespaces, cgroups, and overlay filesystems — none of which are available on Android/Termux.

## The Solution

Tusk uses **QEMU VMs** as container hosts instead of Linux namespaces. It's heavier than native Docker, but it **actually works** on Termux.

```
Termux (Host)                    QEMU VM (Alpine)
┌─────────────┐                 ┌─────────────────┐
│   tusk CLI   │ ◄── JSON-RPC ──►│     tuskd       │
└─────────────┘      socket      │   (daemon)      │
        │                            │              │
        │  9p filesystem             │              │
        ▼                            ▼              │
┌─────────────────┐        ┌────────────────────┐  │
│   ~/.tusk/      │ ◄─────►│   containers       │  │
│  (shared storage)        │   (processes)      │  │
└─────────────────┘        └────────────────────┘  │
```

## Features

- done: **OCI-compliant** — Pull images from Docker Hub
- partial: **Docker CLI compatible** — `tusk run`, `tusk ps`, `tusk exec`, etc.
- partial: **Docker Compose support** — `tusk compose up`
- done: **QEMU-based isolation** — Works where Docker can't
- done: **Alpine Linux base** — Minimal footprint (~50MB RAM)
- stub: Port forwarding
- stub: Volume mounts

## Quick Start

```bash
# Build
go build -o tusk ./cmd/tusk
go build -o tuskd ./cmd/tuskd

# Initialize
./tusk init

# Start VM
./tusk start

# Pull and run
./tusk pull alpine:latest
./tusk run alpine echo "Hello from Tusk!"
```

## Why QEMU?

| Approach | Works on Termux? | Isolation | Complexity |
|----------|------------------|-----------|------------|
| Native Docker | ❌ No | High | Low |
| Docker-in-Docker | ❌ No | Medium | High |
| **Tusk (QEMU)** | **done** | **High** | **Medium** |
| chroot only | partial | Low | Low |

QEMU with TCG (software emulation) is the only practical solution for full container isolation on Android.

## CLI Commands

```
tusk init              Initialize Tusk
tusk start/stop        VM management
tusk pull <image>      Pull from Docker Hub
tusk run <image>       Run container
tusk ps                List containers
tusk exec <id> <cmd>   Execute in container
tusk compose up/down   Docker Compose
```

## Architecture

- **Host CLI** (`cmd/tusk`) — Command parsing, VM management
- **Guest Daemon** (`cmd/tuskd`) — Container management, JSON-RPC API
- **VM Manager** (`internal/vm`) — QEMU lifecycle, QMP, serial sockets
- **Image Store** (`internal/image`) — OCI-compliant registry client
- **Compose** (`internal/compose`) — docker-compose.yaml parser

## Comparison with Docker

| Metric | Docker | Tusk |
|--------|--------|------|
| Startup Time | ~100ms | ~3-5s |
| Memory Overhead | ~10MB | ~50MB |
| Portability | Linux only | Any with QEMU |
| OCI Compatible | Yes | Yes |

## Documentation

- [Overview](docs/overview.md) — Why Tusk exists
- [Installation](docs/installation.md) — Setup guide
- [Usage](docs/usage/basic-commands.md) — CLI commands
- [Architecture](docs/architecture/system-design.md) — System design
- [Development](docs/development/building.md) — Building from source

## Status

**Alpha** — Proof of concept. Core infrastructure done, container runtime partial.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
