package container

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tusk/tusk/internal/image"
	"github.com/tusk/tusk/pkg/types"
)

type Runtime struct {
	store     *image.Store
	tuskDir   string
	containerDir string
}

func New(tuskDir string) *Runtime {
	return &Runtime{
		store:       image.New(filepath.Join(tuskDir, "images")),
		tuskDir:     tuskDir,
		containerDir: filepath.Join(tuskDir, "containers"),
	}
}

func (r *Runtime) Init() error {
	return os.MkdirAll(r.containerDir, 0755)
}

// PrepareRootfs extracts image layers to create container root filesystem
func (r *Runtime) PrepareRootfs(containerID, imageRef string) (string, error) {
	rootfsDir := filepath.Join(r.containerDir, containerID, "rootfs")
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return "", fmt.Errorf("create rootfs dir: %w", err)
	}

	// TODO: Look up image manifest from store
	// For now, this is a placeholder - the actual implementation
	// would extract layers from the OCI image
	_ = imageRef

	return rootfsDir, nil
}

// GenerateRuntimeSpec creates an OCI runtime spec for a container
func GenerateRuntimeSpec(containerID, imageRef string, cmd []string, env []string) *types.RuntimeSpec {
	hostname := fmt.Sprintf("tusk-%s", containerID[:12])

	spec := &types.RuntimeSpec{
		OCIVersion: "1.0.2",
		Hostname:   hostname,
		Process: types.ProcessSpec{
			Terminal: false,
			User: types.User{
				UID: 0,
				GID: 0,
			},
			Args: cmd,
			Cwd:  "/",
			Env:  append(env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"),
		},
		Linux: types.LinuxSpec{
			Namespaces: []types.Namespace{
				{Type: "pid"},
				{Type: "network"},
				{Type: "mount"},
				{Type: "ipc"},
				{Type: "uts"},
			},
		},
	}

	return spec
}

// ExtractLayer extracts a tar.gz layer to a destination directory
func ExtractLayer(layerData []byte, destDir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(layerData))
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reader: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		// Handle whiteout files
		if strings.HasPrefix(header.Name, ".wh.") {
			whiteout := filepath.Dir(target) + "/" + header.Name[4:]
			os.Remove(whiteout)
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			os.Chmod(target, 0644)
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Symlink(header.Linkname, target)
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Link(filepath.Join(destDir, header.Linkname), target)
		}
	}

	return nil
}

// ApplyWhiteouts applies whiteout files (AUFS style .wh.* files)
func ApplyWhiteouts(rootfsDir string) error {
	return filepath.Walk(rootfsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, ".wh.") {
					whiteoutName := name[4:]
					whiteoutPath := filepath.Join(path, whiteoutName)
					os.Remove(whiteoutPath)
					os.Remove(filepath.Join(path, name))
				}
			}
		}
		return nil
	})
}