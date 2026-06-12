package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/asocpro/workshop-builder/backend/store"
)

func testWorkshopRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "workshop")
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return newTestServerAt(t, testWorkshopRoot(t), "container")
}

func newTestServerAt(t *testing.T, root, mode string) http.Handler {
	t.Helper()
	meta, err := store.LoadMetadata(root)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	st := store.NewState(meta)
	cmdLog := store.NewCommandLog(filepath.Join(root, "runtime", "command-log.jsonl"))
	return NewServer(meta, st, "http://localhost:9090", cmdLog, mode)
}

func TestGetState(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["activeStep"] != "step-1-intro" {
		t.Errorf("activeStep = %v", resp["activeStep"])
	}
	if resp["navigationMode"] != "linear" {
		t.Errorf("navigationMode = %v", resp["navigationMode"])
	}
	if resp["managementURL"] != "http://localhost:9090" {
		t.Errorf("managementURL = %v", resp["managementURL"])
	}
}

func TestListSteps(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/steps", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var steps []map[string]any
	json.NewDecoder(w.Body).Decode(&steps)
	if len(steps) != 3 {
		t.Fatalf("len(steps) = %d, want 3", len(steps))
	}
	// First step accessible (linear nav, nothing completed yet)
	if steps[0]["accessible"] != true {
		t.Error("step-1 should be accessible")
	}
	// Second step not accessible (step-1 not completed)
	if steps[1]["accessible"] != false {
		t.Error("step-2 should not be accessible until step-1 completed")
	}
}

func TestGetStepContent(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/steps/step-1-intro/content", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() == 0 {
		t.Error("content body is empty")
	}
}

func TestGetStepContent_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/steps/nonexistent/content", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestNavigate_Blocked(t *testing.T) {
	srv := newTestServer(t)
	// Try to navigate to step-2 before step-1 is completed (linear mode)
	req := httptest.NewRequest("POST", "/api/steps/step-2-files/navigate", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestActivate_NotRoutedInContainerMode(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/steps/step-1-intro/activate", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("activate should not be routed in container mode, got 200")
	}
}

// inPlaceWorkshopRoot builds a minimal compiled workshop with setup.json files.
func inPlaceWorkshopRoot(t *testing.T) (root, marker string) {
	t.Helper()
	root = t.TempDir()
	marker = filepath.Join(t.TempDir(), "marker.txt")

	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("workshop.json", `{
		"name": "inplace-test", "image": "localhost/inplace-test", "navigation": "free",
		"steps": [
			{"id": "step-a", "title": "A", "position": 0},
			{"id": "step-b", "title": "B", "position": 1}
		]
	}`)
	write("steps/step-a/meta.json", `{"id": "step-a", "title": "A", "position": 0}`)
	write("steps/step-a/content.md", "# A")
	write("steps/step-a/setup.json", `{"files": [], "commands": ["echo applied > `+marker+`"], "env": {}}`)
	write("steps/step-b/meta.json", `{"id": "step-b", "title": "B", "position": 1}`)
	write("steps/step-b/content.md", "# B")
	write("steps/step-b/setup.json", `{"files": [], "commands": [], "env": {}}`)
	return root, marker
}

func TestActivate_InPlaceMode(t *testing.T) {
	root, marker := inPlaceWorkshopRoot(t)
	srv := newTestServerAt(t, root, "standalone")

	// First activation runs setup
	req := httptest.NewRequest("POST", "/api/steps/step-a/activate", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["setup"] != "applied" {
		t.Errorf("setup = %q, want applied", resp["setup"])
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("setup command should have created marker: %v", err)
	}

	// Second activation skips setup
	req = httptest.NewRequest("POST", "/api/steps/step-a/activate", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["setup"] != "already_applied" {
		t.Errorf("setup = %q, want already_applied", resp["setup"])
	}
}

func TestState_ReportsMode(t *testing.T) {
	root, _ := inPlaceWorkshopRoot(t)
	srv := newTestServerAt(t, root, "standalone")
	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["mode"] != "standalone" {
		t.Errorf("mode = %v, want standalone", resp["mode"])
	}
	if resp["inPlace"] != true {
		t.Errorf("inPlace = %v, want true", resp["inPlace"])
	}
}
