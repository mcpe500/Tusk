#!/bin/bash
# Tusk Auto-Install: Full automated Alpine installation in QEMU
# Runs everything from disk creation to VM with tuskd running

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[TUSK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

TUSK_DIR="$HOME/.tusk"
TUSK_REPO="$HOME/Tusk"
DISK_IMAGE="$TUSK_DIR/vm/disk.qcow2"
ALPINE_ISO="$HOME/alpine-virt-3.19.1-x86_64.iso"
QMP_SOCK="$TUSK_DIR/vm/qmp.sock"
SERIAL_SOCK="$TUSK_DIR/vm/serial.sock"

# Auto-answer for setup-alpine
ANSWERS="
localhost
eth0
dhcp
n
UTC
none
chrony
none
n
openssh
prohibit-password
none
vda
sys
none
none
y
"

check_requirements() {
    log "Checking requirements..."
    if ! command -v qemu-system-x86_64 &> /dev/null; then
        error "QEMU not found. Run: pkg install qemu-system-x86-64 qemu-utils"
        exit 1
    fi
    log "Requirements OK"
}

create_disk() {
    if [ -f "$DISK_IMAGE" ]; then
        warn "Disk already exists: $DISK_IMAGE"
        read -p "Delete and recreate? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -f "$DISK_IMAGE"
        else
            log "Using existing disk"
            return 0
        fi
    fi

    log "Creating VM disk (2GB)..."
    mkdir -p "$(dirname "$DISK_IMAGE")"
    qemu-img create -f qcow2 "$DISK_IMAGE" 2G
    log "Disk created: $DISK_IMAGE"
}

download_iso() {
    if [ -f "$ALPINE_ISO" ]; then
        log "Alpine ISO exists"
        return 0
    fi

    log "Downloading Alpine ISO..."
    curl -L -o "$ALPINE_ISO" "https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-virt-3.19.1-x86_64.iso"
    log "ISO downloaded: $ALPINE_ISO"
}

install_alpine() {
    log "Installing Alpine Linux (automated)..."

    # Kill any existing QEMU
    pkill -f qemu 2>/dev/null || true
    sleep 1

    # Clean up old sockets
    rm -f "$QMP_SOCK" "$SERIAL_SOCK" 2>/dev/null || true

    # Run QEMU with serial input piped from answers
    log "Booting Alpine installer..."
    echo "$ANSWERS" | qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 1024 \
        -smp 2 \
        -nographic \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0 \
        -device virtio-net-pci,netdev=net0 \
        -serial pipe:/tmp/tusk-serial \
        2>&1 | while IFS= read -t 5 line || true; do
            echo "$line"
        done &

    # Wait for installation to complete (this is a simplified approach)
    log "Waiting for installation to complete..."
    log "This may take several minutes..."

    # Alternative: use expect-style script
    # For now, let's use a simpler serial pipe approach

    sleep 120

    log "Installation should be complete. Check VM status."
}

install_alpine_v2() {
    log "Installing Alpine Linux (automated v2)..."

    # Kill any existing QEMU
    pkill -f qemu 2>/dev/null || true
    sleep 2

    # Clean up old sockets
    rm -f "$QMP_SOCK" "$SERIAL_SOCK" 2>/dev/null || true

    # Create FIFO for serial
    FIFO_IN="/tmp/tusk-serial-in"
    FIFO_OUT="/tmp/tusk-serial-out"
    rm -f "$FIFO_IN" "$FIFO_OUT"
    mkfifo "$FIFO_IN" "$FIFO_OUT"

    log "Starting QEMU with serial pipe..."

    # Start QEMU with serial pipe
    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 1024 \
        -smp 2 \
        -nographic \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0 \
        -device virtio-net-pci,netdev=net0 \
        -serial pipe:tusk-serial \
        -boot d \
        &

    QEMU_PID=$!

    # Wait for pipes to be created
    sleep 3

    log "Sending installation answers..."
    echo "$ANSWERS" > "$FIFO_IN" &

    # Monitor output
    log "Monitoring installation progress..."

    # Wait for poweroff or timeout
    TIMEOUT=300
    ELAPSED=0
    while [ $ELAPSED -lt $TIMEOUT ]; do
        if ! kill -0 $QEMU_PID 2>/dev/null; then
            log "QEMU exited"
            break
        fi
        sleep 5
        ELAPSED=$((ELAPSED + 5))
        echo "Still running... ($ELAPSED/$TIMEOUT sec)"
    done

    if [ $ELAPSED -ge $TIMEOUT ]; then
        warn "Installation timed out, killing QEMU"
        kill $QEMU_PID 2>/dev/null || true
    fi

    # Cleanup
    rm -f "$FIFO_IN" "$FIFO_OUT"
    pkill -f qemu 2>/dev/null || true

    log "Installation process completed"
}

manual_install() {
    log "Starting manual installation mode..."
    log "The installer will start in QEMU. Please:"
    log "1. Login as root (no password)"
    log "2. Run: setup-alpine"
    log "3. At disk prompt, choose: vda"
    log "4. Choose: sys"
    log "5. After install: poweroff"
    log ""
    log "Then run this script again with: $0 configure"
    echo ""
    read -p "Press Enter to start installer..."
    echo ""

    # Kill any existing QEMU
    pkill -f qemu 2>/dev/null || true
    sleep 1

    rm -f "$QMP_SOCK" "$SERIAL_SOCK" 2>/dev/null || true

    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 1024 \
        -smp 2 \
        -nographic \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0 \
        -device virtio-net-pci,netdev=net0 \
        -boot d
}

configure_vm() {
    log "Configuring Alpine VM with tuskd..."

    # Make sure VM is running
    if [ ! -S "$SERIAL_SOCK" ]; then
        warn "VM not running. Starting..."
        start_vm
        sleep 5
    fi

    log "VM should be running. Connect via: ./scripts/tusk-vm.sh attach"
    log ""
    log "To configure manually:"
    log "1. Login as root"
    log "2. Run: ~/Tusk/scripts/configure-alpine.sh"
    log "3. Reboot"
}

start_vm() {
    log "Starting VM..."

    pkill -f qemu 2>/dev/null || true
    sleep 1
    rm -f "$QMP_SOCK" "$SERIAL_SOCK" 2>/dev/null || true

    # Copy tuskd to VM directory if not exists
    if [ ! -f "$TUSK_DIR/tuskd-amd64" ] && [ -f "$TUSK_REPO/cmd/tuskd/main.go" ]; then
        log "Building tuskd..."
        cd "$TUSK_REPO"
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$TUSK_DIR/tuskd-amd64" ./cmd/tuskd
    fi

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
        -serial unix:"$SERIAL_SOCK",server,nowait &

    log "VM started (PID: $!)"
    sleep 5
}

test_tuskd() {
    log "Testing tuskd connection..."

    if [ ! -S "$SERIAL_SOCK" ]; then
        error "Serial socket not found. Is VM running?"
        return 1
    fi

    # Try to ping tuskd via JSON-RPC
    echo '{"jsonrpc":"2.0","method":"Ping","params":{},"id":1}' | \
        nc -U "$SERIAL_SOCK" -w 2 | head -1

    if [ $? -eq 0 ]; then
        log "tuskd is responding!"
        return 0
    else
        warn "tuskd not responding yet. VM may still be booting."
        return 1
    fi
}

usage() {
    echo "Tusk Auto-Install"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  all          Full installation (disk + Alpine + config + start)"
    echo "  disk         Create VM disk only"
    echo "  install      Interactive Alpine installation"
    echo "  configure    Configure Alpine with tuskd"
    echo "  start        Start VM"
    echo "  test         Test tuskd connection"
    echo "  attach       Attach to VM serial console"
    echo ""
}

main() {
    if [ $# -eq 0 ]; then
        usage
        exit 0
    fi

    case $1 in
        all)
            check_requirements
            create_disk
            download_iso
            manual_install
            ;;
        disk)
            check_requirements
            create_disk
            ;;
        install)
            check_requirements
            download_iso
            install_alpine_v2
            ;;
        configure)
            configure_vm
            ;;
        start)
            start_vm
            ;;
        test)
            test_tuskd
            ;;
        attach)
            exec ./scripts/tusk-vm.sh attach
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            error "Unknown command: $1"
            usage
            exit 1
            ;;
    esac
}

main "$@"