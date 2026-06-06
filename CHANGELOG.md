# Changelog

All notable changes will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- [Initial commit] Project structure and Go modules
- `tusk` CLI with basic commands
- `tuskd` daemon with simulation mode
- OCI-compliant types for image, container, and compose
- VM manager with QMP client
- JSON-RPC client for CLI-daemon communication
- Image store with Docker Hub pull support
- Docker Compose YAML parser
- Network manager (basic)
- Complete documentation in `/docs`

### Command Status

| Command | Status |
|---------|--------|
| `tusk init` | done |
| `tusk start` | done |
| `tusk stop` | done |
| `tusk status` | done |
| `tusk pull` | done |
| `tusk images` | done |
| `tusk run` | partial |
| `tusk ps` | partial |
| `tusk exec` | partial |
| `tusk logs` | partial |
| `tusk container` | partial |
| `tusk network` | stub |
| `tusk volume` | stub |
| `tusk compose` | partial |

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
