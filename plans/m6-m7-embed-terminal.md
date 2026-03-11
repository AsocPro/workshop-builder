# M6+M7 — Backend Embeds Frontend + Terminal Integration

## Goal

**M6**: Single binary that serves the full SPA. No separate npm server needed for students.
**M7**: Live browser terminal connected to the container shell via ttyd iframe.

## Prerequisites

- M5 complete (`frontend/` builds with `npm run build` → `frontend/dist/`)
- M4 complete (Dagger pipeline can build workshop images)

## Working Directory

`/home/zach/workshop-builder`

## Acceptance Test

### M6
```bash
make build-workshop  # rebuilds backend (with embedded frontend) + workshop images
docker run --rm -p 8080:8080 localhost/hello-linux:step-1-intro
# Open http://localhost:8080
# → Full SPA loads, step list sidebar, markdown content, no external npm server
```

### M7
```bash
docker run --rm -p 8080:8080 localhost/hello-linux:step-1-intro
# Open http://localhost:8080
# → Terminal pane visible below step content
# → Type commands in browser terminal, shell executes them
```

---

## M6 — Embed Frontend

### Update `dagger/main.go` — `BuildBackend`

The build sequence is now: **build frontend first → copy dist/ into Go source tree → compile backend**.

```go
// BuildBackend builds frontend then compiles backend with embedded assets.
func (m *WorkshopBuilder) BuildBackend(
    ctx context.Context,
    // +defaultPath="/"
    src *dagger.Directory,
) *dagger.File {
    // Step 1: Build frontend
    frontendDist := dag.Container().
        From("node:22-alpine").
        WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
        WithDirectory("/app", src.Directory("frontend")).
        WithWorkdir("/app").
        WithExec([]string{"npm", "ci"}).
        WithExec([]string{"npm", "run", "build"}).
        Directory("/app/dist")

    // Step 2: Inject dist/ into Go source tree at backend/frontend/dist/
    srcWithDist := src.WithDirectory("backend/frontend/dist", frontendDist)

    // Step 3: Compile backend (CGO disabled, linux/amd64)
    return dag.Container().
        From("golang:1.24-alpine").
        WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
        WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
        WithDirectory("/src", srcWithDist).
        WithWorkdir("/src").
        WithEnvVariable("CGO_ENABLED", "0").
        WithEnvVariable("GOOS", "linux").
        WithEnvVariable("GOARCH", "amd64").
        WithExec([]string{
            "go", "build",
            "-ldflags", "-s -w",
            "-o", "/out/workshop-backend",
            "./backend/",
        }).
        File("/out/workshop-backend")
}
```

### Create `backend/frontend/dist/.gitkeep`

The `backend/frontend/dist/` directory must exist for the embed to compile. Add a `.gitkeep` and add to `.gitignore`:

```
# .gitignore addition
backend/frontend/dist/
!backend/frontend/dist/.gitkeep
```

### Create `backend/embed.go`

```go
package main

import (
    "embed"
    "io/fs"
    "net/http"
    "strings"
)

//go:embed frontend/dist
var frontendFS embed.FS

// frontendHandler returns an HTTP handler serving embedded frontend assets.
// For non-asset paths (no file extension or not found), serves index.html
// to support client-side routing (SPA fallback).
func frontendHandler() http.Handler {
    distFS, err := fs.Sub(frontendFS, "frontend/dist")
    if err != nil {
        panic(err)
    }
    fileServer := http.FileServer(http.FS(distFS))

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        path := strings.TrimPrefix(r.URL.Path, "/")

        // Try to open the file
        f, err := distFS.Open(path)
        if err != nil {
            // Fallback: serve index.html for SPA routing
            data, err := fs.ReadFile(distFS, "index.html")
            if err != nil {
                http.Error(w, "index.html not found in embedded assets", http.StatusInternalServerError)
                return
            }
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            w.Write(data)
            return
        }
        f.Close()

        fileServer.ServeHTTP(w, r)
    })
}
```

### Update `backend/handlers/static.go`

The `ServeStatic` handler is now wired to the embedded frontend. But to avoid a circular dependency between `handlers` package and `main`, the frontend handler is passed in during server construction.

Simplest approach: keep `ServeStatic` in `main.go` and wire directly in `server.go`:

**`backend/server.go`** — update to use embedded assets:

```go
func NewServer(meta *store.Metadata, st *store.State, managementURL string) http.Handler {
    r := chi.NewRouter()
    // ... (CORS, logger, recoverer as before)

    h := handlers.New(meta, st, managementURL)

    // API routes
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
    r.Get("/ws/terminal", h.TerminalWS)

    // Serve embedded frontend for all other routes
    r.Mount("/", frontendHandler())

    return r
}
```

Delete `backend/handlers/static.go` — no longer needed.

---

## M7 — Terminal Integration (ttyd)

### Overview

The backend spawns ttyd on port 7681 at startup. The browser embeds ttyd's web UI in an iframe. The backend reverse-proxies `/ttyd/` to ttyd's HTTP server.

No WebSocket proxy needed from the backend — ttyd serves its own WebSocket over HTTP at port 7681, and the backend just proxies the full HTTP path including WebSocket upgrades.

### Create `backend/process/ttyd.go`

```go
package process

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "sync"
    "time"
)

// TTYDManager spawns and supervises ttyd.
type TTYDManager struct {
    mu      sync.Mutex
    cmd     *exec.Cmd
    port    int
    running bool
}

// NewTTYDManager creates a manager for ttyd on the given port.
func NewTTYDManager(port int) *TTYDManager {
    return &TTYDManager{port: port}
}

// Start spawns ttyd and supervises it (restarts on exit).
func (m *TTYDManager) Start() {
    go m.supervise()
}

func (m *TTYDManager) supervise() {
    for {
        if err := m.spawn(); err != nil {
            log.Printf("ttyd exited: %v — restarting in 2s", err)
        }
        time.Sleep(2 * time.Second)
    }
}

func (m *TTYDManager) spawn() error {
    m.mu.Lock()
    cmd := exec.Command(
        "ttyd",
        "--port", fmt.Sprintf("%d", m.port),
        "--interface", "127.0.0.1",  // bind to localhost only; backend proxies externally
        "--base-path", "/ttyd",
        "--writable",                 // allow input from browser
        "--",
        "/bin/bash",
        "--login",
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    m.cmd = cmd
    m.running = true
    m.mu.Unlock()

    err := cmd.Run()

    m.mu.Lock()
    m.running = false
    m.mu.Unlock()

    return err
}
```

### Update `backend/main.go` — spawn ttyd at startup

```go
import "github.com/asocpro/workshop-builder/backend/process"

func main() {
    // ... (env vars, metadata loading as before)

    // Spawn ttyd (terminal)
    ttydMgr := process.NewTTYDManager(7681)
    ttydMgr.Start()
    // Give ttyd a moment to start before accepting connections
    // (the proxy will retry anyway)

    // ... (start HTTP server as before)
}
```

### Update `backend/server.go` — add ttyd proxy

```go
import "net/http/httputil"
import "net/url"

func NewServer(meta *store.Metadata, st *store.State, managementURL string) http.Handler {
    r := chi.NewRouter()
    // ... (middleware, CORS)

    h := handlers.New(meta, st, managementURL)

    // API routes (unchanged)
    // ...

    // ttyd reverse proxy
    ttydURL, _ := url.Parse("http://127.0.0.1:7681")
    ttydProxy := httputil.NewSingleHostReverseProxy(ttydURL)
    // Strip /ttyd prefix so ttyd sees requests at its --base-path /ttyd
    r.Mount("/ttyd", http.StripPrefix("/ttyd", ttydProxy))
    // Also handle WebSocket upgrade at /ttyd/ws (ttyd uses this path internally)

    // Frontend (must be last)
    r.Mount("/", frontendHandler())

    return r
}
```

Note: `httputil.NewSingleHostReverseProxy` handles WebSocket upgrades automatically if configured correctly. ttyd's web UI and WebSocket endpoint are both served at `http://127.0.0.1:7681`. With `--base-path /ttyd`, ttyd expects requests at `/ttyd/...` — the `http.StripPrefix("/ttyd", ...)` removes the prefix.

Actually, the simpler approach: ttyd serves at `/ttyd/...` when `--base-path /ttyd` is set. Forward requests WITH the `/ttyd` prefix to port 7681 (no stripping):

```go
ttydProxy := httputil.NewSingleHostReverseProxy(ttydURL)
r.Mount("/ttyd", ttydProxy)
// ttyd sees /ttyd/... requests, which matches its --base-path /ttyd
```

Test this with `curl http://localhost:8080/ttyd/` to see if ttyd's HTML loads.

### Add `Terminal.svelte` component

```svelte
<!-- frontend/src/components/Terminal.svelte -->
<script lang="ts">
  let { height = '400px' }: { height?: string } = $props()
</script>

<div class="terminal-wrapper border-t border-gray-800" style="height: {height}">
  <iframe
    src="/ttyd/"
    title="Terminal"
    class="w-full h-full border-0"
    allow="clipboard-read; clipboard-write"
  ></iframe>
</div>
```

### Update `frontend/src/components/StepContent.svelte` — add terminal below content

```svelte
<script lang="ts">
  // ... (existing imports)
  import Terminal from './Terminal.svelte'
</script>

<div class="flex flex-col h-full">
  <!-- Existing content area (scrollable) -->
  <div class="flex-1 overflow-y-auto max-w-4xl mx-auto w-full px-8 py-8">
    <!-- ... (all existing content, title, markdown, validate button) -->
  </div>

  <!-- Terminal pane (fixed at bottom) -->
  <Terminal height="350px" />
</div>
```

---

## Key Notes

### ttyd `--base-path` and iframe

ttyd must be started with `--base-path /ttyd` so that its self-referential asset URLs (`/ttyd/js/...`) work through the proxy. Without this, ttyd loads at `/` internally and all asset links break when proxied.

The iframe `src="/ttyd/"` (trailing slash is important for ttyd).

### WebSocket through proxy

`httputil.NewSingleHostReverseProxy` does NOT automatically handle WebSocket upgrades in all Go versions. If the terminal doesn't connect, add explicit WebSocket proxy support:

```go
ttydProxy.ModifyResponse = func(resp *http.Response) error {
    return nil
}

// Use a transport that supports WebSocket upgrades
ttydProxy.Transport = &http.Transport{
    // defaults work for local proxying
}
```

For WebSocket in chi, the alternative is to use `gorilla/websocket` to proxy manually, but the reverse proxy should work for same-host WebSocket with appropriate configuration.

If ttyd WS fails through the proxy, a simpler workaround: embed ttyd's web UI directly and connect the WebSocket to the backend's `/ws/terminal` endpoint, which the backend then proxies. But the iframe approach is simpler for MVP.

### Process supervision

ttyd is restarted on exit with a 2s delay. This handles cases where ttyd crashes or the user exits the shell. The `--login` flag sources `/etc/bash.bashrc` which sources `/etc/workshop-platform.bashrc` (the PROMPT_COMMAND hook).

### Security note

ttyd is bound to `127.0.0.1:7681` only — not exposed externally. The backend proxies it. This means only the backend can reach ttyd. In Docker mode with `--network host` this is as expected; with port mapping (`-p 8080:8080`), only port 8080 is exposed.

### Makefile update

No changes needed — `make build-workshop` already calls `BuildWorkshop` which calls `BuildBackend` (which now builds frontend first).
