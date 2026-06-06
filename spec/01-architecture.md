# Architecture Specification

## Current Architecture

- Host (`tusk`) in `cmd/tusk/main.go`.
- VM manager in `internal/vm`.
- Container operation runtime through daemon in VM (`cmd/tuskd/main.go`).
- Image store in `internal/image`.
- Compose parsing in `internal/compose`.

## Host → VM Execution Path

1. CLI reads arguments and selects handler.
2. For container/compose/ps/exec/logs/remove/stop operations, CLI contacts socket `~/.tusk/vm/serial.sock` via `internal/client`.
3. Daemon receives JSON-RPC request and stores/runs state in `~/.tusk/containers` or related VM location (`/tusk/containers` on VM host mount).

## Architecture Status per Component

| Component | Implementation | Status | Evidence |
|---|---|---|---|
| VM lifecycle | `internal/vm/manager.go` | done | `Start()`, `Stop()`, `WaitForSerial()`, `WaitForQMP()` |
| VM transport | `internal/vm/serial.go` + `internal/client/client.go` | partial | unix socket connection and basic marshaling, framing and error handling still minimal |
| QMP control | `internal/vm/qmp.go`, `internal/vm/qmp_fd_unix.go` | partial | basic QMP commands available, error handling exists, fd-passing platform-aware |
| Daemon API runtime | `cmd/tuskd/main.go` | partial | listener + switch handler, many methods simulated |
| Image store | `internal/image/store.go` | done | pull manifest/config/layer + persist blobs & manifest |
| Container runtime host-side | `internal/container/runtime.go` | partial | prepare rootfs works, spec generator still placeholder in `SaveSpec` |
| Compose parser | `internal/compose/parser.go` | partial | parse YAML and resolve dependency, execution is only partial |
| Network manager | `internal/network/manager.go` | stub | print log and dummy values |

## Directories and Storage

- Host data: `~/.tusk` (`tuskDir`).
- VM runtime sockets: `~/.tusk/vm/{qmp.sock,serial.sock}` (`internal/vm/manager.go`).
- Image store: `~/.tusk/images/{blobs,index,manifests}` (`internal/image/store.go`).
- Container metadata: `~/.tusk/containers` (daemon `ContainerStore`).
