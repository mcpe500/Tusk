package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := struct {
		Name string `json:"name"`
	}{Name: "tusk"}

	if err := WriteJSON(path, want, 0o600); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}

	var got struct {
		Name string `json:"name"`
	}
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if got != want {
		t.Fatalf("ReadJSON() = %#v, want %#v", got, want)
	}
}

func TestReadJSONRejectsTrailingValue(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{}\n{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := ReadJSON(path, &value); err == nil {
		t.Fatal("ReadJSON() accepted multiple JSON values")
	}
}
