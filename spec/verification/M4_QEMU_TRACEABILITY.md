# M4 QEMU Traceability

`internal/engine/qemu.go` implements the architecture-neutral QEMU system
adapter used by the local candidate factory. It pins machine selection from
candidate ID, defaults to multi-threaded TCG, persists system/data image paths,
records PID start identity, exposes QMP through a Unix socket, uses serial logs,
and requires strict SSH host-key verification before guest commands run.

| Requirement | Evidence | Status |
|---|---|---|
| QEMU ARM64/x86_64/i386/ARM32 candidates | `QEMUAdapter`, `newLocalQEMUTarget` | Implemented; environment pending |
| `VM-007`, `VM-008`, `VM-017` | TCG default, machine/option selection | Implemented; release-device validation pending |
| `TRN-003..007` | pinned `KnownHosts`, vector-safe quoted SSH command | Implemented; guest validation pending |
| `GST-007`, `GST-008`, `GST-015` | persistent data image, SSH/Podman health, startup gates | Implemented; guest validation pending |
| `AT-031..033` | conformance and benchmark through initialized guests | **Pending**: no guest images/Podman/device evidence |

QEMU binary availability alone is retained as preflight evidence and never
qualifies a candidate.
