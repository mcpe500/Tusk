# Tusk Project Specification (State Snapshot)

This document is a snapshot of the actual implementation of the repo `D:\Ivan\projects\Tusk` at the current state.

## Scope Summary

- `cmd/tusk/main.go` and `cmd/tuskd/main.go` define user-facing behavior.
- VM/daemon lifecycle runs through `internal/vm` and communicates via Unix socket JSON-RPC.
- Image store and layer extraction are in `internal/image`.
- Container orchestration is in `internal/container` and `internal/compose`.
- Feature status used: `done`, `partial`, `stub`.

## Status Criteria (across all spec files)

| Status | Criteria |
|---|---|
| `done` | The flow already runs end-to-end and produces real actions according to the feature name. |
| `partial` | There is a runnable flow, but there are still placeholder data, partial controls, or important implementation gaps. |
| `stub` | Does not perform real actions; output is only placeholder/stub error messages. |

## Main Notes (global)

- Documentation (README/docs) is ahead of the actual runtime.
- Many CLI commands already exist but several subcommands are still placeholders.
- There is a minimal test suite (`cmd/tuskd/main_test.go`), but it does not yet cover the entire command/container stack.
- JSON-RPC protocol contract is not fully consistent between host, protocol, and handlers in the daemon.
