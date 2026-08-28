package setup

import (
	"os"
	"path/filepath"
	"testing"
)

// writeWorkshop builds a minimal compiled workshop dir: workshop.json listing
// the given step IDs, each with an empty setup.json (no files/commands).
func writeWorkshop(t *testing.T, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	steps := ""
	for i, id := range ids {
		if i > 0 {
			steps += ","
		}
		steps += `{"id":"` + id + `"}`
	}
	if err := os.WriteFile(filepath.Join(dir, "workshop.json"), []byte(`{"steps":[`+steps+`]}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		stepDir := filepath.Join(dir, "steps", id)
		if err := os.MkdirAll(stepDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stepDir, "setup.json"), []byte(`{"files":[],"commands":[],"env":{}}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunThroughAppliesThroughStep(t *testing.T) {
	dir := writeWorkshop(t, "step-1", "step-2", "step-3")
	if err := RunThrough(dir, "step-2"); err != nil {
		t.Fatalf("RunThrough: %v", err)
	}
}

func TestRunThroughUnknownStep(t *testing.T) {
	dir := writeWorkshop(t, "step-1", "step-2")
	if err := RunThrough(dir, "nope"); err == nil {
		t.Fatal("expected error for unknown step, got nil")
	}
}

func TestRunThroughEmptyStepIsNoop(t *testing.T) {
	dir := writeWorkshop(t, "step-1")
	if err := RunThrough(dir, ""); err != nil {
		t.Fatalf("RunThrough empty: %v", err)
	}
}
