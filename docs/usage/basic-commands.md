# Basic Commands

## VM Management

### `tusk init`
Initialize Tusk storage and the required directories.

```bash
tusk init
# Output: Tusk initialized successfully!
```

### `tusk start`
Start QEMU VM with Alpine Linux.

```bash
tusk start
# Output: Starting Tusk VM...
```

### `tusk stop`
Stop QEMU VM.

```bash
tusk stop
```

### `tusk status`
Check VM and sockets status.

```bash
tusk status
# Output:
# VM Status: running
# QMP Socket: ~/.tusk/vm/qmp.sock
# Serial Socket: ~/.tusk/vm/serial.sock
```

## Image Management

### `tusk pull <image>`
Pull image from Docker Hub.

```bash
tusk pull alpine:latest
tusk pull nginx:latest
tusk pull postgres:15
```

### `tusk images`
List all images that have been pulled.

```bash
tusk images
# Output:
# REPOSITORY   TAG      SIZE
# alpine       latest   3 MB
# nginx        latest   140 MB
```

---

## Container Operations

### `tusk run`
Run container from image.

```bash
# Run with command
tusk run alpine echo hello

# Run detached
tusk run -d --name web nginx

# Mount volume
tusk run -v /data:/app alpine

# Port forwarding
tusk run -p 8080:80 nginx
```

**Options:**
- `-d, --detach` - Run in background
- `--name` - Container name
- `-v, --volume` - Mount volume
- `-p, --publish` - Port forwarding
- `-e, --env` - Environment variable
- `-w, --workdir` - Working directory
- `-i, --interactive` - Interactive mode
- `-t, --tty` - Allocate pseudo-TTY

### `tusk ps`
List running containers.

```bash
tusk ps
# Output:
# CONTAINER ID   NAME   IMAGE    STATUS   PORTS
# abc123         web    nginx    running  0.0.0.0:8080->80/tcp
```

### `tusk exec <container> <command>`
Execute command in running container.

```bash
tusk exec web ls /app
tusk exec -it web /bin/sh
```

### `tusk logs <container>`
Display logs from container.

```bash
tusk logs web
tusk logs --follow web  # Follow logs
```

### `tusk stop <container>`
Stop running container.

```bash
tusk stop web
```

### `tusk rm <container>`
Remove stopped container.

```bash
tusk rm web
tusk rm -f web  # Force remove
```

## Network & Volume

### `tusk network ls`
List networks.

```bash
tusk network ls
```

### `tusk volume ls`
List volumes.

```bash
tusk volume ls
```

---

*Back to [docs](../README.md)*
