# Installation

## Requirements

### Software
- **Termux** (Android 7.0+)
- **QEMU** (x86_64 system emulator)
- **Go** 1.18+ (untuk build dari source)

### Install QEMU di Termux

```bash
pkg install qemu-system-x86-64
```

### Install Go di Termux (jika belum)

```bash
pkg install golang
```

## Installation Methods

### Method 1: Pre-built Binary (Coming Soon)

Download binary dari GitHub Releases.

```bash
# TODO: Tambahkan link releases saat tersedia
```

### Method 2: Build dari Source

```bash
# Clone repository
git clone https://github.com/mcpe500/Tusk.git
cd Tusk

# Build CLI
go build -o tusk ./cmd/tusk

# Build daemon (untuk VM)
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

## Verifikasi Installation

```bash
# Check version
./tusk --version

# Check VM status
./tusk status

# List commands
./tusk --help
```

## Troubleshooting

### QEMU tidak ditemukan

```bash
which qemu-system-x86_64
# Jika tidak ada, install dengan:
pkg install qemu-system-x86-64
```

### VM tidak mau start

Pastikan Alpine ISO tersedia:
```bash
ls ~/alpine-virt-*.iso
```

### Permission error

Tusk memerlukan akses untuk:
- Create socket files di `~/.tusk/`
- Execute QEMU

---

*Back to [docs](./README.md)*