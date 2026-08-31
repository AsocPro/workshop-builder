package store

import (
	"os"
	"regexp"
	"sort"
	"time"
)

// RecordingInfo describes one asciicast session file in the runtime dir.
type RecordingInfo struct {
	Filename  string `json:"filename"`
	StartTime string `json:"startTime"` // RFC3339, parsed from the filename timestamp
	Size      int64  `json:"size"`
}

var recordingNameRE = regexp.MustCompile(`^session-[0-9TZ]+\.cast$`)

// ValidRecordingName reports whether name is a well-formed session recording
// filename. Used as a path-traversal / arbitrary-read guard before serving.
func ValidRecordingName(name string) bool {
	return recordingNameRE.MatchString(name)
}

// ListRecordings returns the session recordings in runtimeDir, newest first.
// A missing directory yields an empty list, not an error.
func ListRecordings(runtimeDir string) ([]RecordingInfo, error) {
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var recs []RecordingInfo
	for _, e := range entries {
		if e.IsDir() || !ValidRecordingName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		recs = append(recs, RecordingInfo{
			Filename:  e.Name(),
			StartTime: parseSessionStart(e.Name()),
			Size:      info.Size(),
		})
	}

	// Filenames are fixed-width zero-padded timestamps, so lexical descending
	// order is chronological newest-first.
	sort.Slice(recs, func(i, j int) bool { return recs[i].Filename > recs[j].Filename })
	return recs, nil
}

// parseSessionStart turns "session-20250102T000000Z.cast" into RFC3339. If the
// timestamp can't be parsed, it returns the raw token so callers still get
// something stable.
func parseSessionStart(name string) string {
	ts := name[len("session-") : len(name)-len(".cast")]
	if t, err := time.Parse("20060102T150405Z", ts); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return ts
}
