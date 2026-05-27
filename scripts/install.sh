#!/bin/bash
# Tusk Installer - One script to setup everything
# Usage: curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash

set -e

TUSK_DIR="$HOME/.tusk"
TUSK_BIN="$HOME/tusk"
TUSKD_BIN="$TUSK_DIR/tuskd-amd64"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[TUSK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

check_requirements() {
    log "Checking requirements..."

    # Check Termux
    if [ ! -d "/data/data/com.termux" ]; then
        error "This script is designed for Termux on Android"
        exit 1
    fi

    # Check for QEMU
    if ! command -v qemu-system-x86_64 &> /dev/null; then
        log "Installing QEMU..."
        pkg update && pkg install -y qemu-system-x86-64 qemu-utils
    fi

    # Check for Go
    if ! command -v go &> /dev/null; then
        log "Installing Go..."
        pkg update && pkg install -y golang
    fi

    # Check for git
    if ! command -v git &> /dev/null; then
        log "Installing git..."
        pkg update && pkg install -y git
    fi

    log "All requirements satisfied!"
}

clone_or_update() {
    log "Getting Tusk source..."

    if [ -d "$HOME/Tusk" ]; then
        cd "$HOME/Tusk"
        git pull
        log "Updated Tusk"
    else
        git clone https://github.com/mcpe500/Tusk.git "$HOME/Tusk"
        cd "$HOME/Tusk"
        log "Cloned Tusk"
    fi
}

build_tusk() {
    log "Building tusk..."

    cd "$HOME/Tusk"

    # Build host CLI
    go build -o "$TUSK_BIN" ./cmd/tusk

    # Build VM daemon (x86_64 Linux)
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$TUSKD_BIN" ./cmd/tuskd

    chmod +x "$TUSK_BIN" "$TUSKD_BIN"

    log "Built tusk ($TUSK_BIN)"
    log "Built tuskd ($TUSKD_BIN)"
}

init_tusk() {
    log "Initializing Tusk..."

    "$TUSK_BIN" init

    # Copy tuskd to VM directory
    mkdir -p "$TUSK_DIR/vm"
    cp "$TUSKD_BIN" "$TUSK_DIR/"

    log "Tusk initialized!"
}

print_next_steps() {
    echo ""
    log "Tusk installed successfully!"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Setup Alpine VM disk:"
    echo "   ./scripts/setup-vm.sh"
    echo ""
    echo "2. Or use pre-built disk (when available):"
    echo "   # Download from releases"
    echo ""
    echo "3. Start Tusk:"
    echo "   tusk init"
    echo "   tusk start"
    echo "   tusk status"
    echo ""
    echo "4. Run a container:"
    echo "   tusk pull alpine:latest"
    echo "   tusk run alpine echo hello"
    echo ""
}

main() {
    echo "=================================="
    echo "  Tusk Installer for Termux"
    echo "=================================="
    echo ""

    check_requirements
    clone_or_update
    build_tusk
    init_tusk
    print_next_steps
}

main "$@"