# Incomplete Tasks / Bug List

This document contains the top 10 highest-priority incomplete items in the current snapshot.

Not all items below are specific to Termux. Most are general Tusk architecture gaps; Termux-related items are usually tied to `HOME` path handling and VM/daemon runtime flow.

## Top 10 Priorities

1. [P1] Implement container execution runtime inside daemon
   - **Termux-specific:** Not directly (generic path, affects all hosts)
   - **File:** `cmd/tuskd/main.go`
   - **Issue:** `ContainerExec` currently runs commands on the host, not in container context.
   - **Impact:** `tusk exec`/`tusk run` is not actually executed within container lifecycle.
   - **Plan:** connect to container runtime (`internal/container`) and run processes inside container namespace/rootfs.

2. [P1] Implement `ContainerInspect` in protocol handler
   - **Termux-specific:** No
   - **File:** `cmd/tuskd/main.go`, `pkg/protocol/api.go`
   - **Issue:** `ContainerInspect` is defined in protocol but has no handler.
   - **Impact:** `tusk container inspect` cannot return real container data.
   - **Plan:** add an inspect endpoint that reads container state from the store.

3. [P1] Complete runtime spec persistence
   - **Termux-specific:** No
   - **File:** `internal/container/spec.go`
   - **Issue:** `SpecGenerator.SaveSpec` still contains a placeholder custom `jsonMarshal` that returns an error.
   - **Impact:** The real OCI create/start pipeline is incomplete.
   - **Plan:** use standard `encoding/json` and write a valid `config.json`.

4. [P1] Complete image operations in daemon
   - **Termux-specific:** No
   - **File:** `cmd/tuskd/main.go`
   - **Issue:** `ImageList` handler returns an empty array and `ImagePull` only returns static status.
   - **Impact:** daemon-backed `tusk images` output does not represent real image data.
   - **Plan:** map from local image store (`internal/image`) into JSON-RPC responses.

5. [P2] Implement compose subcommands other than `up`
   - **Termux-specific:** No
   - **File:** `cmd/tusk/main.go`, `internal/compose/parser.go`
   - **Issue:** `compose down/ps/logs/build/rm/stop` still display `not implemented yet`.
   - **Impact:** compose features remain mostly non-functional.
   - **Plan:** wire subcommands to existing orchestrator methods or implement new handlers.

6. [P2] Implement network and volume management
   - **Termux-specific:** Not directly (general), but effects are visible in Termux/VM bridge runtime.
   - **File:** `cmd/tusk/main.go`, `internal/network/manager.go`
   - **Issue:** `tusk network`/`tusk volume` CLI paths are inactive and manager is currently a stub.
   - **Impact:** compose setups that rely on network/volume have no real effect.
   - **Plan:** add command paths and API that create real resources with persistent state.

7. [P2] Process `-p/--publish` and `-v/--volume` flags in `tusk run`
   - **Termux-specific:** No
   - **File:** `cmd/tusk/main.go:380-403`
   - **Issue:** these flags are skipped during argument parsing.
   - **Impact:** port mappings and mounts are ignored even when requested by users.
   - **Plan:** parse these options and persist metadata into container payload.

8. [P2] Fix displayed image tag mapping
   - **Termux-specific:** No
   - **File:** `cmd/tusk/main.go` (`runImages`)
   - **Issue:** longstanding TODO for tag index resolution is still unimplemented.
   - **Impact:** `tusk images` output for `REPOSITORY/TAG` is inaccurate.
   - **Plan:** use manifest index (`index/`) for proper repo+tag resolution.

9. [P3] Stabilize VM lifecycle and graceful shutdown
   - **Termux-specific:** Yes, because VM manager path and operations are directly used in Termux QEMU workflow
   - **File:** `internal/vm/manager.go`
   - **Issue:** VM status is based on `os.Signal(nil)` and stop uses force kill.
   - **Impact:** inconsistent status reporting and resource cleanup.
   - **Plan:** query status via QMP and call `system_powerdown` before fallback kill.

10. [P3] Expand end-to-end testing
   - **Termux-specific:** No
   - **File:** entire workspace; only `cmd/tuskd/main_test.go` exists today
   - **Issue:** no test suite for command matrix, compose, and networking contract.
   - **Impact:** critical regressions can slip through undetected.
   - **Plan:** add tests in `cmd/tusk` plus integration tests for command flow and JSON-RPC handlers.

## Notes

- Priority **P1** = functional blockers.
- Priority **P2** = user-facing features and usability.
- Priority **P3** = stability and maintainability.

## Scope Categories

- **Global (cross-platform):** 1, 2, 3, 4, 5, 6, 7, 8, 10
- **Termux-relevant:** 9
