//go:build linux || android

package lock

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAcquireIsExclusive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "global.lock")
	first, err := Acquire(path, true)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	defer first.Release()

	if _, err := Acquire(path, true); !errors.Is(err, ErrLocked) {
		t.Fatalf("second Acquire() error = %v, want ErrLocked", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	second, err := Acquire(path, true)
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}
}
