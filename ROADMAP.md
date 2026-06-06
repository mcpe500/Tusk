# Roadmap

## Overview

The following is the Tusk development plan. This roadmap will be updated periodically.

## Version 1.0.0 - MVP (Current)

**Goal:** Basic container runtime that can pull and run containers.

### Phase 1: Core Infrastructure (done)

- [x] Go CLI skeleton
- [x] QEMU VM Manager (start/stop/status)
- [x] QMP client for VM control
- [x] Serial socket communication
- [x] JSON-RPC protocol

### Phase 2: Image Management (done)

- [x] OCI image store
- [x] Image pull from Docker Hub
- [x] Blob storage with SHA256 digest
- [x] Manifest parsing

### Phase 3: Container Runtime (partial)

- [ ] VM image with Alpine + tuskd
- [ ] Container creation
- [ ] Container lifecycle (start/stop/rm)
- [ ] Container exec
- [ ] Container logs

### Phase 4: Networking & Storage (stub)

- [ ] Port forwarding
- [ ] Volume mounts (bind)
- [ ] Network isolation

## Version 1.1.0

**Goal:** Full-featured container management.

### Planned Features

- [ ] Container exec with PTY
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
