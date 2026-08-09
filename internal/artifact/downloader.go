package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Downloader struct {
	Client        *http.Client
	Progress      func(downloaded, total int64)
	AllowInsecure bool
}

func (d Downloader) Download(ctx context.Context, artifact Artifact, destination string) error {
	if err := validateDownloadArtifact(artifact, d.AllowInsecure); err != nil {
		return err
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	if _, err := os.Stat(destination); err == nil {
		if err := verifyFile(destination, artifact); err == nil {
			return nil
		}
		if err := quarantine(destination); err != nil {
			return fmt.Errorf("quarantine invalid destination: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect destination: %w", err)
	}

	partial := destination + ".part"
	offset, err := partialOffset(partial, artifact.Size)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return fmt.Errorf("create artifact request: %w", err)
	}
	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download artifact: %w", err)
	}
	defer response.Body.Close()
	if !d.AllowInsecure && response.Request.URL.Scheme != "https" {
		return fmt.Errorf("artifact download redirected to insecure URL %q", response.Request.URL)
	}
	appendMode := offset > 0 && response.StatusCode == http.StatusPartialContent
	if appendMode && !validContentRange(response.Header.Get("Content-Range"), offset) {
		return fmt.Errorf("server returned invalid Content-Range %q", response.Header.Get("Content-Range"))
	}
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset == artifact.Size {
		if err := verifyFile(partial, artifact); err != nil {
			_ = os.Remove(partial)
			return err
		}
		return commitDownload(partial, destination)
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download artifact: HTTP %s", response.Status)
	}
	if err := verifyMediaType(response.Header.Get("Content-Type"), artifact.MediaType); err != nil {
		return err
	}
	if !appendMode {
		offset = 0
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(partial, flags, 0o600)
	if err != nil {
		return fmt.Errorf("open partial artifact: %w", err)
	}
	remaining := artifact.Size - offset
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, remaining+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write partial artifact: %w", copyErr)
	}
	if syncErr != nil {
		return fmt.Errorf("sync partial artifact: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close partial artifact: %w", closeErr)
	}
	if written > remaining {
		_ = os.Remove(partial)
		return fmt.Errorf("artifact exceeds declared size %d", artifact.Size)
	}
	if d.Progress != nil {
		d.Progress(offset+written, artifact.Size)
	}
	if offset+written < artifact.Size {
		return fmt.Errorf("artifact download incomplete: got %d of %d bytes", offset+written, artifact.Size)
	}
	if err := verifyFile(partial, artifact); err != nil {
		_ = os.Remove(partial)
		return err
	}
	return commitDownload(partial, destination)
}

func validateDownloadArtifact(artifact Artifact, allowInsecure bool) error {
	if artifact.Size <= 0 {
		return errors.New("artifact size must be positive")
	}
	if len(artifact.SHA256) != sha256.Size*2 {
		return errors.New("artifact SHA-256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(artifact.SHA256); err != nil {
		return errors.New("artifact SHA-256 is invalid")
	}
	parsedURL, err := url.Parse(artifact.URL)
	if err != nil || parsedURL.Host == "" {
		return errors.New("artifact URL must be absolute")
	}
	if parsedURL.Scheme != "https" && !(allowInsecure && parsedURL.Scheme == "http") {
		return errors.New("artifact URL must use HTTPS")
	}
	if artifact.MediaType == "" {
		return errors.New("artifact media type is required")
	}
	return nil
}

func verifyMediaType(header, expected string) error {
	actual, _, err := mime.ParseMediaType(header)
	if err != nil {
		return fmt.Errorf("parse artifact Content-Type %q: %w", header, err)
	}
	if actual != expected {
		return fmt.Errorf("artifact Content-Type mismatch: got %q, expected %q", actual, expected)
	}
	return nil
}

func partialOffset(path string, expected int64) (int64, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect partial artifact: %w", err)
	}
	if info.Size() > expected {
		if err := os.Remove(path); err != nil {
			return 0, fmt.Errorf("remove oversized partial artifact: %w", err)
		}
		return 0, nil
	}
	return info.Size(), nil
}

func validContentRange(value string, offset int64) bool {
	prefix := "bytes " + strconv.FormatInt(offset, 10) + "-"
	return strings.HasPrefix(value, prefix)
}

func verifyFile(path string, artifact Artifact) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact for verification: %w", err)
	}
	hash := sha256.New()
	written, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash artifact: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close verified artifact: %w", closeErr)
	}
	if written != artifact.Size {
		return fmt.Errorf("artifact size mismatch: got %d, expected %d", written, artifact.Size)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != artifact.SHA256 {
		return fmt.Errorf("artifact SHA-256 mismatch: got %s, expected %s", actual, artifact.SHA256)
	}
	return nil
}

func commitDownload(partial, destination string) error {
	if err := os.Rename(partial, destination); err != nil {
		return fmt.Errorf("commit artifact: %w", err)
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		return fmt.Errorf("set artifact permissions: %w", err)
	}
	return nil
}

func quarantine(path string) error {
	target := path + ".corrupt"
	_ = os.Remove(target)
	return os.Rename(path, target)
}
