# Tusk

**Container runtime untuk Termux yang memanfaatkan QEMU VM sebagai pengganti Docker.**

> "Docker tidak bisa jalan di Termux? Oke, kita bikin sendiri." — Ide awal proyek ini.

---

## Table of Contents

- [Cerita / Story](#-cerita--story)
- [Arsitektur](#-arsitektur)
- [Instalasi](#-instalasi)
- [Penggunaan](#-penggunaan)
- [Docker Compose](#-docker-compose)
- [Roadmap](#-roadmap)
- [Troubleshooting](#-troubleshooting)
- [Development](#-development)
- [FAQ](#-faq)

---

## 📖 Cerita / Story

### Masalah

Docker adalah standar industri untuk containerization. Tapi Docker butuh:
- `dockerd` (Linux daemon)
- Linux namespaces (pid, network, mount, dll)
- Cgroups untuk resource limiting
- Overlay filesystem

Semuanya **tidak tersedia di Termux/Android**. Kernel Android tidak menyediakan namespace isolasi yang dibutuhkan Docker.

### Eksplorasi

Meskipun Docker tidak bisa jalan, ada alternatif: **QEMU bisa berjalan di Termux**.

QEMU adalah emulator yang bisa virtualisasi x86_64 VM secara full. Dengan Alpine Linux (sangat ringan, ~50MB RAM), kita bisa membuatVM yang bertindak sebagai "container host".

### Solusi: Tusk

Tusk menggunakan arsitektur berbeda dari Docker:

```
Docker:           Tusk:
┌──────────┐      ┌─────────────────┐    ┌─────────────────┐
│   Host   │      │     Host        │    │     QEMU VM     │
│ (Termux) │      │   (Termux)      │    │   (Alpine)      │
└──────────┘      └────────┬─────────┘    └────────┬─────────┘
                          │                      │
                          │  tusk CLI             │  tuskd
                          ▼                      ▼
                     ┌─────────────┐        ┌─────────────┐
                     │  socket     │◄──────►│  container  │
                     │  (.tusk/)   │  9p    │  process    │
                     └─────────────┘        └─────────────┘
```

**Intinya:** VM替代 namespace untuk isolasi. Heavy? Ya. Tapi jalan di Termux.

---

## 🏗️ Arsitektur

### Komponen Utama

| Komponen | Lokasi | Fungsi |
|----------|--------|--------|
| `tusk` CLI | `cmd/tusk/` | Command-line interface di host |
| `tuskd` | `cmd/tuskd/` | Daemon yang jalan di dalam VM |
| VM Manager | `internal/vm/` | Kelola lifecycle QEMU VM |
| Image Store | `internal/image/` | Simpan dan pull OCI images |
| Compose | `internal/compose/` | Docker Compose support |

### Data Flow

```
User: $ tusk run alpine echo hello
         │
         ▼
┌────────────────────────────────────────────────────────────┐
│ 1. tusk CLI parse command                                  │
│ 2. Connect ke tuskd via socket (.tusk/vms/serial.sock)    │
│ 3. Kirim JSON-RPC: ContainerCreate                         │
│ 4. tuskd spawn process di Alpine VM                        │
│ 5. Result kembali ke CLI                                   │
└────────────────────────────────────────────────────────────┘
```

### Communication Protocol

Host ↔ VM menggunakan **JSON-RPC 2.0** via Unix socket:

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

## 🚀 Instalasi

### Prerequisites

1. **Termux** - Download dari F-Droid (bukan Play Store)
2. **Storage** - Minimal 3GB free space
3. **Internet** - Untuk download Alpine ISO dan Docker images

### Opsi 1: One-liner Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash
```

Script ini akan:
1. Install dependencies (QEMU, Go, Git)
2. Clone/Update Tusk repo
3. Build `tusk` binary untuk host
4. Build `tuskd-amd64` binary untuk VM
5. Initialize Tusk storage

### Opsi 2: Manual Install

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

### Setup VM

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

# 3. Configure Alpine (jalankan di dalam VM setelah install)
./scripts/configure-alpine.sh
# Ini akan:
#   - Setup 9p filesystem mount
#   - Install tuskd auto-start service
#   - Configure serial console

# 4. Reboot Alpine VM
reboot
```

### Verifikasi Installation

```bash
# Check tusk version
tusk version

# Check VM status
tusk status
# Expected: VM: Running, tuskd: OK

# Or use script directly
./scripts/tusk-vm.sh status
```

---

## 🛠️ Penggunaan

### Initialize & Start

```bash
# Initialize Tusk storage
tusk init

# Start VM
tusk start

# Or use script
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

## 🔧 Keputusan Desain

### 1. Single VM Model vs Per-Container VM

**Pilihan:** Single VM (satu VM untuk semua container)

| Alternatif | Kelebihan | Kekurangan |
|------------|-----------|------------|
| Single VM | Simpel, shared networking | Kurang terisolasi |
| Per-Container VM | Terisolasi sepenuhnya | Heavy (banyak VM), lambat start |
| **Single VM (chosen)** | ✓ Manageable | ✓ Compatible dengan Docker workflow |

### 2. OCI Compliance

**Mengapa penting:**
- Bisa pull dari Docker Hub langsung
- Tools existing (docker, podman) bisa inspect images
- Industry standard

**Implementasi:**
- Image manifest mengikuti OCI Image Spec
- Container config mengikuti OCI Runtime Spec
- Layer storage sebagai content-addressable blobs

### 3. 9p Filesystem untuk Shared Storage

QEMU virtfs dengan 9p protocol memungkinkan host dan VM berbagi filesystem:

```bash
# Host (Termux)
-virtfs local,path=$HOME/.tusk,mount_tag=tusk-data

# Guest (Alpine)
mount -t 9p -o trans=virtio tusk-data /tusk
```

**Keuntungan:**
- Images tinggal di host, tidak perlu copy ke VM
- State container persisten
- Volume mount straightforward

### 4. virtio-serial untuk Control Channel

Daripada QMP (yang complex), kita gunakan virtio-serial yang lebih simpel:

```bash
-serial unix:$HOME/.tusk/vms/serial.sock
```

**Alasan:**
- QMP butuh handshake protocol yang kompleks
- Serial lebih straightforward untuk JSON-RPC
- Reliable, low-latency

### 5. User-mode Networking

```bash
-netdev user,id=net0,hostfwd=tcp::8080-:80
```

**Keuntungan:**
- Tidak butuh root
- Tidak butuh TAP/TUN device
- Bekerja di Android environment

**Kekurangan:**
- Container tidak bisa diakses dari luar secara langsung
- Tidak se-fleksibel bridge networking

---

## 🔄 Roadmap

### Phase 1: Core Infrastructure ✅
- [x] Go CLI skeleton
- [x] QEMU VM Manager
- [x] QMP client
- [x] JSON-RPC protocol

### Phase 2: Image Management ✅
- [x] Image store (OCI format)
- [x] Image pull from registry
- [ ] Layer extraction and caching

### Phase 3: Container Runtime (In Progress)
- [ ] VM with Alpine + tuskd
- [ ] Container creation (runc integration)
- [ ] Container lifecycle (start/stop/rm)
- [ ] Container exec with PTY

### Phase 4: Networking & Storage
- [ ] Port forwarding
- [ ] Volume mounts (9p)
- [ ] Network isolation

### Phase 5: Compose & Distribution
- [x] YAML parsing
- [ ] Service orchestration
- [ ] Dependency resolution
- [ ] Image push to registry

---

## ❓ Troubleshooting

### VM tidak bisa start

```bash
# Check QEMU installed
which qemu-system-x86_64

# Check disk exists
ls -la ~/.tusk/vm/disk.qcow2

# Check free storage
df -h $HOME

# View QEMU logs
cat ~/.tusk/vm/qemu.log

# Start dengan verbose
qemu-system-x86_64 -M pc-i440fx-9.2 -m 512 -nographic \
  -drive file=$HOME/.tusk/vm/disk.qcow2,if=virtio,format=qcow2
```

### tuskd tidak mau start di VM

```bash
# Attach ke serial console
./scripts/tusk-vm.sh attach

# Login sebagai root, check:
rc-status              # Lihat service status
ls -la /usr/local/bin/tuskd
cat /etc/init.d/tuskd

# Manual start:
rc-service tuskd start

# Check logs:
cat /var/log/tuskd.log 2>/dev/null || echo "No log file"
```

### tusk CLI tidak connect ke VM

```bash
# Check VM running
./scripts/tusk-vm.sh status

# Check serial socket exists
ls -la ~/.tusk/vm/serial.sock

# Manual test dengan nc
echo '{"jsonrpc":"2.0","method":"Ping","params":{},"id":1}' | \
  nc -U ~/.tusk/vm/serial.sock

# Expected response: {"jsonrpc":"2.0","result":"pong","id":1}
```

### Image pull gagal

```bash
# Check internet
ping -c 1 registry-1.docker.io

# Check storage space
df -h $HOME

# Retry dengan verbose
tusk pull alpine:latest --debug 2>&1

# Manual download test
curl -I https://registry-1.docker.io/v2/
```

### Container tidak bisa exec

```bash
# Check container running
tusk ps

# Check container exists
ls -la ~/.tusk/containers/

# View container logs
tusk logs <container-name>
```

### Alpine install gagal

```bash
# Pastikan足够的 disk space (minimal 2GB)
qemu-img info ~/.tusk/vm/disk.qcow2

# Re-create disk
rm ~/.tusk/vm/disk.qcow2
./scripts/tusk-vm.sh create

# Download Alpine ISO lagi
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

## 📁 Struktur Direktori

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
│   │   └── main.go               # 12 commands implemented
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
│   └── network/                 # Network manager
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

## 📊 Perbandingan dengan Docker

| Aspek | Docker | Tusk |
|-------|--------|------|
| Isolation | Linux namespaces | QEMU VM |
| Startup Time | ~100ms | ~3-5 detik |
| Memory Overhead | ~10MB | ~50MB |
| Resource Usage | Low | Medium |
| Portability | Linux only | Any platform dengan QEMU |
| OCI Compatible | Yes | Partial |
| Root Required | Usually | No |
| Cgroups | Yes | No (VM-based instead) |

---

## 🔍 FAQ

### Q: Kenapa gak pake rootless Docker di Termux?

A: Rootless Docker tetap butuh:
- `runc` yang compile untuk Android
- Overlay filesystem support
- User namespace mapping

Kernel Android tidak menyediakan fitur-fitur ini.

### Q: Kenapa Alpine, bukan distro lain?

A: Alpine sangat ringan (~5MB base) dan designed untuk container. Boot time cepat dan RAM usage minimal - sempurna untuk QEMU VM di mobile.

### Q: Bisa pake distro lain?

A:理论上 bisa, tapi Alpine adalah pilihan terbaik karena:
- Ultra-lightweight
- Desain untuk embedded/container
- Package manager sederhana (apk)
- Banyak Docker images based on Alpine

### Q: Performance impact gimana?

A: QEMU adds ~50-100MB RAM overhead dan ~3-5 detik boot time. Untuk development/testing ini acceptable. Production di mobile设备 belum direkomendasikan.

### Q: Multi-arch support?

A: Currently x86_64 only. ARM64 VMs butuh lebih banyak resources. Future: mungkin support aarch64 guests.

### Q: Bisa run Windows container?

A: Tidak. Tusk is Linux-only container runtime. Windows containers butuh Hyper-V atau Windows Server - tidak compatible dengan QEMU emulation.

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

1. Define params struct di `pkg/protocol/api.go`
2. Handle di `cmd/tuskd/main.go` (handleConnection function)
3. Add client method di `internal/client/client.go`

### Testing VM Integration

Untuk full integration test, butuh:
1. Alpine VM image dengan tuskd binary
2. QEMU configured untuk serial + 9p
3. Network connectivity

### Building

```bash
# Build host CLI
go build -o tusk ./cmd/tusk

# Build VM daemon (x86_64 Linux)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tuskd-amd64 ./cmd/tuskd

# Build untuk release
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o tusk ./cmd/tusk
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o tuskd-amd64 ./cmd/tuskd
```

---

## 📜 License

MIT

---

## 🙏 Credits

- Docker untuk OCI spec inspiration
- Alpine Linux untuk minimal base image
- QEMU untuk virtualization
- Termux untuk environment ini

---

## 📝 Changelog

### v0.1.0 (2026-05-27)
- Initial release
- Basic VM management (QEMU-based)
- Image pull from Docker Hub (OCI format)
- JSON-RPC communication protocol
- Basic container lifecycle (stub)
- Docker Compose YAML parsing
- Alpine VM setup scripts

---

*Last updated: 2026-05-27*
