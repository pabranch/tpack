package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tmuxpack/tpack/internal/plug"
	"github.com/tmuxpack/tpack/internal/state"
)

func TestSaveAndLoadLoadErrors(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "tpack")
	want := []plug.LoadFailure{
		{Name: "tmux-statusline", Message: "error sourcing statusline.tmux: exec format error"},
		{Name: "other", Message: "boom"},
	}

	if err := state.SaveLoadErrors(statePath, want); err != nil {
		t.Fatalf("SaveLoadErrors failed: %v", err)
	}

	got := state.LoadLoadErrors(statePath)
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("round-trip mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestSaveLoadErrorsOverwrites(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "tpack")
	_ = state.SaveLoadErrors(statePath, []plug.LoadFailure{{Name: "a", Message: "1"}})
	_ = state.SaveLoadErrors(statePath, []plug.LoadFailure{{Name: "b", Message: "2"}})

	got := state.LoadLoadErrors(statePath)
	if len(got) != 1 || got[0].Name != "b" {
		t.Errorf("expected only the second save to remain, got %v", got)
	}
}

func TestSaveLoadErrorsEmptyRemovesFile(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "tpack")
	_ = state.SaveLoadErrors(statePath, []plug.LoadFailure{{Name: "a", Message: "1"}})

	if err := state.SaveLoadErrors(statePath, nil); err != nil {
		t.Fatalf("SaveLoadErrors(nil) failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(statePath, "load-errors.yml")); !os.IsNotExist(err) {
		t.Error("expected load-errors.yml to be removed on empty save")
	}
	if got := state.LoadLoadErrors(statePath); got != nil {
		t.Errorf("expected nil after empty save, got %v", got)
	}
}

func TestLoadLoadErrorsCorruptFile(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "tpack")
	os.MkdirAll(statePath, 0o755)
	os.WriteFile(filepath.Join(statePath, "load-errors.yml"), []byte("{{bad yaml!"), 0o644)

	if got := state.LoadLoadErrors(statePath); got != nil {
		t.Errorf("expected nil on corrupt file, got %v", got)
	}
}

func TestLoadLoadErrorsMissingFile(t *testing.T) {
	if got := state.LoadLoadErrors(filepath.Join(t.TempDir(), "tpack")); got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}
