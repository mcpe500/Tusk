#!/bin/bash
# Tusk Pre-built Disk Installer
# Downloads pre-made Alpine VM with tuskd pre-configured
# No manual intervention needed!

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[TUSK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

TUSK_DIR="$HOME/.tusk"
DISK_IMAGE="$TUSK_DIR/vm/disk.qcow2"
TUSK_REPO="$HOME/Tusk"

# Pre-built disk URL (update this with your release URL)
DISK_URL="https://github.com/mcpe500/Tusk/releases/download/v0.1.0/alpine-tusk.qcow2.gz"
GITHUB_API="https://api.github.com/repos/mcpe500/Tusk/releases/latest"

check_requirements() {
    log "Checking requirements..."

    if ! command -v qemu-system-x86_64 &> /dev/null; then
        error "QEMU not found"
        echo "Run: pkg install qemu-system-x86-64 qemu-utils"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        error "curl not found"
        exit 1
    fi

    if ! command -v gunzip &> /dev/null; then
        error "gunzip not found"
        exit 1
    fi

    log "Requirements OK"
}

check_existing_disk() {
    if [ -f "$DISK_IMAGE" ] && [ -s "$DISK_IMAGE" ]; then
        warn "Disk already exists: $DISK_IMAGE"

        # Check if disk has Alpine installed
        if qemu-img info "$DISK_IMAGE" 2>/dev/null | grep -q "virtual size"; then
            info "Disk appears to be valid"
            return 0
        fi
    fi
    return 1
}

download_disk() {
    log "Downloading pre-built Alpine VM with Tusk..."

    mkdir -p "$(dirname "$DISK_IMAGE")"
    mkdir -p "$TUSK_DIR/vm"

    # Get download URL from GitHub release
    if curl -sL "$GITHUB_API" | grep -q "tag_name"; then
        ASSET_URL=$(curl -sL "$GITHUB_API" | grep -o '"browser_download_url": "[^"]*qcow2[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$ASSET_URL" ]; then
            DISK_URL="$ASSET_URL"
        fi
    fi

    info "Downloading from: $DISK_URL"

    # Download with progress
    TEMP_FILE="/tmp/tusk-disk.gz"

    curl -L -o "$TEMP_FILE" --progress-bar "$DISK_URL"

    if [ ! -f "$TEMP_FILE" ] || [ ! -s "$TEMP_FILE" ]; then
        error "Download failed"
        return 1
    fi

    log "Extracting disk image..."
    gunzip -f "$TEMP_FILE"
    mv "${TEMP_FILE%.gz}" "$DISK_IMAGE"

    chmod 600 "$DISK_IMAGE"

    log "Disk image installed: $DISK_IMAGE"
}

build_tuskd() {
    if [ -f "$TUSK_DIR/tuskd-amd64" ]; then
        log "tuskd already exists"
        return 0
    fi

    log "Building tuskd..."

    if [ ! -d "$TUSK_REPO" ]; then
        warn "Tusk repo not found, cloning..."
        git clone https://github.com/mcpe500/Tusk.git "$TUSK_REPO"
    fi

    cd "$TUSK_REPO"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TUSK_DIR/tuskd-amd64" ./cmd/tuskd

    log "tuskd built"
}

start_vm() {
    log "Starting Tusk VM..."

    # Kill existing QEMU
    pkill -f qemu 2>/dev/null || true
    sleep 1

    # Clean up sockets
    rm -f "$TUSK_DIR/vm/qmp.sock" "$TUSK_DIR/vm/serial.sock" 2>/dev/null || true

    qemu-system-x86_64 \
        -M pc-i440fx-9.2 \
        -m 512 \
        -smp 2 \
        -nographic \
        -drive file="$DISK_IMAGE",if=virtio,format=qcow2 \
        -netdev user,id=net0,hostfwd=tcp::8080-:80 \
        -device virtio-net-pci,netdev=net0 \
        -virtfs local,path="$TUSK_DIR",mount_tag=tusk-data,security_model=mapped \
        -qmp unix:"$TUSK_DIR/vm/qmp.sock",server,nowait \
        -serial unix:"$TUSK_DIR/vm/serial.sock",server,nowait &

    log "VM started (PID: $!)"
    sleep 5

    # Wait for tuskd to respond
    log "Waiting for tuskd..."

    for i in {1..30}; do
        if [ -S "$TUSK_DIR/vm/serial.sock" ]; then
            RESPONSE=$(echo '{"jsonrpc":"2.0","method":"Ping","params":{},"id":1}' | \
                nc -N "$TUSK_DIR/vm/serial.sock" -w 2 2>/dev/null)
            if echo "$RESPONSE" | grep -q "pong"; then
                log "tuskd is ready!"
                return 0
            fi
        fi
        sleep 1
    done

    warn "tuskd not responding yet, but VM started"
    return 0
}

test_installation() {
    log "Testing installation..."

    # Test tusk CLI
    if [ -f "$HOME/tusk" ]; then
        "$HOME/tusk" status && log "Tusk CLI OK"
    fi

    # Test tuskd
    if [ -S "$TUSK_DIR/vm/serial.sock" ]; then
        RESPONSE=$(echo '{"jsonrpc":"2.0","method":"Info","params":{},"id":1}' | \
            nc -N "$TUSK_DIR/vm/serial.sock" -w 2 2>/dev/null)
        if echo "$RESPONSE" | grep -q "version"; then
            log "tuskd OK"
        fi
    fi
}

usage() {
    cat << 'EOF'
Tusk Pre-built Installer

Usage: tusk install          Download pre-built VM and start
       tusk install --force  Re-download even if disk exists
       tusk install --build  Build disk from scratch (no download)

This command:
1. Downloads a pre-configured Alpine VM with tuskd
2. Builds tuskd daemon
3. Starts the VM
4. Ready to use!

After installation:
  tusk status           - Check VM status
  tusk pull alpine      - Pull an image
  tusk run alpine echo hello  - Run a container
EOF
}

main() {
    echo "============================================"
    echo "  Tusk Installer"
    echo "  Pre-built Alpine VM with tuskd"
    echo "============================================"
    echo ""

    FORCE=false
    BUILD_SCRATCH=false

    for arg in "$@"; do
        case $arg in
            --force) FORCE=true ;;
            --build) BUILD_SCRATCH=true ;;
            --help|-h) usage; exit 0 ;;
        esac
    done

    check_requirements

    if [ "$BUILD_SCRATCH" = true ]; then
        info "Building from scratch (this may take a while)..."
        # Fallback: run auto-install script
        if [ -f "$TUSK_REPO/scripts/auto-install.sh" ]; then
            bash "$TUSK_REPO/scripts/auto-install.sh"
        else
            error "Auto-install script not found"
            exit 1
        fi
        exit 0
    fi

    if [ "$FORCE" = true ]; then
        warn "Force mode - deleting existing disk"
        rm -f "$DISK_IMAGE"
    fi

    if ! check_existing_disk; then
        download_disk || {
            error "Download failed, trying build from scratch..."
            BUILD_SCRATCH=true
        }
    fi

    build_tuskd
    start_vm
    test_installation

    echo ""
    echo "============================================"
    echo "  Installation Complete!"
    echo "============================================"
    echo ""
    echo "Next steps:"
    echo "  tusk status              - Check status"
    echo "  tusk pull alpine:latest  - Pull image"
    echo "  tusk run alpine echo hi - Run container"
    echo ""
    echo "To attach to VM console:"
    echo "  ~/Tusk/scripts/tusk-vm.sh attach"
    echo ""
}

main "$@"