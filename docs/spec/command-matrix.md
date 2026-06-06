# Tusk Command Specification Matrix

This document serves as a quick reference for contributors: what commands exist, their implementation status, and their execution path.

## Global Status (follows `docs/spec-status.md`)
- `done`: works end-to-end.
- `partial`: callable but behavior is still partial.
- `stub`: still a placeholder / simulation / not running.

## Current Command Matrix

| Command | Subcommand | Status | Handler (Host) | Protocol/API | Affected Lifecycle | Notes |
|---|---|---|---|---|---|---|
| `tusk version` | - | done | `runVersion` | - | -- | Display static CLI version |
| `tusk update` | - | partial | `runUpdate` | shell `git` + `go build` | VM binary | Requires `~/Tusk` repo, no fallback/complete error handling |
| `tusk install` | - | partial | `runInstall` | script (`prebuilt-install.sh`) | VM setup | Depends on external script, retries to auto-install |
| `tusk init` | - | done | `runInit` | - | VM lifecycle, image store | Initialize image/VM directory |
| `tusk start` | - | done | `runStart` | serial/QMP sockets | VM lifecycle | Start QEMU, wait for serial, ping tuskd |
| `tusk stop` | - | done | `runStop` | process kill | VM lifecycle | Kill VM process |
| `tusk status` | - | done | `runStatus` | - | VM status | Display qmp/socket/serial status |
| `tusk pull` | - | done | `runPull` | `ImagePull` | image lifecycle | Real registry pull via HTTP + token |
| `tusk images` | - | partial | `runImages` | - | image lifecycle | List manifests/blobs; tag lookup still placeholder |
| `tusk run` | - | partial | `runRun` | `ContainerCreate`, `ContainerStart`, `ContainerExec` | container lifecycle | Non-detach waits for `exec` after create+start; `-p`/`-v` flags are skipped |
| `tusk ps` | - | partial | `runPS` | `ContainerList` | container lifecycle | Show all containers from daemon (set `all=true`) |
| `tusk exec` | - | partial | `runExec` | `ContainerExec` | container lifecycle | Execute command in container |
| `tusk logs` | - | partial | `runLogs` | `ContainerLogs` | container lifecycle | Return plain logs |
| `tusk rm` | - | partial | `runRMTop` -> `runRM` | `ContainerRemove` | container lifecycle | Force flag not supported |
| `tusk container` | ls | partial | `runContainer` | `runPS` -> `ContainerList` | container lifecycle | Alias to ps |
| `tusk container` | stop | partial | `runContainer` -> `runContainerStop` | `ContainerStop` | container lifecycle | Helper subcommand |
| `tusk container` | rm | partial | `runContainer` -> `runRM` | `ContainerRemove` | container lifecycle | Helper subcommand |
| `tusk container` | inspect | stub | `runContainer` | - | container lifecycle | Placeholder: Container inspect not implemented yet |
| `tusk network` | - | stub | `runNetwork` | - | network lifecycle | Placeholder: Network management not implemented yet |
| `tusk volume` | - | stub | `runVolume` | - | volume lifecycle | Placeholder: Volume management not implemented yet |
| `tusk compose` | up | partial | `runCompose` -> `runComposeUp` -> `Orchestrator.Up` | parsing + orchestrator | compose lifecycle | Only parses and prints start plan, not yet create/start via tuskd |
| `tusk compose` | down, ps, build, logs, rm, stop | stub | `runCompose` | - | compose lifecycle | All show not implemented yet |

## Brief Execution Notes
- For cross-layer lifecycle, see files in `docs/spec/lifecycle/`.
- For status format details, see `docs/spec-status.md`.
