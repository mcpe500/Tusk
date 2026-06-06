# Tusk Command Matrix

## CLI Surface

| Command | Subcommand | Handler | Component | Status | Notes |
|---|---|---|---|---|---|
| `tusk version` | - | `runVersion` | host | done | Print static version |
| `tusk update` | - | `runUpdate` | host | partial | Build shell commands, assumes certain directory/path |
| `tusk install` | - | `runInstall` | host/script | partial | Script path + fallback, no result verification |
| `tusk init` | - | `runInit` | host/image+vm | done | initialize image and vm folders |
| `tusk start` | - | `runStart` | host/vm | partial | start VM then ping tuskd |
| `tusk stop` | - | `runStop` | host/vm | done | kill QEMU process |
| `tusk status` | - | `runStatus` | host/vm | partial | display status and socket status |
| `tusk pull <image>` | - | `runPull` | host/image | done | real pull from registry via HTTP |
| `tusk images` | - | `runImages` | host/image | partial | list local manifest/blobs, tag lookup TODO |
| `tusk run` | - | `runRun` | host/client+daemon | partial | argument parsing exists, `-p/-v` options skipped |
| `tusk ps` | - | `runPS` | host/client/daemon | partial | list from daemon |
| `tusk exec` | - | `runExec` | host/client/daemon | partial | executes `ContainerExec` RPC |
| `tusk logs` | - | `runLogs` | host/client/daemon | partial | placeholder log from daemon |
| `tusk rm` | - | `runRMTop` | host/client/daemon | partial | remove from daemon |
| `tusk container` | `ls` | `runContainer` -> `runPS` | host/client/daemon | partial | alias to `ps` |
| `tusk container` | `stop` | `runContainer` -> `runContainerStop` | host/client/daemon | partial | stop via RPC |
| `tusk container` | `rm` | `runContainer` -> `runRM` | host/client/daemon | partial | remove |
| `tusk container` | `inspect` | `runContainer` | host | stub | "not implemented yet" message |
| `tusk network` | - | `runNetwork` | host | stub | not yet implemented |
| `tusk volume` | - | `runVolume` | host | stub | not yet implemented |
| `tusk compose` | `up` | `runCompose` -> `runComposeUp` | host/compose/daemon | partial | parse + create/start through daemon |
| `tusk compose` | `down` | `runCompose` | host/compose | stub | placeholder message |
| `tusk compose` | `ps` | `runCompose` | host/compose | stub | placeholder message |
| `tusk compose` | `build` | `runCompose` | host/compose | stub | placeholder message |
| `tusk compose` | `logs` | `runCompose` | host/compose | stub | placeholder message |
| `tusk compose` | `rm` | `runCompose` | host/compose | stub | placeholder message |
| `tusk compose` | `stop` | `runCompose` | host/compose | stub | placeholder message |

## Default Behavior for CLI

- Unknown command → `printUsage`.
- Many container operation paths still depend on active socket and `tuskd` responding.
- `--name`, `-e/--env` options are supported; interactive options `-i/-t/--tty` are ignored.
