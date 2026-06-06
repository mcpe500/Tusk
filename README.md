# Tusk

**Container runtime for Termux that uses a QEMU VM as a replacement for Docker.**

> "Docker can't run on Termux? Okay, let's build our own." — The original idea for this project.

---

## Table of Contents

- [Story](#-story)
- [Architecture](#-architecture)
- [Installation](#-installation)
- [Usage](#-usage)
- [Docker Compose](#-docker-compose)
- [Roadmap](#-roadmap)
- [Troubleshooting](#-troubleshooting)
- [Development](#-development)
- [FAQ](#-faq)

---

## 📖 Story

### The Problem

Docker is the industry standard for containerization. But Docker needs:
- `dockerd` (Linux daemon)
- Linux namespaces (pid, network, mount, etc.)
- Cgroups for resource limiting
- Overlay filesystem

All of these **are not available in Termux/Android**. The Android kernel does not provide the namespace isolation that Docker needs.

### Exploration

Although Docker cannot run, there is an alternative: **QEMU can run on Termux**.

QEMU is an emulator that can fully virtualize x86_64 VMs. With Alpine Linux (very lightweight, ~50MB RAM), we can create a VM that acts as a "container host".

### The Solution: Tusk

Tusk uses a different architecture from Docker:

```
Docker:           Tusk:
┌──────────┐      ┌─────────────────┐    ┌─────────────────┐
│   Host   │      │     Host        │    │     QEMU VM     │
│ (Termux) │      │   (Termux)      │    │   (Alpine)      │
└──────────┘      └────────┬────────┘    └────────┬────────┘
                           │                      │
                           │  tusk CLI             │  tuskd
                           ▼                      ▼
                      ┌─────────────┐        ┌─────────────┐
                      │  socket     │◄──────►│  container  │
                      │  (.tusk/)   │  9p    │  process    │
                      └─────────────┘        └─────────────┘
```

**Bottom line:** VM replaces namespaces for isolation. Heavy? Yes. But it works on Termux.

---

## 🏗️ Architecture

### Main Components

| Component | Location | Function |
|----------|----------|----------|
| `tusk` CLI | `cmd/tusk/` | Command-line interface on the host |
| `tuskd` | `cmd/tuskd/` | Daemon that runs inside the VM |
| VM Manager | `internal/vm/` | Manages the QEMU VM lifecycle |
| Image Store | `internal/image/` | Stores and pulls OCI images |
| Compose | `internal/compose/` | Docker Compose support |

### Data Flow

```
User: $ tusk run alpine echo hello
         │
         ▼
┌────────────────────────────────────────────────────────────┐
│ 1. tusk CLI parse command                                  │
│ 2. Connect to tuskd via socket (.tusk/vms/serial.sock)    │
│ 3. Send JSON-RPC: ContainerCreate                          │
│ 4. tuskd spawn process in Alpine VM                        │
│ 5. Result returned to CLI                                  │
└────────────────────────────────────────────────────────────┘
```

### Communication Protocol

Host ↔ VM uses **JSON-RPC 2.0** over a Unix socket:

```json
// Host → tuskd
{ "jsonrpc": "2.0", "method": "ContainerCreate", "params": { "image": "alpine", "name": "hello" }, "id": 1 }

// tuskd → Host
{ "jsonrpc": "2.0", "result": { "id": "abc123", "pid": 1234 }, "id": 1 }
```

### Socket Files

```
~/.tusk/vm/
├── qmp.sock      # QEMU Machine Protocol (VM control)
└── serial.sock   # JSON-RPC API (container operations)
```

### Image Store Structure

```
~/.tusk/images/
├── blobs/                    # Content-addressable layers
│   └── sha256/
│       ├── abc123...         # Layer tarballs
│       └── def456...         # Config blobs
├── index/                    # Image index (tags → digest)
│   └── manifest.json
└── manifests/               # Manifest per digest
    └── sha256/
        └── ghi789...         # Full manifest
```

---

## 🚀 Installation

### Prerequisites

1. **Termux** - Download from F-Droid (not Play Store)
2. **Storage** - At least 3GB free space
3. **Internet** - For downloading Alpine ISO and Docker images

### Option 1: One-liner Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash
```

This script will:
1. Install dependencies (QEMU, Go, Git)
2. Clone/Update the Tusk repo
3. Build the `tusk` binary for the host
4. Build the `tuskd-amd64` binary for the VM
5. Initialize Tusk storage

### Option 2: Manual Install

```bash
# Install dependencies
pkg update && pkg install -y golang git qemu-system-x86-64 qemu-utils

# Clone repo
git clone https://github.com/mcpe500/Tusk.git ~/Tusk

# Build binaries
cd ~/Tusk
go build -o ~/tusk ./cmd/tusk
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ~/.tusk/tuskd-amd64 ./cmd/tuskd

# Initialize
~/tusk init
```

### VM Setup

```bash
# 1. Create VM disk (2GB qcow2)
./scripts/tusk-vm.sh create

# 2. Install Alpine Linux (interactive)
./scripts/tusk-vm.sh install
# Follow prompts:
#   - Login: root
#   - Run: setup-alpine
#   - Choose 'virt' as disk mode
#   - Use 'sys' install
#   - After installation: poweroff

# 3. Configure Alpine (run inside the VM after install)
./scripts/configure-alpine.sh
# This will:
#   - Setup 9p filesystem mount
#   - Install tuskd auto-start service
#   - Configure serial console

# 4. Reboot Alpine VM
reboot
```

### Verifying the Installation

```bash
# Check tusk version
tusk version

# Check VM status
tusk status
# Expected: VM: Running, tuskd: OK

# Or use the script directly
./scripts/tusk-vm.sh status
```

---

## 🛠️ Usage

### Initialize & Start

```bash
# Initialize Tusk storage
tusk init

# Start VM
tusk start

# Or use the script
./scripts/tusk-vm.sh start

# Check status
tusk status
```

### Image Operations

```bash
# Pull image from Docker Hub
tusk pull alpine:latest
tusk pull nginx:alpine
tusk pull postgres:15

# List downloaded images
tusk images

# Remove image
tusk rmi alpine:latest
```

### Container Operations

```bash
# Run container (interactive)
tusk run -it alpine /bin/sh

# Run container (detached)
tusk run -d --name web nginx

# Run with port forwarding
tusk run -d -p 8080:80 --name web nginx

# Run with volume mount
tusk run -d -v /data:/data --name app alpine

# Run with environment variables
tusk run -d -e MODE=production --name app alpine

# Run simple command
tusk run alpine echo "Hello from Tusk!"
tusk run alpine ls -la

# List containers
tusk ps              # Running only
tusk ps -a           # All containers

# Container logs
tusk logs web

# Execute command in container
tusk exec web ls /var/log

# Stop container
tusk stop web

# Remove container
tusk rm web

# Start stopped container
tusk start web
```

### Flag Reference

| Flag | Description | Example |
|------|-------------|---------|
| `-d, --detach` | Run in background | `tusk run -d nginx` |
| `--name` | Container name | `tusk run --name myapp alpine` |
| `-i, --interactive` | Keep stdin open | `tusk run -i alpine` |
| `-t, --tty` | Allocate pseudo-TTY | `tusk run -it alpine sh` |
| `-e, --env` | Environment variable | `tusk run -e FOO=bar alpine` |
| `-p, --publish` | Port forwarding | `tusk run -p 8080:80 nginx` |
| `-v, --volume` | Volume mount | `tusk run -v /data:/data alpine` |
| `-w, --workdir` | Working directory | `tusk run -w /app alpine` |

### Network Operations

```bash
# List networks
tusk network ls

# Create network
tusk network create mynet

# Inspect network
tusk network inspect mynet
```

### Volume Operations

```bash
# List volumes
tusk volume ls

# Create volume
tusk volume create mydata

# Inspect volume
tusk volume inspect mydata
```

### VM Management

```bash
# Using tusk CLI
tusk vm start
tusk vm stop
tusk vm status
tusk vm attach

# Using script
./scripts/tusk-vm.sh start
./scripts/tusk-vm.sh stop
./scripts/tusk-vm.sh status
./scripts/tusk-vm.sh attach
```

---

## 📦 Docker Compose

### Example docker-compose.yml

```yaml
version: "3.8"

services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    volumes:
      - ./html:/usr/share/nginx/html:ro
    depends_on:
      - api

  api:
    image: node:18-alpine
    command: node server.js
    working_dir: /app
    volumes:
      - ./api:/app
    environment:
      - NODE_ENV=production
      - PORT=3000
    depends_on:
      - db

  db:
    image: postgres:15-alpine
    volumes:
      - db-data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=mydb

volumes:
  db-data:
```

### Compose Commands

```bash
# Start services
tusk compose up              # Start in foreground
tusk compose up -d           # Start in background

# List services
tusk compose ps

# View logs
tusk compose logs             # All services
tusk compose logs web         # Specific service

# Stop services
tusk compose down             # Stop and remove
tusk compose down -v           # Also remove volumes

# Rebuild images
tusk compose build
```

---

## 🔧 Design Decisions

### 1. Single VM Model vs Per-Container VM

**Choice:** Single VM (one VM for all containers)

| Alternative | Advantages | Disadvantages |
|------------|-----------|------------|
| Single VM | Simple, shared networking | Less isolated |
| Per-Container VM | Fully isolated | Heavy (many VMs), slow start |
| **Single VM (chosen)** | ✓ Manageable | ✓ Compatible with Docker workflow |

### 2. OCI Compliance

**Why it matters:**
- Can pull from Docker Hub directly
- Existing tools (docker, podman) can inspect images
- Industry standard

**Implementation:**
- Image manifest follows OCI Image Spec
- Container config follows OCI Runtime Spec
- Layer storage as content-addressable blobs

### 3. 9p Filesystem for Shared Storage

QEMU virtfs with the 9p protocol allows the host and VM to share a filesystem:

```bash
# Host (Termux)
-virtfs local,path=$HOME/.tusk,mount_tag=tusk-data

# Guest (Alpine)
mount -t 9p -o trans=virtio tusk-data /tusk
```

**Advantages:**
- Images stay on the host, no need to copy to the VM
- Container state is persistent
- Volume mounts are straightforward

### 4. virtio-serial for Control Channel

Instead of QMP (which is complex), we use virtio-serial which is simpler:

```bash
-serial unix:$HOME/.tusk/vms/serial.sock
```

**Reason:**
- QMP requires a complex handshake protocol
- Serial is more straightforward for JSON-RPC
- Reliable, low-latency

### 5. User-mode Networking

```bash
-netdev user,id=net0,hostfwd=tcp::8080-:80
```

**Advantages:**
- Does not require root
- Does not require a TAP/TUN device
- Works in the Android environment

**Disadvantages:**
- Containers cannot be accessed from outside directly
- Not as flexible as bridge networking

---

## 🔄 Roadmap

### Phase 1: Core Infrastructure (done)
- [x] Go CLI skeleton
- [x] QEMU VM Manager
- [x] QMP client
- [x] JSON-RPC protocol

### Phase 2: Image Management (done)
- [x] Image store (OCI format)
- [x] Image pull from registry
- [ ] Layer extraction and caching

### Phase 3: Container Runtime (partial)
- [ ] VM with Alpine + tuskd
- [ ] Container creation (runc integration)
- [ ] Container lifecycle (start/stop/rm)
- [ ] Container exec with PTY

### Phase 4: Networking & Storage (stub)
- [ ] Port forwarding
- [ ] Volume mounts (9p)
- [ ] Network isolation

### Phase 5: Compose & Distribution (partial)
- [x] YAML parsing
- [ ] Service orchestration
- [ ] Dependency resolution
- [ ] Image push to registry

---

## ❓ Troubleshooting

### VM cannot start

```bash
# Check QEMU installed
which qemu-system-x86_64

# Check disk exists
ls -la ~/.tusk/vm/disk.qcow2

# Check free storage
df -h $HOME

# View QEMU logs
cat ~/.tusk/vm/qemu.log

# Start with verbose
qemu-system-x86_64 -M pc-i440fx-9.2 -m 512 -nographic \
  -drive file=$HOME/.tusk/vm/disk.qcow2,if=virtio,format=qcow2
```

### tuskd refuses to start in the VM

```bash
# Attach to serial console
./scripts/tusk-vm.sh attach

# Login as root, check:
rc-status              # View service status
ls -la /usr/local/bin/tuskd
cat /etc/init.d/tuskd

# Manual start:
rc-service tuskd start

# Check logs:
cat /var/log/tuskd.log 2>/dev/null || echo "No log file"
```

### tusk CLI cannot connect to the VM

```bash
# Check VM running
./scripts/tusk-vm.sh status

# Check serial socket exists
ls -la ~/.tusk/vm/serial.sock

# Manual test with nc
echo '{"jsonrpc":"2.0","method":"Ping","params":{},"id":1}' | \
  nc -U ~/.tusk/vm/serial.sock

# Expected response: {"jsonrpc":"2.0","result":"pong","id":1}
```

### Image pull fails

```bash
# Check internet
ping -c 1 registry-1.docker.io

# Check storage space
df -h $HOME

# Retry with verbose
tusk pull alpine:latest --debug 2>&1

# Manual download test
curl -I https://registry-1.docker.io/v2/
```

### Container exec fails

```bash
# Check container running
tusk ps

# Check container exists
ls -la ~/.tusk/containers/

# View container logs
tusk logs <container-name>
```

### Alpine install fails

```bash
# Make sure sufficient disk space (at least 2GB)
qemu-img info ~/.tusk/vm/disk.qcow2

# Re-create disk
rm ~/.tusk/vm/disk.qcow2
./scripts/tusk-vm.sh create

# Download Alpine ISO again
rm ~/alpine-virt-3.19.1-x86_64.iso
./scripts/tusk-vm.sh install
```

---

## 🧪 Testing

### Basic Functionality Tests

```bash
# 1. Initialize
./tusk init
# Expected: Creates ~/.tusk/ directory structure

# 2. Check tusk version
./tusk version
# Expected: Show version info

# 3. Check status (no VM)
./tusk status
# Expected: VM: Stopped

# 4. List images (empty)
./tusk images
# Expected: No images found

# 5. tuskd simulation mode (no /tusk dir)
cd /tmp && echo -e "ping\ninfo\nexit" | ./tuskd
# Expected: pong, {"version": ...}, exit
```

### VM Integration Tests

```bash
# 1. Create disk
./scripts/tusk-vm.sh create
# Expected: Creates 2GB qcow2 image

# 2. Install Alpine (manual, interactive)
./scripts/tusk-vm.sh install
# Steps:
#   - Boot Alpine from ISO
#   - Login: root
#   - Run: setup-alpine
#   - Follow prompts, choose 'virt', 'sys'
#   - After install: poweroff

# 3. Configure Alpine
./scripts/tusk-vm.sh start
# Inside VM:
#   ./configure-alpine.sh
# reboot

# 4. Check VM status
./scripts/tusk-vm.sh status
# Expected: VM: Running, tuskd: OK

# 5. Test JSON-RPC manually
echo '{"jsonrpc":"2.0","method":"Ping","params":{},"id":1}' | \
  nc -U ~/.tusk/vm/serial.sock
# Expected: {"jsonrpc":"2.0","result":"pong","id":1}
```

### End-to-End Container Tests

```bash
# 1. Pull image
./tusk pull alpine:latest
# Expected: Download layers, save to ~/.tusk/images/

# 2. Run simple container
./tusk run alpine echo hello
# Expected: Print "hello"

# 3. Run detached container
./tusk run -d --name test alpine sleep 60
./tusk ps
# Expected: Show test container running

# 4. View logs
./tusk logs test
# Expected: Empty (sleep doesn't output)

# 5. Exec in container
./tusk exec test echo "exec works"
# Expected: Print "exec works"

# 6. Stop container
./tusk stop test
./tusk ps -a
# Expected: test in stopped state

# 7. Remove container
./tusk rm test
./tusk ps -a
# Expected: test removed
```

---

## 📁 Directory Structure

```
~/.tusk/                          # Runtime data
├── images/                       # OCI image store
│   ├── blobs/                    # Content-addressable layers
│   │   └── sha256/
│   ├── index/                    # Image index (tags → digest)
│   └── manifests/                # Manifest per digest
├── vm/                           # VM configuration
│   ├── qmp.sock                  # QMP control socket
│   └── serial.sock               # JSON-RPC API socket
├── containers/                   # Container state
│   └── <id>.json
├── state/
│   ├── containers.json           # Container index
│   └── networks.json             # Network state
└── tuskd-amd64                   # Daemon binary for VM

Tusk/                             # Source code
├── cmd/
│   ├── tusk/                     # Host CLI
│   │   └── main.go               # 12 commands available
│   └── tuskd/                    # Guest daemon
│       └── main.go               # JSON-RPC handler
├── internal/
│   ├── vm/                       # QEMU lifecycle
│   │   ├── manager.go            # Start/stop VM
│   │   ├── qmp.go                # QMP protocol client
│   │   └── serial.go             # Serial communication
│   ├── client/                   # JSON-RPC client
│   ├── image/                    # OCI image store + pull
│   ├── container/                # Container runtime + spec
│   ├── compose/                  # YAML parser
│   └── network/                  # Network manager
├── scripts/                      # Helper scripts
│   ├── install.sh                # One-liner installer
│   ├── setup-vm.sh               # VM setup
│   ├── boot-vm.sh                # Boot VM
│   ├── tusk-vm.sh                # VM management
│   └── configure-alpine.sh       # Configure Alpine inside VM
└── pkg/
    ├── types/                    # OCI data types
    ├── protocol/                 # API protocol
    └── util/                     # Utilities
```

---

## 📊 Comparison with Docker

| Aspect | Docker | Tusk |
|-------|--------|------|
| Isolation | Linux namespaces | QEMU VM |
| Startup Time | ~100ms | ~3-5 seconds |
| Memory Overhead | ~10MB | ~50MB |
| Resource Usage | Low | Medium |
| Portability | Linux only | Any platform with QEMU |
| OCI Compatible | Yes | partial |
| Root Required | Usually | No |
| Cgroups | Yes | No (VM-based instead) |

---

## 🔍 FAQ

### Q: Why not use rootless Docker on Termux?

A: Rootless Docker still needs:
- `runc` compiled for Android
- Overlay filesystem support
- User namespace mapping

The Android kernel does not provide these features.

### Q: Why Alpine, and not another distro?

A: Alpine is very lightweight (~5MB base) and designed for containers. Fast boot time and minimal RAM usage - perfect for a QEMU VM on mobile.

### Q: Can I use another distro?

A: Theoretically possible, but Alpine is the best choice because:
- Ultra-lightweight
- Designed for embedded/container
- Simple package manager (apk)
- Many Docker images are based on Alpine

### Q: What is the performance impact?

A: QEMU adds ~50-100MB RAM overhead and ~3-5 seconds of boot time. For development/testing this is acceptable. Production on mobile devices is not yet recommended.

### Q: Multi-arch support?

A: Currently x86_64 only. ARM64 VMs need more resources. Future: might support aarch64 guests.

### Q: Can I run Windows container?

A: No. Tusk is a Linux-only container runtime. Windows containers need Hyper-V or Windows Server - not compatible with QEMU emulation.

---

## 🧑‍💻 Development

### Adding New Commands

Edit `cmd/tusk/main.go`:

```go
case "newcommand":
    if len(os.Args) < 3 {
        fmt.Println("Usage: tusk newcommand <arg>")
        return
    }
    runNewCommand(os.Args[2])
```

### Adding API Methods

1. Define params struct in `pkg/protocol/api.go`
2. Handle in `cmd/tuskd/main.go` (handleConnection function)
3. Add client method in `internal/client/client.go`

### Testing VM Integration

For a full integration test, you need:
1. An Alpine VM image with the tuskd binary
2. QEMU configured for serial + 9p
3. Network connectivity

### Building

```bash
# Build host CLI
go build -o tusk ./cmd/tusk

# Build VM daemon (x86_64 Linux)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tuskd-amd64 ./cmd/tuskd

# Build for release
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o tusk ./cmd/tusk
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o tuskd-amd64 ./cmd/tuskd
```

---

## 📜 License

MIT

---

## 🙏 Credits

- Docker for OCI spec inspiration
- Alpine Linux for the minimal base image
- QEMU for virtualization
- Termux for this environment

---

## 📝 Changelog

### v0.1.0 (2026-05-27)
- Initial release
- Basic VM management (QEMU-based)
- Image pull from Docker Hub (OCI format)
- JSON-RPC communication protocol
- Basic container lifecycle (partial)
- Docker Compose YAML parsing
- Alpine VM setup scripts

---

*Last updated: 2026-05-27*
