package handlers

import (
	"net/http"

	"github.com/asocpro/workshop-builder/backend/setup"
)

type stateResponse struct {
	ActiveStep     string   `json:"activeStep"`
	CompletedSteps []string `json:"completedSteps"`
	NavigationMode string   `json:"navigationMode"`
	ManagementURL  string   `json:"managementURL,omitempty"`
	Mode           string   `json:"mode"`
	InPlace        bool     `json:"inPlace"`
}

func (h *Handlers) GetState(w http.ResponseWriter, r *http.Request) {
	completed := h.State.CompletedSteps()
	if completed == nil {
		completed = []string{}
	}
	writeJSON(w, http.StatusOK, stateResponse{
		ActiveStep:     h.State.ActiveStepID(),
		CompletedSteps: completed,
		NavigationMode: h.Meta.Workshop.Navigation,
		ManagementURL:  h.ManagementURL,
		Mode:           h.Mode,
		InPlace:        setup.InPlaceMode(h.Mode),
	})
}
