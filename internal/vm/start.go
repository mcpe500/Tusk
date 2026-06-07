package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (m *Manager) Start(ctx context.Context, cfg *Config) error {
	if m.isRunning() {
		return nil
	}
	if err := m.Init(); err != nil {
		return err
	}

	applyConfigDefaults(cfg)
	args := m.qemuArgs(cfg)
	m.removeStaleSockets()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start qemu: %w", err)
	}

	m.cmd = cmd
	if err := m.writePID(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	return nil
}

func applyConfigDefaults(cfg *Config) {
	if cfg.Memory == 0 {
		cfg.Memory = 512
	}
	if cfg.CPUs == 0 {
		cfg.CPUs = 2
	}
}

func (m *Manager) qemuArgs(cfg *Config) []string {
	args := []string{
		"qemu-system-x86_64",
		"-M", "pc-i440fx-9.2",
		"-m", fmt.Sprintf("%d", cfg.Memory),
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-nographic",
		"-qmp", fmt.Sprintf("unix:%s,server,nowait", m.qmpSock),
		"-device", "virtio-serial-pci",
		"-device", "virtserialport,chardev=ch0,name=tusk0",
		"-chardev", fmt.Sprintf("socket,id=ch0,path=%s,server,nowait", m.serialSock),
		"-serial", fmt.Sprintf("unix:%s,server,nowait", m.ConsoleSocket()),
		"-netdev", "user,id=net0",
		"-device", "virtio-net-pci,netdev=net0",
		"-virtfs", fmt.Sprintf("local,path=%s,mount_tag=tusk-data,security_model=mapped,id=tusk", m.baseDir),
	}

	if cfg.DiskPath != "" {
		args = append(args, "-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio", cfg.DiskPath))
	}
	if cfg.KernelPath != "" {
		args = append(args, "-kernel", cfg.KernelPath)
	}
	if cfg.InitrdPath != "" {
		args = append(args, "-initrd", cfg.InitrdPath)
		args = append(args, "-append", "console=ttyS0 root=/dev/vda")
	}
	if cfg.KernelPath == "" && cfg.DiskPath == "" {
		args = append(args, "-cdrom", filepath.Join(os.Getenv("HOME"), "alpine-virt-3.19.1-x86_64.iso"))
	}
	return args
}

func (m *Manager) removeStaleSockets() {
	_ = os.Remove(m.qmpSock)
	_ = os.Remove(m.serialSock)
	_ = os.Remove(m.ConsoleSocket())
}
