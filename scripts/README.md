# Tusk Scripts

This directory contains helper scripts for Tusk VM management.

## Scripts

### install.sh
One-liner installer for Tusk. Installs all dependencies and builds from source.

```bash
curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash
```

Or manually:
```bash
./scripts/install.sh
```

### setup-vm.sh
Setup Alpine VM disk image. Downloads Alpine ISO and creates qcow2 disk.

```bash
./scripts/setup-vm.sh
```

### boot-vm.sh
Boot Alpine VM from disk or ISO.

```bash
# Boot from disk
./scripts/boot-vm.sh

# Run Alpine installer (first time)
./scripts/boot-vm.sh --install

# Boot with tuskd injection
./scripts/boot-vm.sh --boot --tuskd
```

### tusk-vm.sh
Complete VM management utility.

```bash
# Create new disk
./scripts/tusk-vm.sh create

# Run Alpine installer
./scripts/tusk-vm.sh install

# Start VM
./scripts/tusk-vm.sh start

# Stop VM
./scripts/tusk-vm.sh stop

# Check status
./scripts/tusk-vm.sh status

# Attach to serial console
./scripts/tusk-vm.sh attach
```

### configure-alpine.sh
Configuration script to run INSIDE Alpine VM after installation.

```bash
# Run this inside the Alpine VM
./configure-alpine.sh
```

## Quick Start

### Option 1: Full Installation (Recommended)

```bash
# Install everything
./scripts/install.sh

# Create VM disk
./scripts/tusk-vm.sh create

# Install Alpine (interactive)
./scripts/tusk-vm.sh install
# Follow prompts, login as root, then run:
./configure-alpine.sh

# Start VM
./scripts/tusk-vm.sh start

# Use Tusk
tusk status
tusk pull alpine:latest
tusk run alpine echo "Hello from Tusk!"
```

### Option 2: Use Pre-built Disk

(Coming soon - download pre-built disk from releases)

## Directory Structure

```
~/.tusk/
├── images/           # Container images
├── containers/       # Container state
├── state/           # Runtime state
├── vm/
│   ├── disk.qcow2  # Alpine VM disk
│   ├── qmp.sock     # QMP control socket
│   └── serial.sock  # Serial API socket
└── tuskd-amd64       # Daemon binary for VM
```

## Requirements

- Termux (Android 7.0+)
- QEMU (`pkg install qemu-system-x86-64 qemu-utils`)
- Go (`pkg install golang`)
- Git (`pkg install git`)
- ~3GB free storage