# M6 Storage and Networking Traceability

`internal/engine/share.go` provides canonical, non-overlapping host shares,
stable guest mount paths, symlink-aware validation, read-only preservation, and
argv-level path translation. `internal/engine/ports.go` parses loopback TCP
publishes and persists derived state only after an adapter `PortForwarder`
successfully creates the route.

| Requirement | Evidence | Status |
|---|---|---|
| `FS-001..015` | `ShareRegistry`, `TranslatePodmanArgs`, CLI `tusk share` | Implemented; real mount validation pending |
| `NET-003..012` | `PortReconciler`, collision/forwarder gates, atomic state | Implemented; adapter route pending |
| `NET-013..014` | loopback TCP parser, UDP rejection | Implemented |
| `AT-007..011` | real build/mount/publish/recovery workflows | **Pending**: no qualified backend |

No port is reported ready when the adapter forwarder is absent or the host
port is occupied.

Shared-filesystem limitation: the guest export is an explicit bind-style root,
not a transparent Termux home mount. Ownership, special files, file watching,
and event semantics can differ across proot/QEMU sharing mechanisms; workflows
must use registered paths and qualify these limitations on each backend.
