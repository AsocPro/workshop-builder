package compilecli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunEmitsWorkshopJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Run("../../../examples/hello-linux", "", &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var out Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if out.WorkshopJSON == "" {
		t.Fatal("workshopJson is empty")
	}
	if len(out.Steps) == 0 {
		t.Fatal("no steps in output")
	}
}
