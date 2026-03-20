# M13 — DevContainer Feature

## Goal

`ghcr.io/asocpro/workshop-builder/workshop:1` is a DevContainer Feature that installs the workshop platform into any devcontainer. Workshop authors add it to their `devcontainer.json`, open in VS Code or Codespaces, and get the full workshop UI at `localhost:8080` with step navigation, web terminal, and goss validation — no CLI or Docker knowledge required.

## Prerequisites

- M4 complete (Dagger build pipeline)
- M6 complete (backend embeds frontend)
- M7 complete (terminal works)
- M8 complete (goss validation works)

## Working Directory

`/home/zach/workshop-builder`

## Constraint

DevContainer features are published as OCI artifacts. The feature's `install.sh` downloads binaries from GitHub Releases at container build time — the feature tarball itself stays small. Binary releases must be set up first.

## Acceptance Test

```bash
# 1. Build compile-workshop and workshop-setup
dagger call build-compile-workshop --src . --target-os linux --target-arch amd64 -o ./compile-workshop
dagger call build-setup --src . --target-os linux --target-arch amd64 -o ./workshop-setup
chmod +x ./compile-workshop ./workshop-setup

# 2. Compile example workshop to directory
./compile-workshop --workshop ./examples/hello-linux --output-dir /tmp/workshop-out
# Verify structure:
#   /tmp/workshop-out/workshop.json
#   /tmp/workshop-out/steps/step-1-intro/meta.json
#   /tmp/workshop-out/steps/step-1-intro/content.md
#   /tmp/workshop-out/steps/step-1-intro/setup.json
#   /tmp/workshop-out/steps/step-2-files/stage/hello.txt  (if step has files)

# 2b. Test workshop-setup binary (shared logic with backend activate handler)
./workshop-setup --workshop-dir /tmp/workshop-out --step step-2-files
# Verify: files from steps 1-2 are copied to targets, commands executed

# 3. Build feature tarball
dagger call build-feature --src . -o /tmp/workshop-feature.tgz
tar tzf /tmp/workshop-feature.tgz
# Should contain: devcontainer-feature.json, install.sh

# 4. Test in VS Code (manual)
# Create a test project with .devcontainer/devcontainer.json referencing local feature
# Open in VS Code → "Reopen in Container"
# Verify: workshop UI at localhost:8080, step list renders, markdown displays
# Navigate to step 2 → verify files copied and commands run
# Validate step via goss → verify pass/fail works

# 5. Test step parameter
# Set "step": "step-2-files" in devcontainer.json
# Rebuild container → verify steps 1-2 are pre-applied

# 6. Test on GitHub Codespaces
# Push to a test repo, open in Codespaces, verify same behavior
```

---

## Directory Structure

```
devcontainer-feature/
  src/
    workshop/
      devcontainer-feature.json
      install.sh
  test/
    workshop/
      test.sh

cmd/
  compile-workshop/
    main.go                  ← MODIFY: add --output-dir flag
  workshop-setup/
    main.go                  ← NEW: Go binary for step setup (shared logic with backend)

backend/
  handlers/
    activate.go              ← NEW: in-place step setup handler
  setup/
    apply.go                 ← NEW: shared step-application logic (used by activate.go AND cmd/workshop-setup)
  store/
    metadata.go              ← MODIFY: add StepSetup struct, load setup.json
  server.go                  ← MODIFY: wire activate handler in devcontainer mode
  main.go                    ← MODIFY: detect WORKSHOP_MODE env var

dagger/
  main.go                    ← MODIFY: extract buildGoBinary helper, add BuildFeature/BuildCompileWorkshop/BuildSetup, parameterize BuildBackend

.github/
  workflows/
    release.yml              ← NEW: thin CI — calls `dagger call build-release`, uploads to GitHub release
    publish-feature.yml      ← NEW: publish feature to ghcr.io on tag push

Makefile                     ← MODIFY: add build-feature, build-compile-workshop, build-setup targets
```

---

## Step 1: Compile-Workshop `--output-dir` Flag

**File: `cmd/compile-workshop/main.go`**

Add `--output-dir` flag alongside the existing behavior. When set, write the full `/workshop/` directory structure instead of printing to stdout.

```go
var outputDir string
flag.StringVar(&outputDir, "output-dir", "", "Write compiled workshop to this directory (default: print JSON to stdout)")
```

The compilation pipeline is unchanged — `workshop.Parse()` → `workshop.Validate()` → `workshop.Compile()`. The new flag just changes the output sink.

### Output structure under `--output-dir`:

```
<output-dir>/
  workshop.json                          ← global metadata (existing compiled output)
  steps/
    step-1-intro/
      meta.json                          ← step metadata (id, title, position)
      content.md                         ← tutorial markdown
      goss.yaml                          ← validation spec (if present)
      hints.md                           ← hint content (if present)
      explain.md                         ← explanation content (if present)
      solve.md                           ← solution content (if present)
      llm.json                           ← LLM config (if present)
      llm-docs/                          ← LLM reference docs (if present)
      setup.json                         ← NEW: { files, commands, env }
      stage/                             ← NEW: copies of step's source files
        hello.txt                        ← ready to copy to target path
    step-2-files/
      ...
```

### `setup.json` format:

```json
{
  "files": [
    {
      "source": "hello.txt",
      "target": "/workspace/hello.txt"
    }
  ],
  "commands": [
    "echo 'Step setup complete'"
  ],
  "env": {}
}
```

`files[].source` is relative to the step's `stage/` directory. `files[].target` is the absolute path where the file should be placed when the step is activated.

### Key implementation details:

- Read `files` from the step's `step.yaml` (the `files:` block maps source → target)
- Copy each source file into `stage/` with its original filename
- Read `commands` from `step.yaml` `commands:` block
- Read `env` from `step.yaml` `env:` block (if it exists)
- Write `setup.json` even if all fields are empty (simplifies backend logic)

---

## Step 2: Shared Step-Application Logic + Backend DevContainer Mode

### Consolidation principle

Step application (read `setup.json`, copy files from `stage/`, run commands) is needed in two places:
1. **At container build time** — `postCreateCommand` pre-applies steps up to the `step` option
2. **At runtime** — backend's activate handler applies the next step on forward navigation

Rather than implementing this twice (once in bash/python, once in Go), we implement it **once in Go** as a shared package, then consume it from both:
- `cmd/workshop-setup/main.go` — standalone CLI binary for build-time use
- `backend/handlers/activate.go` — HTTP handler for runtime use

### 2a. Shared Package

**New file: `backend/setup/apply.go`**

Core step-application logic, no HTTP or CLI dependencies:

```go
package setup

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
)

type FileMapping struct {
    Source string `json:"source"`
    Target string `json:"target"`
}

type StepSetup struct {
    Files    []FileMapping     `json:"files"`
    Commands []string          `json:"commands"`
    Env      map[string]string `json:"env"`
}

// LoadSetup reads setup.json for a given step.
func LoadSetup(workshopDir, stepID string) (*StepSetup, error) {
    path := filepath.Join(workshopDir, "steps", stepID, "setup.json")
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading setup.json for %s: %w", stepID, err)
    }
    var s StepSetup
    if err := json.Unmarshal(data, &s); err != nil {
        return nil, fmt.Errorf("parsing setup.json for %s: %w", stepID, err)
    }
    return &s, nil
}

// Apply executes a step's setup: copies staged files to targets, runs commands.
func Apply(workshopDir, stepID string, setup *StepSetup) error {
    stageDir := filepath.Join(workshopDir, "steps", stepID, "stage")

    // Copy files
    for _, f := range setup.Files {
        src := filepath.Join(stageDir, f.Source)
        if err := copyFile(src, f.Target); err != nil {
            return fmt.Errorf("copying %s → %s: %w", f.Source, f.Target, err)
        }
    }

    // Run commands
    for _, cmdStr := range setup.Commands {
        cmd := exec.Command("sh", "-c", cmdStr)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("running command %q: %w", cmdStr, err)
        }
    }
    return nil
}

func copyFile(src, dst string) error {
    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
        return err
    }
    // ... standard file copy
}
```

### 2b. `cmd/workshop-setup` CLI

**New file: `cmd/workshop-setup/main.go`**

Standalone binary that applies steps up to a target. No python3/jq dependency — pure Go.

```go
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/asocpro/workshop-builder/backend/setup"
)

func main() {
    workshopDir := flag.String("workshop-dir", "/workshop", "compiled workshop directory")
    throughStep := flag.String("step", "", "apply setup through this step ID (inclusive)")
    flag.Parse()

    if *throughStep == "" {
        fmt.Println("No step specified, starting at step 1.")
        return
    }

    // Read step order from workshop.json
    stepIDs, err := readStepOrder(*workshopDir)
    // ... iterate steps, call setup.LoadSetup + setup.Apply for each up to throughStep
}
```

This binary is:
- Built by the same Dagger pipeline and included in GitHub releases
- Downloaded by `install.sh` alongside the other binaries
- Called by the `workshop-compile-and-setup` helper in `postCreateCommand`

### 2c. Mode Detection

**File: `backend/main.go`**

Read `WORKSHOP_MODE` env var at startup:

```go
workshopMode := os.Getenv("WORKSHOP_MODE")
```

When `workshopMode == "devcontainer"`:
- Skip management URL requirement (no external CLI)
- Log that we're in devcontainer mode

### 2d. Setup Metadata in Store

**File: `backend/store/metadata.go`**

Add method to load setup per step (delegates to the shared package):

```go
func (s *MetadataStore) GetStepSetup(stepID string) (*setup.StepSetup, error) {
    return setup.LoadSetup(s.basePath, stepID)
}
```

### 2e. Activate Handler

**New file: `backend/handlers/activate.go`**

Uses the shared `setup.Apply()` — no duplicated file-copy or command-execution logic:

```go
func ActivateStep(metaStore *store.MetadataStore, state *store.State, workshopDir string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stepID := chi.URLParam(r, "id")

        // 1. Check if already applied
        if state.IsStepApplied(stepID) {
            state.SetActiveStep(stepID)
            json.NewEncoder(w).Encode(map[string]string{"status": "already_applied"})
            return
        }

        // 2. Load and apply setup (shared logic)
        stepSetup, err := setup.LoadSetup(workshopDir, stepID)
        if err != nil { /* ... */ }

        if err := setup.Apply(workshopDir, stepID, stepSetup); err != nil { /* ... */ }

        // 3. Mark as applied
        state.MarkStepApplied(stepID)
        state.SetActiveStep(stepID)
    }
}
```

### 2f. Wire It Up

**File: `backend/server.go`**

In devcontainer mode, the navigate endpoint should trigger activation:

```go
if workshopMode == "devcontainer" {
    r.Post("/api/steps/{id}/activate", handlers.ActivateStep(metaStore, state, workshopDir))
    // Navigate also triggers activation in devcontainer mode
}
```

The frontend's "Next Step" action calls `/api/steps/{id}/activate` (in devcontainer mode) or `/api/steps/{id}/navigate` (in container mode). The frontend reads the mode from the state endpoint.

---

## Step 3: Feature Scaffold

### 3a. Feature Metadata

**New file: `devcontainer-feature/src/workshop/devcontainer-feature.json`**

```json
{
  "id": "workshop",
  "version": "1.0.0",
  "name": "Workshop Platform",
  "description": "Adds workshop-builder platform tools (backend UI, web terminal, validation)",
  "options": {
    "workshopPath": {
      "type": "string",
      "default": ".",
      "description": "Path to workshop directory (containing workshop.yaml), relative to workspace root"
    },
    "step": {
      "type": "string",
      "default": "",
      "description": "Step ID to start at. All prior steps have their setup pre-applied."
    },
    "version": {
      "type": "string",
      "default": "latest",
      "description": "Version of workshop platform tools to install"
    }
  },
  "containerEnv": {
    "WORKSHOP_MODE": "devcontainer"
  },
  "forwardPorts": [8080],
  "portsAttributes": {
    "8080": {
      "label": "Workshop UI",
      "onAutoForward": "openBrowserOnce"
    }
  },
  "postCreateCommand": "workshop-compile-and-setup",
  "postStartCommand": "workshop-backend &"
}
```

### 3b. Install Script

**New file: `devcontainer-feature/src/workshop/install.sh`**

Key consolidation decisions:
- **Bashrc**: Downloads the canonical `base-images/bashrc` from the GitHub release (asset: `workshop-platform.bashrc`). Single source of truth — no inline copy.
- **Tool binaries (ttyd, goss)**: Downloaded from the GitHub release as bundled assets (not from upstream). The release workflow downloads them once and attaches them. This eliminates hardcoded version constants in `install.sh` — one version tag controls everything.
- **`workshop-setup`**: A Go binary (not a bash+python script). Uses the same `backend/setup` package as the backend's activate handler. No python3/jq dependency.

```bash
#!/bin/bash
set -e

VERSION="${VERSION:-latest}"
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

REPO="asocpro/workshop-builder"
GITHUB="https://github.com/${REPO}"

# Resolve "latest" to actual tag
if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi

echo "Installing workshop platform tools ${VERSION} (${ARCH})..."

# All binaries (ours + vendored tools) are in the GitHub release.
# This eliminates version drift — one release tag pins everything.
for BINARY in workshop-backend compile-workshop workshop-setup ttyd goss; do
    URL="${GITHUB}/releases/download/${VERSION}/${BINARY}-linux-${ARCH}"
    echo "  Downloading ${BINARY}..."
    curl -fsSL "$URL" -o "/usr/local/bin/${BINARY}"
    chmod +x "/usr/local/bin/${BINARY}"
done

# Install command logging instrumentation from the release.
# Single source of truth: base-images/bashrc (same file used by base image pipeline).
curl -fsSL "${GITHUB}/releases/download/${VERSION}/workshop-platform.bashrc" \
    -o /etc/workshop-platform.bashrc

# Source from system bash profile (same pattern as base image build)
if ! grep -q 'workshop-platform.bashrc' /etc/bash.bashrc 2>/dev/null; then
    echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc
fi

# Create runtime directory with open permissions (postCreateCommand runs as container user)
mkdir -p /workshop/runtime
chmod 777 /workshop /workshop/runtime

# Install compile-and-setup helper script (thin wrapper — all logic is in Go binaries)
cat > /usr/local/bin/workshop-compile-and-setup << 'HELPER'
#!/bin/bash
set -e

WORKSHOP_PATH="${WORKSHOPPATH:-.}"
STEP="${STEP:-}"

# Compile workshop content to /workshop/
compile-workshop --workshop "$WORKSHOP_PATH" --output-dir /workshop

# If a starting step is specified, apply setup for all steps up to it
if [ -n "$STEP" ]; then
    workshop-setup --workshop-dir /workshop --step "$STEP"
fi

echo "Workshop compiled and ready."
HELPER
chmod +x /usr/local/bin/workshop-compile-and-setup

echo "Workshop platform tools installed successfully."
```

The `workshop-setup` helper is now just a 10-line bash wrapper calling two Go binaries. No python3, no jq, no JSON parsing in shell.

### 3c. Feature Test

**New file: `devcontainer-feature/test/workshop/test.sh`**

```bash
#!/bin/bash
set -e

# Feature test — verify binaries are installed
echo "Testing workshop feature installation..."

check() {
    if command -v "$1" &>/dev/null; then
        echo "  PASS: $1 found"
    else
        echo "  FAIL: $1 not found"
        exit 1
    fi
}

check workshop-backend
check compile-workshop
check workshop-setup
check ttyd
check goss
check workshop-compile-and-setup

# Verify runtime directory exists
if [ -d /workshop/runtime ]; then
    echo "  PASS: /workshop/runtime exists"
else
    echo "  FAIL: /workshop/runtime not found"
    exit 1
fi

# Verify bashrc instrumentation is installed
if grep -q 'workshop-platform.bashrc' /etc/bash.bashrc; then
    echo "  PASS: bashrc instrumentation installed"
else
    echo "  FAIL: bashrc instrumentation not found in /etc/bash.bashrc"
    exit 1
fi

# Verify WORKSHOP_MODE is set
if [ "$WORKSHOP_MODE" = "devcontainer" ]; then
    echo "  PASS: WORKSHOP_MODE=devcontainer"
else
    echo "  FAIL: WORKSHOP_MODE not set (expected from containerEnv)"
    exit 1
fi

echo "All tests passed."
```

---

## Step 4: Build + Publish Pipeline

### 4a. Dagger Functions

**File: `dagger/main.go`**

#### Refactor: `buildGoBinary` helper

Currently `BuildBackend`, `BuildCLI`, and (soon) `BuildCompileWorkshop` + `BuildSetup` all duplicate the same golang cross-compile boilerplate. Extract a private helper:

```go
// buildGoBinary cross-compiles a Go binary from the given package path.
// All Go binary builds go through this single function.
func (m *WorkshopBuilder) buildGoBinary(
    src *dagger.Directory,
    pkg string,           // e.g. "./cli/", "./cmd/compile-workshop/", "./cmd/workshop-setup/"
    outputName string,    // e.g. "workshop", "compile-workshop", "workshop-setup"
    targetOS string,
    targetArch string,
) *dagger.File {
    return dag.Container().
        From("golang:1.24-alpine").
        WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
        WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
        WithDirectory("/src", src).
        WithWorkdir("/src").
        WithEnvVariable("CGO_ENABLED", "0").
        WithEnvVariable("GOOS", targetOS).
        WithEnvVariable("GOARCH", targetArch).
        WithExec([]string{"go", "mod", "download"}).
        WithExec([]string{
            "go", "build",
            "-ldflags", "-s -w",
            "-o", "/out/" + outputName,
            pkg,
        }).
        File("/out/" + outputName)
}
```

Then refactor existing functions to use it:

```go
// BuildCLI — now a one-liner
func (m *WorkshopBuilder) BuildCLI(ctx context.Context, src *dagger.Directory, targetOS, targetArch string) *dagger.File {
    if targetOS == "" { targetOS = "linux" }
    if targetArch == "" { targetArch = "amd64" }
    return m.buildGoBinary(src, "./cli/", "workshop", targetOS, targetArch)
}

// BuildCompileWorkshop — new, same pattern
func (m *WorkshopBuilder) BuildCompileWorkshop(ctx context.Context, src *dagger.Directory, targetOS, targetArch string) *dagger.File {
    if targetOS == "" { targetOS = "linux" }
    if targetArch == "" { targetArch = "amd64" }
    return m.buildGoBinary(src, "./cmd/compile-workshop/", "compile-workshop", targetOS, targetArch)
}

// BuildSetup — new, same pattern
func (m *WorkshopBuilder) BuildSetup(ctx context.Context, src *dagger.Directory, targetOS, targetArch string) *dagger.File {
    if targetOS == "" { targetOS = "linux" }
    if targetArch == "" { targetArch = "amd64" }
    return m.buildGoBinary(src, "./cmd/workshop-setup/", "workshop-setup", targetOS, targetArch)
}
```

#### Parameterize `BuildBackend` for OS/arch

Add optional `targetOS` and `targetArch` parameters to the **existing** `BuildBackend` (default linux/amd64). Do NOT create a separate `BuildBackendBinary` — just extend the existing function:

```go
func (m *WorkshopBuilder) BuildBackend(ctx context.Context, src *dagger.Directory, targetOS, targetArch string) *dagger.File {
    if targetOS == "" { targetOS = "linux" }
    if targetArch == "" { targetArch = "amd64" }
    // Step 1: Build frontend (unchanged)
    frontendDist := /* ... same as before ... */
    // Step 2: Inject dist/ into Go source tree (unchanged)
    srcWithDist := src.WithDirectory("backend/frontend/dist", frontendDist)
    // Step 3: Use shared helper with the enriched source
    return m.buildGoBinary(srcWithDist, "./backend/", "workshop-backend", targetOS, targetArch)
}
```

All existing callers (`BuildBaseImages`, `BuildWorkshop`, etc.) pass no OS/arch → defaults to linux/amd64 → no behavior change.

#### `BuildFeature`

Tarballs the feature directory into the devcontainer feature format:

```go
func (m *WorkshopBuilder) BuildFeature(
    ctx context.Context,
    src *dagger.Directory,
) *dagger.File {
    featureDir := src.Directory("devcontainer-feature/src/workshop")
    return dag.Container().
        From("alpine:3.21").
        WithDirectory("/feature", featureDir).
        WithWorkdir("/feature").
        WithExec([]string{"tar", "czf", "/out/workshop.tgz", "."}).
        File("/out/workshop.tgz")
}
```

### 4b. Dagger: `BuildRelease` — all release assets in one function

**File: `dagger/main.go`**

All release logic lives in Dagger. A new `BuildRelease` function produces a directory containing every asset needed for a GitHub release:

```go
// BuildRelease builds all release assets for both architectures.
// Returns a directory containing all binaries, vendored tools, and bashrc.
//
//   dagger call build-release --src . --output /tmp/release
func (m *WorkshopBuilder) BuildRelease(
    ctx context.Context,
    src *dagger.Directory,
) *dagger.Directory {
    out := dag.Directory()

    for _, arch := range []string{"amd64", "arm64"} {
        // Our Go binaries
        out = out.WithFile("workshop-backend-linux-"+arch,
            m.BuildBackend(ctx, src, "linux", arch))
        out = out.WithFile("compile-workshop-linux-"+arch,
            m.BuildCompileWorkshop(ctx, src, "linux", arch))
        out = out.WithFile("workshop-setup-linux-"+arch,
            m.BuildSetup(ctx, src, "linux", arch))
        out = out.WithFile("workshop-cli-linux-"+arch,
            m.BuildCLI(ctx, src, "linux", arch))

        // Vendored tools — versions pinned here alongside downloadGoss/downloadTtyd
        out = out.WithFile("ttyd-linux-"+arch, m.downloadTtydArch(arch))
        out = out.WithFile("goss-linux-"+arch, m.downloadGossArch(arch))
    }

    // Bashrc — single source of truth is base-images/bashrc
    out = out.WithFile("workshop-platform.bashrc", src.File("base-images/bashrc"))

    return out
}
```

This requires parameterizing the existing `downloadTtyd`/`downloadGoss` helpers for arch (currently they hardcode amd64). Add arch-aware variants:

```go
func (m *WorkshopBuilder) downloadTtydArch(arch string) *dagger.File {
    ttydArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
    return dag.HTTP("https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd." + ttydArch)
}

func (m *WorkshopBuilder) downloadGossArch(arch string) *dagger.File {
    return dag.HTTP("https://github.com/goss-org/goss/releases/download/v0.4.9/goss-linux-" + arch)
}
```

The existing no-arg `downloadTtyd()`/`downloadGoss()` (used by `buildBaseImage`) become wrappers: `return m.downloadTtydArch("amd64")`. Tool versions are pinned in exactly one place: these Dagger functions.

### 4c. GitHub Actions: Release (thin CI caller)

**New file: `.github/workflows/release.yml`**

CI is a thin wrapper — it calls Dagger and attaches the output to a GitHub release. Zero build logic in the workflow itself.

```yaml
name: Release

on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
      - name: Build all release assets via Dagger
        uses: dagger/dagger-for-github@v7
        with:
          verb: call
          args: build-release --src . --output /tmp/release
      - uses: softprops/action-gh-release@v2
        with:
          files: /tmp/release/*
```

That's it. All logic is in `BuildRelease`. CI just calls it and uploads.

### 4d. GitHub Actions: Publish Feature

**New file: `.github/workflows/publish-feature.yml`**

The `devcontainers/action` is the standard way to publish features as OCI artifacts. This is the one piece that can't move into Dagger (it requires OCI push to ghcr.io with specific manifest format).

```yaml
name: Publish DevContainer Feature

on:
  push:
    tags: ['v*']

jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: devcontainers/action@v1
        with:
          publish-features: true
          base-path-to-features: devcontainer-feature/src
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

This publishes to `ghcr.io/asocpro/workshop-builder/workshop:1`.

### 4e. Makefile Targets

**File: `Makefile`**

Add:

```makefile
build-feature:
	dagger call build-feature --src . --output /tmp/workshop-feature.tgz

build-compile-workshop:
	dagger call build-compile-workshop --src . --target-os linux --target-arch amd64 --output ./compile-workshop
	chmod +x ./compile-workshop

build-setup:
	dagger call build-setup --src . --target-os linux --target-arch amd64 --output ./workshop-setup
	chmod +x ./workshop-setup

# Build all release assets (both arches, vendored tools, bashrc) — same as what CI uploads
build-release:
	dagger call build-release --src . --output /tmp/release
```

---

## Step 5: Testing

### Local Feature Testing

The devcontainer CLI has built-in test support:

```bash
# Install devcontainer CLI
npm install -g @devcontainers/cli

# Test the feature
devcontainer features test \
  --features workshop \
  --base-image mcr.microsoft.com/devcontainers/base:ubuntu \
  --project-folder devcontainer-feature
```

### Integration Test with Example Workshop

1. Build binaries locally
2. Create a test `.devcontainer/devcontainer.json` in `examples/hello-linux/` referencing local feature
3. Open in VS Code → "Reopen in Container"
4. Verify: UI at 8080, step list, markdown, terminal, goss validation
5. Navigate steps, verify file copying and command execution
6. Test `step` parameter for skip-ahead

### Codespaces Test

Push to a test branch with `.devcontainer/devcontainer.json` and open in GitHub Codespaces.

---

## Key Gotchas

1. **Feature `install.sh` runs as root** — binaries go to `/usr/local/bin/`, bashrc to `/etc/`. No permission issues.
2. **`postCreateCommand` runs as the container user** — `install.sh` creates `/workshop/` with `chmod 777` so the non-root user can write to it.
3. **Binary size** — `workshop-backend` includes the embedded frontend (~5MB gzipped). Total download for all binaries is ~15-20MB. Acceptable for a one-time container build.
4. **Architecture detection** — `uname -m` returns `x86_64` or `aarch64`. Map to `amd64`/`arm64` for GitHub release asset names.
5. **Feature version vs tool version** — the feature's `version` field in `devcontainer-feature.json` is the feature format version. The `version` _option_ controls which binary release to download.

## Consolidation Notes

This milestone was designed to avoid duplicating logic that already exists in the codebase. Dagger is the primary build tool — CI workflows are thin callers that just invoke Dagger and upload results.

| What | Single Source of Truth | Consumed By |
|------|----------------------|-------------|
| Bashrc (`workshop-platform.bashrc`) | `base-images/bashrc` | Dagger base image build, `BuildRelease` (copies to release dir), `install.sh` downloads from release |
| Tool versions (ttyd, goss) | `dagger/main.go` → `downloadTtydArch()` / `downloadGossArch()` | Base image build, `BuildRelease` (vendors into release dir) |
| Step-application logic (copy files, run commands) | `backend/setup/apply.go` | `cmd/workshop-setup` CLI binary, `backend/handlers/activate.go` HTTP handler |
| Go binary cross-compile boilerplate | `dagger/main.go` → `buildGoBinary()` | `BuildBackend`, `BuildCLI`, `BuildCompileWorkshop`, `BuildSetup` |
| All release assets | `dagger/main.go` → `BuildRelease()` | `make build-release`, CI release workflow |

**Dagger owns all build logic.** CI never downloads tools, compiles binaries, or makes version decisions. The release workflow is: `dagger call build-release` → `gh-release upload`.

**Bashrc**: The feature's `install.sh` downloads `workshop-platform.bashrc` from the GitHub release. `BuildRelease` copies it from `base-images/bashrc`. Never inline a second copy.

**Tool versions**: Pinned once in `dagger/main.go` (the `downloadTtydArch`/`downloadGossArch` functions). Both the base image pipeline and `BuildRelease` use the same functions. No version constants in CI or `install.sh`.

**Step application**: The `workshop-setup` CLI binary and the backend's activate handler both import `backend/setup.Apply()`. If step-application logic changes (e.g., adding env var injection, permission handling), it changes in one place.

**Go builds**: Four Dagger functions (`BuildBackend`, `BuildCLI`, `BuildCompileWorkshop`, `BuildSetup`) all call `buildGoBinary()`. `BuildBackend` is the only one that adds a pre-step (frontend build) before calling the helper. Changing Go version, cache volumes, or ldflags happens once.
