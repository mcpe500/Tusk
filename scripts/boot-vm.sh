#!/usr/bin/env bash
# Tusk Boot Script - Boots Alpine VM from qcow2 disk
# Usage: ./scripts/boot-vm.sh [--install] [--tuskd]

set -e

TUSK_DIR="$HOME/.tusk"
DISK_IMAGE="$TUSK_DIR/vm/disk.qcow2"
ALPINE_ISO="$HOME/alpine-virt-3.19.1-x86_64.iso"
TUSKD_BINARY="$TUSK_DIR/tuskd-amd64"

QMP_SOCK="$TUSK_DIR/vm/qmp.sock"
SERIAL_SOCK="$TUSK_DIR/vm/serial.sock"

# Defaults
MODE="boot"  # boot, install, interactive
INSTALL_TUSKD=false

# Colors
GREEN='\033[0;32m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[TUSK]${NC} $1"
}

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --install     Run Alpine installation (first time only)"
    echo "  --tuskd       Inject tuskd binary to disk"
    echo "  --interactive Boot in interactive mode (QEMU GUI)"
    echo "  --help        Show this help"
    echo ""
    echo "Examples:"
    echo "  $0                    # Boot from disk"
    echo "  $0 --install           # Install Alpine to disk"
    echo "  $0 --boot --tuskd      # Boot and inject tuskd"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --install)
            MODE="install"
            shift
            ;;
        --tuskd)
            INSTALL_TUSKD=true
            shift
            ;;
        --interactive)
            # Remove -nographic for GUI mode
            shift
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

check_disk() {
    if [ ! -f "$DISK_IMAGE" ]; then
        echo "Error: Disk image not found at $DISK_IMAGE"
        echo "Run 'tusk vm create' or 'tusk setup' first"
        exit 1
    fi
}

boot_from_disk() {
    log "Booting Alpine from disk..."

    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 512 \
        -smp 2 \
        -nographic \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0,hostfwd=tcp::8080-:80 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path="$TUSK_DIR",mount_tag=tusk-data,security_model=mapped,id=tusk \
        -qmp unix:"$QMP_SOCK",server,nowait \
        -serial unix:"$SERIAL_SOCK",server,nowait \
        "$@"
}

boot_from_iso() {
    log "Booting Alpine from ISO..."

    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 512 \
        -smp 2 \
        -nographic \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0,hostfwd=tcp::8080-:80 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path="$TUSK_DIR",mount_tag=tusk-data,security_model=mapped,id=tusk \
        -qmp unix:"$QMP_SOCK",server,nowait \
        -serial unix:"$SERIAL_SOCK",server,nowait \
        "$@"
}

inject_tuskd() {
    if [ ! -f "$TUSKD_BINARY" ]; then
        log "tuskd binary not found. Building..."
        cd "$(dirname "$0")/.."
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$TUSKD_BINARY" ./cmd/tuskd
    fi

    log "tuskd binary ready at $TUSKD_BINARY"
    log "To inject into VM, mount the disk and copy manually:"
    log "  qemu-nbd --connect=/dev/nbd0 $DISK_IMAGE"
    log "  mount /dev/nbd0p1 /mnt"
    log "  cp $TUSKD_BINARY /mnt/tusk/"
    log "  umount /mnt"
    log "  qemu-nbd --disconnect /dev/nbd0"
}

main() {
    log "Tusk VM Boot"
    echo "============="

    case $MODE in
        install)
            check_disk
            boot_from_iso
            ;;
        boot)
            check_disk
            boot_from_disk
            ;;
        interactive)
            check_disk
            boot_from_disk
            ;;
    esac

    if [ "$INSTALL_TUSKD" = true ]; then
        inject_tuskd
    fi
}

main "$@"