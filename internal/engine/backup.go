package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mcpe500/Tusk/internal/state"
)

const BackupSchemaVersion = 1

type BackupMetadata struct {
	SchemaVersion    int       `json:"schemaVersion"`
	BackendID        string    `json:"backendId"`
	Architecture     string    `json:"architecture"`
	TuskVersion      string    `json:"tuskVersion"`
	RuntimeVersion   string    `json:"runtimeVersion"`
	DataSchema       int       `json:"dataSchema"`
	Checksum         string    `json:"checksum"`
	CreatedAt        time.Time `json:"createdAt"`
	WorkspaceOutside bool      `json:"workspaceOutsideBackupScope"`
	Stopped          bool      `json:"backendStopped"`
}

func CreateBackup(ctx context.Context, sourceDir, destination string, metadata BackupMetadata) (BackupMetadata, error) {
	if err := ctx.Err(); err != nil {
		return BackupMetadata{}, err
	}
	if metadata.SchemaVersion == 0 {
		metadata.SchemaVersion = BackupSchemaVersion
	}
	if metadata.SchemaVersion != BackupSchemaVersion || metadata.BackendID == "" || metadata.Architecture == "" || metadata.TuskVersion == "" || metadata.RuntimeVersion == "" || metadata.DataSchema < 1 || !metadata.Stopped {
		return BackupMetadata{}, errors.New("backup metadata is incomplete")
	}
	info, err := os.Stat(sourceDir)
	if err != nil || !info.IsDir() {
		return BackupMetadata{}, fmt.Errorf("backup source is not a directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return BackupMetadata{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".backup-*")
	if err != nil {
		return BackupMetadata{}, err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	hash := sha256.New()
	writer := io.MultiWriter(temporary, hash)
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	err = filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil || relative == "." {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup refuses symlink entry %q", relative)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tarWriter, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
		return nil
	})
	if err == nil {
		err = tarWriter.Close()
	}
	if err == nil {
		err = gzipWriter.Close()
	}
	if err == nil {
		err = temporary.Sync()
	}
	if err == nil {
		err = temporary.Close()
	}
	if err != nil {
		return BackupMetadata{}, err
	}
	metadata.Checksum = hex.EncodeToString(hash.Sum(nil))
	metadata.CreatedAt = time.Now().UTC()
	if err := os.Rename(temporaryPath, destination); err != nil {
		return BackupMetadata{}, err
	}
	committed = true
	if err := os.Chmod(destination, 0o600); err != nil {
		return BackupMetadata{}, err
	}
	if err := state.WriteJSON(destination+".json", metadata, 0o600); err != nil {
		return BackupMetadata{}, err
	}
	return metadata, nil
}

func RestoreBackup(ctx context.Context, backupPath, destination string, expected BackendIdentity) error {
	var metadata BackupMetadata
	if err := state.ReadJSON(backupPath+".json", &metadata); err != nil {
		return err
	}
	if metadata.SchemaVersion != BackupSchemaVersion || metadata.BackendID != expected.BackendID || metadata.Architecture != expected.Architecture {
		return errors.New("backup identity or schema is incompatible")
	}
	if err := verifyChecksum(backupPath, metadata.Checksum); err != nil {
		return err
	}
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		return err
	}
	available, err := availableBytes(filepath.Dir(destination))
	if err != nil {
		return fmt.Errorf("check restore capacity: %w", err)
	}
	if available < uint64(backupInfo.Size()) {
		return fmt.Errorf("insufficient capacity for restore: %d available, %d required", available, backupInfo.Size())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	parent := filepath.Dir(destination)
	temporary, err := os.MkdirTemp(parent, ".restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	extractedSize, err := extractBackup(backupPath, temporary)
	if err != nil {
		return err
	}
	if extractedSize > available {
		return fmt.Errorf("insufficient capacity for extracted restore: %d available, %d required", available, extractedSize)
	}
	previous := destination + ".previous"
	_ = os.RemoveAll(previous)
	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, previous); err != nil {
			return err
		}
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Rename(previous, destination)
		return err
	}
	return nil
}

type BackendIdentity struct {
	BackendID    string
	Architecture string
}

func verifyChecksum(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("backup checksum mismatch: got %s, expected %s", actual, expected)
	}
	return nil
}

func extractBackup(path, destination string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var extractedSize uint64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return extractedSize, nil
		}
		if err != nil {
			return 0, err
		}
		target, err := safeArchiveTarget(destination, header.Name)
		if err != nil {
			return 0, err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return 0, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return 0, err
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return 0, err
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return 0, copyErr
			}
			if closeErr != nil {
				return 0, closeErr
			}
			extractedSize += uint64(header.Size)
		default:
			return 0, fmt.Errorf("unsupported backup entry type %d", header.Typeflag)
		}
	}
}

func safeArchiveTarget(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") {
		return "", fmt.Errorf("unsafe backup path %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("backup path traversal %q", name)
	}
	return filepath.Join(root, clean), nil
}

func availableBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}
