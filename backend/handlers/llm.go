package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

var validHelpModes = map[string]bool{
	"hints":   true,
	"explain": true,
	"solve":   true,
}

func (h *Handlers) LLMHelp(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mode := r.URL.Query().Get("mode")

	if !validHelpModes[mode] {
		http.Error(w, "invalid mode; must be hints, explain, or solve", http.StatusBadRequest)
		return
	}

	meta, ok := h.Meta.StepsByID[id]
	if !ok {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}

	// Check if the requested mode is available
	switch mode {
	case "hints":
		if !meta.HasHints {
			http.Error(w, "hints not available for this step", http.StatusNotFound)
			return
		}
	case "explain":
		if !meta.HasExplain {
			http.Error(w, "explain not available for this step", http.StatusNotFound)
			return
		}
	case "solve":
		if !meta.HasSolve {
			http.Error(w, "solve not available for this step", http.StatusNotFound)
			return
		}
	}

	content, err := os.ReadFile(h.Meta.StepHelpPath(id, mode))
	if err != nil {
		http.Error(w, "help content not found", http.StatusNotFound)
		return
	}

	// Serve as SSE stream
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// JSON-encode the content so newlines are escaped into a single SSE data line
	encoded, _ := json.Marshal(string(content))
	fmt.Fprintf(w, "data: %s\n\n", encoded)
	flusher.Flush()

	// Send done signal
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *Handlers) LLMHistory(w http.ResponseWriter, r *http.Request) {
	// For static mode, history is not tracked — return empty array
	writeJSON(w, http.StatusOK, []any{})
}
