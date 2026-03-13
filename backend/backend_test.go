package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	meta, err := store.LoadMetadata(testWorkshopRoot(t))
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	st := store.NewState(meta)
	return NewServer(meta, st, "http://localhost:9090")
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
