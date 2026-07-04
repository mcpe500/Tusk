# Build Engine

## Overview

Tusk can now build Docker images from Dockerfiles on-device (Termux/Android) without Docker. The build engine uses proot to execute `RUN` commands and creates OCI-compatible layer diffs.

## Architecture

```
internal/build/
├── parser.go    — Dockerfile parser (FROM, RUN, COPY, ADD, WORKDIR, ENV, CMD, etc.)
├── layer.go     — Filesystem snapshot/diff/tar layer creation
├── context.go   — .dockerignore handling + build context collection
└── engine.go    — Build orchestrator (ties parser + proot + image store)
```

## How it works

1. **Parse Dockerfile** into instruction list
2. **Create temp rootfs** directory
3. For each instruction:
   - `FROM` → pull base image, extract layers into rootfs
   - `RUN` → snapshot rootfs, execute command via proot, diff → new layer
   - `COPY`/`ADD` → snapshot rootfs, copy files from context, diff → new layer
   - `WORKDIR`/`ENV`/`CMD`/`ENTRYPOINT`/`EXPOSE`/`LABEL` → metadata only
4. **Build OCI config** (architecture, OS, env, cmd, entrypoint, workingdir, diff_ids)
5. **Store** config blob + manifest in image store

## Layer creation

- Before each `RUN`/`COPY`: snapshot all file paths + sizes + modtimes
- After: diff snapshots → changed + deleted paths
- Create tar.gz with changed files + whiteout entries for deleted
- SHA256 of uncompressed tar content = diff_id
- SHA256 of compressed layer = blob digest

## CLI commands

```bash
# Standalone build
tusk build -t myapp:latest .
tusk build -f Dockerfile.prod -t myapp:v2 /path/to/context
tusk build --build-arg VERSION=1.0 -t myapp .

# Compose build (builds all services with build: directives)
tusk compose build

# Compose up (auto-builds if build: present and image not yet built)
tusk compose up
```

## Compose integration

Services with `build:` directive and no `image:` are auto-built on `compose up`:
- Image tagged as `tusk-local/<service-name>:latest`
- `compose build` pre-builds all images
- `compose up` checks if image exists; builds if missing

## Daemon RPC

```json
{
  "method": "ImageBuild",
  "params": {
    "contextDir": "/path/to/context",
    "dockerfile": "Dockerfile",
    "tag": "myapp:latest",
    "buildArgs": {"VERSION": "1.0"}
  }
}
```

## Limitations

- `ADD` with remote URLs not supported (only local files)
- No build cache (each build starts fresh)
- `USER` instruction recorded as label only (proot runs as root)
- `VOLUME` instruction recorded as metadata only
- Multi-stage builds: not yet (only final stage)
- No `--target` flag support
- `.dockerignore` uses glob matching (no `**` pattern)
