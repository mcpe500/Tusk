#!/usr/bin/env bash
# Tusk VM Management Script
# Usage: ./scripts/tusk-vm.sh <command>

set -e

TUSK_DIR="${TUSK_DIR:-$HOME/.tusk}"
DISK_IMAGE="$TUSK_DIR/vm/disk.qcow2"
ALPINE_ISO="$HOME/alpine-virt-3.19.1-x86_64.iso"
QMP_SOCK="$TUSK_DIR/vm/qmp.sock"
SERIAL_SOCK="$TUSK_DIR/vm/serial.sock"
CONSOLE_SOCK="$TUSK_DIR/vm/console.sock"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[TUSK]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_disk() {
    if [ ! -f "$DISK_IMAGE" ]; then
        error "Disk not found. Run: tusk vm create"
        exit 1
    fi
}

check_qemu() {
    if ! command -v qemu-system-x86_64 &> /dev/null; then
        error "QEMU not installed. Run: pkg install qemu-system-x86-64"
        exit 1
    fi
}

cmd_status() {
    check_qemu

    if [ -S "$QMP_SOCK" ]; then
        log "VM: Running"
        python3 << EOF 2>/dev/null
import socket, os
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect(os.path.expanduser("$QMP_SOCK"))
s.recv(1024)
s.send(b'{"execute":"qmp_capabilities","arguments":{},"id":1}\n')
s.recv(1024)
s.send(b'{"execute":"query-status","arguments":{},"id":2}\n')
data = s.recv(1024)
import json
print(json.loads(data)["return"]["status"])
s.close()
EOF
    else
        echo "VM: Stopped"
    fi

    echo "QMP Socket: $QMP_SOCK"
    echo "Serial Socket (API): $SERIAL_SOCK"
    echo "Console Socket: $CONSOLE_SOCK"
}

cmd_start() {
    check_qemu
    check_disk

    log "Starting VM..."

    # Kill existing VM
    pkill -f qemu 2>/dev/null || true
    rm -f "$QMP_SOCK" "$SERIAL_SOCK" "$CONSOLE_SOCK" 2>/dev/null || true

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
        -device virtio-serial-pci \
        -device virtserialport,chardev=ch0,name=tusk0 \
        -chardev socket,id=ch0,path="$SERIAL_SOCK",server,nowait \
        -serial unix:"$CONSOLE_SOCK",server,nowait &

    log "VM started (PID: $!)"
    log "Waiting for boot..."
    sleep 5

    # Try to ping
    python3 << EOF 2>/dev/null
import socket, json, time, os

for i in range(120):
    try:
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.settimeout(1)
        s.connect(os.path.expanduser("$SERIAL_SOCK"))
        s.sendall(json.dumps({"jsonrpc":"2.0","method":"Ping","params":{},"id":1}).encode() + b"\n")
        data = s.recv(1024)
        print("tuskd: OK")
        s.close()
        break
    except:
        time.sleep(1)

if i == 119:
    print("tuskd: Not ready (Alpine may still be booting)")
EOF
}

cmd_stop() {
    log "Stopping VM..."
    pkill -f qemu 2>/dev/null || true
    rm -f "$QMP_SOCK" "$SERIAL_SOCK" "$CONSOLE_SOCK" 2>/dev/null || true
    log "VM stopped"
}

cmd_create() {
    check_qemu

    if [ -f "$DISK_IMAGE" ]; then
        error "Disk already exists: $DISK_IMAGE"
        exit 1
    fi

    log "Creating VM disk (2GB)..."
    qemu-img create -f qcow2 "$DISK_IMAGE" 2G

    mkdir -p "$(dirname "$DISK_IMAGE")"

    log "Disk created: $DISK_IMAGE"
    log ""
    log "To install Alpine:"
    echo "  ./scripts/tusk-vm.sh install"
}

cmd_install() {
    check_qemu

    if [ ! -f "$DISK_IMAGE" ]; then
        error "No disk. Run: tusk vm create"
        exit 1
    fi

    if [ ! -f "$ALPINE_ISO" ]; then
        log "Downloading Alpine ISO..."
        curl -L -o "$ALPINE_ISO" "https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-virt-3.19.1-x86_64.iso"
    fi

    log "Starting Alpine installer..."
    log "When prompted:"
    echo "  - Login: root (no password)"
    echo "  - Run: setup-alpine"
    echo "  - Choose 'virt' as disk"
    echo "  - Use 'sys' install (not 'data')"
    echo "  - After install, run: poweroff"

    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 1024 \
        -smp 2 \
        -nographic \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0 \
        -device virtio-net-pci,netdev=net0

    log "Installation complete!"
}

cmd_attach() {
    check_qemu

    if [ ! -S "$CONSOLE_SOCK" ]; then
        error "VM not running. Run: tusk vm start"
        exit 1
    fi

    log "Connecting to serial console... (Ctrl+C to detach)"
    exec socat - unix:"$CONSOLE_SOCK" 2>/dev/null || \
        python3 << EOF
import socket, os, termios, tty

s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect(os.path.expanduser("$CONSOLE_SOCK"))

# Simple read-only mode
while True:
    import select
    r, _, _ = select.select([s], [], [], 0.1)
    if r:
        data = s.recv(1024)
        if data:
            print(data.decode('utf-8', errors='replace'), end='')
EOF
}

cmd_help() {
    echo "Tusk VM Management"
    echo ""
    echo "Usage: tusk vm <command>"
    echo ""
    echo "Commands:"
    echo "  create    Create new VM disk"
    echo "  install   Run Alpine installer"
    echo "  start     Start VM"
    echo "  stop      Stop VM"
    echo "  status    Show VM status"
    echo "  attach    Attach to serial console"
    echo ""
}

main() {
    if [ $# -eq 0 ]; then
        cmd_help
        exit 0
    fi

    case $1 in
        status) cmd_status ;;
        start) cmd_start ;;
        stop) cmd_stop ;;
        create) cmd_create ;;
        install) cmd_install ;;
        attach) cmd_attach ;;
        help|--help|-h) cmd_help ;;
        *) echo "Unknown command: $1"; cmd_help; exit 1 ;;
    esac
}

main "$@"