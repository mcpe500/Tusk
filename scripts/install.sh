#!/usr/bin/env bash
# Tusk Installer - One script to setup everything
# Usage: curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash

set -e

TUSK_DIR="$HOME/.tusk"
TUSK_BIN="$HOME/tusk"
TUSKD_BIN="$TUSK_DIR/tuskd-amd64"

# Check if running from Tusk directory
if [ -f "scripts/install.sh" ] && [ "$(dirname "$0")" = "scripts" ] || [ "$(basename "$0")" = "install.sh" ]; then
    cd "$HOME/Tusk" 2>/dev/null || true
fi

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

    # Copy tuskd to VM directory (skip if already exists)
    mkdir -p "$TUSK_DIR/vm"
    if [ ! -f "$TUSK_DIR/tuskd-amd64" ]; then
        cp "$TUSKD_BIN" "$TUSK_DIR/"
    fi

    log "Tusk initialized!"
}

add_to_path() {
    # Add $HOME to PATH if not already there
    if ! echo "$PATH" | grep -q "$HOME"; then
        echo 'export PATH=$HOME:$PATH' >> ~/.bashrc
        export PATH="$HOME:$PATH"
        warn "Added $HOME to PATH. Restart shell or run: source ~/.bashrc"
    fi
}

print_next_steps() {
    echo ""
    log "Tusk installed successfully!"
    echo ""
    echo "Usage:"
    echo "  ~/tusk <command>"
    echo ""
    echo "Or add to PATH:"
    echo "  echo 'export PATH=\$HOME:\$PATH' >> ~/.bashrc"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Start VM installer (recommended):"
    echo "   ~/tusk install --verbose"
	    echo "   (This downloads the pre-built image and starts the VM automatically)"
    echo ""
    echo "2. Or run manual setup (legacy flow):"
    echo "   ~/Tusk/scripts/tusk-vm.sh create"
    echo "   ~/Tusk/scripts/tusk-vm.sh install"
    echo "   ~/Tusk/scripts/tusk-vm.sh start"
    echo "   # Inside VM: run ~/Tusk/scripts/configure-alpine.sh"
    echo ""
    echo "3. Start using Tusk:"
    echo "   ~/tusk status"
    echo "   ~/tusk pull alpine:latest"
    echo "   ~/tusk run alpine echo hello"
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
    add_to_path
    print_next_steps
}

main "$@"
