# VM Lifecycle (QEMU)

## Scope
This specification describes the lifecycle of the VM (virtual machine) run through `tusk start`, `tusk stop`, and `tusk status`, as well as folder initialization through `tusk init`.

## Implementation Summary (based on CLI + VM manager)

| Command | Host Handler | Status | Implementation Facts | Limitation Notes |
|---------|--------------|--------|----------------------|------------------|
| `tusk init` | `runInit` | done | Creates image store structure (`~/.tusk/images`) and VM dir (`~/.tusk/vm`, `sockets`). | No data integrity verification after init. |
| `tusk start` | `runStart` | done | Calls `vm.Manager.Start`, waits for `serial.sock` up to 60s, then pings tuskd via JSON-RPC. | No complex fallback if pinning old `tuskd`; if no daemon exists, only waits for timeout and continues with a warning. |
| `tusk stop` | `runStop` | done | Calls `vm.Manager.Stop` => `Process.Kill()` on the QEMU process. | Does not use `QMP SystemPowerdown`; shutdown is not graceful. |
| `tusk status` | `runStatus` | done | Displays internal manager status, socket path, and QMP connection check if the socket exists. | `Status()` is still based on process signal + socket presence, not on rich metadata VM-state queries. |

## `runStart` Flow
1. `runStart` creates a `vm.Manager` with base dir `~/.tusk`.
2. Builds a default `vm.Config` of `Memory: 512, CPUs: 2` if not filled in.
3. `Manager.Start(...)` builds QEMU arguments:
   - `-M pc-i440fx-9.2`
   - `-m`, `-smp`
   - `-nographic`
   - `-qmp unix:<path>,server,nowait`
   - `-serial unix:<path>,server,nowait`
   - `-netdev user` + `virtio-net-pci`
   - `-virtfs local,path=~/.tusk,mount_tag=tusk-data,mapped,id=tusk`
   - optional: disk/kernel/initrd/cdrom.
4. Wait until `Manager.WaitForSerial(60s)` succeeds.
5. Try `client.Connect()` to `serial.sock`, then `cli.Ping()`.

## `runStop` Flow
- Stop is triggered as `mgr.Stop()` which only kills the process if it exists. No structured drain/cleanup.

## Stability Notes
- `Status()` is considered `running` if `Process.Signal(0)` succeeds.
- `Status()` falls back to `stopped` when `qmp.sock` exists but the process cannot be signaled.
- `ensureVM()` (helper in CLI) exists, but is not used in the entire current flow.

## Relevant `vm.Manager` Methods
- `New(tuskDir)` → set paths `~/.tusk/vm/qmp.sock`, `~/.tusk/vm/serial.sock`.
- `Init()` → `~/.tusk/vm` and `~/.tusk/vm/sockets`.
- `WaitForSerial()` and `WaitForQMP()` for startup synchronization.
- `Stop()` / `Wait()` manage host-side process lifecycle.

## Relation to `tuskd`
`runStart` waits for `serial.sock` then sends a ping through the CLI client to confirm the daemon is responding. If not, the CLI prints `tuskd not responding yet` without failing fatally.
