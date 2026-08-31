package handlers

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/asocpro/workshop-builder/backend/store"
)

func (h *Handlers) ListRecordings(w http.ResponseWriter, r *http.Request) {
	recs, err := store.ListRecordings(h.RuntimeDir)
	if err != nil {
		log.Printf("ListRecordings: %v", err)
		http.Error(w, "listing recordings", http.StatusInternalServerError)
		return
	}
	if recs == nil {
		recs = []store.RecordingInfo{}
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *Handlers) GetRecording(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if !store.ValidRecordingName(name) {
		http.Error(w, "invalid recording name", http.StatusBadRequest)
		return
	}
	f, err := os.Open(filepath.Join(h.RuntimeDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "stat recording", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-asciicast")
	http.ServeContent(w, r, name, fi.ModTime(), f)
}
