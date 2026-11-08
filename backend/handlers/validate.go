package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// defaultGossTimeout bounds a single goss run. Override with WORKSHOP_GOSS_TIMEOUT
// (a Go duration string such as "90s") for workshops with slow checks.
const defaultGossTimeout = 30 * time.Second

func gossTimeout() time.Duration {
	v := os.Getenv("WORKSHOP_GOSS_TIMEOUT")
	if v == "" {
		return defaultGossTimeout
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("invalid WORKSHOP_GOSS_TIMEOUT %q, using %s", v, defaultGossTimeout)
		return defaultGossTimeout
	}
	return d
}

func (h *Handlers) Validate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	meta, ok := h.Meta.StepsByID[id]
	if !ok {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}

	// Same gate as navigate/activate: a locked step can't be validated or completed.
	if !h.State.Accessible(id) {
		writeJSON(w, http.StatusForbidden, ValidateResponse{
			Checks: []CheckItem{},
			Error:  "step not accessible: complete the previous steps first",
		})
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
		writeJSON(w, http.StatusInternalServerError, ValidateResponse{
			Checks: []CheckItem{},
			Error:  "goss.yaml not found",
		})
		return
	}

	// Run goss validate
	timeout := gossTimeout()
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx,
		"goss",
		"-g", gossPath,
		"validate",
		"--format", "json",
		"--no-color",
	)

	cmd.WaitDelay = 2 * time.Second // don't hang on grandchildren holding the pipes

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		msg := fmt.Sprintf("goss timed out after %s", timeout)
		log.Printf("%s, stderr: %s", msg, stderr.String())
		h.appendErrorEvent(id, msg)
		writeJSON(w, http.StatusGatewayTimeout, ValidateResponse{Checks: []CheckItem{}, Error: msg})
		return
	}
	exitedClean := runErr == nil
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); !ok {
			// Real execution error (goss not found, etc.) — not a test failure
			log.Printf("goss execution error: %v, stderr: %s", runErr, stderr.String())
			writeJSON(w, http.StatusInternalServerError, ValidateResponse{
				Checks: []CheckItem{},
				Error:  fmt.Sprintf("failed to run goss: %v", runErr),
			})
			return
		}
		// ExitError = goss ran but tests failed — continue to parse output
	}

	// Parse goss JSON output
	var gossOut GossOutput
	if err := json.Unmarshal(stdout.Bytes(), &gossOut); err != nil {
		// Raw output goes to the log only, not the response.
		log.Printf("goss output parse error: %v, stdout: %s, stderr: %s", err, stdout.String(), stderr.String())
		h.appendErrorEvent(id, fmt.Sprintf("failed to parse goss output: %v", err))
		writeJSON(w, http.StatusInternalServerError, ValidateResponse{
			Checks: []CheckItem{},
			Error:  fmt.Sprintf("failed to parse goss output: %v", err),
		})
		return
	}

	// Both must agree: exit 0 AND zero reported failures.
	// Guards against JSON parse producing zero-value structs or future goss changes.
	resp := ValidateResponse{
		Checks: []CheckItem{},
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
	h.writeStateEvent(map[string]any{
		"ts":     time.Now().UTC().Format(time.RFC3339),
		"event":  "goss_result",
		"step":   stepID,
		"passed": passed,
		"checks": map[string]int{
			"total":  summary.TestCount,
			"passed": summary.TestCount - summary.Failed,
		},
	})
}

// appendErrorEvent records a goss run that produced no usable result (timeout, bad output).
func (h *Handlers) appendErrorEvent(stepID, errMsg string) {
	h.writeStateEvent(map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"event": "goss_error",
		"step":  stepID,
		"error": errMsg,
	})
}

// writeStateEvent appends one JSON line to runtime/state-events.jsonl.
// State events are best-effort: failures are logged, never returned.
func (h *Handlers) writeStateEvent(event map[string]any) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("state event marshal: %v", err)
		return
	}
	dir := filepath.Join(h.Meta.WorkshopRoot, "runtime")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("state event: create %s: %v", dir, err)
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "state-events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("state event: open: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		log.Printf("state event: write: %v", err)
	}
}
