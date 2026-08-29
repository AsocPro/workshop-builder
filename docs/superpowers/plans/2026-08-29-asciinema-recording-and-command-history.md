# Asciinema Recording + Command-History UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Capture each terminal session as a per-session asciicast file the student can replay, and surface the already-captured command log in the student UI — across container, standalone, and devcontainer modes.

**Architecture:** Vendor the `asciinema` static binary through the Dagger pipeline (same mechanism as ttyd/goss); wrap ttyd's shell in `asciinema rec` with graceful degradation; implement the stubbed `/api/recordings` handlers backed by a `store` helper; add a tabbed right-hand panel (Terminal | Commands | Recordings) whose Recordings tab embeds asciinema-player.

**Tech Stack:** Go 1.24 (chi, fsnotify), Dagger v0.19.11, Svelte 5 (runes) + Vite, asciinema 3.x CLI, asciinema-player ≥3.10.

## Global Constraints

- Module path: `github.com/asocpro/workshop-builder`; Go 1.24.
- **All builds/tests run through Dagger.** Tests: `make test` (→ `dagger call test`, runs `go test ./pkg/workshop/... ./backend/... ./cmd/...`). No local Go toolchain assumption.
- asciinema CLI pinned to **v3.2.1**; assets: `asciinema-x86_64-unknown-linux-musl` (amd64), `asciinema-aarch64-unknown-linux-gnu` (arm64).
- Record format is asciicast **v3**; frontend `asciinema-player` floor is **^3.10.0**.
- Recording files are **per-session** `session-<ts>.cast` where `<ts>` is UTC `+%Y%m%dT%H%M%SZ`; served filenames must match `^session-[0-9TZ]+\.cast$`.
- Frontend deps use **`npm ci`** — any `package.json` change requires regenerating `frontend/package-lock.json` via `dagger call frontend-lockfile --frontend ./frontend --output frontend/package-lock.json`.
- Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`) — match existing components.
- Commit after each task.

---

### Task 1: Vendor the asciinema binary (Dagger + release + installer)

**Files:**
- Modify: `dagger/main.go` (download helpers ~618-636; `buildBaseImage` 156-177; `buildUbuntuBase` 179-199; `buildDebianBase` 201-221; `buildRockyBase` 223-242; `BuildRelease` 647-664)
- Modify: `scripts/install-standalone.sh:38`

**Interfaces:**
- Produces: `/usr/local/bin/asciinema` present in all three base images; `asciinema-linux-<arch>` release asset; installer fetches `asciinema`.

- [ ] **Step 1: Add the download helpers**

In `dagger/main.go`, after `downloadTtydArch` (ends line 636), add:

```go
func (m *WorkshopBuilder) downloadAsciinema(_ context.Context) *dagger.File {
	return m.downloadAsciinemaArch("amd64")
}

func (m *WorkshopBuilder) downloadAsciinemaArch(arch string) *dagger.File {
	// asciinema 3.x ships prebuilt static binaries. amd64 has a musl build;
	// arm64 ships the gnu build. Vendored like ttyd/goss so recording reaches
	// container, standalone, and devcontainer modes from one pinned version.
	asset := map[string]string{
		"amd64": "asciinema-x86_64-unknown-linux-musl",
		"arm64": "asciinema-aarch64-unknown-linux-gnu",
	}[arch]
	return dag.HTTP("https://github.com/asciinema/asciinema/releases/download/v3.2.1/" + asset)
}
```

- [ ] **Step 2: Thread asciinema through the base-image builder**

Replace `buildBaseImage` (lines 156-177) so it downloads asciinema and passes it to each variant:

```go
func (m *WorkshopBuilder) buildBaseImage(
	ctx context.Context,
	src *dagger.Directory,
	variant string,
	workshopBin *dagger.File,
) (*dagger.Container, error) {
	tini := m.downloadTini(ctx)
	goss := m.downloadGoss(ctx)
	ttyd := m.downloadTtyd(ctx)
	asciinema := m.downloadAsciinema(ctx)
	bashrc := src.File("backend/instrumentation/workshop-platform.bashrc")

	switch variant {
	case "ubuntu":
		return m.buildUbuntuBase(bashrc, workshopBin, tini, goss, ttyd, asciinema), nil
	case "rocky":
		return m.buildRockyBase(bashrc, workshopBin, tini, goss, ttyd, asciinema), nil
	case "debian":
		return m.buildDebianBase(bashrc, workshopBin, tini, goss, ttyd, asciinema), nil
	default:
		return nil, fmt.Errorf("unknown base image variant: %s", variant)
	}
}
```

- [ ] **Step 3: Install asciinema in each of the three base builders**

For `buildUbuntuBase`, `buildDebianBase`, and `buildRockyBase`: change the signature parameter line `workshopBin, tini, goss, ttyd *dagger.File,` to `workshopBin, tini, goss, ttyd, asciinema *dagger.File,`, and add one `WithFile` immediately after the existing ttyd line in each:

```go
		WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/asciinema", asciinema, dagger.ContainerWithFileOpts{Permissions: 0755}).
```

- [ ] **Step 4: Add asciinema to the release assets**

In `BuildRelease`, inside the `for _, arch` loop (lines 655-658), add one line:

```go
		out = out.
			WithFile("workshop-linux-"+arch, m.BuildWorkshopBinary(ctx, src, "linux", arch)).
			WithFile("ttyd-linux-"+arch, m.downloadTtydArch(arch)).
			WithFile("goss-linux-"+arch, m.downloadGossArch(arch)).
			WithFile("asciinema-linux-"+arch, m.downloadAsciinemaArch(arch))
```

- [ ] **Step 5: Fetch asciinema in the standalone installer**

In `scripts/install-standalone.sh`, change line 38:

```sh
BINARIES="workshop ttyd goss asciinema"
```

- [ ] **Step 6: Verify the Dagger module compiles**

Run: `dagger functions`
Expected: lists functions with no compile error (confirms `dagger/main.go` is valid).

- [ ] **Step 7: (Heavier, optional) verify asciinema lands in a base image**

Run: `dagger call build-base-image --src . --variant ubuntu terminal --cmd "asciinema --version"` — or if that entrypoint is not available, `make base-images` then inspect one image for `/usr/local/bin/asciinema`.
Expected: asciinema binary present / prints a 3.x version. Skip if base-image builds are too slow in this environment; Step 6 is the required gate.

- [ ] **Step 8: Commit**

```bash
git add dagger/main.go scripts/install-standalone.sh
git commit -m "Vendor asciinema static binary into base images, release, and installer"
```

---

### Task 2: Wrap the ttyd shell in asciinema (capture)

**Files:**
- Modify: `backend/process/ttyd.go`
- Modify: `backend/container.go:52`
- Modify: `backend/standalone.go:88-91`
- Test: `backend/process/ttyd_test.go` (create)

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `NewTTYDManager(port int, runtimeDir string) *TTYDManager`
  - `NewTTYDManagerWithShell(port int, runtimeDir string, shell []string, extraEnv map[string]string) *TTYDManager`
  - `(*TTYDManager).args() []string` and `(*TTYDManager).env() []string` (used by `spawn`, exercised by tests).

- [ ] **Step 1: Write the failing test**

Create `backend/process/ttyd_test.go`:

```go
package process

import (
	"strings"
	"testing"
)

func TestArgsWrapShellInRecorder(t *testing.T) {
	m := NewTTYDManager(7681, "/workshop/runtime")
	args := m.args()

	// ttyd args must end with: -- sh -c <recorderScript>
	if len(args) < 3 {
		t.Fatalf("args too short: %v", args)
	}
	if args[len(args)-3] != "sh" || args[len(args)-2] != "-c" {
		t.Fatalf("expected shell wrapper `sh -c`, got tail %v", args[len(args)-3:])
	}
	script := args[len(args)-1]
	if !strings.Contains(script, "asciinema rec") {
		t.Errorf("recorder script missing asciinema rec: %q", script)
	}
	if !strings.Contains(script, "session-$ts.cast") {
		t.Errorf("recorder script missing per-session filename: %q", script)
	}
	if !strings.Contains(script, `exec $WORKSHOP_SHELL_CMD`) {
		t.Errorf("recorder script missing degradation fallback: %q", script)
	}
}

func TestEnvCarriesRuntimeDirAndShellCmd(t *testing.T) {
	m := NewTTYDManager(7681, "/workshop/runtime")
	env := m.env()
	want := map[string]string{
		"WORKSHOP_RUNTIME_DIR": "/workshop/runtime",
		"WORKSHOP_SHELL_CMD":   "/bin/bash --login",
	}
	for k, v := range want {
		if !hasEnv(env, k+"="+v) {
			t.Errorf("env missing %s=%s\ngot: %v", k, v, env)
		}
	}
}

func TestWithShellSetsShellCmd(t *testing.T) {
	m := NewTTYDManagerWithShell(7681, "/root/.workshop/x/runtime",
		[]string{"/bin/bash", "--rcfile", "/root/.workshop/x/workshop-rcfile", "-i"},
		map[string]string{"WORKSHOP_ROOT": "/root/.workshop/x"})
	if !hasEnv(m.env(), "WORKSHOP_SHELL_CMD=/bin/bash --rcfile /root/.workshop/x/workshop-rcfile -i") {
		t.Errorf("shell cmd not set from explicit shell; env: %v", m.env())
	}
	if !hasEnv(m.env(), "WORKSHOP_ROOT=/root/.workshop/x") {
		t.Errorf("extra env not preserved; env: %v", m.env())
	}
}

func hasEnv(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — `NewTTYDManager` now takes a second argument; `args`/`env` methods don't exist.

- [ ] **Step 3: Rewrite `backend/process/ttyd.go`**

Replace the whole file with:

```go
package process

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// recorderScript wraps the shell in asciinema. It runs per ttyd client
// connection (ttyd re-execs its `--` command for each client), so the
// timestamp is generated at connect time and each session gets its own file.
// If asciinema is not installed (possible in standalone mode), it degrades to
// running the shell directly — the terminal still works, only recording is
// skipped.
const recorderScript = `if command -v asciinema >/dev/null 2>&1; then
  ts=$(date -u +%Y%m%dT%H%M%SZ)
  exec asciinema rec --stdin "$WORKSHOP_RUNTIME_DIR/session-$ts.cast" -c "$WORKSHOP_SHELL_CMD"
else
  exec $WORKSHOP_SHELL_CMD
fi`

// TTYDManager spawns and supervises ttyd, recording each session via asciinema.
type TTYDManager struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	port       int
	runtimeDir string
	shellCmd   string
	env        []string
	running    bool
}

// NewTTYDManager creates a manager for ttyd on the given port with the default
// shell (bash login shell — container modes, where instrumentation is sourced
// globally via /etc/bash.bashrc). runtimeDir is where session-<ts>.cast files
// are written.
func NewTTYDManager(port int, runtimeDir string) *TTYDManager {
	return NewTTYDManagerWithShell(port, runtimeDir, nil, nil)
}

// NewTTYDManagerWithShell creates a manager with an explicit shell argv and
// extra environment variables. Standalone mode uses this to scope
// instrumentation to workshop sessions only:
//
//	bash --rcfile <root>/workshop-rcfile -i    (with WORKSHOP_ROOT set)
func NewTTYDManagerWithShell(port int, runtimeDir string, shell []string, extraEnv map[string]string) *TTYDManager {
	if len(shell) == 0 {
		shell = []string{"/bin/bash", "--login"}
	}
	env := os.Environ()
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	return &TTYDManager{
		port:       port,
		runtimeDir: runtimeDir,
		shellCmd:   strings.Join(shell, " "),
		env:        env,
	}
}

// args builds the ttyd argv. ttyd runs `sh -c <recorderScript>` per client;
// the recorder wraps the real shell in asciinema.
func (m *TTYDManager) args() []string {
	return []string{
		"--port", fmt.Sprintf("%d", m.port),
		"--interface", "127.0.0.1", // bind to localhost only; backend proxies externally
		"--base-path", "/ttyd",
		"--writable", // allow input from browser
		"--",
		"sh", "-c", recorderScript,
	}
}

// env is the child environment: the base environment plus the recorder inputs.
func (m *TTYDManager) env() []string {
	return append(m.env,
		"WORKSHOP_RUNTIME_DIR="+m.runtimeDir,
		"WORKSHOP_SHELL_CMD="+m.shellCmd,
	)
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
	cmd := exec.Command("ttyd", m.args()...)
	cmd.Env = m.env()
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

- [ ] **Step 4: Update the two call sites**

In `backend/container.go` line 52:

```go
	ttydMgr := process.NewTTYDManager(7681, runtimeDir)
```

In `backend/standalone.go` lines 88-91:

```go
	ttydMgr := process.NewTTYDManagerWithShell(7681, runtimeDir,
		[]string{"/bin/bash", "--rcfile", rcfile, "-i"},
		map[string]string{"WORKSHOP_ROOT": root},
	)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `make test`
Expected: PASS (the new process tests and the existing suite; `container.go`/`standalone.go` compile against the new signatures).

- [ ] **Step 6: Commit**

```bash
git add backend/process/ttyd.go backend/process/ttyd_test.go backend/container.go backend/standalone.go
git commit -m "Record terminal sessions via asciinema-wrapped ttyd (graceful fallback)"
```

---

### Task 3: Recordings store helper

**Files:**
- Create: `backend/store/recordings.go`
- Test: `backend/store/recordings_test.go`

**Interfaces:**
- Produces:
  - `type RecordingInfo struct { Filename string; StartTime string; Size int64 }` (JSON tags `filename`, `startTime`, `size`)
  - `func ListRecordings(runtimeDir string) ([]RecordingInfo, error)` — newest-first
  - `func ValidRecordingName(name string) bool`

- [ ] **Step 1: Write the failing test**

Create `backend/store/recordings_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListRecordingsSortsAndFilters(t *testing.T) {
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	must("session-20250101T000000Z.cast", "a")
	must("session-20250102T000000Z.cast", "bb")
	must("command-log.jsonl", "not a recording") // must be ignored

	recs, err := ListRecordings(dir)
	if err != nil {
		t.Fatalf("ListRecordings: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 recordings, got %d: %+v", len(recs), recs)
	}
	if recs[0].Filename != "session-20250102T000000Z.cast" {
		t.Errorf("want newest first, got %s", recs[0].Filename)
	}
	if recs[0].Size != 2 {
		t.Errorf("want size 2, got %d", recs[0].Size)
	}
	if recs[0].StartTime != "2025-01-02T00:00:00Z" {
		t.Errorf("want RFC3339 startTime, got %q", recs[0].StartTime)
	}
}

func TestListRecordingsMissingDir(t *testing.T) {
	recs, err := ListRecordings(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("want empty, got %+v", recs)
	}
}

func TestValidRecordingName(t *testing.T) {
	ok := []string{"session-20250101T000000Z.cast"}
	bad := []string{"../etc/passwd", "session-.cast", "other.cast", "session-1/2.cast", "session-1.cast.bak"}
	for _, n := range ok {
		if !ValidRecordingName(n) {
			t.Errorf("expected valid: %s", n)
		}
	}
	for _, n := range bad {
		if ValidRecordingName(n) {
			t.Errorf("expected invalid: %s", n)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — `ListRecordings`/`ValidRecordingName`/`RecordingInfo` undefined.

- [ ] **Step 3: Implement `backend/store/recordings.go`**

```go
package store

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// RecordingInfo describes one asciicast session file in the runtime dir.
type RecordingInfo struct {
	Filename  string `json:"filename"`
	StartTime string `json:"startTime"` // RFC3339, parsed from the filename timestamp
	Size      int64  `json:"size"`
}

var recordingNameRE = regexp.MustCompile(`^session-[0-9TZ]+\.cast$`)

// ValidRecordingName reports whether name is a well-formed session recording
// filename. Used as a path-traversal / arbitrary-read guard before serving.
func ValidRecordingName(name string) bool {
	return recordingNameRE.MatchString(name)
}

// ListRecordings returns the session recordings in runtimeDir, newest first.
// A missing directory yields an empty list, not an error.
func ListRecordings(runtimeDir string) ([]RecordingInfo, error) {
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var recs []RecordingInfo
	for _, e := range entries {
		if e.IsDir() || !ValidRecordingName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		recs = append(recs, RecordingInfo{
			Filename:  e.Name(),
			StartTime: parseSessionStart(e.Name()),
			Size:      info.Size(),
		})
	}

	// Filenames are fixed-width zero-padded timestamps, so lexical descending
	// order is chronological newest-first.
	sort.Slice(recs, func(i, j int) bool { return recs[i].Filename > recs[j].Filename })
	return recs, nil
}

// parseSessionStart turns "session-20250102T000000Z.cast" into RFC3339. If the
// timestamp can't be parsed, it returns the raw token so callers still get
// something stable.
func parseSessionStart(name string) string {
	ts := name[len("session-") : len(name)-len(".cast")]
	if t, err := time.Parse("20060102T150405Z", ts); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return ts
}

// runtimeRecordingPath joins runtimeDir with an already-validated name.
func runtimeRecordingPath(runtimeDir, name string) string {
	return filepath.Join(runtimeDir, name)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `make test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/store/recordings.go backend/store/recordings_test.go
git commit -m "Add recordings store helper (list + filename guard)"
```

---

### Task 4: Recordings HTTP handlers + RuntimeDir wiring

**Files:**
- Modify: `backend/handlers/handlers.go` (add `RuntimeDir`; extend `New`)
- Modify: `backend/handlers/recordings.go` (implement both handlers)
- Modify: `backend/server.go:16,35` (add `runtimeDir` param, pass to `handlers.New`)
- Modify: `backend/container.go:56`, `backend/standalone.go:94`
- Modify: `backend/backend_test.go:26-34` (test helper passes a runtime dir)
- Test: `backend/recordings_handler_test.go` (create)

**Interfaces:**
- Consumes: `store.ListRecordings`, `store.ValidRecordingName` (Task 3).
- Produces: `NewServer(meta, st, managementURL, cmdLog, mode, runtimeDir)`; working `GET /api/recordings` and `GET /api/recordings/{filename}`.

- [ ] **Step 1: Write the failing test**

Create `backend/recordings_handler_test.go`:

```go
package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/asocpro/workshop-builder/backend/store"
)

func serverWithRuntime(t *testing.T, runtimeDir string) http.Handler {
	t.Helper()
	root := testWorkshopRoot(t)
	meta, err := store.LoadMetadata(root)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	st := store.NewState(meta)
	cmdLog := store.NewCommandLog(filepath.Join(runtimeDir, "command-log.jsonl"))
	return NewServer(meta, st, "", cmdLog, "container", runtimeDir)
}

func TestListRecordingsEndpoint(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "session-20250101T000000Z.cast"), []byte(`{"version":3}`), 0644)
	srv := serverWithRuntime(t, dir)

	req := httptest.NewRequest("GET", "/api/recordings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var recs []store.RecordingInfo
	json.NewDecoder(w.Body).Decode(&recs)
	if len(recs) != 1 || recs[0].Filename != "session-20250101T000000Z.cast" {
		t.Fatalf("unexpected recordings: %+v", recs)
	}
}

func TestGetRecordingServesWithRange(t *testing.T) {
	dir := t.TempDir()
	body := `{"version":3}` + "\n" + `[0.1, "o", "hi"]`
	os.WriteFile(filepath.Join(dir, "session-20250101T000000Z.cast"), []byte(body), 0644)
	srv := serverWithRuntime(t, dir)

	// Full fetch
	req := httptest.NewRequest("GET", "/api/recordings/session-20250101T000000Z.cast", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("full fetch status = %d", w.Code)
	}
	if w.Body.String() != body {
		t.Errorf("body mismatch")
	}

	// Range request → 206
	req2 := httptest.NewRequest("GET", "/api/recordings/session-20250101T000000Z.cast", nil)
	req2.Header.Set("Range", "bytes=0-4")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusPartialContent {
		t.Errorf("range status = %d, want 206", w2.Code)
	}
}

func TestGetRecordingRejectsBadName(t *testing.T) {
	srv := serverWithRuntime(t, t.TempDir())
	req := httptest.NewRequest("GET", "/api/recordings/evil.txt", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — `NewServer` signature mismatch; handlers still return 501.

- [ ] **Step 3: Add `RuntimeDir` to Handlers**

In `backend/handlers/handlers.go`, add the field and extend `New`:

```go
type Handlers struct {
	Meta          *store.Metadata
	State         *store.State
	ManagementURL string
	CommandLog    *store.CommandLog
	Mode          string // WORKSHOP_MODE: container (default), devcontainer, standalone
	RuntimeDir    string // dir holding runtime files (command log, session-*.cast)
}

func New(meta *store.Metadata, st *store.State, managementURL string, cmdLog *store.CommandLog, mode, runtimeDir string) *Handlers {
	return &Handlers{
		Meta:          meta,
		State:         st,
		ManagementURL: managementURL,
		CommandLog:    cmdLog,
		Mode:          mode,
		RuntimeDir:    runtimeDir,
	}
}
```

- [ ] **Step 4: Implement the handlers**

Replace `backend/handlers/recordings.go`:

```go
package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/asocpro/workshop-builder/backend/store"
)

func (h *Handlers) ListRecordings(w http.ResponseWriter, r *http.Request) {
	recs, err := store.ListRecordings(h.RuntimeDir)
	if err != nil {
		http.Error(w, "listing recordings", http.StatusInternalServerError)
		return
	}
	if recs == nil {
		recs = []store.RecordingInfo{}
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *Handlers) GetRecording(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if !store.ValidRecordingName(name) {
		http.Error(w, "invalid recording name", http.StatusBadRequest)
		return
	}
	f, err := os.Open(filepath.Join(h.RuntimeDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-asciicast")
	http.ServeContent(w, r, name, fi.ModTime(), f)
}
```

- [ ] **Step 5: Thread `runtimeDir` through `NewServer`**

In `backend/server.go`, change the signature (line 16) and the `handlers.New` call (line 35):

```go
func NewServer(meta *store.Metadata, st *store.State, managementURL string, cmdLog *store.CommandLog, mode, runtimeDir string) http.Handler {
```
```go
	h := handlers.New(meta, st, managementURL, cmdLog, mode, runtimeDir)
```

- [ ] **Step 6: Update production call sites**

`backend/container.go` line 56:

```go
	srv := NewServer(meta, st, managementURL, cmdLog, mode, runtimeDir)
```

`backend/standalone.go` line 94:

```go
	var handler http.Handler = NewServer(meta, st, "", cmdLog, "standalone", runtimeDir)
```

- [ ] **Step 7: Update the existing test helper**

`backend/backend_test.go` lines 26-34 — pass the runtime dir:

```go
func newTestServerAt(t *testing.T, root, mode string) http.Handler {
	t.Helper()
	meta, err := store.LoadMetadata(root)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	st := store.NewState(meta)
	cmdLog := store.NewCommandLog(filepath.Join(root, "runtime", "command-log.jsonl"))
	return NewServer(meta, st, "http://localhost:9090", cmdLog, mode, filepath.Join(root, "runtime"))
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `make test`
Expected: PASS (new recordings handler tests + existing suite).

- [ ] **Step 9: Commit**

```bash
git add backend/handlers/handlers.go backend/handlers/recordings.go backend/server.go backend/container.go backend/standalone.go backend/backend_test.go backend/recordings_handler_test.go
git commit -m "Implement recordings API (list + range-serve with path guard)"
```

---

### Task 5: Frontend API — fix getCommands, add recordings

**Files:**
- Modify: `frontend/src/lib/api.ts`

**Interfaces:**
- Produces: `api.getCommands()` returns `Command[]` (unwrapped); `api.listRecordings()`; `api.recordingURL(filename)`; `RecordingInfo` type.

- [ ] **Step 1: Fix `getCommands` and add recordings helpers**

The backend returns `{commands, total}`, but `getCommands` is typed as `Command[]` — a latent bug. Fix it and add the recordings API. Replace the `getCommands` method (lines 77-79) and add after it, before the closing `}`:

```ts
  getCommands(limit = 50): Promise<Command[]> {
    return fetchJSON<{ commands: Command[]; total: number }>(
      `/api/commands?limit=${limit}`,
    ).then(r => r.commands)
  },

  listRecordings(): Promise<RecordingInfo[]> {
    return fetchJSON('/api/recordings')
  },

  recordingURL(filename: string): string {
    return `/api/recordings/${encodeURIComponent(filename)}`
  },
```

Add the type near the other interfaces (e.g. after `Command`, lines 35-39):

```ts
export interface RecordingInfo {
  filename: string
  startTime: string
  size: number
}
```

- [ ] **Step 2: Type-check via the frontend build**

Run: `make build-cli`
Expected: builds successfully (vite compiles the TS; a type error here would fail the build). This binary build also embeds the frontend, so it is the integration gate for all frontend tasks.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "Frontend API: unwrap getCommands, add recordings helpers"
```

---

### Task 6: Tabbed right panel — command history + recordings UI

**Files:**
- Create: `frontend/src/components/RightPanel.svelte`
- Create: `frontend/src/components/CommandHistory.svelte`
- Create: `frontend/src/components/Recordings.svelte`
- Modify: `frontend/src/components/StepContent.svelte:7,135-138`
- Modify: `frontend/package.json:11-21`
- Regenerate: `frontend/package-lock.json`

**Interfaces:**
- Consumes: `api.getCommands`, `api.listRecordings`, `api.recordingURL` (Task 5).

- [ ] **Step 1: Add the asciinema-player dependency**

In `frontend/package.json`, add to `dependencies` (keep alphabetical-ish; place after `@types/markdown-it`):

```json
    "asciinema-player": "^3.10.0",
```

- [ ] **Step 2: Regenerate the lockfile**

Run: `dagger call frontend-lockfile --frontend ./frontend --output frontend/package-lock.json`
Expected: `frontend/package-lock.json` updated with asciinema-player (frontend uses `npm ci`, so package.json and the lockfile must stay in sync).

- [ ] **Step 3: Create `CommandHistory.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type Command } from '../lib/api.js'

  let commands = $state<Command[]>([])
  let error = $state<string | null>(null)

  async function refresh() {
    try {
      commands = await api.getCommands(200)
      error = null
    } catch (e) {
      error = String(e)
    }
  }

  // Poll while mounted; the parent unmounts this component when its tab is not
  // active, so the interval is torn down automatically via the returned cleanup.
  onMount(() => {
    refresh()
    const id = setInterval(refresh, 2000)
    return () => clearInterval(id)
  })
</script>

<div class="h-full overflow-y-auto bg-gray-950 p-4 font-mono text-xs">
  {#if error}
    <p class="text-red-400">{error}</p>
  {:else if commands.length === 0}
    <p class="text-gray-500">No commands yet. Run something in the terminal.</p>
  {:else}
    <ul class="space-y-1">
      {#each commands as c}
        <li class="flex items-start gap-2">
          <span class="text-gray-500 flex-shrink-0">{c.ts.slice(11, 19)}</span>
          <span class="flex-1 break-all text-gray-200">{c.cmd}</span>
          <span class="flex-shrink-0 {c.exit === 0 ? 'text-green-500' : 'text-red-400'}">
            {c.exit}
          </span>
        </li>
      {/each}
    </ul>
  {/if}
</div>
```

- [ ] **Step 4: Create `Recordings.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type RecordingInfo } from '../lib/api.js'
  import * as AsciinemaPlayer from 'asciinema-player'
  import 'asciinema-player/dist/bundle/asciinema-player.css'

  let recs = $state<RecordingInfo[]>([])
  let selected = $state<string | null>(null)
  let error = $state<string | null>(null)
  let playerEl: HTMLDivElement
  let player: { dispose: () => void } | null = null

  async function load() {
    try {
      recs = await api.listRecordings()
      error = null
    } catch (e) {
      error = String(e)
    }
  }

  $effect(() => {
    if (selected && playerEl) {
      player?.dispose()
      player = AsciinemaPlayer.create(api.recordingURL(selected), playerEl)
    }
    return () => {
      player?.dispose()
      player = null
    }
  })

  onMount(load)
</script>

<div class="flex h-full flex-col bg-gray-950">
  <div class="border-b border-gray-800 p-2">
    {#if error}
      <p class="text-red-400 text-xs">{error}</p>
    {:else if recs.length === 0}
      <p class="text-gray-500 text-xs">No recordings yet.</p>
    {:else}
      <ul class="flex flex-wrap gap-2 text-xs">
        {#each recs as r}
          <li>
            <button
              class="rounded px-2 py-1 {selected === r.filename ? 'bg-blue-700 text-white' : 'bg-gray-800 text-gray-300 hover:bg-gray-700'}"
              onclick={() => (selected = r.filename)}
            >
              {r.startTime.slice(11, 19)} · {(r.size / 1024).toFixed(1)}kb
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <div class="flex-1 overflow-auto p-2">
    <div bind:this={playerEl}></div>
    {#if !selected}
      <p class="text-gray-500 text-xs">Select a session to replay.</p>
    {/if}
  </div>
</div>
```

- [ ] **Step 5: Create `RightPanel.svelte`**

The terminal iframe must stay mounted across tab switches (unmounting it kills the ttyd session and its recording), so it is hidden with CSS, not `{#if}`. The other two tabs use `{#if}` so their pollers/players tear down when hidden.

```svelte
<script lang="ts">
  import Terminal from './Terminal.svelte'
  import CommandHistory from './CommandHistory.svelte'
  import Recordings from './Recordings.svelte'

  type Tab = 'terminal' | 'commands' | 'recordings'
  let tab = $state<Tab>('terminal')

  const tabs: { id: Tab; label: string }[] = [
    { id: 'terminal', label: 'Terminal' },
    { id: 'commands', label: 'Commands' },
    { id: 'recordings', label: 'Recordings' },
  ]
</script>

<div class="flex h-full flex-col">
  <div class="flex flex-shrink-0 border-b border-gray-800 text-sm">
    {#each tabs as t}
      <button
        class="px-4 py-2 {tab === t.id
          ? 'border-b-2 border-blue-500 text-white'
          : 'text-gray-400 hover:text-gray-200'}"
        onclick={() => (tab = t.id)}
      >
        {t.label}
      </button>
    {/each}
  </div>
  <div class="relative min-h-0 flex-1">
    <!-- Terminal stays mounted so switching tabs never tears down the session. -->
    <div class="absolute inset-0" class:hidden={tab !== 'terminal'}>
      <Terminal />
    </div>
    {#if tab === 'commands'}
      <div class="absolute inset-0"><CommandHistory /></div>
    {:else if tab === 'recordings'}
      <div class="absolute inset-0"><Recordings /></div>
    {/if}
  </div>
</div>
```

- [ ] **Step 6: Mount `RightPanel` in `StepContent.svelte`**

Change the import (line 7) from `Terminal` to `RightPanel`:

```svelte
  import RightPanel from './RightPanel.svelte'
```

Replace the right column (lines 135-138):

```svelte
  <!-- Right: tabbed panel (terminal / commands / recordings) -->
  <div class="w-1/2 h-full">
    <RightPanel />
  </div>
```

- [ ] **Step 7: Build to verify the frontend compiles and embeds**

Run: `make build-cli`
Expected: builds successfully — vite resolves `asciinema-player` + its CSS, svelte compiles the three new components, and the dist is embedded into the binary.

- [ ] **Step 8: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/components/RightPanel.svelte frontend/src/components/CommandHistory.svelte frontend/src/components/Recordings.svelte frontend/src/components/StepContent.svelte
git commit -m "Add tabbed right panel: command history + recordings player"
```

---

### Task 7: Reconcile docs + status

**Files:**
- Modify: `docs/platform/instrumentation.md`
- Modify: `docs/platform/base-images.md`
- Modify: `docs/platform/backend-service.md`
- Modify: `docs/presentation/frontend.md`
- Modify: `docs/plan.md`
- Modify: `STATUS.md`

**Interfaces:** none (documentation).

- [ ] **Step 1: Fix the recording model in `instrumentation.md`**

In the "Asciinema Recording" section, resolve the single-file/plural-route contradiction. Replace the sentence that says the backend "serves `session.cast` at `GET /api/recording` with HTTP Range support" with wording describing the built API:

> The backend lists recorded sessions at `GET /api/recordings` (newest first) and serves each `session-<ts>.cast` at `GET /api/recordings/{filename}` with HTTP Range support. Served filenames are validated against `^session-[0-9TZ]+\.cast$`.

Also add a note near the `ttyd ... -- asciinema rec ...` line: asciinema is a **vendored static binary** (installed into the base images by the Dagger pipeline; shipped in release assets for standalone/devcontainer), the record format is **asciicast v3**, and if asciinema is not on `PATH` the shell runs un-recorded (graceful degradation).

- [ ] **Step 2: Reconcile `base-images.md`**

Where asciinema is listed (the "Python package or static build" note around line 126), state that the platform vendors a pinned asciinema **static binary** (v3.2.1) installed at `/usr/local/bin/asciinema` by the Dagger base-image build — not a distro/Python package.

- [ ] **Step 3: Update `backend-service.md`**

Resolve the open "command history polling behavior" item: `GET /api/commands` returns an on-demand snapshot; the student UI **polls it periodically (~2s)** while the Commands tab is visible. Document the recordings API surface: `GET /api/recordings`, `GET /api/recordings/{filename}` (Range-capable, filename-guarded).

- [ ] **Step 4: Update `frontend.md`**

Resolve the two open items: command history is **shown in the UI** as a Commands tab in the right-hand panel (periodic poll); session recordings are replayed in a Recordings tab via the embedded **asciinema-player** (≥3.10), fetched from `/api/recordings/{filename}`. The Terminal tab stays mounted across tab switches.

- [ ] **Step 5: Tick the boxes in `plan.md`**

Check these previously-open items:
- Section 6 (backend-service): "Define command history polling behavior" → `[x]` (periodic poll).
- Section 7 (frontend): "Define command history display" → `[x]` (Commands tab); "Define session recordings UI" → `[x]` (asciinema-player Recordings tab).

- [ ] **Step 6: Flip the STATUS.md row**

In `STATUS.md`, the *Session recording (asciinema)* row: change 🟡 → ✅ and update the note to: "Per-session `session-<ts>.cast` capture via asciinema-wrapped ttyd; `/api/recordings` list + range-serve; asciinema-player UI." Remove the "playback API stubbed" caveat here and from the summary section at the bottom.

- [ ] **Step 7: Commit**

```bash
git add docs/platform/instrumentation.md docs/platform/base-images.md docs/platform/backend-service.md docs/presentation/frontend.md docs/plan.md STATUS.md
git commit -m "Reconcile docs + STATUS with built asciinema recording + command-history UI"
```

---

## Manual Verification (post-implementation)

Unit tests cover the store helper, handlers (list, range, traversal guard), and the ttyd arg/env construction. Real capture needs a live tty, so verify end-to-end once by hand:

1. Build and run the example workshop in a container (`make build-workshop` then `workshop run <image>`), or standalone (`workshop serve examples/hello-linux` with asciinema on PATH).
2. Open the UI, run a few commands in the Terminal tab.
3. **Commands tab:** the commands appear within ~2s, non-zero exit codes shown in red.
4. **Recordings tab:** a session appears; selecting it replays the terminal in asciinema-player.
5. Confirm `session-<ts>.cast` exists under the runtime dir.
6. Standalone-without-asciinema check: remove asciinema from PATH → terminal still works, no recording produced, no crash.

## Self-Review Notes

- **Spec coverage:** vendoring (T1), capture+degradation (T2), serve API (T3/T4), command-history UI + recordings UI (T5/T6), doc reconciliation + STATUS flip (T7). All spec sections mapped.
- **Type consistency:** `RecordingInfo{filename,startTime,size}` identical in Go (`store`), the API (`recordings_handler_test`), and TS (`api.ts`). `NewServer(...,mode,runtimeDir)` and `handlers.New(...,mode,runtimeDir)` updated at every call site (container, standalone, both test helpers). `NewTTYDManager(port,runtimeDir)` / `NewTTYDManagerWithShell(port,runtimeDir,shell,extraEnv)` updated at both call sites.
- **Latent bug fixed:** `getCommands` unwraps `{commands}` (T5).
