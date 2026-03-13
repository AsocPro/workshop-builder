package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/go-chi/chi/v5"
)

// GossOutput matches goss's JSON output format (--format json)
type GossOutput struct {
	Results []GossResult `json:"results"`
	Summary GossSummary  `json:"summary"`
}

type GossResult struct {
	ResourceID   string `json:"resource-id"`
	ResourceType string `json:"resource-type"`
	Title        string `json:"title"`
	Result       int    `json:"result"` // 0=pass, 1=fail, 2=skip
	Property     string `json:"property"`
	Skipped      bool   `json:"skipped"`
	SummaryLine  string `json:"summary-line"`
}

type GossSummary struct {
	TestCount     int   `json:"test-count"`
	Failed        int   `json:"failed-count"`
	Skipped       int   `json:"skipped-count"`
	TotalDuration int64 `json:"total-duration"`
}

// ValidateResponse is returned to the frontend
type ValidateResponse struct {
	Passed bool        `json:"passed"`
	Checks []CheckItem `json:"checks"`
	Error  string      `json:"error,omitempty"`
}

type CheckItem struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Summary string `json:"summary,omitempty"`
}

func (h *Handlers) Validate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	meta, ok := h.Meta.StepsByID[id]
	if !ok {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}

	if !meta.HasGoss {
		// No goss spec — auto-pass, mark complete
		h.State.MarkCompleted(id)
		writeJSON(w, http.StatusOK, ValidateResponse{
			Passed: true,
			Checks: []CheckItem{{Name: "No validation required", Passed: true}},
		})
		return
	}

	gossPath := h.Meta.StepGossPath(id)
	if _, err := os.Stat(gossPath); err != nil {
		http.Error(w, "goss.yaml not found", http.StatusInternalServerError)
		return
	}

	// Run goss validate
	cmd := exec.CommandContext(r.Context(),
		"goss",
		"-g", gossPath,
		"validate",
		"--format", "json",
		"--no-color",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitedClean := runErr == nil
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); !ok {
			// Real execution error (goss not found, etc.) — not a test failure
			log.Printf("goss execution error: %v, stderr: %s", runErr, stderr.String())
			writeJSON(w, http.StatusInternalServerError, ValidateResponse{
				Error: fmt.Sprintf("failed to run goss: %v", runErr),
			})
			return
		}
		// ExitError = goss ran but tests failed — continue to parse output
	}

	// Parse goss JSON output
	var gossOut GossOutput
	if err := json.Unmarshal(stdout.Bytes(), &gossOut); err != nil {
		log.Printf("goss output parse error: %v, stdout: %s", err, stdout.String())
		writeJSON(w, http.StatusInternalServerError, ValidateResponse{
			Error: fmt.Sprintf("failed to parse goss output: %v", err),
		})
		return
	}

	// Both must agree: exit 0 AND zero reported failures.
	// Guards against JSON parse producing zero-value structs or future goss changes.
	resp := ValidateResponse{
		Passed: exitedClean && gossOut.Summary.Failed == 0,
	}
	for _, result := range gossOut.Results {
		check := CheckItem{
			Name:   describeGossResult(result),
			Passed: result.Result == 0,
		}
		if !check.Passed {
			check.Summary = result.SummaryLine
		}
		resp.Checks = append(resp.Checks, check)
	}

	// Append state event
	h.appendStateEvent(id, resp.Passed, gossOut.Summary)

	// Mark complete on pass
	if resp.Passed {
		h.State.MarkCompleted(id)
	}

	writeJSON(w, http.StatusOK, resp)
}

func describeGossResult(r GossResult) string {
	if r.Title != "" {
		return r.Title
	}
	if r.Property != "" {
		return fmt.Sprintf("%s: %s: %s", r.ResourceType, r.ResourceID, r.Property)
	}
	return fmt.Sprintf("%s: %s", r.ResourceType, r.ResourceID)
}

func (h *Handlers) appendStateEvent(stepID string, passed bool, summary GossSummary) {
	event := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"event": "goss_result",
		"step":  stepID,
		"passed": passed,
		"checks": map[string]int{
			"total":  summary.TestCount,
			"passed": summary.TestCount - summary.Failed,
		},
	}
	data, _ := json.Marshal(event)

	eventsPath := h.Meta.WorkshopRoot + "/runtime/state-events.jsonl"
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Not fatal — state events are best-effort
		return
	}
	defer f.Close()
	f.Write(data)
	f.Write([]byte("\n"))
}
