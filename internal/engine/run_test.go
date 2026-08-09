package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTuneJournalRoundTripsAtomically(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "journal.json")
	want := NewTuneJournal("run-1", []string{"a", "b"})
	want.Completed = []string{"a"}
	if err := SaveTuneJournal(path, want); err != nil {
		t.Fatalf("SaveTuneJournal() error = %v", err)
	}
	got, err := LoadTuneJournal(path)
	if err != nil {
		t.Fatalf("LoadTuneJournal() error = %v", err)
	}
	if got.RunID != want.RunID || len(got.Completed) != 1 {
		t.Fatalf("journal = %#v, want %#v", got, want)
	}
	if mode := func() os.FileMode { info, _ := os.Stat(path); return info.Mode().Perm() }(); mode != 0o600 {
		t.Fatalf("journal permissions = %o, want 600", mode)
	}
}

func TestSelectQualifiedFailsClosed(t *testing.T) {
	t.Parallel()
	if _, err := SelectQualified([]Ranking{{CandidateID: "failed"}}); err == nil {
		t.Fatal("SelectQualified() accepted an unqualified candidate")
	}
	ranking, err := SelectQualified([]Ranking{{CandidateID: "ok", Qualified: true}})
	if err != nil || ranking.CandidateID != "ok" {
		t.Fatalf("SelectQualified() = %#v, %v", ranking, err)
	}
}
