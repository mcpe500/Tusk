# M3 Proot Traceability

The M3 adapter implementation is present in `internal/engine/proot.go` and is
wired into `engine tune` through `NewLocalCandidateFactory`. It uses real
`proot` and optional `qemu-user` command vectors, persistent backend-local data,
explicit shared/runtime bind roots, a local Unix Podman API bridge, Podman
health/readiness checks, and atomic lifecycle state.

| Requirement | Evidence | Status |
|---|---|---|
| `proot-native` / `proot-qemu-user-x86_64` adapter | `ProotAdapter`, `NewLocalCandidateFactory` | Implemented |
| `GST-007`, `GST-008`, `GST-010`, `GST-015` | data bind, command/API execution, Unix socket, writable-storage gate | Implemented; device validation pending |
| `VM-001`, `VM-004`, `VM-010..016` | serialized lifecycle, bounded API wait, graceful stop/escalation, persisted state | Implemented; device validation pending |
| `API-001`, `API-002`, `API-006`, `API-007` | socket endpoint, `_ping`, degraded state, `RecreateAPI` | Implemented; device validation pending |
| `AT-019` | real Podman conformance and benchmark through proot | **Pending**: no proot/rootfs/Podman on current host |

The adapter fails closed when rootfs, executables, storage, API readiness, or
Podman health is unavailable. No test double can create a qualified M3 result.
