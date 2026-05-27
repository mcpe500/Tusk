# System Design

## High-Level Architecture

Tusk menggunakan arsitektur Client-Server dengan QEMU VM sebagai container host:

```
┌─────────────────────────────────────────────────────────────┐
│                         Termux (Host)                       │
│                                                              │
│    ┌─────────────────────────────────────────────────┐       │
│    │                   tusk CLI                      │       │
│    │  • Command parsing                              │       │
│    │  • VM management                                │       │
│    │  • JSON-RPC client                              │       │
│    └──────────────────────┬──────────────────────────┘       │
│                           │                                   │
│    ┌──────────────────────▼──────────────────────────┐      │
│    │              ~/.tusk/ (shared storage)           │      │
│    │  ┌──────────┐  ┌──────────┐  ┌──────────────┐    │      │
│    │  │  images  │  │  state   │  │  containers  │    │      │
│    │  │  (blobs) │  │          │  │              │    │      │
│    │  └──────────┘  └──────────┘  └──────────────┘    │      │
│    └──────────────────────┬──────────────────────────┘       │
│                           │                                   │
│    ┌──────────────────────▼──────────────────────────┐      │
│    │              QEMU Virtual Machine                │      │
│    │  ┌────────────────────────────────────────────┐  │      │
│    │  │                 Alpine Linux               │  │      │
│    │  │  ┌──────────────────────────────────────┐  │  │      │
│    │  │  │              tuskd (daemon)          │  │  │      │
│    │  │  │  • Container management              │  │  │      │
│    │  │  │  • Image extraction                  │  │  │      │
│    │  │  │  • Network (Dnsmasq)                 │  │  │      │
│    │  │  └──────────────────────────────────────┘  │  │      │
│    │  └────────────────────────────────────────────┘  │      │
│    └─────────────────────────────────────────────────┘       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. tusk CLI (Host)

Command-line interface yang berjalan di Termux.

**Lokasi:** `cmd/tusk/`

**Responsibilities:**
- Parse user commands
- Manage QEMU VM lifecycle
- Communicate with tuskd via serial socket
- Handle local operations (init, images, etc.)

### 2. tuskd (Guest)

Daemon yang berjalan di dalam Alpine VM.

**Lokasi:** `cmd/tuskd/`

**Responsibilities:**
- Handle JSON-RPC requests
- Manage containers (create, start, stop, delete)
- Extract images and build rootfs
- Manage networking
- Report container state

### 3. Image Store

OCI-compliant image storage.

**Lokasi:** `~/.tusk/images/`

```
images/
├── blobs/                    # Content-addressable storage
│   └── sha256/
│       ├── abc123...        # Layer data
│       └── def456...
├── index/                   # Image references by tag
│   └── alpine/
│       └── latest -> sha256:manifestdigest
└── manifests/               # Manifests by digest
    └── sha256:manifest.json
```

### 4. VM Manager

Manages QEMU VM lifecycle.

**Lokasi:** `internal/vm/`

**Features:**
- Start/stop VM
- QMP protocol for VM control
- Serial socket for API communication
- 9p filesystem for shared storage

## Data Flow

### Pull Image

```
tusk pull alpine:latest
         │
         ▼
┌────────────────────────────────────────────────────────────┐
│ 1. Registry authentication (Bearer token)                  │
│ 2. Fetch manifest from Docker Hub                          │
│ 3. Download config blob                                    │
│ 4. Download all layer blobs                                │
│ 5. Store in ~/.tusk/images/                                │
│ 6. Update index with tag reference                        │
└────────────────────────────────────────────────────────────┘
```

### Run Container

```
tusk run alpine echo hello
         │
         ▼
┌────────────────────────────────────────────────────────────┐
│ 1. tusk CLI parse command                                  │
│ 2. Find image in local store                               │
│ 3. Connect to tuskd via ~/.tusk/vm/serial.sock             │
│ 4. Send ContainerCreate RPC:                               │
│    { "image": "alpine", "command": ["echo", "hello"] }    │
│ 5. tuskd:                                                  │
│    a. Extract layers to rootfs                             │
│    b. Generate OCI runtime config                          │
│    c. Create container (runc)                              │
│    d. Start container process                              │
│ 6. Return container ID and PID                             │
│ 7. tusk CLI displays result                               │
└────────────────────────────────────────────────────────────┘
```

## Security Considerations

### Isolation

- Each container runs in Linux namespaces (pid, network, mount, ipc, uts)
- Resource limits via cgroups
- No container can access host directly

### VM Security

- VM provides additional isolation layer
- 9p filesystem is read-only by default for guest
- Network isolation via user-mode networking

### Image Verification

- SHA256 digest verification for all blobs
- Manifest signature verification (future)

## Performance

### Startup Time

| Component | Time |
|-----------|------|
| VM boot | ~3-5 seconds |
| tuskd ready | ~1 second |
| Container start | ~500ms |
| **Total** | **~5-7 seconds** |

### Memory Usage

| Component | Memory |
|-----------|--------|
| QEMU overhead | ~20-30 MB |
| Alpine VM | ~30-50 MB |
| tuskd | ~10 MB |
| Container (minimal) | ~10 MB |
| **Total minimum** | **~70-100 MB** |

---

*Back to [docs](../README.md)*