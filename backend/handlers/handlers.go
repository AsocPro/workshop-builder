package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/asocpro/workshop-builder/backend/store"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	Meta          *store.Metadata
	State         *store.State
	ManagementURL string
}

func New(meta *store.Metadata, st *store.State, managementURL string) *Handlers {
	return &Handlers{
		Meta:          meta,
		State:         st,
		ManagementURL: managementURL,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
