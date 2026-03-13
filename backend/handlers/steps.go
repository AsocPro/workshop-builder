package handlers

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

type stepListItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Group      string   `json:"group,omitempty"`
	Requires   []string `json:"requires,omitempty"`
	Position   int      `json:"position"`
	Accessible bool     `json:"accessible"`
	Completed  bool     `json:"completed"`
	HasGoss    bool     `json:"hasGoss"`
	HasHints   bool     `json:"hasHints"`
	HasExplain bool     `json:"hasExplain"`
	HasSolve   bool     `json:"hasSolve"`
}

func (h *Handlers) ListSteps(w http.ResponseWriter, r *http.Request) {
	items := make([]stepListItem, 0, len(h.Meta.Steps))
	for _, step := range h.Meta.Steps {
		items = append(items, stepListItem{
			ID:         step.ID,
			Title:      step.Title,
			Group:      step.Group,
			Requires:   step.Requires,
			Position:   step.Position,
			Accessible: h.State.Accessible(step.ID),
			Completed:  h.State.IsCompleted(step.ID),
			HasGoss:    step.HasGoss,
			HasHints:   step.HasHints,
			HasExplain: step.HasExplain,
			HasSolve:   step.HasSolve,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) GetStepContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.Meta.StepsByID[id]; !ok {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}
	content, err := os.ReadFile(h.Meta.StepContentPath(id))
	if err != nil {
		http.Error(w, "content not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write(content)
}

func (h *Handlers) Navigate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.Meta.StepsByID[id]; !ok {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}
	if !h.State.Accessible(id) {
		http.Error(w, "step not accessible", http.StatusForbidden)
		return
	}
	h.State.SetActiveStep(id)
	writeJSON(w, http.StatusOK, map[string]string{"activeStep": id})
}
