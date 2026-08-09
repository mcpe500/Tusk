# M5 Podman Bridge Traceability

`internal/engine/forward.go` owns no container semantics. It checks the
selected adapter, optionally starts it, forwards the original argv and streams,
and returns the remote exit code. `cmd/tusk/main.go` supports both `tusk podman`
and invocation as `podman`/`podman-compose`, plus status and shim help.

| Requirement | Evidence | Status |
|---|---|---|
| `ARC-001..006`, `CLI-001..009` | `ForwardPodman`, `runPodman` | Implemented; backend acceptance pending |
| `CLI-012..014` | `PodmanShimHelp`, status/report paths | Implemented |
| `API-001..007` | proot `_ping` bridge and QEMU SSH Unix-socket bridge | Implemented; guest validation pending |
| `VM-002`, `ENG-025..026` | auto-start flag and exact recovery command | Implemented |
| `AT-003..005`, `AT-018` | real selected adapter forwarding | **Pending**: no qualified backend on current host |

TTY commands fail explicitly until a PTY transport is provided; they are not
reported as supported from a non-TTY test double.
