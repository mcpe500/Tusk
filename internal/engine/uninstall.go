package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type UninstallOptions struct {
	Root      string
	PurgeData bool
	Stop      func() error
}

func Uninstall(options UninstallOptions) error {
	if options.Root == "" {
		return errors.New("uninstall root is required")
	}
	if options.Stop != nil {
		if err := options.Stop(); err != nil {
			return fmt.Errorf("stop active backend before uninstall: %w", err)
		}
	}
	paths := []string{"config.json", "release.json", "locks", "run", "downloads"}
	if options.PurgeData {
		paths = append(paths, "engines", "benchmarks", "keys")
	}
	for _, relative := range paths {
		if err := os.RemoveAll(filepath.Join(options.Root, relative)); err != nil {
			return fmt.Errorf("remove %s: %w", relative, err)
		}
	}
	if err := removeOwnedShellHooks(); err != nil {
		return err
	}
	return nil
}

func removeOwnedShellHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	for _, name := range []string{".profile", ".bashrc", ".zshrc"} {
		path := filepath.Join(home, name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		value := string(data)
		start, end := strings.Index(value, "# tusk:begin"), strings.Index(value, "# tusk:end")
		if start < 0 || end < start {
			continue
		}
		value = value[:start] + value[end+len("# tusk:end"):]
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func UninstallPlan(options UninstallOptions) []string {
	paths := []string{"config.json", "release.json", "locks", "run", "downloads"}
	if options.PurgeData {
		paths = append(paths, "engines", "benchmarks", "keys")
	}
	return paths
}
