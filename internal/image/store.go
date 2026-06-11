package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Store struct {
	baseDir string
}

func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) Init() error {
	dirs := []string{
		filepath.Join(s.baseDir, "blobs"),
		filepath.Join(s.baseDir, "index"),
		filepath.Join(s.baseDir, "manifests"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

func (s *Store) blobPath(digest string) string {
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return filepath.Join(s.baseDir, "blobs", parts[0], parts[1])
}

func (s *Store) manifestIndexPath(ref string) string {
	refHash := sha1.Sum([]byte(ref))
	return filepath.Join(s.baseDir, "index", hex.EncodeToString(refHash[:])+".txt")
}

func (s *Store) SaveBlob(digest string, data []byte) error {
	path := s.blobPath(digest)
	if path == "" {
		return fmt.Errorf("invalid digest: %s", digest)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Store) GetBlob(digest string) ([]byte, error) {
	path := s.blobPath(digest)
	return os.ReadFile(path)
}

func (s *Store) HasBlob(digest string) bool {
	path := s.blobPath(digest)
	_, err := os.Stat(path)
	return err == nil
}

func (s *Store) SaveManifest(ref string, manifest Manifest) error {
	// Calculate digest
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	digest := "sha256:" + sha256Bytes(data)

	path := filepath.Join(s.baseDir, "manifests", digest+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	indexPath := s.manifestIndexPath(ref)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(indexPath, []byte(digest), 0644)
}

func (s *Store) GetManifest(digest string) (*Manifest, error) {
	path := filepath.Join(s.baseDir, "manifests", digest+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) ResolveManifestRef(ref string) (string, error) {
	if strings.HasPrefix(ref, "sha256:") {
		return ref, nil
	}

	if at := strings.Index(ref, "@"); at != -1 {
		digest := ref[at+1:]
		if strings.HasPrefix(digest, "sha256:") {
			return digest, nil
		}
	}

	path := s.manifestIndexPath(ref)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("manifest index missing for %q: %w", ref, err)
	}

	digest := strings.TrimSpace(string(data))
	if !strings.HasPrefix(digest, "sha256:") {
		return "", fmt.Errorf("invalid manifest digest %q for %q", digest, ref)
	}

	return digest, nil
}

func (s *Store) GetManifestByRef(ref string) (*Manifest, error) {
	digest, err := s.ResolveManifestRef(ref)
	if err != nil {
		return nil, err
	}
	return s.GetManifest(digest)
}

type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType,omitempty"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

type Descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type Config struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Config       struct {
		Env        []string `json:"Env"`
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

func sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (s *Store) ExtractLayer(digest string, dest string) error {
	data, err := s.GetBlob(digest)
	if err != nil {
		return fmt.Errorf("get blob %s: %w", digest, err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
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

		target := filepath.Join(dest, header.Name)
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
			os.Chmod(target, header.FileInfo().Mode())
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Symlink(header.Linkname, target)
		}
	}

	return nil
}

type Puller struct {
	store   *Store
	verbose bool
}

func NewPuller(store *Store) *Puller {
	return &Puller{store: store}
}

func (p *Puller) SetVerbose(v bool) {
	p.verbose = v
}

func (p *Puller) Pull(ctx context.Context, ref string) error {
	// Parse reference: registry/name:tag or registry/name@sha256:digest
	ref = strings.TrimPrefix(ref, "docker://")

	registry := "registry-1.docker.io"
	name := ref
	tag := "latest"

	if strings.Contains(ref, "@") {
		parts := strings.SplitN(ref, "@", 2)
		name = parts[0]
		// digest mode
	} else if strings.Contains(ref, ":") {
		parts := strings.SplitN(ref, ":", 2)
		name = parts[0]
		tag = parts[1]
	}

	// Official Docker Hub images live under the "library/" namespace.
	// e.g. "alpine" -> "library/alpine". Names that already contain a
	// slash (user/repo) are left untouched.
	if !strings.Contains(name, "/") {
		name = "library/" + name
	}

	fmt.Printf("Pulling %s (tag: %s)\n", name, tag)

	// Step 1: Get token from registry
	token, err := p.getToken(ctx, registry, name)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	// Step 2: Fetch manifest (resolves multi-arch manifest lists to this host's arch)
	manifest, err := p.fetchManifest(ctx, registry, name, tag, token)
	if err != nil {
		return fmt.Errorf("fetch manifest: %w", err)
	}

	// Step 3: Fetch config
	fmt.Printf("Downloading config...\n")
	configData, err := p.fetchBlob(ctx, registry, name, manifest.Config.Digest, token)
	if err != nil {
		return fmt.Errorf("fetch config: %w", err)
	}
	if err := p.store.SaveBlob(manifest.Config.Digest, configData); err != nil {
		return err
	}

	// Step 4: Fetch layers
	for i, layer := range manifest.Layers {
		fmt.Printf("Downloading layer %d/%d: %s\n", i+1, len(manifest.Layers), layer.Digest)
		layerData, err := p.fetchBlob(ctx, registry, name, layer.Digest, token)
		if err != nil {
			return fmt.Errorf("fetch layer %s: %w", layer.Digest, err)
		}
		if err := p.store.SaveBlob(layer.Digest, layerData); err != nil {
			return err
		}
	}

	// Step 5: Save manifest
	if err := p.store.SaveManifest(ref, *manifest); err != nil {
		return err
	}

	fmt.Printf("Pull complete: %s\n", name)
	return nil
}

func (p *Puller) getToken(ctx context.Context, registry, name string) (string, error) {
	// name already includes any namespace (e.g. "library/alpine").
	authURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", name)
	if registry != "registry-1.docker.io" {
		authURL = fmt.Sprintf("https://%s/token?service=%s&scope=repository:%s:pull", registry, registry, name)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", authURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("auth failed: %d", resp.StatusCode)
	}

	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Token != "" {
		return result.Token, nil
	}
	return result.AccessToken, nil
}

// acceptManifestTypes lists every manifest media type we understand, in a
// single Accept header (setting the header twice overwrites the first value).
const acceptManifestTypes = "application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.oci.image.index.v1+json"

// manifestList models a multi-arch manifest list / OCI image index.
type manifestList struct {
	MediaType string `json:"mediaType"`
	Manifests []struct {
		Digest    string `json:"digest"`
		MediaType string `json:"mediaType"`
		Platform  struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
			Variant      string `json:"variant"`
		} `json:"platform"`
	} `json:"manifests"`
}

func (p *Puller) fetchManifest(ctx context.Context, registry, name, ref, token string) (*Manifest, error) {
	raw, mediaType, err := p.fetchManifestRaw(ctx, registry, name, ref, token)
	if err != nil {
		return nil, err
	}

	// If we got a manifest list / index, select the digest matching this host.
	if strings.Contains(mediaType, "manifest.list") || strings.Contains(mediaType, "image.index") {
		var list manifestList
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("parse manifest list: %w", err)
		}
		digest := selectPlatformDigest(list)
		if digest == "" {
			return nil, fmt.Errorf("no manifest for %s/%s in image %s", runtime.GOOS, runtime.GOARCH, name)
		}
		if p.verbose {
			fmt.Printf("Selected %s/%s manifest: %s\n", runtime.GOOS, runtime.GOARCH, digest)
		}
		raw, _, err = p.fetchManifestRaw(ctx, registry, name, digest, token)
		if err != nil {
			return nil, fmt.Errorf("fetch platform manifest: %w", err)
		}
	}

	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if len(m.Layers) == 0 {
		return nil, fmt.Errorf("manifest has no layers (unexpected media type %q)", mediaType)
	}
	return &m, nil
}

// selectPlatformDigest picks the manifest digest matching the host arch,
// falling back to amd64 (runnable under QEMU) if the native arch is absent.
// Container images always target OS "linux" — note runtime.GOOS is "android"
// on Termux, so we match the image OS against "linux", not runtime.GOOS.
func selectPlatformDigest(list manifestList) string {
	const wantOS = "linux"
	wantArch := runtime.GOARCH // "arm64" on aarch64 Termux, "amd64" on x86
	var amd64Fallback string
	for _, m := range list.Manifests {
		if m.Platform.OS != "" && m.Platform.OS != wantOS {
			continue
		}
		if m.Platform.Architecture == wantArch {
			return m.Digest
		}
		if m.Platform.Architecture == "amd64" {
			amd64Fallback = m.Digest
		}
	}
	return amd64Fallback
}

func (p *Puller) fetchManifestRaw(ctx context.Context, registry, name, ref, token string) ([]byte, string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, name, ref)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", acceptManifestTypes)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", fmt.Errorf("manifest fetch failed: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	mediaType := resp.Header.Get("Content-Type")
	return data, mediaType, nil
}

func (p *Puller) fetchBlob(ctx context.Context, registry, name, digest string, token string) ([]byte, error) {
	url := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, name, digest)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("blob fetch failed: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func ParseReference(ref string) (registry, name, tag string, err error) {
	u, err := url.Parse("docker://" + ref)
	if err != nil {
		return "", "", "", err
	}

	registry = u.Host
	if registry == "" {
		registry = "docker.io"
	}

	name = strings.TrimPrefix(u.Path, "/")
	name = strings.TrimSuffix(name, "/")

	tag = "latest"
	if u.Fragment != "" {
		tag = u.Fragment
	}

	return registry, name, tag, nil
}
