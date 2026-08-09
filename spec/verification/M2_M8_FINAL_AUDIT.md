# M2-M8 Final Audit

Date: 2026-08-10

This report records the final host-side audit of the M1-M8 delivery sequence.
Implementation checklist items are not equivalent to guest or device acceptance.
An external gate remains pending unless its required runtime evidence exists.

## Audit Matrix

| Milestone | Implementation checklist | Independent audit | Commit/push | Regression | Exit gate |
|---|---|---|---|---|---|
| M1 | Complete | PASS | `70a6b18` | PASS | PASS |
| M2 | Complete | PASS | `ef64ef8` | PASS | PASS as host framework; no backend qualified |
| M3 | Complete | PASS after hardening | `38509cb`, `2929871`, `d11f8fc` | PASS | Pending AT-019: real proot/rootfs/Podman |
| M4 | Complete | PASS at code level | `2d51f49` | PASS | Pending AT-031..033: initialized guests/device |
| M5 | Complete | PASS at code level | `10fc5c4` | PASS | Pending AT-003..005, AT-018: qualified backend |
| M6 | Complete | PASS at code level | `802214b`, `d11f8fc` | PASS | Pending AT-007..011: real backend workflows |
| M7 | Complete | PASS at code level | `634ed0f` | PASS | Pending AT-012..013: real Compose workflow |
| M8 | Complete | PASS at host level | `129d86c`, `d11f8fc` | PASS | Pending AT-001..036: release/device evidence |

## Regression Evidence

The final hardening batch was verified with:

- `make check` using `/tmp/opencode/go`: vet, coverage tests, race tests, and Android ARM64 build.
- `go test -race ./...` using `/tmp/opencode/go`.
- Focused QEMU, proot, share, tune, acceptance, and CLI tests.
- Subagent reviews after M1, each milestone implementation, the proot command-vector fix, and the final hardening batch.

## Graphify Evidence

The final code-only incremental Graphify run is recorded in `graphify-out/`:

- 709 nodes.
- 1,516 edges.
- 46 communities.
- Graph freshness commit: `d11f8fc`.

Graphify is a navigation and traceability aid; it is not runtime acceptance
evidence. Semantic document extraction was unavailable without a configured
Gemini backend, so no semantic success is claimed.

## External Blockers

The host does not provide the evidence needed to close the remaining gates:

- No initialized Android/Termux reference device or device performance record.
- No signed packaged guest system/data images with pinned SSH host keys.
- No qualified proot rootfs with working Podman, OCI runtime, storage, network,
  and Compose health checks.
- No full real-backend PTY, API bridge, mount, port, Compose, or release workflow
  evidence.

The canonical specification and acceptance command therefore remain fail-closed
and must continue to report MVP acceptance as incomplete.
