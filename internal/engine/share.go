package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcpe500/Tusk/internal/state"
)

const ShareSchemaVersion = 1

type Share struct {
	Name      string `json:"name"`
	HostPath  string `json:"hostPath"`
	GuestPath string `json:"guestPath"`
	ReadOnly  bool   `json:"readOnly"`
}

type ShareRegistry struct {
	SchemaVersion int     `json:"schemaVersion"`
	Shares        []Share `json:"shares"`
}

func DefaultShareRegistry(workspaces string) (ShareRegistry, error) {
	path, err := canonicalDirectory(workspaces, true)
	if err != nil {
		return ShareRegistry{}, err
	}
	return ShareRegistry{SchemaVersion: ShareSchemaVersion, Shares: []Share{{Name: "workspaces", HostPath: path, GuestPath: "/mnt/tusk/workspaces"}}}, nil
}

func LoadShares(path, defaultWorkspaces string) (ShareRegistry, error) {
	var registry ShareRegistry
	if err := state.ReadJSON(path, &registry); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultShareRegistry(defaultWorkspaces)
		}
		return ShareRegistry{}, err
	}
	if err := ValidateShares(registry); err != nil {
		return ShareRegistry{}, err
	}
	return registry, nil
}

func SaveShares(path string, registry ShareRegistry) error {
	if err := ValidateShares(registry); err != nil {
		return err
	}
	return state.WriteJSON(path, registry, 0o600)
}

func AddShare(registry ShareRegistry, hostPath, name string, readOnly bool) (ShareRegistry, error) {
	if registry.SchemaVersion == 0 {
		registry.SchemaVersion = ShareSchemaVersion
	}
	canonical, err := canonicalDirectory(hostPath, false)
	if err != nil {
		return ShareRegistry{}, err
	}
	if name == "" {
		name = filepath.Base(canonical)
	}
	if !validShareName(name) {
		return ShareRegistry{}, fmt.Errorf("invalid share name %q", name)
	}
	for _, share := range registry.Shares {
		if share.Name == name {
			return ShareRegistry{}, fmt.Errorf("share %q already exists", name)
		}
		if pathsOverlap(canonical, share.HostPath) {
			return ShareRegistry{}, fmt.Errorf("share %q overlaps registered share %q", canonical, share.Name)
		}
	}
	registry.Shares = append(registry.Shares, Share{Name: name, HostPath: canonical, GuestPath: "/mnt/tusk/shares/" + name, ReadOnly: readOnly})
	return registry, ValidateShares(registry)
}

func RemoveShare(registry ShareRegistry, name string) (ShareRegistry, error) {
	if name == "workspaces" {
		return ShareRegistry{}, errors.New("the default workspaces share cannot be removed")
	}
	filtered := registry.Shares[:0]
	found := false
	for _, share := range registry.Shares {
		if share.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, share)
	}
	if !found {
		return ShareRegistry{}, fmt.Errorf("share %q does not exist", name)
	}
	registry.Shares = filtered
	return registry, nil
}

func ValidateShares(registry ShareRegistry) error {
	if registry.SchemaVersion != ShareSchemaVersion {
		return fmt.Errorf("unsupported share schema %d", registry.SchemaVersion)
	}
	seenNames := make(map[string]struct{}, len(registry.Shares))
	for _, share := range registry.Shares {
		if !validShareName(share.Name) || !filepath.IsAbs(share.GuestPath) {
			return fmt.Errorf("invalid share %q", share.Name)
		}
		if _, exists := seenNames[share.Name]; exists {
			return fmt.Errorf("duplicate share name %q", share.Name)
		}
		seenNames[share.Name] = struct{}{}
		canonical, err := canonicalDirectory(share.HostPath, false)
		if err != nil || canonical != share.HostPath {
			return fmt.Errorf("share %q host path is not canonical", share.Name)
		}
	}
	for index, left := range registry.Shares {
		for _, right := range registry.Shares[index+1:] {
			if pathsOverlap(left.HostPath, right.HostPath) {
				return fmt.Errorf("shares %q and %q overlap", left.Name, right.Name)
			}
		}
	}
	return nil
}

func TranslateHostPath(path, workingDirectory string, registry ShareRegistry) (string, error) {
	if strings.HasPrefix(path, "/mnt/tusk/") {
		return filepath.Clean(path), nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolvedPath, err := resolvePathForContainment(path)
	if err != nil {
		return "", err
	}
	for _, share := range registry.Shares {
		if relative, ok := relativeWithin(share.HostPath, resolvedPath); ok {
			if relative == "." {
				return share.GuestPath, nil
			}
			return filepath.Join(share.GuestPath, relative), nil
		}
	}
	return "", fmt.Errorf("host path %q is outside registered shares; run `tusk share add %s`", path, path)
}

func TranslateMount(spec, workingDirectory string, registry ShareRegistry) (string, error) {
	if strings.HasPrefix(spec, "type=bind,") || strings.Contains(spec, "source=") {
		return translateKeyValueMount(spec, workingDirectory, registry)
	}
	if strings.HasPrefix(spec, "/mnt/tusk/") {
		return spec, nil
	}
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid bind mount %q", spec)
	}
	if strings.Contains(parts[0], "\\") || strings.Count(parts[0], ":") > 0 {
		return "", fmt.Errorf("bind mount host path contains an unsupported colon: %q", parts[0])
	}
	translated, err := TranslateHostPath(parts[0], workingDirectory, registry)
	if err != nil {
		return "", err
	}
	options := parts[2:]
	if shareIsReadOnly(parts[0], workingDirectory, registry) && !containsReadOnly(options) {
		options = append(options, "ro")
	}
	result := translated + ":" + parts[1]
	if len(options) > 0 {
		result += ":" + strings.Join(options, ":")
	}
	return result, nil
}

func translateKeyValueMount(spec, workingDirectory string, registry ShareRegistry) (string, error) {
	fields := strings.Split(spec, ",")
	values := make(map[string]string)
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 {
			values[parts[0]] = parts[1]
		}
	}
	source := values["source"]
	if source == "" {
		source = values["src"]
	}
	target := values["target"]
	if target == "" {
		target = values["dst"]
	}
	if source == "" || target == "" {
		return "", fmt.Errorf("bind mount requires source and target: %q", spec)
	}
	translated, err := TranslateHostPath(source, workingDirectory, registry)
	if err != nil {
		return "", err
	}
	for index, field := range fields {
		if strings.HasPrefix(field, "source=") || strings.HasPrefix(field, "src=") {
			fields[index] = "source=" + translated
		}
	}
	if shareIsReadOnly(source, workingDirectory, registry) && !containsMountReadOnly(fields) {
		fields = append(fields, "readonly")
	}
	return strings.Join(fields, ","), nil
}

func TranslatePodmanArgs(args []string, workingDirectory string, registry ShareRegistry) ([]string, string, error) {
	guestWorkingDirectory, err := TranslateHostPath(workingDirectory, workingDirectory, registry)
	if err != nil {
		guestWorkingDirectory = "/mnt/tusk/workspaces"
	}
	translated := append([]string(nil), args...)
	for index := 0; index < len(translated); index++ {
		argument := translated[index]
		if argument == "-v" || argument == "--volume" || argument == "--mount" {
			if index+1 >= len(translated) {
				return nil, "", fmt.Errorf("%s requires a value", argument)
			}
			value, err := TranslateMount(translated[index+1], workingDirectory, registry)
			if err != nil {
				return nil, "", err
			}
			translated[index+1] = value
			index++
			continue
		}
		if argument == "-f" || argument == "--file" || argument == "--env-file" || argument == "--project-directory" {
			if index+1 >= len(translated) {
				return nil, "", fmt.Errorf("%s requires a value", argument)
			}
			value, err := TranslateHostPath(translated[index+1], workingDirectory, registry)
			if err != nil {
				return nil, "", err
			}
			translated[index+1] = value
			index++
		}
	}
	if len(translated) > 1 && translated[0] == "build" {
		last := len(translated) - 1
		if !strings.HasPrefix(translated[last], "-") {
			translated[last], err = TranslateHostPath(translated[last], workingDirectory, registry)
			if err != nil {
				return nil, "", err
			}
		}
	}
	return translated, guestWorkingDirectory, nil
}

func canonicalDirectory(path string, create bool) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("share path must be absolute: %q", path)
	}
	if create {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return "", err
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("share path unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("share path is not a directory: %q", path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func relativeWithin(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return relative, true
}

func pathsOverlap(left, right string) bool {
	_, leftContains := relativeWithin(left, right)
	_, rightContains := relativeWithin(right, left)
	return leftContains || rightContains
}

func validShareName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\:`) {
		return false
	}
	return filepath.Base(name) == name
}

func shareIsReadOnly(path, workingDirectory string, registry ShareRegistry) bool {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absolute, err = resolvePathForContainment(absolute)
	if err != nil {
		return false
	}
	for _, share := range registry.Shares {
		if _, ok := relativeWithin(share.HostPath, absolute); ok {
			return share.ReadOnly
		}
	}
	return false
}

func resolvePathForContainment(path string) (string, error) {
	path = filepath.Clean(path)
	suffix := []string{}
	for current := path; ; current = filepath.Dir(current) {
		if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path contains symlink component %q", current)
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path, nil
		}
		suffix = append(suffix, filepath.Base(current))
	}
}

func containsReadOnly(options []string) bool {
	for _, option := range options {
		if option == "ro" || option == "readonly" {
			return true
		}
	}
	return false
}

func containsMountReadOnly(fields []string) bool {
	return containsReadOnly(fields)
}
