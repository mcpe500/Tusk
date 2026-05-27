# Roadmap

## Overview

Berikut rencana pengembangan Tusk. Roadmap ini akan diupdate secara berkala.

## Version 1.0.0 - MVP (Current)

**Goal:** Basic container runtime yang bisa pull dan run containers.

### Phase 1: Core Infrastructure ✅

- [x] Go CLI skeleton
- [x] QEMU VM Manager (start/stop/status)
- [x] QMP client untuk VM control
- [x] Serial socket communication
- [x] JSON-RPC protocol

### Phase 2: Image Management ✅

- [x] OCI image store
- [x] Image pull dari Docker Hub
- [x] Blob storage dengan SHA256 digest
- [x] Manifest parsing

### Phase 3: Container Runtime (In Progress) 🚧

- [ ] VM image dengan Alpine + tuskd
- [ ] Container creation
- [ ] Container lifecycle (start/stop/rm)
- [ ] Container exec
- [ ] Container logs

### Phase 4: Networking & Storage ⬜

- [ ] Port forwarding
- [ ] Volume mounts (bind)
- [ ] Network isolation

## Version 1.1.0

**Goal:** Full-featured container management.

### Planned Features

- [ ] Container exec dengan PTY
- [ ] Container pause/resume
- [ ] Resource limits (CPU, memory)
- [ ] Container inspect
- [ ] Image layers caching

## Version 1.2.0

**Goal:** Docker Compose support.

### Planned Features

- [ ] Full docker-compose.yaml parsing
- [ ] Multi-container orchestration
- [ ] Service dependency resolution
- [ ] Health checks
- [ ] Restart policies

## Version 1.3.0

**Goal:** Production readiness.

### Planned Features

- [ ] Image push to registry
- [ ] Container checkpoint/restore
- [ ] Multi-arch image support
- [ ] Volume plugins
- [ ] Network plugins (CNI)

## Future Ideas

- [ ] Web UI dashboard
- [ ] Desktop app (Electron/Flutter)
- [ ] Kubernetes compatibility layer
- [ ] Multi-VM support (per-container isolation)

---

## Progress Tracking

Last updated: 2026-05-27