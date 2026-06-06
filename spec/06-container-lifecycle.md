# Container Lifecycle Specification

## Feature Status

| Command | Status | Implementation |
|---|---|---|
| `tusk run` | partial | Connect daemon -> `ContainerCreate` -> `ContainerStart`; detach/fg branch |
| `tusk ps` | partial | `ContainerList(true)` from daemon |
| `tusk exec` | partial | `ContainerExec` via daemon and exit according to result |
| `tusk logs` | partial | `ContainerLogs` from daemon (placeholder value) |
| `tusk rm` / `tusk container rm` | partial | `ContainerRemove` without `force` |
| `tusk container stop` / `tusk stop` (container) | partial | `ContainerStop` |
| `tusk container inspect` | stub | not implemented message |

## `tusk run` Flow

1. Parse arguments (`-d`, `--name`, `-e` are processed; `-p`, `-v` are skipped).
2. Connect `~/.tusk/vm/serial.sock` and `Ping`.
3. `ContainerCreate` with image/name/command/env.
4. `ContainerStart`.
5. If detach: print `Container <id> started`.
6. If foreground: run `ContainerExec` with command, then exit with exit code.

## Risk Notes

- For command without command (`tusk run alpine`), CLI still creates container and starts, then tries to exec with empty arguments on the foreground branch.
- `Detach` displays `result.ID[:12]` without ID length guard.
- Host runtime (`internal/container/runtime.go`) provides spec generator & `PrepareRootfs`, but daemon currently does not perform complete real OCI workflow.

## Anticipated vs Real Runtime and State

- Docs specification mentions bundle/OCI state (`config.json`, `state.json`), but actual runtime has not yet produced that flow end-to-end.
- Daemon currently stores simple `ContainerInfo` state in per-ID JSON file.
