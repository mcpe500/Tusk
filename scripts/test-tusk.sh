#!/bin/bash
# Tusk Quick Test Script
# Run this to verify basic functionality

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }
log_info() { echo -e "[INFO] $1"; }

cd "$(dirname "$0")/.."

echo "========================================"
echo "  Tusk Quick Test Suite"
echo "========================================"
echo ""

# Test 1: Build tusk CLI
log_info "Test 1: Building tusk CLI..."
if go build -o "$HOME/tusk-test" ./cmd/tusk 2>/dev/null; then
    log_pass "Build successful"
else
    log_fail "Build failed"
    exit 1
fi

# Test 2: Version command
log_info "Test 2: Testing version command..."
if $HOME/tusk-test version 2>&1 | grep -q "tusk version"; then
    log_pass "Version command works"
else
    log_fail "Version command failed"
fi

# Test 3: Help command
log_info "Test 3: Testing help command..."
if $HOME/tusk-test --help 2>&1 | grep -q "Tusk - Container runtime"; then
    log_pass "Help command works"
else
    log_fail "Help command failed"
fi

# Test 4: Init command
log_info "Test 4: Testing init command..."
$HOME/tusk-test init >/dev/null 2>&1
if [ -d "$HOME/.tusk/images" ]; then
    log_pass "Init command works"
else
    log_fail "Init command failed"
fi

# Test 5: Status command
log_info "Test 5: Testing status command..."
if $HOME/tusk-test status 2>&1 | grep -q "VM Status:"; then
    log_pass "Status command works"
else
    log_fail "Status command failed"
fi

# Test 6: Images command
log_info "Test 6: Testing images command..."
if $HOME/tusk-test images 2>&1 | grep -q "images\|found"; then
    log_pass "Images command works"
else
    log_fail "Images command failed"
fi

# Test 7: tuskd simulation mode
log_info "Test 7: Testing tuskd simulation..."
echo -e "ping\ninfo\nexit" | go run ./cmd/tuskd 2>&1 | grep -q "pong"
if [ $? -eq 0 ]; then
    log_pass "tuskd simulation works"
else
    log_fail "tuskd simulation failed"
fi

# Test 8: Scripts syntax
log_info "Test 8: Checking scripts syntax..."
all_ok=true
for script in scripts/*.sh; do
    if ! bash -n "$script" 2>/dev/null; then
        log_fail "Script $script has syntax errors"
        all_ok=false
    fi
done
if $all_ok; then
    log_pass "All scripts have valid syntax"
fi

# Test 9: VM disk exists
log_info "Test 9: Checking VM disk..."
if [ -f "$HOME/.tusk/vm/disk.qcow2" ]; then
    log_pass "VM disk exists"
else
    log_fail "VM disk not found (run: ./scripts/tusk-vm.sh create)"
fi

# Test 10: Alpine ISO exists
log_info "Test 10: Checking Alpine ISO..."
if [ -f "$HOME/alpine-virt-3.19.1-x86_64.iso" ]; then
    log_pass "Alpine ISO exists"
else
    log_fail "Alpine ISO not found"
fi

# Cleanup
rm -f $HOME/tusk-test

echo ""
echo "========================================"
echo "  Quick Test Complete"
echo "========================================"
echo ""
echo "Next steps:"
echo "1. Install Alpine to VM disk:"
echo "   ./scripts/tusk-vm.sh install"
echo ""
echo "2. After Alpine installation, configure:"
echo "   ./scripts/configure-alpine.sh"
echo ""
echo "3. Start VM:"
echo "   ./scripts/tusk-vm.sh start"
echo ""
echo "4. Test container:"
echo "   ./tusk run alpine echo hello"
echo ""
