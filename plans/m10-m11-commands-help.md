# M10+M11 — Command Logging + Static LLM Help

## Goal

**M10**: Shell commands typed in the terminal are logged to `/workshop/runtime/command-log.jsonl`. The backend watches this file and serves history via `GET /api/commands`.

**M11**: Help panel in the frontend serving `hints.md`, `explain.md`, `solve.md` from the image. No API key needed. Uses SSE stream so frontend uses same rendering path as future LLM streaming.

## Prerequisites

- M7 complete (terminal integration — commands are being typed)
- M4 complete (workshop images built with PROMPT_COMMAND hook in bashrc)
- M8 complete (goss validation — state complete)

## Working Directory

`/home/zach/workshop-builder`

## Acceptance Tests

### M10
```bash
docker run --rm -p 8080:8080 localhost/hello-linux:step-1-intro
# Open http://localhost:8080
# Type commands in the terminal (M7)
curl http://localhost:8080/api/commands
# → JSON array with recent commands, timestamps, exit codes

# Verify the log file is being written
docker exec <container-id> cat /workshop/runtime/command-log.jsonl
```

### M11
```bash
docker run --rm -p 8080:8080 localhost/hello-linux:step-1-intro
# Navigate to step-1-intro
# → "Hints" button visible (step-1 has hints.md)
# Click Hints → hints.md content streams in and renders

# Navigate to step-2-files
# → No Hints button (step-2 has no hints.md)

# Navigate to step-3-validate
# → Hints and Solve buttons visible
# Click Solve → solve.md content renders
```

---

## M10 — Command Log Watching

### Go dependency

Add to `go.mod`:
```
github.com/fsnotify/fsnotify v1.7.x
```

### `backend/store/commandlog.go`

```go
package store

import (
    "bufio"
    "encoding/json"
    "io"
    "log"
    "os"
    "sync"
    "time"

    "github.com/fsnotify/fsnotify"
)

const maxCommandBuffer = 1000

// Command is one entry from command-log.jsonl
type Command struct {
    TS   string `json:"ts"`
    Cmd  string `json:"cmd"`
    Exit int    `json:"exit"`
}

// CommandLog watches command-log.jsonl and maintains an in-memory ring buffer.
type CommandLog struct {
    mu       sync.RWMutex
    path     string
    commands []Command // ring buffer, newest at end
    offset   int64     // byte offset for incremental reads
}

// NewCommandLog creates a CommandLog for the given file path.
// The file may not exist yet — the watcher handles creation.
func NewCommandLog(path string) *CommandLog {
    return &CommandLog{path: path}
}

// Start begins watching the log file. Non-blocking (runs goroutine).
func (cl *CommandLog) Start() {
    // Do an initial read if the file already exists
    cl.readNew()

    go cl.watch()
}

func (cl *CommandLog) watch() {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Printf("commandlog: creating watcher: %v", err)
        return
    }
    defer watcher.Close()

    // Watch the parent directory so we catch file creation events
    dir := dirOf(cl.path)
    if err := watcher.Add(dir); err != nil {
        log.Printf("commandlog: watching dir %s: %v", dir, err)
        // Fall back to polling
        cl.pollLoop()
        return
    }

    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Name == cl.path &&
                (event.Op&(fsnotify.Write|fsnotify.Create)) != 0 {
                cl.readNew()
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            log.Printf("commandlog: watcher error: %v", err)
        }
    }
}

// pollLoop is a fallback for systems where fsnotify doesn't work.
func (cl *CommandLog) pollLoop() {
    for {
        cl.readNew()
        time.Sleep(2 * time.Second)
    }
}

// readNew reads any new lines appended since last read.
func (cl *CommandLog) readNew() {
    f, err := os.Open(cl.path)
    if err != nil {
        return // file doesn't exist yet — that's fine
    }
    defer f.Close()

    cl.mu.Lock()
    offset := cl.offset
    cl.mu.Unlock()

    if _, err := f.Seek(offset, io.SeekStart); err != nil {
        return
    }

    scanner := bufio.NewScanner(f)
    var newCmds []Command
    var newOffset int64 = offset

    for scanner.Scan() {
        line := scanner.Bytes()
        newOffset += int64(len(line)) + 1 // +1 for newline

        var cmd Command
        if err := json.Unmarshal(line, &cmd); err != nil {
            continue // skip malformed lines
        }
        newCmds = append(newCmds, cmd)
    }

    if len(newCmds) == 0 {
        return
    }

    cl.mu.Lock()
    cl.commands = append(cl.commands, newCmds...)
    // Trim to maxCommandBuffer
    if len(cl.commands) > maxCommandBuffer {
        cl.commands = cl.commands[len(cl.commands)-maxCommandBuffer:]
    }
    cl.offset = newOffset
    cl.mu.Unlock()
}

// GetRecent returns the most recent n commands (newest last).
func (cl *CommandLog) GetRecent(n int) []Command {
    cl.mu.RLock()
    defer cl.mu.RUnlock()

    if n <= 0 || n > len(cl.commands) {
        n = len(cl.commands)
    }
    result := make([]Command, n)
    copy(result, cl.commands[len(cl.commands)-n:])
    return result
}

// Len returns the number of buffered commands.
func (cl *CommandLog) Len() int {
    cl.mu.RLock()
    defer cl.mu.RUnlock()
    return len(cl.commands)
}

func dirOf(path string) string {
    for i := len(path) - 1; i >= 0; i-- {
        if path[i] == '/' {
            return path[:i]
        }
    }
    return "."
}
```

### Update `backend/main.go`

Start the command log watcher at startup:

```go
import "github.com/asocpro/workshop-builder/backend/store"

// In main():
commandLogPath := filepath.Join(workshopRoot, "runtime", "command-log.jsonl")
cmdLog := store.NewCommandLog(commandLogPath)
cmdLog.Start()

// Pass cmdLog to server
srv := NewServer(meta, st, managementURL, cmdLog)
```

### Update `backend/server.go`

```go
func NewServer(meta *store.Metadata, st *store.State, managementURL string, cmdLog *store.CommandLog) http.Handler {
    h := handlers.New(meta, st, managementURL, cmdLog)
    // ... (rest unchanged)
}
```

### Update `backend/handlers/handlers.go`

```go
type Handlers struct {
    Meta          *store.Metadata
    State         *store.State
    ManagementURL string
    CommandLog    *store.CommandLog
}

func New(meta *store.Metadata, st *store.State, managementURL string, cmdLog *store.CommandLog) *Handlers {
    return &Handlers{
        Meta:          meta,
        State:         st,
        ManagementURL: managementURL,
        CommandLog:    cmdLog,
    }
}
```

### `backend/handlers/commands.go` (full implementation)

```go
package handlers

import (
    "net/http"
    "strconv"
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
```

---

## M11 — Static LLM Help

### `backend/handlers/llm.go` (full implementation for static mode)

The response is SSE (Server-Sent Events) for consistency with future LLM streaming. Static mode sends the entire file as one content event, then a done event.

SSE format:
```
data: <chunk>\n\n
data: [DONE]\n\n
```

```go
package handlers

import (
    "fmt"
    "net/http"
    "os"

    "github.com/go-chi/chi/v5"
)

var validHelpModes = map[string]bool{
    "hints":   true,
    "explain": true,
    "solve":   true,
}

func (h *Handlers) LLMHelp(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    mode := r.URL.Query().Get("mode")

    if !validHelpModes[mode] {
        http.Error(w, "invalid mode; must be hints, explain, or solve", http.StatusBadRequest)
        return
    }

    meta, ok := h.Meta.StepsByID[id]
    if !ok {
        http.Error(w, "step not found", http.StatusNotFound)
        return
    }

    // Check if the requested mode is available
    switch mode {
    case "hints":
        if !meta.HasHints {
            http.Error(w, "hints not available for this step", http.StatusNotFound)
            return
        }
    case "explain":
        if !meta.HasExplain {
            http.Error(w, "explain not available for this step", http.StatusNotFound)
            return
        }
    case "solve":
        if !meta.HasSolve {
            http.Error(w, "solve not available for this step", http.StatusNotFound)
            return
        }
    }

    content, err := os.ReadFile(h.Meta.StepHelpPath(id, mode))
    if err != nil {
        http.Error(w, "help content not found", http.StatusNotFound)
        return
    }

    // Serve as SSE stream
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if proxied

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    // Send content as a single data event (JSON-escaped to handle newlines)
    // Use a simple approach: send as multiple lines each prefixed with "data: "
    // This way markdown with newlines works correctly in SSE
    fmt.Fprintf(w, "data: %s\n\n", sseEscape(string(content)))
    flusher.Flush()

    // Send done signal
    fmt.Fprintf(w, "data: [DONE]\n\n")
    flusher.Flush()
}

// sseEscape makes content safe for a single SSE data field.
// SSE data fields cannot contain raw newlines — each line needs "data: " prefix.
// Simpler: encode the entire content as a JSON string so it's one line.
func sseEscape(s string) string {
    // JSON-encode the string to escape newlines
    // Then strip the surrounding quotes
    encoded, _ := json.Marshal(s)
    return string(encoded)
}

func (h *Handlers) LLMHistory(w http.ResponseWriter, r *http.Request) {
    // For static mode, history is not tracked — return empty array
    writeJSON(w, http.StatusOK, []any{})
}
```

Note: `json.Marshal` needs to be imported. The SSE data field contains a JSON string (e.g. `"# Hints\n\nContent here"`) — the frontend decodes it.

### Frontend: `frontend/src/components/HelpPanel.svelte`

```svelte
<script lang="ts">
  import { renderMarkdown } from '../lib/markdown.js'
  import type { StepListItem } from '../lib/api.js'

  let {
    stepId,
    step,
  }: {
    stepId: string
    step: StepListItem | null
  } = $props()

  type Mode = 'hints' | 'explain' | 'solve'

  let activeMode = $state<Mode | null>(null)
  let content = $state('')
  let loading = $state(false)
  let error = $state<string | null>(null)

  const modeLabels: Record<Mode, string> = {
    hints: 'Hints',
    explain: 'Explain',
    solve: 'Solve',
  }

  function hasModeAvailable(mode: Mode): boolean {
    if (!step) return false
    switch (mode) {
      case 'hints': return step.hasHints
      case 'explain': return step.hasExplain
      case 'solve': return step.hasSolve
    }
  }

  const availableModes = $derived(
    (['hints', 'explain', 'solve'] as Mode[]).filter(m => hasModeAvailable(m))
  )

  async function loadHelp(mode: Mode) {
    activeMode = mode
    loading = true
    content = ''
    error = null

    try {
      const res = await fetch(`/api/steps/${stepId}/llm/help?mode=${mode}`)
      if (!res.ok) {
        throw new Error(`${res.status} ${res.statusText}`)
      }

      const reader = res.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let accumulated = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6).trim()
            if (data === '[DONE]') {
              // Stream complete
              break
            }
            // Decode JSON string
            try {
              const decoded = JSON.parse(data) as string
              accumulated += decoded
              content = accumulated
            } catch {
              // Non-JSON data — treat as literal
              accumulated += data
              content = accumulated
            }
          }
        }
      }
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  // Reset when step changes
  $effect(() => {
    stepId
    activeMode = null
    content = ''
    error = null
  })
</script>

{#if availableModes.length > 0}
  <div class="border-t border-gray-800 mt-6 pt-6">
    <h3 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">Help</h3>

    <!-- Mode buttons -->
    <div class="flex gap-2 mb-4">
      {#each availableModes as mode}
        <button
          class="px-3 py-1.5 text-xs font-medium rounded transition-colors
            {activeMode === mode
              ? 'bg-blue-600 text-white'
              : 'bg-gray-800 text-gray-400 hover:text-gray-200 hover:bg-gray-700'}"
          onclick={() => loadHelp(mode)}
        >
          {modeLabels[mode]}
        </button>
      {/each}
    </div>

    <!-- Content area -->
    {#if loading}
      <p class="text-gray-500 text-sm">Loading…</p>
    {:else if error}
      <p class="text-red-400 text-sm">{error}</p>
    {:else if content}
      <div class="prose prose-invert prose-sm max-w-none bg-gray-900/50 rounded p-4">
        {@html renderMarkdown(content)}
      </div>
    {/if}
  </div>
{/if}
```

### Add HelpPanel to `StepContent.svelte`

In `frontend/src/components/StepContent.svelte`, add the help panel below the validate button:

```svelte
<script lang="ts">
  // ... existing imports
  import HelpPanel from './HelpPanel.svelte'
</script>

<!-- Inside the content area, after validate button section: -->
<HelpPanel {stepId} {step} />
```

### Update `/api/steps` response to include help flags

The `ListSteps` handler in `backend/handlers/steps.go` already includes `HasHints`, `HasExplain`, `HasSolve` in `stepListItem`. No changes needed — these flow through to the frontend via `api.listSteps()`.

### Update `/api/state` — LLM capability flag (optional for M11)

The frontend `HelpPanel` shows buttons based on `step.hasHints` etc. (from `/api/steps`). No capability flag needed for static mode — if a step has `hasHints: true`, the Hints button shows.

If/when real LLM is added, `/api/state` would include an `llmEnabled: bool` flag so the frontend knows whether to show LLM-generated help vs static-only help. Skip this for M11.

---

## API Surface Summary

### `POST /api/steps/:id/llm/help?mode=hints|explain|solve`

Responses:
- `200` + SSE stream: success
- `400`: invalid mode
- `404`: step not found or mode not available for this step
- `501`: (removed — this is now fully implemented)

SSE format:
```
data: "<JSON-encoded markdown string>"\n\n
data: [DONE]\n\n
```

The frontend reads the SSE stream, JSON-parses each `data:` payload to get the markdown string, accumulates, and renders.

### `GET /api/commands?limit=N`

Response:
```json
{
  "commands": [
    {"ts": "2025-03-15T14:22:01Z", "cmd": "ls -la", "exit": 0},
    {"ts": "2025-03-15T14:22:05Z", "cmd": "cat /workspace/hello.sh", "exit": 0}
  ],
  "total": 42
}
```

---

## Bashrc PROMPT_COMMAND Hook

The hook is injected at `/etc/workshop-platform.bashrc` by the Dagger pipeline (M4). It writes to `/workshop/runtime/command-log.jsonl`. Format matches the `Command` struct:

```json
{"ts":"2025-03-15T14:22:01Z","cmd":"ls -la","exit":0}
```

The watcher reads new lines incrementally (tracks byte offset) and appends to the in-memory ring buffer.

If the file doesn't exist when the backend starts (normal — created on first command), the watcher handles this correctly:
1. `readNew()` returns early (file not found)
2. When the directory watcher sees a `CREATE` event for the file, it calls `readNew()` which succeeds

## Key Notes

### SSE JSON encoding of markdown

The content contains markdown with newlines, code blocks, etc. Sending it raw in SSE would break the protocol (SSE `data:` fields are newline-delimited). JSON encoding the string makes it a single line.

The frontend decodes with `JSON.parse(data)` to get back the original markdown string.

### Ring buffer

The `commands` slice acts as a ring buffer capped at 1000 entries. Trimming is done after each `readNew()` batch. This bounds memory regardless of session length.

### fsnotify on Linux

On Linux, fsnotify uses inotify. Watching the parent directory (`/workshop/runtime/`) catches both `CREATE` (when the file is first created) and `WRITE` events.

### Command log format — timestamps

The bashrc hook uses `date -u +%Y-%m-%dT%H:%M:%SZ` (no subsecond precision). This is fine for display — the format matches what `time.RFC3339` produces when truncated.
