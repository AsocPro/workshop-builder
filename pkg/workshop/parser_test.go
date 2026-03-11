package workshop_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/asocpro/workshop-builder/pkg/workshop"
)

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestParse_ValidLinear(t *testing.T) {
	w, err := workshop.Parse(testdataPath(t, "valid-linear"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if w.Manifest.Workshop.Name != "test-linear" {
		t.Errorf("name = %q, want test-linear", w.Manifest.Workshop.Name)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(w.Steps))
	}
	if w.Steps[0].ID != "step-one" {
		t.Errorf("Steps[0].ID = %q, want step-one", w.Steps[0].ID)
	}
}

func TestParse_ConventionFlags(t *testing.T) {
	w, err := workshop.Parse(testdataPath(t, "valid-with-goss"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	s := w.Steps[0]
	if !s.HasGoss {
		t.Error("HasGoss should be true")
	}
	if !s.HasHints {
		t.Error("HasHints should be true")
	}
	if !s.HasSolve {
		t.Error("HasSolve should be true")
	}
	if s.HasExplain {
		t.Error("HasExplain should be false")
	}
}
