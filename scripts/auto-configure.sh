#!/bin/sh
# Tusk Alpine Auto-Configure Script
# Runs inside Alpine VM to fully configure it for Tusk
# This script does NOT use setup-alpine - it configures everything manually

set -e

TUSK_INIT_SCRIPT="/etc/init.d/tuskd"

echo "============================================"
echo "  Tusk Alpine Auto-Configuration"
echo "============================================"
echo ""

# Wait for network
echo "[1/8] Waiting for network..."
while ! ping -c 1 -W 1 8.8.8.8 >/dev/null 2>&1; do
    sleep 1
done
echo "Network is up"

# Set hostname
echo "[2/8] Setting hostname..."
echo "tusk-vm" > /etc/hostname
hostname -F /etc/hostname

# Configure networking
echo "[3/8] Configuring networking..."
cat > /etc/network/interfaces << 'EOF'
auto lo
iface lo inet loopback

auto eth0
iface eth0 inet dhcp
EOF

# Setup APK repositories
echo "[4/8] Setting up APK repositories..."
setup-apkrepos -f

# Update package index
echo "[5/8] Updating packages..."
apk update

# Set root password (simple for now)
echo "root:tusk" | chpasswd

# Configure serial console
echo "[6/8] Configuring serial console..."
echo "ttyS0" >> /etc/securetty
echo "TtyS0" >> /etc/securetty

# Enable serial getty
if ! grep -q "ttyS0" /etc/inittab; then
    echo "ttyS0::respawn:/sbin/getty -L ttyS0 115200 vt100" >> /etc/inittab
fi

# Allow root login
sed -i 's/^root:\*:/root::/' /etc/shadow

# Install useful packages
echo "[7/8] Installing packages..."
apk add openssh-server dropbear-sftp /etc/ssh/sshd_config

# Setup SSH
rc-service sshd start 2>/dev/null || true
rc-update add sshd default 2>/dev/null || true

# Create tuskd init script
echo "[8/8] Creating tuskd service..."
mkdir -p /etc/init.d

cat > "$TUSK_INIT_SCRIPT" << 'TUSK_INIT'
#!/bin/sh
# Tusk Container Daemon - OpenRC Init Script

name=tuskd
description="Tusk container runtime daemon"
command="/tusk/tuskd-amd64"
command_background=true
pidfile="/run/tuskd.pid"

depend() {
    after netmount localmount
    need localmount
}

start() {
    # Mount 9p shared filesystem from host
    mkdir -p /tusk
    mount -t 9p -o trans=virtio,version=9p2000.L tusk-data /tusk 2>/dev/null || {
        mount -t 9p -o trans=virtio tusk-data /tusk 2>/dev/null || true
    }

    # Create required directories
    mkdir -p /tusk/containers 2>/dev/null || true
    mkdir -p /tusk/state 2>/dev/null || true

    # Copy tuskd if present in /tusk
    if [ -f "/tusk/tuskd-amd64" ]; then
        cp /tusk/tuskd-amd64 /usr/local/bin/tuskd
        chmod +x /usr/local/bin/tuskd
    fi

    # Start tuskd
    ebegin "Starting tuskd"
    start-stop-daemon --start --background --make-pidfile --pidfile $pidfile \
        --exec /usr/local/bin/tuskd -- /usr/local/bin/tuskd
    eend $?
}

stop() {
    ebegin "Stopping tuskd"
    start-stop-daemon --stop --pidfile $pidfile
    eend $?
}
TUSK_INIT

chmod +x "$TUSK_INIT_SCRIPT"
rc-update add tuskd default 2>/dev/null || true

echo ""
echo "============================================"
echo "  Configuration Complete!"
echo "============================================"
echo ""
echo "To start tuskd now:"
echo "  rc-service tuskd start"
echo ""
echo "To reboot:"
echo "  reboot"
echo ""
echo "Close this script and type 'exit' to reboot."