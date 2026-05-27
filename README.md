# Tusk

**Container runtime untuk Termux yang memanfaatkan QEMU VM sebagai pengganti Docker.**

> "Docker tidak bisa jalan di Termux? Oke, kita bikin sendiri." — Ide awal proyek ini.

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
│ 2. Connect ke tuskd via socket (.tusk/vms/serial.sock)     │
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
└── state/
    ├── containers.json           # Container index
    └── networks.json             # Network state

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
│   ├── compose/                 # YAML parser
│   └── network/                 # Network manager
├── scripts/                     # Helper scripts
│   ├── install.sh                # One-liner installer
│   ├── setup-vm.sh             # VM setup
│   ├── boot-vm.sh               # Boot VM
│   ├── tusk-vm.sh               # VM management
│   └── configure-alpine.sh       # Configure Alpine inside VM
└── pkg/
    ├── types/                    # OCI data types
    ├── protocol/                 # API protocol
    └── util/                     # Utilities
```

---

## 🚀 Quick Start dengan VM

### 1. Install Script (One-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash
```

### 2. Setup VM

```bash
# Create VM disk
./scripts/tusk-vm.sh create

# Install Alpine (interactive - follow prompts)
./scripts/tusk-vm.sh install

# Login as root, then run:
./scripts/configure-alpine.sh

# Reboot
reboot
```

### 3. Start & Use

```bash
# Start VM
./scripts/tusk-vm.sh start

# Or use tusk CLI
tusk init
tusk start

# Run container!
tusk pull alpine:latest
tusk run alpine echo "Hello from Tusk!"
```

---

## 🛠️ Penggunaan

### Setup

```bash
# Initialize
tusk init

# Start VM
tusk start
```

### Container Operations

```bash
# Pull image
tusk pull alpine:latest

# Run container
tusk run alpine echo hello
tusk run -d --name web nginx
tusk run -p 8080:80 -v /data:/app nginx

# Manage containers
tusk ps
tusk logs web
tusk exec web ls /app
tusk stop web
tusk rm web
```

### Docker Compose

```yaml
# docker-compose.yml
version: "3.8"
services:
  web:
    image: nginx
    ports:
      - "8080:80"
  db:
    image: postgres
    volumes:
      - db-data:/var/lib/postgresql

volumes:
  db-data:
```

```bash
tusk compose up
tusk compose ps
tusk compose down
```

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

## 📊 Perbandingan dengan Docker

| Aspek | Docker | Tusk |
|-------|--------|------|
| Isolation | Linux namespaces | QEMU VM |
| Startup Time | ~100ms | ~3-5 detik |
| Memory Overhead | ~10MB | ~50MB |
| Resource Usage | Low | Medium |
| Portability | Linux only | Any platform dengan QEMU |
| OCI Compatible | Yes | Partial |

---

## 🧪 Testing

```bash
# Basic functionality
./tusk init                    # Initialize storage
./tusk status                  # Check VM status
./tusk images                 # List images

# Simulation mode (no VM needed)
echo -e "ping\ninfo\nexit" | ./tuskd

# Try VM start (but won't be fully functional)
./tusk start
```

---

## 📝 Notes untuk Developer

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

*Last updated: 2026-05-27*