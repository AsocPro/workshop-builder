package workshop_test

import (
	"encoding/json"
	"testing"

	"github.com/asocpro/workshop-builder/pkg/workshop"
)

func TestCompile_ValidLinear(t *testing.T) {
	w, err := workshop.Parse(testdataPath(t, "valid-linear"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := workshop.Validate(w); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	c, err := workshop.Compile(w)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// workshop.json
	var wj workshop.WorkshopJSON
	if err := json.Unmarshal(c.WorkshopJSON, &wj); err != nil {
		t.Fatalf("unmarshal workshop.json: %v", err)
	}
	if wj.Name != "test-linear" {
		t.Errorf("name = %q", wj.Name)
	}
	if wj.Navigation != "linear" {
		t.Errorf("navigation = %q", wj.Navigation)
	}
	if len(wj.Steps) != 2 {
		t.Errorf("len(steps) = %d", len(wj.Steps))
	}
	if wj.Steps[0].Position != 0 || wj.Steps[1].Position != 1 {
		t.Error("positions wrong")
	}

	// per-step meta.json
	if len(c.Steps) != 2 {
		t.Fatalf("len(compiled steps) = %d", len(c.Steps))
	}
	var meta workshop.MetaJSON
	if err := json.Unmarshal(c.Steps[0].MetaJSON, &meta); err != nil {
		t.Fatalf("unmarshal meta.json: %v", err)
	}
	if meta.ID != "step-one" {
		t.Errorf("meta.ID = %q", meta.ID)
	}
	if meta.HasGoss {
		t.Error("HasGoss should be false for valid-linear fixture")
	}
}

func TestCompile_WithGoss(t *testing.T) {
	w, _ := workshop.Parse(testdataPath(t, "valid-with-goss"))
	workshop.Validate(w)
	c, err := workshop.Compile(w)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	var meta workshop.MetaJSON
	json.Unmarshal(c.Steps[0].MetaJSON, &meta)
	if !meta.HasGoss {
		t.Error("HasGoss should be true")
	}
	if !meta.HasHints {
		t.Error("HasHints should be true")
	}
}
