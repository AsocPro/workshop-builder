package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/asocpro/workshop-builder/backend/setup"
)

// Activate switches the workspace to a step in-place: applies the step's
// setup.json (staged files + commands) on first visit, then sets it active.
// Only routed in in-place modes (devcontainer, standalone) — container-swap
// modes have no API for the backend to change its own environment.
func (h *Handlers) Activate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.Meta.StepsByID[id]; !ok {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}
	if !h.State.Accessible(id) {
		http.Error(w, "step not accessible", http.StatusForbidden)
		return
	}

	if h.State.IsStepApplied(id) {
		h.State.SetActiveStep(id)
		writeJSON(w, http.StatusOK, map[string]string{"activeStep": id, "setup": "already_applied"})
		return
	}

	s, err := setup.Load(h.Meta.WorkshopRoot, id)
	if err != nil {
		http.Error(w, "loading setup: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := setup.Apply(h.Meta.WorkshopRoot, id, s); err != nil {
		http.Error(w, "applying setup: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.State.MarkStepApplied(id)
	h.State.SetActiveStep(id)
	writeJSON(w, http.StatusOK, map[string]string{"activeStep": id, "setup": "applied"})
}
