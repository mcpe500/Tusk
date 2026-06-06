# Container Management

## Container Lifecycle

```
┌─────────┐    ┌─────────┐    ┌──────────┐    ┌───────────┐
│ created │───►│ running │───►│ stopped  │───►│ deleted   │
└─────────┘    └─────────┘    └──────────┘    └───────────┘
     │              │              │
     ▼              ▼              ▼
   start         stop           rm
```

## Container State

| State | Description |
|-------|-------------|
| `created` | Container was created but not yet started |
| `running` | Container is running |
| `paused` | Container is paused (stub) |
| `stopped` | Container has stopped |
| `deleted` | Container has been deleted |

## Container Configuration

Containers are configured using the OCI Runtime Spec:

```json
{
  "ociVersion": "1.0.2",
  "hostname": "tusk-container",
  "process": {
    "terminal": false,
    "user": { "uid": 0, "gid": 0 },
    "args": ["/bin/sh"],
    "cwd": "/",
    "env": ["PATH=/usr/local/bin:/usr/bin:/bin"]
  },
  "linux": {
    "namespaces": [
      { "type": "pid" },
      { "type": "network" },
      { "type": "mount" },
      { "type": "ipc" },
      { "type": "uts" }
    ]
  }
}
```

## Container Storage

Container state and data are stored in:

```
~/.tusk/
├── containers/
│   └── <container-id>/
│       ├── config.json      # OCI runtime config
│       ├── rootfs/          # Container filesystem
│       └── state.json       # Container state
├── images/                  # Image layers
└── volumes/                 # Volume data
```

## Resource Limits

Tusk supports resource limiting via the OCI spec:

```yaml
resources:
  memory:
    limit: 256MB
  cpu:
    shares: 1024
    cpuset: "0-1"
  pids:
    limit: 1024
```

## Health Checks

Containers can be configured with a health check:

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:80"]
  interval: 30s
  timeout: 10s
  retries: 3
```

---

*Back to [docs](../README.md)*
