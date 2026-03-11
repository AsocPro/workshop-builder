# M8 — Goss Validation

## Goal

Closes the learning loop: read → do → validate → marked complete. Implement the validate endpoint and update the frontend Validate button.

## Prerequisites

- M3 complete (backend API with stub validate handler)
- M5 complete (frontend with ValidateButton placeholder)
- M4 complete (workshop images built with goss baked in)
- M6 complete (embedded frontend, single binary)
- M7 complete (terminal integration)

## Working Directory

`/home/zach/workshop-builder`

## Acceptance Test

```bash
# Run step-2-files image (has goss.yaml)
docker run --rm -p 8080:8080 localhost/hello-linux:step-2-files
# Open http://localhost:8080
# Navigate to step-2-files in sidebar
# Click Validate → should PASS (hello.sh is baked into the step image)
# Completion indicator turns green ✓

# Test FAIL case: run step-1-intro (base state, no hello.sh)
docker run --rm -p 8080:8080 localhost/hello-linux:step-1-intro
# Navigate to step-2-files, click Validate
# → FAIL because /workspace/hello.sh doesn't exist in step-1 image
```

---

## Backend: `backend/handlers/validate.go` (full implementation)

```go
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
    Meta         any    `json:"meta"`
    Err          []string `json:"err"`
    Result       int    `json:"result"` // 0=pass, 1=fail, 2=skip
    Property     string `json:"property"`
    Skipped      bool   `json:"skipped"`
    Duration     int64  `json:"duration"`
    Expected     []string `json:"expected"`
    Found        []string `json:"found"`
}

type GossSummary struct {
    TestCount int   `json:"test-count"`
    Failed    int   `json:"failed"`
    Skipped   int   `json:"skipped"`
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

    err := cmd.Run()
    // goss exits non-zero on failures — that's expected, not an error
    if err != nil {
        exitErr, ok := err.(*exec.ExitError)
        if !ok {
            // Real execution error (goss not found, etc.)
            log.Printf("goss execution error: %v, stderr: %s", err, stderr.String())
            writeJSON(w, http.StatusInternalServerError, ValidateResponse{
                Error: fmt.Sprintf("failed to run goss: %v", err),
            })
            return
        }
        // exitErr != nil means goss ran but tests failed — normal path
        _ = exitErr
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

    // Convert to response
    resp := ValidateResponse{
        Passed: gossOut.Summary.Failed == 0,
    }
    for _, result := range gossOut.Results {
        summary := ""
        if len(result.Err) > 0 {
            summary = result.Err[0]
        } else if len(result.Found) > 0 && len(result.Expected) > 0 {
            summary = fmt.Sprintf("expected %v, found %v", result.Expected, result.Found)
        }
        check := CheckItem{
            Name:    describeGossResult(result),
            Passed:  result.Result == 0,
            Summary: summary,
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
        return fmt.Sprintf("%s: %s (%s)", r.ResourceType, r.ResourceID, r.Property)
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
```

---

## Frontend: `frontend/src/components/ValidateButton.svelte` (full implementation)

```svelte
<script lang="ts">
  import { api, type ValidateResult } from '../lib/api.js'

  let {
    stepId,
    onSuccess,
  }: {
    stepId: string
    onSuccess: () => void
  } = $props()

  let loading = $state(false)
  let result = $state<ValidateResult | null>(null)
  let errorMsg = $state<string | null>(null)

  async function validate() {
    loading = true
    result = null
    errorMsg = null

    try {
      result = await api.validate(stepId)
      if (result.passed) {
        onSuccess()
      }
    } catch (e) {
      errorMsg = String(e)
    } finally {
      loading = false
    }
  }
</script>

<div class="space-y-3">
  <button
    class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-medium text-sm
           disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
    disabled={loading}
    onclick={validate}
  >
    {loading ? 'Validating…' : 'Validate'}
  </button>

  {#if errorMsg}
    <p class="text-red-400 text-sm">{errorMsg}</p>
  {/if}

  {#if result}
    <div class="space-y-2">
      <!-- Overall result banner -->
      <div class="flex items-center gap-2 px-3 py-2 rounded text-sm font-medium
        {result.passed ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300'}">
        <span>{result.passed ? '✓' : '✗'}</span>
        <span>{result.passed ? 'All checks passed!' : 'Some checks failed.'}</span>
      </div>

      <!-- Per-check results -->
      {#if result.checks && result.checks.length > 0}
        <div class="space-y-1 text-sm">
          {#each result.checks as check}
            <div class="flex items-start gap-2 text-xs">
              <span class="{check.passed ? 'text-green-400' : 'text-red-400'} flex-shrink-0 mt-0.5">
                {check.passed ? '✓' : '✗'}
              </span>
              <div>
                <span class="text-gray-300">{check.name}</span>
                {#if !check.passed && check.summary}
                  <p class="text-red-400 mt-0.5">{check.summary}</p>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
```

---

## Update `api.ts` — wire validate properly

The validate endpoint returns `ValidateResponse`. Update types in `api.ts`:

```typescript
export interface ValidateResult {
  passed: boolean
  checks: CheckItem[]
  error?: string
}

export interface CheckItem {
  name: string
  passed: boolean
  summary?: string
}
```

The `api.validate()` function is already implemented in M5 — the types just need to match.

---

## Backend: State — ensure `MarkCompleted` unblocks next step

In linear nav mode, after `step-2-files` is completed, `step-3-validate` becomes accessible. The `Accessible()` function in `store/state.go` already handles this correctly based on the completion set — no changes needed.

However, verify that `ListSteps` re-reads the completion set dynamically (it does, since `State.IsCompleted()` and `State.Accessible()` both use the current in-memory state).

---

## Key Notes

### Goss exit codes

- `goss validate` exits 0 on full pass, 1 on any failure
- `exec.ExitError` is not a fatal error — it means goss ran but tests failed
- Actual execution errors (goss not found, goss.yaml unreadable) should return 500

### Goss JSON format

Run `goss validate --format json` inside a container to see the actual output shape. The schema shown above is the documented format, but verify with:
```bash
docker run --rm localhost/hello-linux:step-2-files \
  goss -g /workshop/steps/step-2-files/goss.yaml validate --format json --no-color
```

### State event file path

The state events file is at `/workshop/runtime/state-events.jsonl`. The `/workshop/runtime/` directory must exist before writing. The backend creates it at startup (see M3 `main.go` — add `os.MkdirAll`).

Add to `backend/main.go`:
```go
runtimeDir := filepath.Join(workshopRoot, "runtime")
if err := os.MkdirAll(runtimeDir, 0755); err != nil {
    log.Printf("warning: could not create runtime dir: %v", err)
}
```

### goss path in container

goss is at `/usr/local/bin/goss` (installed by Dagger pipeline in M4). The `exec.Command("goss", ...)` will find it if it's on PATH. In the container, PATH includes `/usr/local/bin`.

### Completed steps lock validation

Per the frontend spec:
> Completed steps lock validation. The Validate button is replaced with a static "Completed" indicator.

This is already handled in `StepContent.svelte` (M5):
```svelte
{#if step?.hasGoss && !step?.completed}
  <ValidateButton ... />
{:else if step?.completed}
  <p>✓ This step is complete.</p>
{/if}
```

After `onValidated()` is called, the parent re-fetches state and steps, which updates `step.completed` to `true`. The `ValidateButton` is replaced by the completed indicator.
