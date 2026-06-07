#!/usr/bin/env bash
# Tusk Auto-Install - Fully automated Alpine installation in QEMU
# Run this ONCE after installing Tusk: tusk install
# No manual intervention needed!

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/temp.sh"

log() { echo -e "${GREEN}[TUSK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

TUSK_DIR="$HOME/.tusk"
TUSK_REPO="$HOME/Tusk"
DISK_IMAGE="$TUSK_DIR/vm/disk.qcow2"
ALPINE_ISO="$HOME/alpine-virt-3.19.1-x86_64.iso"
QMP_SOCK="$TUSK_DIR/vm/qmp.sock"
SERIAL_SOCK="$TUSK_DIR/vm/serial.sock"
CONSOLE_SOCK="$TUSK_DIR/vm/console.sock"
INPUT_FIFO=""

# Auto-answer file for setup-alpine
ANSWERS_FILE="$(tusk_temp_file "tusk-alpine-answers")"

check_requirements() {
    log "Checking requirements..."

    # Check QEMU
    if ! command -v qemu-system-x86_64 &> /dev/null; then
        error "QEMU not found. Run: pkg install qemu-system-x86-64 qemu-utils"
        exit 1
    fi

    if ! command -v qemu-img &> /dev/null; then
        error "qemu-img not found. Run: pkg install qemu-utils"
        exit 1
    fi

    # Check Go
    if ! command -v go &> /dev/null; then
        error "Go not found. Run: pkg install golang"
        exit 1
    fi

    # Check Tusk repo
    if [ ! -d "$TUSK_REPO" ]; then
        error "Tusk repo not found at $TUSK_REPO"
        error "Run: curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash"
        exit 1
    fi

    # Build tuskd if not exists
    if [ ! -f "$TUSK_DIR/tuskd-amd64" ]; then
        log "Building tuskd..."
        cd "$TUSK_REPO"
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TUSK_DIR/tuskd-amd64" ./cmd/tuskd
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
        log "Alpine ISO exists: $ALPINE_ISO"
        return 0
    fi

    log "Downloading Alpine ISO..."
    curl -L -o "$ALPINE_ISO" "https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-virt-3.19.1-x86_64.iso"
    log "ISO downloaded: $ALPINE_ISO"
}

cleanup() {
    log "Cleaning up..."
    pkill -f qemu 2>/dev/null || true
    rm -f "$QMP_SOCK" "$SERIAL_SOCK" "$CONSOLE_SOCK" 2>/dev/null || true
    rm -f "$ANSWERS_FILE" 2>/dev/null || true
    rm -f "$INPUT_FIFO" 2>/dev/null || true
}

# Create answers file for setup-alpine
create_answers() {
    cat > "$ANSWERS_FILE" << 'EOF'

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
EOF
}

run_installer() {
    log "Starting Alpine installer..."
    log "This will take a few minutes. Please wait..."

    if [ ! -f "$DISK_IMAGE" ] || [ ! -s "$DISK_IMAGE" ]; then
        error "Disk image not found: $DISK_IMAGE"
        return 1
    fi

    cleanup
    create_answers

    # Wait for network
    log "Waiting for network..."
    for i in {1..30}; do
        if ping -c 1 -W 1 8.8.8.8 &>/dev/null; then
            break
        fi
        sleep 1
    done

    # Create a named pipe for input
    INPUT_FIFO="$(tusk_temp_fifo "tusk-install-input")"
    rm -f "$INPUT_FIFO"
    mkfifo "$INPUT_FIFO"

    # Start QEMU with serial output piped to auto-answer
    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 1024 \
        -smp 2 \
        -nographic \
        -cdrom "$ALPINE_ISO" \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path="$TUSK_DIR",mount_tag=tusk-data,security_model=mapped \
        -serial unix:"$SERIAL_SOCK",server,nowait \
        2>&1 &

    QEMU_PID=$!

    log "QEMU started (PID: $QEMU_PID)"
    if ! kill -0 "$QEMU_PID" 2>/dev/null; then
        error "QEMU exited before installation completed"
        return 1
    fi

    # Wait for serial socket to appear
    log "Waiting for serial socket..."
    for i in {1..30}; do
        if ! kill -0 "$QEMU_PID" 2>/dev/null; then
            log "QEMU process ended before serial socket available"
            return 1
        fi

        if [ -S "$SERIAL_SOCK" ]; then
            break
        fi
        sleep 1
    done

    # Feed answers to the installer via serial
    sleep 5  # Wait for boot

    log "Sending installation answers..."

    # Read and send each line with delay
    while IFS= read -r line; do
        echo "$line" | nc -N "$SERIAL_SOCK" 2>/dev/null || true
        sleep 2  # Wait for each prompt
    done < "$ANSWERS_FILE"

    # Wait for installation to complete
    log "Installation in progress... (this may take 5-10 minutes)"

    # Monitor for completion
    INSTALL_TIMEOUT=1200
    ELAPSED=0
    while [ $ELAPSED -lt $INSTALL_TIMEOUT ]; do
        if ! kill -0 $QEMU_PID 2>/dev/null; then
            log "QEMU process ended"
            break
        fi

        # Try to detect installation completion
        if [ -S "$SERIAL_SOCK" ]; then
            echo "poweroff" | nc -N "$SERIAL_SOCK" 2>/dev/null || true
        fi

        sleep 10
        ELAPSED=$((ELAPSED + 10))
        echo "Still installing... ($ELAPSED/${INSTALL_TIMEOUT}s)"
    done

    if ! kill -0 $QEMU_PID 2>/dev/null; then
        if [ "$ELAPSED" -lt 120 ]; then
            error "QEMU exited too early before installation could finish"
            return 1
        fi
    else
        warn "Installation taking too long, forcing shutdown..."
        kill -9 $QEMU_PID 2>/dev/null || true
        return 1
    fi

    cleanup

    log "Installation phase complete!"
}

configure_vm() {
    log "Configuring VM with tuskd..."

    if [ ! -f "$DISK_IMAGE" ] || [ ! -s "$DISK_IMAGE" ]; then
        error "Disk image not found: $DISK_IMAGE"
        return 1
    fi

    cleanup

    # Start VM for configuration
    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 512 \
        -smp 2 \
        -nographic \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0,hostfwd=tcp::8080-:80 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path="$TUSK_DIR",mount_tag=tusk-data,security_model=mapped \
        -qmp unix:"$QMP_SOCK",server,nowait \
        -device virtio-serial-pci \
        -device virtserialport,chardev=ch0,name=tusk0 \
        -chardev socket,id=ch0,path="$SERIAL_SOCK",server,nowait \
        -serial unix:"$CONSOLE_SOCK",server,nowait &

    VM_PID=$!
    if ! kill -0 "$VM_PID" 2>/dev/null; then
        error "QEMU exited before configuration started"
        return 1
    fi

    log "VM started for configuration (PID: $VM_PID)"

    if [ ! -S "$CONSOLE_SOCK" ]; then
        warn "Console socket did not appear; configuration may fail"
    fi

    # Wait for boot
    log "Waiting for VM to boot..."
    sleep 10

    # Wait for console socket
    for i in {1..120}; do
        if [ -S "$CONSOLE_SOCK" ]; then
            break
        fi
        sleep 1
    done

    log "VM booted. Sending configuration commands..."

    # Send configure script via serial
    {
        sleep 5
        echo ""
        sleep 2

        # Create tuskd init script directly
        cat << 'CONFIG_SCRIPT' | nc -N "$CONSOLE_SOCK" 2>/dev/null || true

# Wait for login
sleep 5

# Login
echo "root" | nc -N "$CONSOLE_SOCK" 2>/dev/null || true
sleep 2

# Set password (simple for auto-setup)
echo "echo 'root:tusk' | chpasswd" | nc -N "$CONSOLE_SOCK" 2>/dev/null || true
sleep 1

# Create tuskd init script
cat > /etc/init.d/tuskd << 'TUSKD_EOF'
#!/bin/sh
name=tuskd
description="Tusk container runtime daemon"
command="/usr/local/bin/tuskd"
command_args="--device /dev/virtio-ports/tusk0"
command_background=true
pidfile="/run/tuskd.pid"

depend() {
    after netmount localmount
    need localmount
}

start() {
    mkdir -p /tusk
    mount -t 9p -o trans=virtio,version=9p2000.L tusk-data /tusk 2>/dev/null || true
    mkdir -p /tusk/containers 2>/dev/null || true
    mkdir -p /tusk/state 2>/dev/null || true

    if [ -f "/tusk/tuskd-amd64" ]; then
        cp /tusk/tuskd-amd64 /usr/local/bin/tuskd
        chmod +x /usr/local/bin/tuskd
    fi

    ebegin "Starting tuskd"
    start-stop-daemon --start --background --make-pidfile --pidfile $pidfile \
        --exec $command -- $command_args
    eend $?
}

stop() {
    ebegin "Stopping tuskd"
    start-stop-daemon --stop --pidfile $pidfile
    eend $?
}
TUSKD_EOF

chmod +x /etc/init.d/tuskd
rc-update add tuskd default 2>/dev/null || true

# Enable serial console
echo "ttyS0" >> /etc/securetty
if ! grep -q "ttyS0" /etc/inittab; then
    echo "ttyS0::respawn:/sbin/getty -L ttyS0 115200 vt100" >> /etc/inittab
fi

# Allow root login
sed -i 's/^root:\*:/root::/' /etc/shadow

# Reboot
reboot
CONFIG_SCRIPT

        sleep 5
    } &

    # Wait for configuration
    log "Configuring... (this may take a few minutes)"
    sleep 120

    # Kill VM
    if kill -0 $VM_PID 2>/dev/null; then
        kill -9 $VM_PID 2>/dev/null || true
    fi

    cleanup

    log "Configuration complete!"
}

start_vm() {
    log "Starting Tusk VM..."

    if [ ! -f "$DISK_IMAGE" ] || [ ! -s "$DISK_IMAGE" ]; then
        error "Disk image not found: $DISK_IMAGE"
        return 1
    fi

    cleanup

    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 512 \
        -smp 2 \
        -nographic \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0,hostfwd=tcp::8080-:80 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path="$TUSK_DIR",mount_tag=tusk-data,security_model=mapped \
        -qmp unix:"$QMP_SOCK",server,nowait \
        -device virtio-serial-pci \
        -device virtserialport,chardev=ch0,name=tusk0 \
        -chardev socket,id=ch0,path="$SERIAL_SOCK",server,nowait \
        -serial unix:"$CONSOLE_SOCK",server,nowait &

    VM_PID=$!
    if ! kill -0 "$VM_PID" 2>/dev/null; then
        error "QEMU exited before vm started"
        return 1
    fi

    log "VM started (PID: $VM_PID)"
    log "Waiting for tuskd..."
    sleep 5

    # Test tuskd
    for i in {1..120}; do
        if [ -S "$SERIAL_SOCK" ]; then
            if echo '{"jsonrpc":"2.0","method":"Ping","params":{},"id":1}' | \
                nc -N "$SERIAL_SOCK" -w 2 | grep -q "pong\|result"; then
                log "tuskd responding!"
                return 0
            fi
        fi
        sleep 1
    done

    warn "tuskd not responding"
    return 1
}

main() {
    echo "============================================"
    echo "  Tusk Auto-Install"
    echo "  Fully Automated Alpine Setup"
    echo "============================================"
    echo ""

    check_requirements
    create_disk
    download_iso
    create_answers

    echo ""
    echo "Starting installation..."
    echo "This will:"
    echo "  1. Boot Alpine from ISO"
    echo "  2. Run setup-alpine automatically"
    echo "  3. Install to disk"
    echo "  4. Configure tuskd"
    echo "  5. Start the VM"
    echo ""
    echo "No manual intervention needed!"
    echo ""

    run_installer || exit 1
    configure_vm || exit 1
    start_vm || exit 1

    echo ""
    echo "============================================"
    echo "  Installation Complete!"
    echo "============================================"
    echo ""
    echo "Usage:"
    echo "  tusk status    - Check VM status"
    echo "  tusk ps         - List containers"
    echo "  tusk pull alpine:latest"
    echo "  tusk run alpine echo hello"
    echo ""
}

main "$@"
