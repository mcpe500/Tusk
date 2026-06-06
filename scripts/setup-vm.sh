#!/bin/bash
# Tusk VM Setup Script
# Creates Alpine Linux qcow2 disk image with tuskd auto-start

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/temp.sh"

ALPINE_VERSION="3.19.1"
ALPINE_ISO_URL="https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION}/releases/x86_64/alpine-virt-${ALPINE_VERSION}-x86_64.iso"
ALPINE_ISO="$HOME/alpine-virt-${ALPINE_VERSION}-x86_64.iso"
DISK_IMAGE="$HOME/.tusk/vm/disk.qcow2"
DISK_SIZE="2G"

TUSK_DIR="$HOME/.tusk"
TUSKD_BINARY="$TUSK_DIR/tuskd-amd64"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[TUSK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_requirements() {
    log "Checking requirements..."

    # Check QEMU
    if ! command -v qemu-system-x86_64 &> /dev/null; then
        error "QEMU not found. Install with: pkg install qemu-system-x86-64"
        exit 1
    fi
    log "QEMU: $(qemu-system-x86_64 --version | head -1)"

    # Check qemu-img
    if ! command -v qemu-img &> /dev/null; then
        error "qemu-img not found. Install qemu-utils."
        exit 1
    fi

    # Check Go
    if ! command -v go &> /dev/null; then
        warn "Go not found. You can build tuskd binary manually."
    fi

    mkdir -p "$TUSK_DIR/vm"
}

download_alpine() {
    if [ -f "$ALPINE_ISO" ]; then
        log "Alpine ISO already exists: $ALPINE_ISO"
    else
        log "Downloading Alpine ISO..."
        curl -L -o "$ALPINE_ISO" "$ALPINE_ISO_URL"
        log "Downloaded to: $ALPINE_ISO"
    fi
}

create_disk() {
    if [ -f "$DISK_IMAGE" ]; then
        warn "Disk image already exists: $DISK_IMAGE"
        read -p "Recreate disk? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log "Using existing disk."
            return
        fi
        rm "$DISK_IMAGE"
    fi

    log "Creating disk image: $DISK_IMAGE"
    qemu-img create -f qcow2 "$DISK_IMAGE" "$DISK_SIZE"
    log "Disk created ($DISK_SIZE)"
}

install_alpine() {
    log "Installing Alpine Linux to disk..."
    log "This will take a few minutes. Press Ctrl+C to cancel."
    sleep 3

    # Run Alpine setup in QEMU with virtio for installation
    qemu-system-x86_64 \
        -m 1024 \
        -smp 2 \
        -boot d \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0 \
        -device virtio-net-pci,netdev=net0 \
        -nographic \
        << 'INSTALL_SCRIPT'
# Alpine setup script runs inside QEMU during installation
# This is a placeholder - actual installation requires interactive setup
# or pre-configured answer file
INSTALL_SCRIPT

    # For automated install, we need to use answer file
    warn "Interactive installation required. Use manual setup for now."
}

setup_tuskd_service() {
    log "Setting up tuskd service..."

    # Check if tuskd binary exists
    if [ ! -f "$TUSKD_BINARY" ]; then
        warn "tuskd binary not found at $TUSKD_BINARY"
        warn "Build with: GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $TUSKD_BINARY ./cmd/tuskd"
    fi

    # Create OpenRC service script
    init_script="$(tusk_temp_file "tuskd-init")"

    cat > "$init_script" << 'TUSKD_INIT'
#!/bin/sh
# OpenRC init script for tuskd

name=tuskd
description="Tusk container daemon"
command="/tusk/tuskd-amd64"
command_background=true
pidfile="/run/tuskd.pid"

depend() {
    after net
    need localmount
}

start() {
    # Mount 9p shared filesystem
    mkdir -p /tusk
    mount -t 9p -o trans=virtio,version=9p2000.L tusk-data /tusk 2>/dev/null || true

    # Create required directories
    mkdir -p /tusk/containers
    mkdir -p /tusk/state

    # Start tuskd
    start-stop-daemon --start --background --make-pidfile --pidfile $pidfile \
        --exec $command -- $command_args
}

stop() {
    start-stop-daemon --stop --pidfile $pidfile
}
TUSKD_INIT

    log "Created tuskd init script (copy to VM manually)"
    log "Contents at: $init_script"
}

main() {
    log "Tusk VM Setup"
    echo "=============="
    echo ""

    check_requirements
    download_alpine
    create_disk

    echo ""
    log "Setup complete!"
    log ""
    log "Next steps:"
    log "1. Boot Alpine and install to disk: ./scripts/boot-alpine.sh"
    log "2. Setup networking and install tuskd manually"
    log "3. Or use pre-built disk image (coming soon)"
}

main "$@"
