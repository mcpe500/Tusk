#!/usr/bin/env bash
# Tusk VM Pre-configuration Script
# Configures installed Alpine to auto-start tuskd
# Run this INSIDE the Alpine VM after installation

set -e

TUSKD_INIT_SCRIPT="/etc/init.d/tuskd"

cat << 'EOF'
============================================
  Tusk VM Configuration
============================================

This script configures Alpine Linux to:
1. Mount 9p shared filesystem from host
2. Auto-start tuskd daemon on boot

Login to Alpine VM and run this script.

============================================

EOF

# Create init script
cat > "$TUSKD_INIT_SCRIPT" << 'TUSK_INIT'
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
        # Try alternative method
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

chmod +x "$TUSKD_INIT_SCRIPT"

# Enable service
rc-update add tuskd default 2>/dev/null || echo "Run: rc-update add tuskd default"

# Configure serial console for login
echo "ttyS0" >> /etc/securetty
echo "TtyS0" >> /etc/securetty

# Enable serial getty
sed -i '/^#.*ttyS0/s/^#//' /etc/inittab 2>/dev/null || true
cat >> /etc/inittab << 'INITTAB'
ttyS0::respawn:/sbin/getty -L ttyS0 115200 vt100
INITTAB

# Configure networking
cat > /etc/network/interfaces << 'NETWORK'
auto lo
iface lo inet loopdown

auto eth0
iface eth0 inet dhcp
NETWORK

# Allow root login via serial
sed -i 's/^root:\*:/root::/' /etc/shadow

echo ""
echo "============================================"
echo "  Configuration Complete!"
echo "============================================"
echo ""
echo "To start tuskd now:"
echo "  rc-service tuskd start"
echo ""
echo "To verify:"
echo "  rc-status"
echo ""
echo "To reboot:"
echo "  reboot"
echo ""
echo "The VM will now:"
echo "  1. Mount /tusk from host on boot"
echo "  2. Start tuskd daemon"
echo ""
echo "Close this script and type 'exit' to reboot."