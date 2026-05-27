# Building from Source

## Requirements

- Go 1.18 or later
- QEMU (for testing VM integration)
- Git

## Quick Build

```bash
# Clone repository
git clone https://github.com/mcpe500/Tusk.git
cd Tusk

# Build CLI (tusk)
go build -o tusk ./cmd/tusk

# Build daemon (tuskd)
go build -o tuskd ./cmd/tuskd

# Build both
make build
```

## Makefile Targets

```bash
make build         # Build both binaries
make clean         # Clean build artifacts
make test          # Run tests
make lint          # Run linters
make all           # Build, test, and lint
```

## Cross-Compilation

### Android (aarch64)

```bash
GOOS=linux GOARCH=arm64 go build -o tusk-arm64 ./cmd/tusk
```

### Other platforms

```bash
# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o tusk-darwin-amd64 ./cmd/tusk

# Windows
GOOS=windows GOARCH=amd64 go build -o tusk.exe ./cmd/tusk
```

## Build Flags

### Verbose output

```bash
go build -v ./cmd/tusk
```

### With version info

```bash
go build -ldflags="-X main.version=1.0.0" -o tusk ./cmd/tusk
```

## Installing

```bash
# Local install
go install ./cmd/tusk

# Or copy binary to PATH
cp tusk $PREFIX/bin/
```

## Dependencies

```bash
# Update dependencies
go mod tidy

# Download dependencies
go mod download

# List dependencies
go list -m all
```

## Build Size

| Binary | Size (approx) |
|--------|---------------|
| tusk (CLI) | ~9 MB |
| tuskd (Daemon) | ~4 MB |

## Troubleshooting

### "package not found"

Run `go mod tidy` to sync dependencies.

### "command not found: make"

Use direct go commands instead.

---

*Back to [docs](../README.md)*