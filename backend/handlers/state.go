package handlers

import "net/http"

type stateResponse struct {
	ActiveStep     string   `json:"activeStep"`
	CompletedSteps []string `json:"completedSteps"`
	NavigationMode string   `json:"navigationMode"`
	ManagementURL  string   `json:"managementURL,omitempty"`
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
	})
}
