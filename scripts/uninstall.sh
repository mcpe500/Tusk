#!/usr/bin/env bash
# Tusk Uninstaller
# This script removes Tusk and all its data from your system.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TUSK_DIR="$HOME/.tusk"
TUSK_REPO="$HOME/Tusk"
TUSK_BINARY="$HOME/tusk"

log() { echo -e "${GREEN}[TUSK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

echo "=================================="
echo "  Tusk Uninstaller"
echo "=================================="
echo ""
warn "This will delete:"
echo "1. Tusk data directory ($TUSK_DIR) - Includes VM disks and containers!"
echo "2. Tusk binary ($TUSK_BINARY)"
echo "3. Tusk source repository ($TUSK_REPO)"
echo ""
read -p "Are you sure you want to proceed? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi

# 1. Stop VM if running
log "Stopping Tusk VM..."
pkill -f qemu-system-x86_64 2>/dev/null || true

# 2. Remove binary
if [ -f "$TUSK_BINARY" ]; then
    log "Removing tusk binary..."
    rm -f "$TUSK_BINARY"
fi

# 3. Remove data directory
if [ -d "$TUSK_DIR" ]; then
    log "Removing tusk data directory..."
    rm -rf "$TUSK_DIR"
fi

# 4. Remove repository
if [ -d "$TUSK_REPO" ]; then
    log "Removing tusk repository..."
    rm -rf "$TUSK_REPO"
fi

# 5. Optional: Remove ISO files
log "Checking for Alpine ISO files..."
rm -f "$HOME"/alpine-virt-*.iso 2>/dev/null || true

echo ""
log "Tusk has been uninstalled successfully!"
echo "Note: If you added '~/tusk' to your PATH in ~/.bashrc or ~/.zshrc, you may want to remove it manually."
