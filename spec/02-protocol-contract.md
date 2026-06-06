# Protocol Contract Specification

## Centralized Interface

- Request/response schema is defined in `pkg/protocol/api.go`.
- Host client builds JSON-RPC messages in `internal/client/client.go`.
- Daemon reads raw requests via `json.Decoder` in `cmd/tuskd/main.go`.

## Advertised Method List

- `ContainerCreate`, `ContainerList`, `ContainerStart`, `ContainerStop`, `ContainerRemove`
- `ContainerExec`, `ContainerLogs`
- `ImagePull`, `ImageList`
- `NetworkCreate`, `NetworkList`, `NetworkRemove`
- `ContainerInspect`, `Ping`, `Info`

## Actual Implementation Status

| Method | Status | Realization |
|---|---|---|
| `Ping` | done | handler returns `pong` |
| `Info` | done | returns static version info |
| `ContainerCreate` | partial | creates JSON file entry in container store |
| `ContainerList` | partial | list from file store |
| `ContainerStart` | partial | changes state + dummy pid |
| `ContainerStop` | partial | state changed to stopped |
| `ContainerRemove` | partial | deletes container state file |
| `ContainerExec` | partial | executes host process, not within container runtime |
| `ContainerLogs` | partial | placeholder timestamp string |
| `ImagePull` | partial | returns `pulled` status without transfer process |
| `ImageList` | partial | returns empty array |
| `NetworkCreate` | partial | returns dummy id, without actual resource |
| `NetworkList` | partial | returns empty array |
| `NetworkRemove` | stub | no handler |
| `ContainerInspect` | stub | no handler |

## Contract Incoherencies

- In old parser mode there was potential panic due to direct `type assertion`; currently handlers are standardized so that `invalid params` produces a JSON-RPC error.
- `ImagePull` and `ImageList` in CLI are not used for real pull/list in normal flow; CLI `tusk pull` uses `internal/image` directly.
- `ImagePull` in daemon remains statically successful.

## I/O Contract That Must Be Verified

- JSON-RPC response must include `jsonrpc: 2.0`, `id` matching the request, and `result/error`.
- Daemon must reject malformed requests with JSON-RPC error (not panic).
