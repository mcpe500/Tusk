# Communication Protocol

## Overview

Tusk uses JSON-RPC 2.0 for communication between the host CLI (`tusk`) and the guest daemon (`tuskd`).

## Transport Layer

```
┌──────────────────────┐       ┌──────────────────────┐
│       Host (CLI)      │       │      Guest (VM)      │
│                      │       │                      │
│  ┌────────────────┐  │  9p   │  ┌────────────────┐   │
│  │  tusk CLI      │  │       │  │  tuskd         │   │
│  └───────┬────────┘  │       │  └───────┬────────┘   │
│          │           │       │          │             │
│          │  Unix Socket    │          │             │
│          │  ~/.tusk/vms/    │          │             │
│          │  serial.sock     │          │             │
│          ▼                 │          ▼             │
│  ┌──────────────────────┐   │   ┌──────────────────┐ │
│  │  Socket (host side)  │◄──┼──►│  Serial device   │ │
│  └──────────────────────┘   │   └──────────────────┘ │
└──────────────────────────────┘   └────────────────────┘
```

## Protocol Format

### Request

```json
{
  "jsonrpc": "2.0",
  "method": "ContainerCreate",
  "params": {
    "image": "alpine:latest",
    "name": "my-container",
    "command": ["/bin/sh", "-c", "echo hello"]
  },
  "id": 1234567890
}
```

### Response (Success)

```json
{
  "jsonrpc": "2.0",
  "result": {
    "id": "container-abc123",
    "name": "my-container",
    "pid": 1234,
    "ipAddress": "10.0.0.2"
  },
  "id": 1234567890
}
```

### Response (Error)

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid params: image not found"
  },
  "id": 1234567890
}
```

## API Methods

### Container Operations

#### `ContainerCreate`

Create a new container.

**Params:**
```json
{
  "image": "alpine:latest",
  "name": "my-app",
  "command": ["/bin/sh"],
  "env": ["FOO=bar"],
  "mounts": [
    { "type": "bind", "source": "/data", "destination": "/app" }
  ],
  "network": "default"
}
```

**Result:**
```json
{
  "id": "abc123",
  "name": "my-app",
  "pid": 1234,
  "ipAddress": "10.0.0.2"
}
```

#### `ContainerStart`

Start a stopped container.

**Params:** `{ "id": "container-id" }`

**Result:** `{ "status": "started" }`

#### `ContainerStop`

Stop a running container.

**Params:** `{ "id": "container-id" }`

**Result:** `{ "status": "stopped" }`

#### `ContainerRemove`

Remove a container.

**Params:** `{ "id": "container-id", "force": false }`

**Result:** `{ "status": "removed" }`

#### `ContainerList`

List containers.

**Params:** `{ "all": true }`

**Result:**
```json
{
  "containers": [
    { "id": "abc123", "name": "web", "image": "nginx", "status": "running" }
  ]
}
```

#### `ContainerExec`

Execute command in running container.

**Params:**
```json
{
  "containerId": "abc123",
  "command": ["/bin/ls", "-la"],
  "attachStdin": false,
  "attachStdout": true,
  "attachStderr": true,
  "tty": false
}
```

**Result:**
```json
{
  "exitCode": 0,
  "stdout": "total 48\ndrwxr-xr-x 2 root root 4096 ...\n",
  "stderr": ""
}
```

#### `ContainerLogs`

Get container logs.

**Params:** `{ "id": "container-id" }`

**Result:** `{ "logs": "2024-01-15T10:30:00Z Hello world\n" }`

### Image Operations

#### `ImagePull`

Pull image from registry.

**Params:** `{ "reference": "alpine:latest" }`

**Result:** `{ "status": "pulling", "id": "layer-abc" }`

#### `ImageList`

List local images.

**Result:**
```json
{
  "images": [
    { "id": "sha256:abc123", "tags": ["alpine:latest"], "size": 3000000 }
  ]
}
```

### Network Operations

#### `NetworkCreate`

Create a network.

**Params:** `{ "name": "app-net", "driver": "bridge" }`

**Result:** `{ "id": "net-123", "name": "app-net" }`

#### `NetworkList`

List networks.

**Result:**
```json
{
  "networks": [
    { "id": "net-123", "name": "app-net", "driver": "bridge" }
  ]
}
```

### System Operations

#### `Ping`

Health check.

**Result:** `"pong"`

#### `Info`

Get daemon info.

**Result:**
```json
{
  "version": "1.0.0",
  "apiVersion": "v1",
  "os": "linux",
  "arch": "x86_64"
}
```

## Error Codes

| Code | Name | Description |
|------|------|-------------|
| -32700 | Parse Error | Invalid JSON |
| -32600 | Invalid Request | Not a valid JSON-RPC request |
| -32601 | Method Not Found | Unknown method |
| -32602 | Invalid Params | Invalid method parameters |
| -32603 | Internal Error | Internal server error |

## Message Framing

Messages are newline-delimited:

```
{"jsonrpc":"2.0","method":"Ping","id":1}\n
{"jsonrpc":"2.0","result":"pong","id":1}\n
```

---

*Back to [docs](../README.md)*