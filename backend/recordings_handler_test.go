package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/asocpro/workshop-builder/backend/store"
)

func serverWithRuntime(t *testing.T, runtimeDir string) http.Handler {
	t.Helper()
	root := testWorkshopRoot(t)
	meta, err := store.LoadMetadata(root)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	st := store.NewState(meta)
	cmdLog := store.NewCommandLog(filepath.Join(runtimeDir, "command-log.jsonl"))
	return NewServer(meta, st, "", cmdLog, "container", runtimeDir)
}

func TestListRecordingsEndpoint(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "session-20250101T000000Z.cast"), []byte(`{"version":3}`), 0644)
	srv := serverWithRuntime(t, dir)

	req := httptest.NewRequest("GET", "/api/recordings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var recs []store.RecordingInfo
	json.NewDecoder(w.Body).Decode(&recs)
	if len(recs) != 1 || recs[0].Filename != "session-20250101T000000Z.cast" {
		t.Fatalf("unexpected recordings: %+v", recs)
	}
}

func TestGetRecordingServesWithRange(t *testing.T) {
	dir := t.TempDir()
	body := `{"version":3}` + "\n" + `[0.1, "o", "hi"]`
	os.WriteFile(filepath.Join(dir, "session-20250101T000000Z.cast"), []byte(body), 0644)
	srv := serverWithRuntime(t, dir)

	// Full fetch
	req := httptest.NewRequest("GET", "/api/recordings/session-20250101T000000Z.cast", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("full fetch status = %d", w.Code)
	}
	if w.Body.String() != body {
		t.Errorf("body mismatch")
	}

	// Range request → 206
	req2 := httptest.NewRequest("GET", "/api/recordings/session-20250101T000000Z.cast", nil)
	req2.Header.Set("Range", "bytes=0-4")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusPartialContent {
		t.Errorf("range status = %d, want 206", w2.Code)
	}
}

func TestGetRecordingRejectsBadName(t *testing.T) {
	srv := serverWithRuntime(t, t.TempDir())
	req := httptest.NewRequest("GET", "/api/recordings/evil.txt", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
