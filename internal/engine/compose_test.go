package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeArgsAllowsTestedProviderSubsetOnly(t *testing.T) {
	t.Parallel()
	args, err := ComposeArgs([]string{"up", "-d"})
	if err != nil || len(args) != 3 || args[0] != "compose" {
		t.Fatalf("ComposeArgs() = %#v, %v", args, err)
	}
	if _, err := ComposeArgs([]string{"destroy"}); err == nil || !strings.Contains(err.Error(), "not suppressed") {
		t.Fatal("unsupported Compose command was silently accepted")
	}
}

func TestComposeMetadataIsPinnedAndVersioned(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "compose.json")
	want := ComposeMetadata{SchemaVersion: ComposeMetadataSchemaVersion, Provider: ComposeProvider{Name: "podman-compose", Version: "1.2.0"}}
	if err := SaveComposeMetadata(path, want); err != nil {
		t.Fatalf("SaveComposeMetadata() error = %v", err)
	}
	got, err := LoadComposeMetadata(path)
	if err != nil || got.Provider.Version != "1.2.0" {
		t.Fatalf("LoadComposeMetadata() = %#v, %v", got, err)
	}
}
