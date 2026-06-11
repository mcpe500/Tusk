#!/usr/bin/env bash
# Tusk Installer — one command to get Tusk running on Termux
# Usage: curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash
set -euo pipefail

TUSK_DIR="$HOME/.tusk"
TUSK_BIN="$HOME/tusk"
TUSK_REPO="$HOME/Tusk"
TUSKD_LOCAL="$TUSK_DIR/tuskd"

# ── Colors (TTY only) ─────────────────────────────────────────────
if [ -t 1 ]; then R='\033[0;31m'; G='\033[0;32m'; Y='\033[1;33m'; B='\033[1;36m'; N='\033[0m'
else                R='';           G='';           Y='';           B='';           N=''; fi

log()  { printf "${G}[tusk]${N} %s\n" "$1"; }
warn() { printf "${Y}[warn]${N} %s\n" "$1"; }
err()  { printf "${R}[err]${N}  %s\n" "$1"; }
step() { printf "${B}[>>]${N}   %s\n" "$1"; }

# ── 1. Termux guard ───────────────────────────────────────────────
[ -d "/data/data/com.termux" ] || { err "Tusk requires Termux on Android."; exit 1; }
command -v pkg >/dev/null 2>&1 || { err "pkg not found."; exit 1; }

# ── 2. Install deps (one shot) ───────────────────────────────────
step "Installing system packages..."
DEPS=(golang git curl proot)
[ "$(uname -m)" != "aarch64" ] && DEPS+=(qemu-user-x86-64)
command -v nc >/dev/null 2>&1 || command -v ncat >/dev/null 2>&1 || DEPS+=(nmap)
pkg update -y -q 2>/dev/null || true
pkg install -y -q "${DEPS[@]}" 2>/dev/null || pkg install -y "${DEPS[@]}"
log "Dependencies installed"

# ── 3. Clone / update source ──────────────────────────────────────
step "Fetching Tusk source..."
if [ -d "$TUSK_REPO/.git" ]; then
    (cd "$TUSK_REPO" && git pull --ff-only -q 2>/dev/null) || warn "git pull failed — using local copy"
else
    git clone --depth 1 -q https://github.com/mcpe500/Tusk.git "$TUSK_REPO"
fi
log "Source ready"

# ── 4. Build tusk CLI (native ARM) ────────────────────────────────
step "Building tusk CLI..."
cd "$TUSK_REPO"
go build -o "$TUSK_BIN" ./cmd/tusk
chmod +x "$TUSK_BIN"
log "tusk → $TUSK_BIN"

# ── 5. Build tuskd (native proot runtime) ───────────────────────
step "Building tuskd (native proot runtime)..."
mkdir -p "$TUSK_DIR/vm"
go build -o "$TUSKD_LOCAL" ./cmd/tuskd
chmod +x "$TUSKD_LOCAL"
log "tuskd → $TUSKD_LOCAL"

# ── 6. init ───────────────────────────────────────────────────────
"$TUSK_BIN" init 2>/dev/null || true

# ── 7. Add ~/ to PATH ─────────────────────────────────────────────
case ":$PATH:" in
    *":$HOME:"*) ;;
    *)
        echo 'export PATH="$HOME:$PATH"' >> "$HOME/.bashrc" 2>/dev/null
        # Also write .bash_profile so non-interactive shells pick it up.
        grep -qF 'PATH="$HOME:$PATH"' "$HOME/.bash_profile" 2>/dev/null || \
            echo 'export PATH="$HOME:$PATH"' >> "$HOME/.bash_profile" 2>/dev/null
        export PATH="$HOME:$PATH"
        ;;
esac

# ── 8. Kill stale processes & sockets ─────────────────────────────
pkill -f "tuskd" 2>/dev/null || true
sleep 1
rm -f "$TUSK_DIR/vm/serial.sock" "$TUSK_DIR/vm/qmp.sock" "$TUSK_DIR/vm/console.sock"

# ── 9. Start tuskd (proot runtime) ──────────────────────────────
step "Starting tuskd (proot runtime)..."
nohup "$TUSKD_LOCAL" --socket "$TUSK_DIR/vm/serial.sock" > "$TUSK_DIR/tuskd.log" 2>&1 &
TUSKD_PID=$!
disown $TUSKD_PID

# Wait up to 10s for socket
for i in $(seq 1 20); do
    if [ -S "$TUSK_DIR/vm/serial.sock" ]; then
        break
    fi
    sleep 0.5
done

if ! kill -0 "$TUSKD_PID" 2>/dev/null; then
    err "tuskd failed to start"
    exit 1
fi
log "tuskd running (PID $TUSKD_PID)"

# ── 10. Verify ────────────────────────────────────────────────────
step "Verifying..."
"$TUSK_BIN" rpc Ping 2>&1 | grep -q "pong" && log "Ping OK" || { err "Ping failed"; exit 1; }
step "Pulling alpine image for smoke test..."
"$TUSK_BIN" pull alpine 2>&1 | tail -1
"$TUSK_BIN" run alpine echo ok 2>&1 | grep -q "ok" && log "Container smoke test OK" || warn "Smoke test failed — check tuskd.log for details"

# ── Done ──────────────────────────────────────────────────────────
printf "\n${G}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}\n"
printf "${G}  Tusk installed — native proot runtime ready!${N}\n"
printf "${G}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}\n\n"
printf "  tusk pull alpine          Pull an image\n"
printf "  tusk run alpine sh        Run a container\n"
printf "  tusk compose up -d        Launch a compose stack\n"
printf "  tusk ps                   List containers\n"
printf "  tusk compose down         Tear down\n"
printf "  tusk uninstall -y         Remove everything\n\n"
