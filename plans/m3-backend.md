# M3 — Backend Service (API only, no embedded UI)

## Goal

Deployable Go binary that reads `/workshop/` flat files and serves the full REST API. Testable with `curl` without any frontend.

## Prerequisites

- M1 complete (example workshop exists)
- M2 complete (`pkg/workshop/` compiles, `go.mod` exists)

## Working Directory

`/home/zach/workshop-builder`

## Constraint

No local Go toolchain. Write the source files directly. Testing runs via Dagger (`dagger call test`), which compiles inside a Go container.

## Acceptance Test

Via Dagger integration tests:
```bash
dagger call test
# Compiles backend, starts it against pre-baked /workshop/ testdata fixture,
# issues HTTP requests to verify each endpoint.

# Manual test (after M4 builds workshop images):
docker run --rm -p 8080:8080 localhost/hello-linux:step-1-intro &
curl http://localhost:8080/api/state
curl http://localhost:8080/api/steps
curl http://localhost:8080/api/steps/step-1-intro/content
```

---

## Directory Structure

```
backend/
  main.go
  server.go
  embed.go              (empty/placeholder until M6)
  store/
    metadata.go
    state.go
  handlers/
    state.go
    steps.go
    validate.go         (501 stub)
    commands.go         (501 stub)
    recordings.go       (501 stub)
    llm.go              (501 stubs)
    terminal.go         (501 stub)
  testdata/
    workshop/           (pre-baked /workshop/ dir for integration tests)
      workshop.json
      steps/
        step-1-intro/
          meta.json
          content.md
        step-2-files/
          meta.json
          content.md
          goss.yaml
        step-3-validate/
          meta.json
          content.md
          goss.yaml
  backend_test.go       (integration tests)
```

---

## Go Dependencies

Add to `go.mod`:
```
github.com/go-chi/chi/v5    v5.2.x    (check latest at pkg.go.dev)
```

---

## `backend/main.go`

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"

    "github.com/asocpro/workshop-builder/backend/store"
)

func main() {
    workshopRoot := os.Getenv("WORKSHOP_ROOT")
    if workshopRoot == "" {
        workshopRoot = "/workshop"
    }
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    managementURL := os.Getenv("WORKSHOP_MANAGEMENT_URL")

    // Load metadata from flat files
    meta, err := store.LoadMetadata(workshopRoot)
    if err != nil {
        log.Fatalf("loading workshop metadata: %v", err)
    }

    // Initialize in-memory state
    st := store.NewState(meta)

    // Create and start HTTP server
    srv := NewServer(meta, st, managementURL)

    addr := ":" + port
    fmt.Printf("Workshop backend listening on %s\n", addr)
    fmt.Printf("Workshop: %s (%s navigation)\n", meta.Workshop.Name, meta.Workshop.Navigation)
    if managementURL != "" {
        fmt.Printf("Management URL: %s\n", managementURL)
    }

    if err := http.ListenAndServe(addr, srv); err != nil {
        log.Fatalf("server error: %v", err)
    }
}
```

---

## `backend/server.go`

```go
package main

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/asocpro/workshop-builder/backend/handlers"
    "github.com/asocpro/workshop-builder/backend/store"
)

func NewServer(meta *store.Metadata, st *store.State, managementURL string) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // CORS for dev (frontend dev server proxies, but be permissive)
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusNoContent)
                return
            }
            next.ServeHTTP(w, r)
        })
    })

    h := handlers.New(meta, st, managementURL)

    // Student API
    r.Get("/api/state", h.GetState)
    r.Get("/api/steps", h.ListSteps)
    r.Get("/api/steps/{id}/content", h.GetStepContent)
    r.Post("/api/steps/{id}/navigate", h.Navigate)
    r.Post("/api/steps/{id}/validate", h.Validate)
    r.Get("/api/commands", h.ListCommands)
    r.Get("/api/recordings", h.ListRecordings)
    r.Get("/api/recordings/{filename}", h.GetRecording)
    r.Post("/api/steps/{id}/llm/help", h.LLMHelp)
    r.Get("/api/steps/{id}/llm/history", h.LLMHistory)

    // Terminal WebSocket (501 stub until M7)
    r.Get("/ws/terminal", h.TerminalWS)

    // Frontend static assets (populated in M6 via embed.go)
    r.Get("/*", h.ServeStatic)

    return r
}
```

---

## `backend/store/metadata.go`

Reads the flat files from `/workshop/` on startup.

```go
package store

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// WorkshopJSON mirrors /workshop/workshop.json
type WorkshopJSON struct {
    Name           string          `json:"name"`
    Image          string          `json:"image"`
    Navigation     string          `json:"navigation"`
    Infrastructure *InfraJSON      `json:"infrastructure,omitempty"`
    Steps          []StepRef       `json:"steps"`
}

type InfraJSON struct {
    Cluster         *ClusterJSON         `json:"cluster,omitempty"`
    ExtraContainers []ExtraContainerJSON `json:"extraContainers,omitempty"`
}

type ClusterJSON struct {
    Enabled  bool   `json:"enabled"`
    Provider string `json:"provider"`
}

type ExtraContainerJSON struct {
    Name  string            `json:"name"`
    Image string            `json:"image"`
    Ports []PortJSON        `json:"ports,omitempty"`
    Env   map[string]string `json:"env,omitempty"`
}

type PortJSON struct {
    Port        int    `json:"port"`
    Description string `json:"description,omitempty"`
}

type StepRef struct {
    ID       string   `json:"id"`
    Title    string   `json:"title"`
    Group    string   `json:"group,omitempty"`
    Requires []string `json:"requires,omitempty"`
    Position int      `json:"position"`
}

// MetaJSON mirrors /workshop/steps/<id>/meta.json
type MetaJSON struct {
    ID         string   `json:"id"`
    Title      string   `json:"title"`
    Group      string   `json:"group,omitempty"`
    Requires   []string `json:"requires,omitempty"`
    Position   int      `json:"position"`
    HasGoss    bool     `json:"hasGoss"`
    HasLlm     bool     `json:"hasLlm"`
    HasHints   bool     `json:"hasHints"`
    HasExplain bool     `json:"hasExplain"`
    HasSolve   bool     `json:"hasSolve"`
}

// Metadata is the in-memory representation of all workshop flat files.
type Metadata struct {
    WorkshopRoot string
    Workshop     WorkshopJSON
    Steps        []MetaJSON            // ordered by position
    StepsByID    map[string]*MetaJSON  // fast lookup
}

// LoadMetadata reads workshop.json and all steps/*/meta.json from workshopRoot.
func LoadMetadata(workshopRoot string) (*Metadata, error) {
    // Read workshop.json
    wjPath := filepath.Join(workshopRoot, "workshop.json")
    data, err := os.ReadFile(wjPath)
    if err != nil {
        return nil, fmt.Errorf("reading workshop.json: %w", err)
    }
    var wj WorkshopJSON
    if err := json.Unmarshal(data, &wj); err != nil {
        return nil, fmt.Errorf("parsing workshop.json: %w", err)
    }

    m := &Metadata{
        WorkshopRoot: workshopRoot,
        Workshop:     wj,
        StepsByID:    make(map[string]*MetaJSON),
    }

    // Read each step's meta.json in order
    for _, ref := range wj.Steps {
        metaPath := filepath.Join(workshopRoot, "steps", ref.ID, "meta.json")
        data, err := os.ReadFile(metaPath)
        if err != nil {
            return nil, fmt.Errorf("reading steps/%s/meta.json: %w", ref.ID, err)
        }
        var meta MetaJSON
        if err := json.Unmarshal(data, &meta); err != nil {
            return nil, fmt.Errorf("parsing steps/%s/meta.json: %w", ref.ID, err)
        }
        m.Steps = append(m.Steps, meta)
        m.StepsByID[ref.ID] = &m.Steps[len(m.Steps)-1]
    }

    return m, nil
}

// StepContentPath returns the path to content.md for a step.
func (m *Metadata) StepContentPath(stepID string) string {
    return filepath.Join(m.WorkshopRoot, "steps", stepID, "content.md")
}

// StepGossPath returns the path to goss.yaml for a step.
func (m *Metadata) StepGossPath(stepID string) string {
    return filepath.Join(m.WorkshopRoot, "steps", stepID, "goss.yaml")
}

// StepHelpPath returns the path to a static help file for a step.
// mode is one of: hints, explain, solve
func (m *Metadata) StepHelpPath(stepID, mode string) string {
    return filepath.Join(m.WorkshopRoot, "steps", stepID, mode+".md")
}
```

---

## `backend/store/state.go`

In-memory state: active step, completion set, navigation enforcement.

```go
package store

import "sync"

// State holds in-memory workshop progress. Always starts fresh — no replay.
type State struct {
    mu            sync.RWMutex
    meta          *Metadata
    activeStepID  string
    completed     map[string]bool  // set of completed step IDs
}

// NewState creates fresh state with the first accessible step active.
func NewState(meta *Metadata) *State {
    s := &State{
        meta:      meta,
        completed: make(map[string]bool),
    }
    if len(meta.Steps) > 0 {
        s.activeStepID = meta.Steps[0].ID
    }
    return s
}

// ActiveStepID returns the currently active step.
func (s *State) ActiveStepID() string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.activeStepID
}

// SetActiveStep sets the active step (called on navigate).
func (s *State) SetActiveStep(stepID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.activeStepID = stepID
}

// IsCompleted returns whether a step has been validated successfully.
func (s *State) IsCompleted(stepID string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.completed[stepID]
}

// MarkCompleted marks a step as completed and updates accessible steps.
func (s *State) MarkCompleted(stepID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.completed[stepID] = true
}

// CompletedSteps returns all completed step IDs.
func (s *State) CompletedSteps() []string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]string, 0, len(s.completed))
    for id := range s.completed {
        result = append(result, id)
    }
    return result
}

// Accessible returns whether a step can be navigated to under the current nav mode.
func (s *State) Accessible(stepID string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.accessible(stepID)
}

// accessible is the unlocked internal implementation.
func (s *State) accessible(stepID string) bool {
    nav := s.meta.Workshop.Navigation
    switch nav {
    case "free":
        // All steps always accessible
        _, ok := s.meta.StepsByID[stepID]
        return ok

    case "linear":
        // Only steps up to (and including) the first uncompleted step are accessible.
        // I.e.: step N is accessible if all steps before N are completed.
        for _, step := range s.meta.Steps {
            if step.ID == stepID {
                return true // reached target before finding uncompleted prior step
            }
            if !s.completed[step.ID] {
                return false // blocked by uncompleted prior step
            }
        }
        return false

    case "guided":
        meta, ok := s.meta.StepsByID[stepID]
        if !ok {
            return false
        }
        // Check requires
        for _, req := range meta.Requires {
            if !s.completed[req] {
                return false
            }
        }
        // Check group ordering (groups unlock when all steps in prior group complete)
        // Simple implementation: find all steps in groups that appear before this step's group
        // For now, treat guided like free if no group complexity needed
        // TODO: full group ordering enforcement post-MVP
        return true

    default:
        return false
    }
}
```

---

## `backend/handlers/` — Handler Infrastructure

### `backend/handlers/handlers.go`

```go
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
```

---

### `backend/handlers/state.go`

`GET /api/state`

Response:
```json
{
  "activeStep": "step-1-intro",
  "completedSteps": [],
  "navigationMode": "linear",
  "managementURL": "http://localhost:9090"
}
```

```go
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
```

---

### `backend/handlers/steps.go`

`GET /api/steps` — list with accessible/completed per step
`GET /api/steps/:id/content` — raw markdown
`POST /api/steps/:id/navigate` — set active step (no container swap)

```go
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
```

---

### Stub Handlers

#### `backend/handlers/validate.go`

```go
package handlers

import "net/http"

func (h *Handlers) Validate(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}
```

#### `backend/handlers/commands.go`

```go
package handlers

import "net/http"

func (h *Handlers) ListCommands(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}
```

#### `backend/handlers/recordings.go`

```go
package handlers

import "net/http"

func (h *Handlers) ListRecordings(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}

func (h *Handlers) GetRecording(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}
```

#### `backend/handlers/llm.go`

```go
package handlers

import "net/http"

func (h *Handlers) LLMHelp(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}

func (h *Handlers) LLMHistory(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}
```

#### `backend/handlers/terminal.go`

```go
package handlers

import "net/http"

func (h *Handlers) TerminalWS(w http.ResponseWriter, r *http.Request) {
    notImplemented(w, r)
}
```

#### `backend/handlers/static.go`

Placeholder until M6:

```go
package handlers

import "net/http"

// ServeStatic serves embedded frontend assets. Placeholder until M6.
func (h *Handlers) ServeStatic(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "frontend not yet embedded (see M6)", http.StatusNotFound)
}
```

---

## `backend/testdata/workshop/` — Pre-baked fixture

This mirrors what the Dagger pipeline would produce in `/workshop/`. Create it manually so tests work without running the full pipeline.

### `backend/testdata/workshop/workshop.json`

```json
{
  "name": "hello-linux",
  "image": "localhost/hello-linux",
  "navigation": "linear",
  "steps": [
    {"id": "step-1-intro", "title": "Welcome to Hello Linux", "position": 0},
    {"id": "step-2-files", "title": "Working with Files", "position": 1},
    {"id": "step-3-validate", "title": "Validation and Completion", "position": 2}
  ]
}
```

### `backend/testdata/workshop/steps/step-1-intro/meta.json`

```json
{
  "id": "step-1-intro",
  "title": "Welcome to Hello Linux",
  "position": 0,
  "hasGoss": false,
  "hasLlm": false,
  "hasHints": true,
  "hasExplain": false,
  "hasSolve": false
}
```

### `backend/testdata/workshop/steps/step-1-intro/content.md`

Copy from `examples/hello-linux/steps/step-1-intro/content.md`.

### `backend/testdata/workshop/steps/step-2-files/meta.json`

```json
{
  "id": "step-2-files",
  "title": "Working with Files",
  "position": 1,
  "hasGoss": true,
  "hasLlm": false,
  "hasHints": false,
  "hasExplain": false,
  "hasSolve": false
}
```

### `backend/testdata/workshop/steps/step-2-files/content.md`

Copy from `examples/hello-linux/steps/step-2-files/content.md`.

### `backend/testdata/workshop/steps/step-2-files/goss.yaml`

```yaml
file:
  /workspace/hello.sh:
    exists: true
    mode: "0755"
```

### `backend/testdata/workshop/steps/step-3-validate/meta.json`

```json
{
  "id": "step-3-validate",
  "title": "Validation and Completion",
  "position": 2,
  "hasGoss": true,
  "hasLlm": false,
  "hasHints": true,
  "hasExplain": false,
  "hasSolve": true
}
```

### `backend/testdata/workshop/steps/step-3-validate/content.md`

Copy from `examples/hello-linux/steps/step-3-validate/content.md`.

### `backend/testdata/workshop/steps/step-3-validate/goss.yaml`

```yaml
file:
  /workspace/done.txt:
    exists: true
    contains:
      - "validated"
```

---

## `backend/backend_test.go`

Integration tests that start the backend and hit it with HTTP requests.

```go
package main_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "runtime"
    "testing"

    "github.com/asocpro/workshop-builder/backend/store"
)

func testWorkshopRoot(t *testing.T) string {
    t.Helper()
    _, file, _, _ := runtime.Caller(0)
    return filepath.Join(filepath.Dir(file), "testdata", "workshop")
}

func newTestServer(t *testing.T) http.Handler {
    t.Helper()
    meta, err := store.LoadMetadata(testWorkshopRoot(t))
    if err != nil {
        t.Fatalf("LoadMetadata: %v", err)
    }
    st := store.NewState(meta)
    return NewServer(meta, st, "http://localhost:9090")
}

func TestGetState(t *testing.T) {
    srv := newTestServer(t)
    req := httptest.NewRequest("GET", "/api/state", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("status = %d", w.Code)
    }
    var resp map[string]any
    json.NewDecoder(w.Body).Decode(&resp)
    if resp["activeStep"] != "step-1-intro" {
        t.Errorf("activeStep = %v", resp["activeStep"])
    }
    if resp["navigationMode"] != "linear" {
        t.Errorf("navigationMode = %v", resp["navigationMode"])
    }
    if resp["managementURL"] != "http://localhost:9090" {
        t.Errorf("managementURL = %v", resp["managementURL"])
    }
}

func TestListSteps(t *testing.T) {
    srv := newTestServer(t)
    req := httptest.NewRequest("GET", "/api/steps", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("status = %d", w.Code)
    }
    var steps []map[string]any
    json.NewDecoder(w.Body).Decode(&steps)
    if len(steps) != 3 {
        t.Fatalf("len(steps) = %d, want 3", len(steps))
    }
    // First step accessible (linear nav, nothing completed yet)
    if steps[0]["accessible"] != true {
        t.Error("step-1 should be accessible")
    }
    // Second step not accessible (step-1 not completed)
    if steps[1]["accessible"] != false {
        t.Error("step-2 should not be accessible until step-1 completed")
    }
}

func TestGetStepContent(t *testing.T) {
    srv := newTestServer(t)
    req := httptest.NewRequest("GET", "/api/steps/step-1-intro/content", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
    }
    if w.Body.Len() == 0 {
        t.Error("content body is empty")
    }
}

func TestGetStepContent_NotFound(t *testing.T) {
    srv := newTestServer(t)
    req := httptest.NewRequest("GET", "/api/steps/nonexistent/content", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != http.StatusNotFound {
        t.Errorf("status = %d, want 404", w.Code)
    }
}

func TestNavigate_Blocked(t *testing.T) {
    srv := newTestServer(t)
    // Try to navigate to step-2 before step-1 is completed (linear mode)
    req := httptest.NewRequest("POST", "/api/steps/step-2-files/navigate", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != http.StatusForbidden {
        t.Errorf("status = %d, want 403", w.Code)
    }
}
```

---

## Key Decisions

- Backend reads **only** from `/workshop/` (flat files from image) — no YAML parsing at runtime
- State is always fresh on startup — no replay from `state-events.jsonl`
- `GET /api/steps/:id/navigate` → POST (navigate is a state mutation)
- The `notImplemented` helper returns 501 for unimplemented endpoints
- `ServeStatic` in M3 returns 404 with a clear message — it becomes real in M6
- `WORKSHOP_ROOT` defaults to `/workshop` (inside container)
- `PORT` defaults to `8080`
- `WORKSHOP_MANAGEMENT_URL` is optional — omitted from `/api/state` response if empty

## Notes on Package Structure

The `backend/` directory is a `package main`. The test file uses `package main_test` (external test package) to test via the exported HTTP handler. This avoids circular imports.

The `store` and `handlers` packages are sub-packages under `backend/`. In `go.mod`, they import as:
- `github.com/asocpro/workshop-builder/backend/store`
- `github.com/asocpro/workshop-builder/backend/handlers`
