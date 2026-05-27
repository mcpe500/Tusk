# Changelog

Semua perubahan penting akan didokumentasikan di file ini.

Format mengikuti [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- [Initial commit] Project structure dan Go modules
- `tusk` CLI dengan basic commands
- `tuskd` daemon dengan simulation mode
- OCI-compliant types untuk image, container, dan compose
- VM manager dengan QMP client
- JSON-RPC client untuk komunikasi CLI-daemon
- Image store dengan Docker Hub pull support
- Docker Compose YAML parser
- Network manager (basic)
- Dokumentasi lengkap di `/docs`

### Commands Implemented

| Command | Status |
|---------|--------|
| `tusk init` | ✅ Implemented |
| `tusk start` | ✅ Implemented |
| `tusk stop` | ✅ Implemented |
| `tusk status` | ✅ Implemented |
| `tusk pull` | ✅ Implemented |
| `tusk images` | ✅ Implemented |
| `tusk run` | ⬜ Stub only |
| `tusk ps` | ⬜ Stub only |
| `tusk exec` | ⬜ Stub only |
| `tusk logs` | ⬜ Stub only |
| `tusk container` | ✅ Implemented |
| `tusk network` | ✅ Implemented |
| `tusk volume` | ✅ Implemented |
| `tusk compose` | ✅ Implemented |

### Documentation

- [docs/README.md](docs/README.md) - Documentation index
- [docs/overview.md](docs/overview.md) - Project overview
- [docs/installation.md](docs/installation.md) - Installation guide
- [docs/architecture/system-design.md](docs/architecture/system-design.md) - System design
- [docs/architecture/communication.md](docs/architecture/communication.md) - JSON-RPC protocol
- [docs/architecture/image-format.md](docs/architecture/image-format.md) - OCI image format
- [docs/usage/basic-commands.md](docs/usage/basic-commands.md) - CLI usage
- [docs/usage/container-management.md](docs/usage/container-management.md) - Container management
- [docs/usage/docker-compose.md](docs/usage/docker-compose.md) - Docker Compose guide
- [docs/development/contributing.md](docs/development/contributing.md) - Contributing guide
- [docs/development/building.md](docs/development/building.md) - Build instructions
- [docs/development/testing.md](docs/development/testing.md) - Testing guide

---

## Version History

### [0.0.1] - 2026-05-27

- Initial release
- Basic project structure
- Proof-of-concept implementation