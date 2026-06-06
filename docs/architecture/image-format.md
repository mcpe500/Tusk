# OCI Image Format

Tusk uses the Open Container Initiative (OCI) image format that is compatible with Docker and other container tools.

## Image Structure

```
image/
├── index.json              # Image index (points to manifests)
├── manifest.json           # Image manifest (list of layers)
├── config.json             # Image configuration (env, cmd, etc.)
└── blobs/
    ├── sha256/
    │   ├── config          # Config blob
    │   ├── layer1.tar.gz   # Layer 1
    │   └── layer2.tar.gz   # Layer 2
    └── ...
```

## Image Index

`index.json` is the entry point that points to manifest(s):

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "digest": "sha256:5b0bcabd1ee22...,
      "size": 7143,
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    }
  ]
}
```

## Image Manifest

`manifest.json` mendefinisikan image layers:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.oci.image.config.v1+json",
    "digest": "sha256:b5b2b2c507a094...,
    "size": 702
  },
  "layers": [
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "digest": "sha256:e692418e4cbaf...,
      "size": 52428800
    },
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "digest": "sha256:3c3a4604a545a12...,
      "size": 1084028672
    }
  ],
  "annotations": {
    "org.opencontainers.image.created": "2024-01-15T10:30:00Z"
  }
}
```

## Image Configuration

`config.json` berisi metadata image:

```json
{
  "architecture": "amd64",
  "os": "linux",
  "config": {
    "Env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    ],
    "Cmd": ["/bin/sh"],
    "WorkingDir": "/",
    "ExposedPorts": {
      "8080/tcp": {}
    }
  },
  "rootfs": {
    "type": "layers",
    "diff_ids": [
      "sha256:a3ed95caeb02ffe68cdd9fd05506602...,
      "sha256:9e3cb6f4168e75d4a0f52a0b1...,
    ]
  },
  "history": [
    {
      "created": "2024-01-15T10:30:00Z",
      "created_by": "/bin/sh -c #(nop) ADD file:a3bc..."
    }
  ]
}
```

## Content Addressing

All content is addressed with a SHA256 digest:

```
sha256:<64-character-hex>
```

Example: `sha256:e692418e4cbaf90ca3b5844c4d1477`

## Layer Extraction

To create a container rootfs, layers are applied in order:

```bash
# Each layer is extracted and merged
mkdir rootfs
tar -xzf layer1.tar.gz -C rootfs/
tar -xzf layer2.tar.gz -C rootfs/
# ...
# Final rootfs contains all layers
```

## Media Types

| Type | Value |
|------|-------|
| Image Index | `application/vnd.oci.image.index.v1+json` |
| Image Manifest | `application/vnd.oci.image.manifest.v1+json` |
| Image Config | `application/vnd.oci.image.config.v1+json` |
| Layer (tar+gzip) | `application/vnd.oci.image.layer.v1.tar+gzip` |
| Layer (tar) | `application/vnd.oci.image.layer.v1.tar` |

## Registry API

### Fetch Manifest

```
GET /v2/<name>/manifests/<tag>
Authorization: Bearer <token>
Accept: application/vnd.oci.image.manifest.v1+json
```

### Fetch Blob

```
GET /v2/<name>/blobs/<digest>
Authorization: Bearer <token>
```

### Token Authentication

Registry uses OAuth2 token:

```
GET https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/alpine:pull
```

---

*Back to [docs](../README.md)*