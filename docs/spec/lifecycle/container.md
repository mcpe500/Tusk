# Container Lifecycle

## Scope
This document maps commands that affect the container lifecycle (`run`, `ps`, `exec`, `logs`, `rm`, `container stop`) and implementation limitations against the JSON-RPC daemon.

## Container Command Status Table

| Command | Status | Host Handler | RPC Called | Current Daemon Status | Notes |
|---------|--------|--------------|------------|----------------------|-------|
| `tusk run` | partial | `runRun` | `ContainerCreate`, `ContainerStart`, optional `ContainerExec` | Simulated/partial | `-p` and `-v` are skipped; foreground executes command directly through `ContainerExec`. |
| `tusk ps` | partial | `runPS` | `ContainerList` | Simulated/partial | Always queries all=true; no status filter. |
| `tusk exec` | partial | `runExec` | `ContainerExec` | Simulated/partial | Uses response `stdout/stderr/exitCode`. |
| `tusk logs` | partial | `runLogs` | `ContainerLogs` | Simulated/partial | Only returns placeholder log string from store. |
| `tusk container stop` | partial | `runContainerStop` via `runContainer` | `ContainerStop` | Simulated/partial | Only stops state in store. |
| `tusk container rm` / `tusk rm` | partial | `runRM`/`runRMTop` | `ContainerRemove` | Simulated/partial | Force on CLI not yet supported. |
| `tusk container inspect` | stub | `runContainer` | - | stub | Displays not implemented yet message. |

## `tusk run` Implementation
Current CLI argument parsing supports:
- -d/--detach
- --name
- -e/--env
- interactive options: -i, -t, --interactive, --tty (ignored)
- volume/port options: -v/--volume, -p/--publish (not yet fully implemented)

Normal flow:
1. parse arguments (image + command)
2. connect to `~/.tusk/vm/serial.sock` then ping
3. `ContainerCreate` with image/name/cmd/env
4. `ContainerStart`
5. if detach: print container id+pid
6. if foreground: `ContainerExec(id, cmd)` and exit according to `ExitCode`

## `tusk ps` Implementation
- Connect to socket, call `ContainerList(true)`
- Format output as a table with CONTAINER ID, NAME, IMAGE, STATUS
- No containers found if list is empty.

## `tusk exec` and `tusk logs` Implementation
- exec: requires container id and command, displays stdout/err according to RPC result.
- logs: fetches `ContainerLogs` and prints raw to stdout.

## Stop / Remove Implementation
- container stop id -> `ContainerStop(id)` -> print `Container id stopped`.
- rm id/container rm id -> `ContainerRemove(id, false)` -> print `Container id removed`.

## Daemon `cmd/tuskd`

### Simulation mode (if /tusk does not exist)
- Stores containers as JSON files in `~/.tusk/containers`.
- ContainerCreate: create record with random ID, name, image.
- ContainerStart: set state running, simulated PID 12345.
- ContainerStop: set state stopped.
- ContainerExec: execute command on host via `os/exec` (+ direct output), not in VM namespace.
- ContainerLogs: returns placeholder log lines with timestamp.
- ContainerRemove: removes state file.

### Production mode (/tusk path available)
- JSON-RPC connection handler exists, but does not manage OCI runtime completely; operations are still based on store state.

## Important Limitations
- No real container inspect payload.
- No resource limits, healthcheck, pause/resume.
- Port forwarding and mount not yet connected to container lifecycle.
