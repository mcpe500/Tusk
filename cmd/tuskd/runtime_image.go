package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/tusk/tusk/internal/container"
	"github.com/tusk/tusk/internal/image"
)

// repoIndexMu guards concurrent writes to repositories.json.
var repoIndexMu sync.Mutex

// repoIndexPath is the human-readable ref->digest index. The store's own
// manifest index is sha1-hashed (not reversible), so we keep this alongside
// it to support `tusk images` listing.
func (r *Runtime) repoIndexPath() string {
	return filepath.Join(r.tuskDir, "images", "repositories.json")
}

func (r *Runtime) loadRepoIndex() map[string]string {
	repoIndexMu.Lock()
	defer repoIndexMu.Unlock()
	out := map[string]string{}
	data, err := os.ReadFile(r.repoIndexPath())
	if err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func (r *Runtime) recordRepo(ref, digest string) {
	repoIndexMu.Lock()
	defer repoIndexMu.Unlock()
	idx := map[string]string{}
	if data, err := os.ReadFile(r.repoIndexPath()); err == nil {
		_ = json.Unmarshal(data, &idx)
	}
	idx[ref] = digest
	if data, err := json.MarshalIndent(idx, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(r.repoIndexPath()), 0755)
		_ = os.WriteFile(r.repoIndexPath(), data, 0644)
	}
}

// Pull downloads an image from a registry into the local store and records it
// in the repositories index for listing.
func (r *Runtime) Pull(ref string) error {
	puller := image.NewPuller(r.store)
	if err := puller.Pull(context.Background(), ref); err != nil {
		return err
	}
	if digest, err := r.store.ResolveManifestRef(ref); err == nil {
		r.recordRepo(ref, digest)
	}
	return nil
}

// ImageList returns the locally stored images for `tusk images`.
func (r *Runtime) ImageList() []map[string]interface{} {
	idx := r.loadRepoIndex()
	out := make([]map[string]interface{}, 0, len(idx))
	for ref, digest := range idx {
		var size int64
		if m, err := r.store.GetManifestByRef(ref); err == nil {
			for _, l := range m.Layers {
				size += l.Size
			}
		}
		out = append(out, map[string]interface{}{
			"repository": ref,
			"id":         digest,
			"size":       size,
		})
	}
	return out
}

// Exec runs a one-shot command inside an existing container's rootfs under
// proot and returns captured stdout, stderr, and the exit code. This backs
// `tusk exec`.
func (r *Runtime) Exec(containerID string, cmd []string) (string, string, int) {
	rootfs := r.rootfsDir(containerID)
	if _, err := os.Stat(rootfs); err != nil {
		return "", "container rootfs not found: " + err.Error(), 1
	}

	// Load container meta to get the image ref for config lookup.
	imageRef := r.containerImageRef(containerID)
	cfg, _ := r.imageConfig(imageRef)
	var env []string
	imgArch := ""
	if cfg != nil {
		env = cfg.Config.Env
		imgArch = cfg.Architecture
	}

	pc := &container.ProotConfig{
		RootfsDir: rootfs,
		Command:   cmd,
		Env:       env,
		WorkDir:   "/",
		TmpDir:    filepath.Join(r.tuskDir, "tmp"),
		ImageArch: imgArch,
	}
	c, err := pc.BuildCmd()
	if err != nil {
		return "", err.Error(), 1
	}

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err = c.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if ee, ok := err.(interface{ ExitCode() int }); ok {
			exitCode = ee.ExitCode()
		}
	}
	return stdout.String(), stderr.String(), exitCode
}
