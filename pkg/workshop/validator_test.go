package workshop_test

import (
	"strings"
	"testing"

	"github.com/asocpro/workshop-builder/pkg/workshop"
)

func TestValidate_ValidLinear(t *testing.T) {
	w, _ := workshop.Parse(testdataPath(t, "valid-linear"))
	if err := workshop.Validate(w); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidate_MissingTitle(t *testing.T) {
	w, _ := workshop.Parse(testdataPath(t, "invalid-missing-title"))
	err := workshop.Validate(w)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("error %q does not contain 'title is required'", err.Error())
	}
}

func TestValidate_DuplicateStep(t *testing.T) {
	// Parse will succeed (parser doesn't validate); validator catches it
	w, _ := workshop.Parse(testdataPath(t, "invalid-duplicate-step"))
	err := workshop.Validate(w)
	if err == nil {
		t.Fatal("expected error for duplicate step")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("error %q does not contain 'already used'", err.Error())
	}
}
