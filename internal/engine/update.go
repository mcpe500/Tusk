package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mcpe500/Tusk/internal/artifact"
	"github.com/mcpe500/Tusk/internal/state"
)

type UpdatePlan struct {
	HostBinary    string `json:"hostBinary"`
	SystemImage   string `json:"systemImage"`
	GuestStack    string `json:"guestStack"`
	ConfigSchema  string `json:"configSchema"`
	DataMigration string `json:"dataMigration"`
}

type UpdateManager struct {
	Root string
}

func VerifyReleaseManifest(manifest artifact.Manifest, publicKey []byte) error {
	return manifest.Verify(publicKey)
}

func (manager UpdateManager) Check(ctx context.Context, plan UpdatePlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if manager.Root == "" {
		return errors.New("update root is required")
	}
	return state.WriteJSON(filepath.Join(manager.Root, "update-plan.json"), plan, 0o600)
}

// ReplaceSystem performs a transactional replace and keeps the old system
// until the new readiness callback succeeds. Persistent data is never touched.
func (manager UpdateManager) ReplaceSystem(ctx context.Context, current, replacement, data string, ready func(context.Context) error) error {
	if ready == nil {
		return errors.New("update readiness check is required")
	}
	if _, err := os.Stat(data); err != nil {
		return fmt.Errorf("persistent data must exist before update: %w", err)
	}
	if _, err := os.Stat(replacement); err != nil {
		return fmt.Errorf("replacement system is unavailable: %w", err)
	}
	backup := current + ".previous-" + time.Now().UTC().Format("20060102T150405Z")
	if err := os.Rename(current, backup); err != nil {
		return fmt.Errorf("preserve previous system: %w", err)
	}
	if err := os.Rename(replacement, current); err != nil {
		_ = os.Rename(backup, current)
		return fmt.Errorf("activate replacement system: %w", err)
	}
	if err := ready(ctx); err != nil {
		_ = os.RemoveAll(current)
		if rollbackErr := os.Rename(backup, current); rollbackErr != nil {
			return fmt.Errorf("update health failed: %v; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("update rolled back after failed readiness: %w", err)
	}
	return nil
}
