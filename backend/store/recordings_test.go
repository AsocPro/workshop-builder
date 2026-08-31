package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListRecordingsSortsAndFilters(t *testing.T) {
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	must("session-20250101T000000Z.cast", "a")
	must("session-20250102T000000Z.cast", "bb")
	must("command-log.jsonl", "not a recording") // must be ignored

	recs, err := ListRecordings(dir)
	if err != nil {
		t.Fatalf("ListRecordings: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 recordings, got %d: %+v", len(recs), recs)
	}
	if recs[0].Filename != "session-20250102T000000Z.cast" {
		t.Errorf("want newest first, got %s", recs[0].Filename)
	}
	if recs[0].Size != 2 {
		t.Errorf("want size 2, got %d", recs[0].Size)
	}
	if recs[0].StartTime != "2025-01-02T00:00:00Z" {
		t.Errorf("want RFC3339 startTime, got %q", recs[0].StartTime)
	}
}

func TestListRecordingsMissingDir(t *testing.T) {
	recs, err := ListRecordings(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("want empty, got %+v", recs)
	}
}

func TestValidRecordingName(t *testing.T) {
	ok := []string{"session-20250101T000000Z.cast"}
	bad := []string{"../etc/passwd", "session-.cast", "other.cast", "session-1/2.cast", "session-1.cast.bak"}
	for _, n := range ok {
		if !ValidRecordingName(n) {
			t.Errorf("expected valid: %s", n)
		}
	}
	for _, n := range bad {
		if ValidRecordingName(n) {
			t.Errorf("expected invalid: %s", n)
		}
	}
}
