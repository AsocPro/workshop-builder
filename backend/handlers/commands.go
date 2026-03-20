package handlers

import (
	"net/http"
	"strconv"

	"github.com/asocpro/workshop-builder/backend/store"
)

type commandsResponse struct {
	Commands []store.Command `json:"commands"`
	Total    int             `json:"total"`
}

func (h *Handlers) ListCommands(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	cmds := h.CommandLog.GetRecent(limit)
	if cmds == nil {
		cmds = []store.Command{}
	}

	writeJSON(w, http.StatusOK, commandsResponse{
		Commands: cmds,
		Total:    h.CommandLog.Len(),
	})
}
