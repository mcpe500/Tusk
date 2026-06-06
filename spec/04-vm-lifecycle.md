# VM Lifecycle Specification

## Scope

- `tusk init`
- `tusk start`
- `tusk stop`
- `tusk status`

## Current Implementation

| Command | Host Handler | Status | Implementation Evidence | Limitations |
|---|---|---|---|---|
| `tusk init` | `runInit` | done | `image.New(...).Init()` + `vm.New().Init()` |
| `tusk start` | `runStart` | partial | `vm.Manager.Start()` -> spawns `qemu-system-x86_64`, wait for `serial.sock`, ping `Ping` |
| `tusk stop` | `runStop` | done | `vm.Manager.Stop()` => `Process.Kill()` |
| `tusk status` | `runStatus` | partial | `Manager.Status()` + QMP socket check |

## `tusk start` Flow

1. `vm.New(tuskDir)`.
2. `mgr.Start(...)` with default memory/CPU if zero.
3. QEMU arguments include: `qmp`, `serial`, `-netdev user`, `-device virtio-net-pci`, `-virtfs`.
4. `runStart` waits for `mgr.WaitForSerial(60s)`.
5. If serial connection is active, CLI tries `cli.Ping()`.

## `tusk stop` Flow

1. Call `mgr.Stop()`.
2. QEMU process is forcibly killed.

## `tusk status` Flow

1. `mgr.Status()`.
2. Print socket path and if qmp socket exists, try `mgr.WaitForQMP(5s)`.

## Critical Risks

- `Manager.Status()` now relies on PID/socket probing and can still report `stopped` if the socket is stale; `tusk stop` still kills QEMU directly without graceful QMP shutdown.
- No graceful shutdown via QMP (`system_powerdown`); stop only kills process.
- If serial socket is not yet available during `start`, CLI only gives a warning and continues.
- Daemon default `socketPath` in VM is now `/tusk/vm/serial.sock`, matching host target `~/.tusk/vm/serial.sock`.
