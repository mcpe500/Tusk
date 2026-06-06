# Installation

## Requirements

### Software
- **Termux** (Android 7.0+)
- **QEMU** (x86_64 system emulator)
- **Go** 1.18+ (for building from source)

### Install QEMU on Termux

```bash
pkg install qemu-system-x86-64
```

### Install Go on Termux (if not already)

```bash
pkg install golang
```

## Installation Methods

### Method 1: Pre-built Binary (Coming Soon)

Download the binary from GitHub Releases.

```bash
# TODO: Add release link when available
```

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/mcpe500/Tusk.git
cd Tusk

# Build CLI
go build -o tusk ./cmd/tusk

# Build daemon (for VM)
go build -o tuskd ./cmd/tuskd

# Optionally, add to PATH
cp tusk $PREFIX/bin/
```

## Quick Start

```bash
# 1. Initialize Tusk
./tusk init

# 2. Start VM
./tusk start

# 3. Pull image
./tusk pull alpine:latest

# 4. Run container
./tusk run alpine echo "Hello from Tusk!"
```

## Verifying the Installation

```bash
# Check version
./tusk --version

# Check VM status
./tusk status

# List commands
./tusk --help
```

## Troubleshooting

### QEMU not found

```bash
which qemu-system-x86_64
# If not present, install with:
pkg install qemu-system-x86-64
```

### VM refuses to start

Make sure the Alpine ISO is available:
```bash
ls ~/alpine-virt-*.iso
```

### Permission error

Tusk requires access to:
- Create socket files in `~/.tusk/`
- Execute QEMU

---

*Back to [docs](./README.md)*
