# Tusk - Overview

## What is Tusk?

**Tusk** is a container runtime for Termux that uses a QEMU VM as a replacement for Docker. With Tusk, you can run container images from Docker Hub in a Termux/Android environment.

> "Docker can't run on Termux? Okay, let's build our own."

## Why Tusk Exists

Docker is the industry standard for containerization, but Docker needs:
- `dockerd` (Linux daemon)
- Linux namespaces (pid, network, mount, etc.)
- Cgroups for resource limiting
- Overlay filesystem

**All of these are not available in Termux/Android.**

QEMU is an alternative that can run on Termux. With Alpine Linux (very lightweight, ~50MB RAM), we can create a VM that acts as a "container host".

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Termux (Host)                          │
│                                                             │
│  ┌─────────┐    ┌──────────────────────────────────────┐  │
│  │  tusk   │───►│  QEMU VM (Alpine x86_64, TCG)          │  │
│  │   CLI   │    │                                        │  │
│  └─────────┘    │  ┌─────────────────────────────────┐   │  │
│     │           │  │         tuskd (daemon)           │   │  │
│     │           │  │  - Container management           │   │  │
│     │           │  │  - Image extraction              │   │  │
│     │           │  │  - Network (Dnsmasq)             │   │  │
│     │           │  └─────────────────────────────────┘   │  │
│     │           └──────────────────────────────────────────┘  │
│     │                        ▲                              │
│     │  9p mount              │                              │
│     │  ~/.tusk/              │ virtio-serial                │
│     ▼  (shared storage)       │ (.tusk/vms/serial.sock)     │
│  ~/.tusk/                                                   │
└─────────────────────────────────────────────────────────────┘
```

### Main Components

| Component | Function |
|----------|--------|
| `tusk` CLI | Command-line interface on the host |
| `tuskd` | Daemon that runs inside the VM |
| QEMU VM | Isolation via virtualization |
| Image Store | Stores and pulls OCI images |

## Comparison with Docker

| Aspect | Docker | Tusk |
|-------|--------|------|
| Isolation | Linux namespaces | QEMU VM |
| Startup Time | ~100ms | ~3-5 seconds |
| Memory Overhead | ~10MB | ~50MB |
| Resource Usage | Low | Medium |
| Portability | Linux only | Any platform with QEMU |
| OCI Compatible | Yes | partial |

## Features

- done: Pull images from Docker Hub
- partial: Run containers
- partial: Docker Compose support
- partial: OCI-compliant image format
- stub: Port forwarding
- stub: Volume mounts
- stub: Container exec

## License

MIT

---

*Back to [docs](./README.md)*
