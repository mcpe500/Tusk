#!/bin/sh
# Tusk Alpine Auto-Install Script
# Fully automated Alpine installation WITHOUT interactive prompts
# Uses direct disk partitioning and apk to set up the system

set -e

echo "============================================"
echo "  Tusk Alpine Auto-Install"
echo "============================================"
echo ""

# Variables
DISK="/dev/vda"
BOOT_SIZE=64
ROOT_SIZE=1800

echo "[1/7] Partitioning disk..."
# Partition with fdisk (non-interactive)
fdisk "$DISK" << EOF || true
o
n
p
1
2048
+${BOOT_SIZE}M
n
p
2
131072
+${ROOT_SIZE}M
t
1
83
t
2
83
w
EOF

echo "[2/7] Formatting partitions..."
# Wait for device nodes
sleep 1

# Format partitions
mkfs.ext4 -F /dev/vda1
mkfs.ext4 -F /dev/vda2

echo "[3/7] Mounting partitions..."
mkdir -p /mnt/root
mount /dev/vda2 /mnt/root
mkdir -p /mnt/root/boot
mount /dev/vda1 /mnt/root/boot

echo "[4/7] Setting up Alpine base..."
# Mount pseudo-filesystems
mount -t proc proc /mnt/root/proc
mount -t sysfs sys /mnt/root/sys
mount --bind /dev /mnt/root/dev

# Initialize apk db
apkdb_init() {
    mkdir -p /mnt/root/etc/apk
    touch /mnt/root/etc/apk/world
}

# Setup repositories
cat > /mnt/root/etc/apk/repositories << 'EOF'
http://dl-cdn.alpinelinux.org/alpine/v3.19/main
http://dl-cdn.alpinelinux.org/alpine/v3.19/community
EOF

echo "[5/7] Installing base system..."
# Install base packages
chroot /mnt/root /bin/sh << 'CHROOT_SETUP'
set -e

export HOME=/root
export PATH=/usr/local/sbin:/usr/local/bin:/sbin:/usr/sbin:/bin:/usr/bin

# Initialize apk
apk update

# Install base system
apk add --initdb alpine-base openssh dropbear-sftp

# Set hostname
echo "tusk-vm" > /etc/hostname

# Configure networking
cat > /etc/network/interfaces << 'NETEOF'
auto lo
iface lo inet loopback

auto eth0
iface eth0 inet dhcp
NETEOF

# Configure DNS
echo "nameserver 8.8.8.8" > /etc/resolv.conf

# Configure serial console
echo "ttyS0" >> /etc/securetty
if ! grep -q "ttyS0" /etc/inittab; then
    echo "ttyS0::respawn:/sbin/getty -L ttyS0 115200 vt100" >> /etc/inittab
fi

# Allow root login
sed -i 's/^root:\*:/root::/' /etc/shadow

# Set root password
echo "root:tusk" | chpasswd

# Install grub
apk add grub2

# Setup fstab
cat > /etc/fstab << 'FSTABEOF'
/dev/vda2 / ext4 defaults 0 1
/dev/vda1 /boot ext4 defaults 0 1
FSTABEOF

# Install bootloader
grub-install /dev/vda --boot-directory=/boot 2>/dev/null || true

# Generate grub config
cat > /boot/grub/grub.cfg << 'GRUBEOF'
set default=0
set timeout=2

menuentry 'Alpine Linux' {
    linux /vmlinuz-virt root=/dev/vda2 console=ttyS0 quiet
    initrd /initramfs-virt
}

menuentry 'Alpine Linux (serial)' {
    linux /vmlinuz-virt root=/dev/vda2 console=ttyS0,115200
    initrd /initramfs-virt
}
GRUBEOF

# Copy kernel and initramfs
if [ -f /boot/vmlinuz-virt ]; then
    cp /boot/vmlinuz-virt /mnt/root/boot/
fi
if [ -f /boot/initramfs-virt ]; then
    cp /boot/initramfs-virt /mnt/root/boot/
fi

# Create tuskd service
mkdir -p /etc/init.d
cat > /etc/init.d/tuskd << 'TUSKD_EOF'
#!/bin/sh
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
        --exec /usr/local/bin/tuskd -- /usr/local/bin/tuskd
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

echo "Base system configured!"
CHROOT_SETUP

echo "[6/7] Cleanup..."
# Unmount
umount /mnt/root/dev
umount /mnt/root/proc
umount /mnt/root/sys
umount /mnt/root/boot
umount /mnt/root

echo "[7/7] Installation complete!"
echo ""
echo "============================================"
echo "  Alpine installed successfully!"
echo "============================================"
echo ""
echo "Shutdown and reboot from the installed system:"
echo "  poweroff"
echo ""