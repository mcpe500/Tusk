package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/tusk/tusk/internal/container"
	"github.com/tusk/tusk/internal/image"
)

// Runtime executes containers for real on the host using proot (native,
// non-root). It ties the image store (rootfs + config) to the proot backend
// and tracks live PIDs. This replaces the old fake "Pid=12345" tracking.
type Runtime struct {
	store        *image.Store
	tuskDir      string
	containerDir string

	mu      sync.Mutex
	running map[string]*exec.Cmd // id -> live proot process
}

// NewRuntime builds a Runtime rooted at tuskDir (e.g. ~/.tusk).
func NewRuntime(tuskDir string) *Runtime {
	return &Runtime{
		store:        image.New(filepath.Join(tuskDir, "images")),
		tuskDir:      tuskDir,
		containerDir: filepath.Join(tuskDir, "containers"),
		running:      make(map[string]*exec.Cmd),
	}
}

func (r *Runtime) rootfsDir(id string) string {
	return filepath.Join(r.containerDir, id, "rootfs")
}

func (r *Runtime) logPath(id string) string {
	return filepath.Join(r.containerDir, id, "container.log")
}

// containerImageRef reads the image ref from a container's meta.json.
func (r *Runtime) containerImageRef(id string) string {
	data, err := os.ReadFile(filepath.Join(r.containerDir, id, "meta.json"))
	if err != nil {
		return ""
	}
	var meta struct {
		Image string `json:"image"`
	}
	_ = json.Unmarshal(data, &meta)
	return meta.Image
}

// imageConfig loads the OCI image config (Env, Cmd, WorkingDir, Arch) for ref.
func (r *Runtime) imageConfig(ref string) (*image.Config, error) {
	manifest, err := r.store.GetManifestByRef(ref)
	if err != nil {
		return nil, fmt.Errorf("manifest for %q: %w", ref, err)
	}
	data, err := r.store.GetBlob(manifest.Config.Digest)
	if err != nil {
		return nil, fmt.Errorf("config blob: %w", err)
	}
	var cfg image.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// PrepareRootfs extracts the image layers into the container's rootfs dir.
// Returns the rootfs path. Idempotent: re-extracts cleanly each time.
func (r *Runtime) PrepareRootfs(containerID, imageRef string) (string, error) {
	rt := container.New(r.tuskDir)
	return rt.PrepareRootfs(containerID, imageRef)
}

// Start launches the container process under proot and records the real PID.
// command/env are the explicit values from ContainerCreate; they are merged
// with the image defaults. mounts are user bind mounts from ContainerCreate.
// Returns the OS pid of the proot process.
func (r *Runtime) Start(id, imageRef string, command, env []string, mounts []container.BindMount) (int, error) {
	rootfs := r.rootfsDir(id)
	if _, err := os.Stat(rootfs); err != nil {
		// rootfs not prepared yet (e.g. created before runtime existed) — do it now.
		if _, perr := r.PrepareRootfs(id, imageRef); perr != nil {
			return 0, fmt.Errorf("prepare rootfs: %w", perr)
		}
	}

	cfg, _ := r.imageConfig(imageRef)
	cmd := effectiveCommand(command, cfg)
	var imgEnv []string
	imgArch := ""
	workDir := "/"
	if cfg != nil {
		imgEnv = cfg.Config.Env
		imgArch = cfg.Architecture
		if cfg.Config.WorkingDir != "" {
			workDir = cfg.Config.WorkingDir
		}
	}
	fullEnv := mergeEnv(imgEnv, env)

	pc := &container.ProotConfig{
		RootfsDir:  rootfs,
		Command:    cmd,
		Env:        fullEnv,
		WorkDir:    workDir,
		TmpDir:     filepath.Join(r.tuskDir, "tmp"),
		ImageArch:  imgArch,
		BindMounts: mounts,
	}
	c, err := pc.BuildCmd()
	if err != nil {
		return 0, err
	}

	// Redirect container stdout/stderr to a log file.
	if err := os.MkdirAll(filepath.Dir(r.logPath(id)), 0755); err != nil {
		return 0, fmt.Errorf("create log dir: %w", err)
	}
	logFile, err := os.Create(r.logPath(id))
	if err != nil {
		return 0, fmt.Errorf("create log file: %w", err)
	}
	c.Stdout = logFile
	c.Stderr = logFile

	if err := c.Start(); err != nil {
		logFile.Close()
		return 0, fmt.Errorf("start proot: %w", err)
	}
	pid := c.Process.Pid

	r.mu.Lock()
	r.running[id] = c
	r.mu.Unlock()

	// Reap asynchronously. Keep logFile open until process exits to avoid
	// a race where Logs() reads a closed file handle.
	go func() {
		_ = c.Wait()
		r.mu.Lock()
		delete(r.running, id)
		r.mu.Unlock()
		logFile.Close()
	}()

	return pid, nil
}

// Stop terminates a running container process (and its process group).
func (r *Runtime) Stop(id string) error {
	r.mu.Lock()
	c := r.running[id]
	r.mu.Unlock()
	if c == nil || c.Process == nil {
		return nil // not running (or started in a previous daemon session)
	}
	// Kill the whole process group (proot --kill-on-exit handles children too).
	pgid, err := syscall.Getpgid(c.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = c.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

// IsRunning reports whether the container has a live process. Checks the
// in-memory map first; falls back to /proc/<pid> for containers started
// before this daemon session.
func (r *Runtime) IsRunning(id string) bool {
	r.mu.Lock()
	c, ok := r.running[id]
	r.mu.Unlock()
	if ok && c != nil && c.Process != nil {
		return true
	}
	// Fallback: read persisted PID from meta.json.
	if pid := r.persistedPID(id); pid > 0 {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
			return true
		}
	}
	return false
}

func (r *Runtime) persistedPID(id string) int {
	data, err := os.ReadFile(filepath.Join(r.containerDir, id, "meta.json"))
	if err != nil {
		return 0
	}
	var meta struct {
		Pid int `json:"pid"`
	}
	_ = json.Unmarshal(data, &meta)
	return meta.Pid
}

// Remove stops the container and deletes its on-disk rootfs/logs.
func (r *Runtime) Remove(id string) error {
	_ = r.Stop(id)
	return os.RemoveAll(filepath.Join(r.containerDir, id))
}

// Logs returns the captured container output, or empty if none yet.
func (r *Runtime) Logs(id string) string {
	data, err := os.ReadFile(r.logPath(id))
	if err != nil {
		return ""
	}
	return string(data)
}

// effectiveCommand merges explicit command with image Entrypoint+Cmd.
// Docker semantics: if explicit command given, it replaces Cmd (not Entrypoint).
func effectiveCommand(explicit []string, cfg *image.Config) []string {
	if len(explicit) == 1 && strings.Contains(explicit[0], " ") {
		explicit = strings.Fields(explicit[0])
	}
	var entrypoint, cmd []string
	if cfg != nil {
		entrypoint = cfg.Config.Entrypoint
		cmd = cfg.Config.Cmd
	}
	if len(explicit) > 0 {
		cmd = explicit
	}
	if len(entrypoint) > 0 {
		return append(entrypoint, cmd...)
	}
	if len(cmd) > 0 {
		return cmd
	}
	return []string{"/bin/sh"}
}

// mergeEnv combines image-config env with container-specified env (container
// values override image values for the same key).
func mergeEnv(imageEnv, containerEnv []string) []string {
	seen := map[string]int{}
	out := []string{}
	add := func(kv string) {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if idx, ok := seen[key]; ok {
			out[idx] = kv
			return
		}
		seen[key] = len(out)
		out = append(out, kv)
	}
	for _, e := range imageEnv {
		add(e)
	}
	for _, e := range containerEnv {
		add(e)
	}
	return out
}
