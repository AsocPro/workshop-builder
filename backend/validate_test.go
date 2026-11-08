package backend

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyWorkshop copies the testdata workshop into a temp dir so runtime/ files
// are never written into the source tree.
func copyWorkshop(t *testing.T) string {
	t.Helper()
	src := testWorkshopRoot(t)
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// fakeGoss installs an executable "goss" with the given sh body at the front of
// PATH. Tests using it must not call t.Parallel (PATH is process-wide).
func fakeGoss(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "goss"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// noGoss makes sure no goss binary can be found.
func noGoss(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

const gossPassJSON = `{"results":[{"resource-id":"/workspace/hello.sh","resource-type":"File","property":"exists","result":0,"summary-line":"ok"}],"summary":{"test-count":1,"failed-count":0}}`
const gossFailJSON = `{"results":[{"resource-id":"/workspace/hello.sh","resource-type":"File","property":"exists","result":1,"summary-line":"File: /workspace/hello.sh: exists: expected true"}],"summary":{"test-count":1,"failed-count":1}}`

func fakeGossPass(t *testing.T) { fakeGoss(t, "cat <<'EOF'\n"+gossPassJSON+"\nEOF") }

func postValidate(t *testing.T, srv http.Handler, id string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/steps/"+id+"/validate", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func completedSteps(t *testing.T, srv http.Handler) []any {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	c, _ := resp["completedSteps"].([]any)
	return c
}

func readEvents(t *testing.T, root string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(root, "runtime", "state-events.jsonl"))
	if err != nil {
		t.Fatalf("state events not written: %v", err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("invalid event line %q: %v", sc.Text(), err)
		}
		out = append(out, ev)
	}
	return out
}

func TestValidate_NotFound(t *testing.T) {
	srv := newTestServerAt(t, copyWorkshop(t), "container")
	if code, _ := postValidate(t, srv, "nope"); code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestValidate_BlockedStepDoesNotRunGoss(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	fakeGoss(t, "touch "+marker+"\necho '"+gossPassJSON+"'")
	srv := newTestServerAt(t, copyWorkshop(t), "container")

	code, body := postValidate(t, srv, "step-2-files") // step-1 not complete (linear)
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "not accessible") {
		t.Errorf("error = %q, want a clear message", msg)
	}
	if checks, ok := body["checks"].([]any); !ok || len(checks) != 0 {
		t.Errorf("checks = %v, want empty array", body["checks"])
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("goss must not run for a locked step")
	}
	if c := completedSteps(t, srv); len(c) != 0 {
		t.Errorf("completed = %v, want none", c)
	}
}

func TestValidate_BlockedNoGossStepStaysIncomplete(t *testing.T) {
	root := copyWorkshop(t)
	// Make step-2 a no-goss step; it is still locked behind step-1.
	meta := filepath.Join(root, "steps", "step-2-files", "meta.json")
	data, _ := os.ReadFile(meta)
	os.WriteFile(meta, []byte(strings.Replace(string(data), `"hasGoss": true`, `"hasGoss": false`, 1)), 0644)
	srv := newTestServerAt(t, root, "container")

	if code, _ := postValidate(t, srv, "step-2-files"); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if c := completedSteps(t, srv); len(c) != 0 {
		t.Errorf("completed = %v, want none", c)
	}
}

func TestValidate_NoGossAccessibleAndIdempotent(t *testing.T) {
	srv := newTestServerAt(t, copyWorkshop(t), "container")
	for i := 0; i < 2; i++ {
		code, body := postValidate(t, srv, "step-1-intro")
		if code != http.StatusOK || body["passed"] != true {
			t.Fatalf("attempt %d: status=%d body=%v", i, code, body)
		}
	}
	if c := completedSteps(t, srv); len(c) != 1 || c[0] != "step-1-intro" {
		t.Errorf("completed = %v", c)
	}
}

func TestValidate_PassCompletesAndUnlocksNext(t *testing.T) {
	root := copyWorkshop(t)
	fakeGossPass(t)
	srv := newTestServerAt(t, root, "container")

	postValidate(t, srv, "step-1-intro")
	code, body := postValidate(t, srv, "step-2-files")
	if code != http.StatusOK || body["passed"] != true {
		t.Fatalf("status=%d body=%v", code, body)
	}
	if c := completedSteps(t, srv); len(c) != 2 {
		t.Errorf("completed = %v, want 2 steps", c)
	}
	// step-3 is now accessible
	if code, _ := postValidate(t, srv, "step-3-validate"); code != http.StatusOK {
		t.Errorf("step-3 status = %d, want 200", code)
	}
	evs := readEvents(t, root)
	if len(evs) == 0 || evs[0]["event"] != "goss_result" || evs[0]["passed"] != true {
		t.Errorf("events = %v", evs)
	}
}

func TestValidate_FailIsReportedNotCompleted(t *testing.T) {
	root := copyWorkshop(t)
	srv := newTestServerAt(t, root, "container")
	postValidate(t, srv, "step-1-intro")
	fakeGoss(t, "cat <<'EOF'\n"+gossFailJSON+"\nEOF\nexit 1")

	code, body := postValidate(t, srv, "step-2-files")
	if code != http.StatusOK || body["passed"] != false {
		t.Fatalf("status=%d body=%v", code, body)
	}
	checks, _ := body["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("checks = %v", body["checks"])
	}
	if c := completedSteps(t, srv); len(c) != 1 {
		t.Errorf("completed = %v, want only step-1", c)
	}
	evs := readEvents(t, root)
	if len(evs) != 1 || evs[0]["passed"] != false {
		t.Errorf("events = %v", evs)
	}
}

func TestValidate_ParseError(t *testing.T) {
	root := copyWorkshop(t)
	srv := newTestServerAt(t, root, "container")
	postValidate(t, srv, "step-1-intro")
	fakeGoss(t, "echo 'SECRET-RAW-OUTPUT not json'")

	code, body := postValidate(t, srv, "step-2-files")
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if msg, _ := body["error"].(string); strings.Contains(msg, "SECRET-RAW-OUTPUT") {
		t.Errorf("raw goss output leaked into response: %q", msg)
	}
	if c := completedSteps(t, srv); len(c) != 1 {
		t.Errorf("completed = %v, want only step-1", c)
	}
	evs := readEvents(t, root)
	if len(evs) != 1 || evs[0]["event"] != "goss_error" {
		t.Errorf("events = %v, want one goss_error", evs)
	}
}

func TestValidate_Timeout(t *testing.T) {
	root := copyWorkshop(t)
	srv := newTestServerAt(t, root, "container")
	postValidate(t, srv, "step-1-intro")
	t.Setenv("WORKSHOP_GOSS_TIMEOUT", "200ms")
	fakeGoss(t, "exec sleep 10")

	code, body := postValidate(t, srv, "step-2-files")
	if code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504, body=%v", code, body)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "timed out") {
		t.Errorf("error = %q", msg)
	}
	if c := completedSteps(t, srv); len(c) != 1 {
		t.Errorf("completed = %v, want only step-1", c)
	}
	evs := readEvents(t, root)
	if len(evs) != 1 || evs[0]["event"] != "goss_error" {
		t.Errorf("events = %v", evs)
	}
}

func TestValidate_InvalidTimeoutEnvFallsBack(t *testing.T) {
	srv := newTestServerAt(t, copyWorkshop(t), "container")
	postValidate(t, srv, "step-1-intro")
	t.Setenv("WORKSHOP_GOSS_TIMEOUT", "garbage")
	fakeGossPass(t)
	if code, body := postValidate(t, srv, "step-2-files"); code != http.StatusOK || body["passed"] != true {
		t.Errorf("status=%d body=%v", code, body)
	}
}

func TestValidate_GossBinaryMissing(t *testing.T) {
	srv := newTestServerAt(t, copyWorkshop(t), "container")
	postValidate(t, srv, "step-1-intro")
	noGoss(t)
	code, body := postValidate(t, srv, "step-2-files")
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "failed to run goss") {
		t.Errorf("error = %q", msg)
	}
}

func TestValidate_FreeModeAllowsAnyStep(t *testing.T) {
	root := copyWorkshop(t)
	wj := filepath.Join(root, "workshop.json")
	data, _ := os.ReadFile(wj)
	os.WriteFile(wj, []byte(strings.Replace(string(data), `"linear"`, `"free"`, 1)), 0644)
	fakeGossPass(t)
	srv := newTestServerAt(t, root, "container")
	if code, _ := postValidate(t, srv, "step-3-validate"); code != http.StatusOK {
		t.Errorf("status = %d, want 200", code)
	}
}

func TestValidate_GuidedRequiresEnforced(t *testing.T) {
	root := copyWorkshop(t)
	wj := filepath.Join(root, "workshop.json")
	data, _ := os.ReadFile(wj)
	os.WriteFile(wj, []byte(strings.Replace(string(data), `"linear"`, `"guided"`, 1)), 0644)
	meta := filepath.Join(root, "steps", "step-2-files", "meta.json")
	md, _ := os.ReadFile(meta)
	os.WriteFile(meta, []byte(strings.Replace(string(md), `"position": 1,`, `"position": 1, "requires": ["step-1-intro"],`, 1)), 0644)
	fakeGossPass(t)
	srv := newTestServerAt(t, root, "container")

	if code, _ := postValidate(t, srv, "step-2-files"); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 while required step incomplete", code)
	}
	postValidate(t, srv, "step-1-intro")
	if code, _ := postValidate(t, srv, "step-2-files"); code != http.StatusOK {
		t.Errorf("status = %d, want 200 once requirement met", code)
	}
}

func TestValidate_CreatesRuntimeDir(t *testing.T) {
	root := copyWorkshop(t)
	if _, err := os.Stat(filepath.Join(root, "runtime")); err == nil {
		t.Fatal("runtime dir should not exist yet")
	}
	srv := newTestServerAt(t, root, "container")
	postValidate(t, srv, "step-1-intro")
	fakeGossPass(t)
	postValidate(t, srv, "step-2-files")
	if evs := readEvents(t, root); len(evs) == 0 {
		t.Error("expected at least one state event")
	}
}
