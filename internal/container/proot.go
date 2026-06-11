package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// ProotConfig describes how to launch a container process under proot on
// Termux/Android. proot is the native (non-root) container backend: it runs
// same-arch images directly (fast) and cross-arch images via a qemu-user
// emulator. See references/tusk-install-failures.md (Failure Pattern 4) for
// the LD_PRELOAD / PROOT_TMP_DIR root-cause notes.
// BindMount represents a host-path→container-path bind mount for proot.
type BindMount struct {
	Source      string // absolute host path
	Destination string // absolute path inside the container
}

type ProotConfig struct {
	RootfsDir   string      // extracted image root filesystem
	Command     []string    // entrypoint+cmd to run inside the container
	Env         []string    // KEY=VALUE environment entries
	WorkDir     string      // working directory inside the container (default "/")
	TmpDir      string      // writable temp dir for proot (Termux /tmp is read-only)
	ImageArch   string      // image arch, e.g. "arm64"/"amd64"; "" means same as host
	BindMounts  []BindMount // user bind mounts (in addition to /dev, /proc, /sys)
}

// prootBinary is the proot executable shipped by Termux.
const prootBinary = "proot"

// hostArch maps Go's runtime.GOARCH to the OCI image architecture name.
func hostArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	case "amd64":
		return "amd64"
	case "arm":
		return "arm"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH
	}
}

// qemuForArch returns the qemu-user binary needed to emulate an image arch
// that differs from the host, plus whether emulation is required at all.
func qemuForArch(imageArch string) (qemuBin string, needEmu bool) {
	if imageArch == "" || imageArch == hostArch() {
		return "", false
	}
	switch imageArch {
	case "amd64", "x86_64":
		return "qemu-x86_64", true
	case "arm64", "aarch64":
		return "qemu-aarch64", true
	case "arm":
		return "qemu-arm", true
	case "386", "i386":
		return "qemu-i386", true
	case "riscv64":
		return "qemu-riscv64", true
	default:
		return "qemu-" + imageArch, true
	}
}

// BuildProotArgs assembles the proot argument vector (excluding the proot
// binary name itself) for the given config.
func (c *ProotConfig) BuildProotArgs() ([]string, error) {
	if c.RootfsDir == "" {
		return nil, fmt.Errorf("rootfs dir is required")
	}
	workDir := c.WorkDir
	if workDir == "" {
		workDir = "/"
	}

	args := []string{
		"--kill-on-exit",
		"--sysvipc",
		"--link2symlink",
		"-r", c.RootfsDir,
	}

	// Bind essential pseudo-filesystems if they exist on the host.
	for _, b := range []string{"/dev", "/proc", "/sys"} {
		if _, err := os.Stat(b); err == nil {
			args = append(args, "-b", b)
		}
	}
	// Make DNS work inside the container.
	if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		args = append(args, "-b", "/etc/resolv.conf:/etc/resolv.conf")
	}

	// User-defined bind mounts from ContainerCreate params.
	for _, m := range c.BindMounts {
		if m.Source == "" || m.Destination == "" {
			continue
		}
		if _, err := os.Stat(m.Source); err != nil {
			// Source missing — proot will fail; skip with a warning rather than crashing.
			// The caller (Runtime.Start) already validated and created volume dirs.
			_, _ = fmt.Fprintf(os.Stderr, "tuskd: warning: mount source %q does not exist, skipping\n", m.Source)
			continue
		}
		args = append(args, "-b", m.Source+":"+m.Destination)
	}

	args = append(args, "-w", workDir)

	// Cross-arch images need a qemu-user emulator registered with proot.
	if qemuBin, needEmu := qemuForArch(c.ImageArch); needEmu {
		path, err := exec.LookPath(qemuBin)
		if err != nil {
			return nil, fmt.Errorf("image arch %q needs %s (install with: pkg install qemu-user-%s)",
				c.ImageArch, qemuBin, strings.TrimPrefix(qemuBin, "qemu-"))
		}
		args = append(args, "-q", path)
	}

	cmd := c.Command
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}
	args = append(args, cmd...)
	return args, nil
}

// prootEnviron returns the environment for the proot *host* process. The
// critical part: Termux injects LD_PRELOAD=libtermux-exec which breaks
// proot's execve into a foreign rootfs (ENOSYS). We strip it and point
// PROOT_TMP_DIR at a writable directory.
func (c *ProotConfig) prootEnviron() []string {
	tmp := c.TmpDir
	if tmp == "" {
		tmp = filepath.Join(os.Getenv("HOME"), ".tusk", "tmp")
	}
	_ = os.MkdirAll(tmp, 0755)

	out := make([]string, 0, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "LD_PRELOAD=") {
			continue // drop libtermux-exec — see proot.go root-cause note
		}
		if strings.HasPrefix(e, "PROOT_TMP_DIR=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "PROOT_TMP_DIR="+tmp)
	out = append(out, "PROOT_NO_SECCOMP=1")
	return out
}

// BuildCmd builds an *exec.Cmd ready to start the container under proot.
// The caller is responsible for wiring Stdout/Stderr and calling Start/Wait.
func (c *ProotConfig) BuildCmd() (*exec.Cmd, error) {
	prootPath, err := exec.LookPath(prootBinary)
	if err != nil {
		return nil, fmt.Errorf("proot not found (install with: pkg install proot): %w", err)
	}
	args, err := c.BuildProotArgs()
	if err != nil {
		return nil, err
	}

	// Wrap the guest command with /bin/sh -c so proot only needs to exec
	// /bin/sh (which always exists in the rootfs). This avoids issues where
	// direct binary execution fails due to dynamic linker or env limitations
	// inside proot. Env vars are passed via the host environment which proot
	// forwards to the guest.
	if len(args) > 0 {
		// Find where the guest command starts (after last proot flag).
		cmdStart := findCmdStart(args)
		if cmdStart < len(args) {
			guestCmd := strings.Join(args[cmdStart:], " ")
			// If we have env vars, prepend them as shell exports
			var exports string
			for _, e := range c.Env {
				exports += "export " + shellEscape(e) + "; "
			}
			args = append(args[:cmdStart], "/bin/sh", "-c", exports+guestCmd)
		}
	}

	cmd := exec.Command(prootPath, args...)
	cmd.Env = c.prootEnviron()
	// Detach into its own process group so the daemon can manage it and so
	// it survives independently of the RPC connection.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

// injectGuestEnv inserts `env KEY=VAL ...` immediately before the guest
// command. The guest command is the contiguous tail of args following the
// last proot option. We locate it by finding the "-w" flag and its value,
// after which everything is the command (unless a "-q" emulator pair follows).
func injectGuestEnv(args []string, guestEnv []string) []string {
	cmdStart := findCmdStart(args)
	if cmdStart > len(args) {
		cmdStart = len(args)
	}
	out := make([]string, 0, len(args)+len(guestEnv))
	out = append(out, args[:cmdStart]...)
	out = append(out, guestEnv...)
	out = append(out, args[cmdStart:]...)
	return out
}

// findCmdStart locates the index where the guest command begins in proot args,
// i.e. everything after the last proot flag (-w value, optionally -q value).
func findCmdStart(args []string) int {
	cmdStart := len(args)
	for i := 0; i < len(args); i++ {
		if args[i] == "-w" && i+1 < len(args) {
			cmdStart = i + 2
			i++
			continue
		}
		if args[i] == "-q" && i+1 < len(args) {
			if i+2 > cmdStart {
				cmdStart = i + 2
			}
			i++
		}
	}
	return cmdStart
}

// shellEscape quotes a KEY=VALUE env string for safe embedding in sh -c.
func shellEscape(kv string) string {
	if idx := strings.IndexByte(kv, '='); idx >= 0 {
		key := kv[:idx]
		val := kv[idx+1:]
		return key + "='" + strings.ReplaceAll(val, "'", "'\\'") + "'"
	}
	return kv
}
