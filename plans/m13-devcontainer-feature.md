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
# 1. Build compile-workshop with --output-dir support
dagger call build-compile-workshop --src . --target-os linux --target-arch amd64 -o ./compile-workshop
chmod +x ./compile-workshop

# 2. Compile example workshop to directory
./compile-workshop --workshop ./examples/hello-linux --output-dir /tmp/workshop-out
# Verify structure:
#   /tmp/workshop-out/workshop.json
#   /tmp/workshop-out/steps/step-1-intro/meta.json
#   /tmp/workshop-out/steps/step-1-intro/content.md
#   /tmp/workshop-out/steps/step-1-intro/setup.json
#   /tmp/workshop-out/steps/step-2-files/stage/hello.txt  (if step has files)

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

backend/
  handlers/
    activate.go              ← NEW: in-place step setup handler
  store/
    metadata.go              ← MODIFY: add StepSetup struct, load setup.json
  server.go                  ← MODIFY: wire activate handler in devcontainer mode
  main.go                    ← MODIFY: detect WORKSHOP_MODE env var

cmd/
  compile-workshop/
    main.go                  ← MODIFY: add --output-dir flag

dagger/
  main.go                    ← MODIFY: add BuildFeature, BuildCompileWorkshop, parameterize BuildBackend

.github/
  workflows/
    release.yml              ← NEW: build + publish binaries on tag push
    publish-feature.yml      ← NEW: publish feature to ghcr.io on tag push

Makefile                     ← MODIFY: add build-feature, build-compile-workshop targets
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

## Step 2: Backend DevContainer Mode

### 2a. Mode Detection

**File: `backend/main.go`**

Read `WORKSHOP_MODE` env var at startup:

```go
workshopMode := os.Getenv("WORKSHOP_MODE")
```

When `workshopMode == "devcontainer"`:
- Skip management URL requirement (no external CLI)
- Log that we're in devcontainer mode

### 2b. Setup Metadata

**File: `backend/store/metadata.go`**

Add types and loader for `setup.json`:

```go
type FileMapping struct {
    Source string `json:"source"`
    Target string `json:"target"`
}

type StepSetup struct {
    Files    []FileMapping     `json:"files"`
    Commands []string          `json:"commands"`
    Env      map[string]string `json:"env"`
}
```

Add method to `MetadataStore` to load setup per step:

```go
func (s *MetadataStore) GetStepSetup(stepID string) (*StepSetup, error) {
    path := filepath.Join(s.basePath, "steps", stepID, "setup.json")
    // read and unmarshal
}
```

### 2c. Activate Handler

**New file: `backend/handlers/activate.go`**

This handler applies a step's setup (copy files, run commands, set env) when the student navigates forward in devcontainer mode.

```go
func ActivateStep(store *store.MetadataStore, state *store.State) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stepID := chi.URLParam(r, "id")

        // 1. Check if already applied
        if state.IsStepApplied(stepID) {
            // Already applied — just update active step
            state.SetActiveStep(stepID)
            json.NewEncoder(w).Encode(map[string]string{"status": "already_applied"})
            return
        }

        // 2. Load setup
        setup, err := store.GetStepSetup(stepID)

        // 3. Copy files from stage/ to targets
        for _, f := range setup.Files {
            src := filepath.Join(store.StepDir(stepID), "stage", f.Source)
            // copy src → f.Target (create parent dirs as needed)
        }

        // 4. Run commands
        for _, cmd := range setup.Commands {
            exec.Command("sh", "-c", cmd).Run()
        }

        // 5. Mark as applied
        state.MarkStepApplied(stepID)
        state.SetActiveStep(stepID)
    }
}
```

### 2d. Wire It Up

**File: `backend/server.go`**

In devcontainer mode, the navigate endpoint should trigger activation:

```go
if workshopMode == "devcontainer" {
    r.Post("/api/steps/{id}/activate", handlers.ActivateStep(metaStore, state))
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

# Download platform binaries from GitHub releases
for BINARY in workshop-backend compile-workshop; do
    URL="${GITHUB}/releases/download/${VERSION}/${BINARY}-linux-${ARCH}"
    echo "  Downloading ${BINARY}..."
    curl -fsSL "$URL" -o "/usr/local/bin/${BINARY}"
    chmod +x "/usr/local/bin/${BINARY}"
done

# Download ttyd
TTYD_VERSION="1.7.7"
curl -fsSL "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${ARCH}" \
    -o /usr/local/bin/ttyd
chmod +x /usr/local/bin/ttyd

# Download goss
GOSS_VERSION="v0.4.9"
curl -fsSL "https://github.com/goss-org/goss/releases/download/${GOSS_VERSION}/goss-linux-${ARCH}" \
    -o /usr/local/bin/goss
chmod +x /usr/local/bin/goss

# Install command logging instrumentation
cat > /etc/workshop-platform.bashrc << 'BASHRC'
# Workshop platform command logging
if [ -n "$WORKSHOP_MODE" ] && [ -d /workshop/runtime ]; then
    _workshop_log_command() {
        local cmd
        cmd=$(history 1 | sed 's/^ *[0-9]* *//')
        if [ -n "$cmd" ]; then
            printf '{"ts":"%s","cmd":"%s","cwd":"%s","exit":%d}\n' \
                "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                "$(echo "$cmd" | sed 's/"/\\"/g')" \
                "$PWD" \
                "$?" >> /workshop/runtime/commands.jsonl
        fi
    }
    PROMPT_COMMAND="_workshop_log_command${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
BASHRC

# Source from system bash profile
if ! grep -q 'workshop-platform.bashrc' /etc/bash.bashrc 2>/dev/null; then
    echo '[ -f /etc/workshop-platform.bashrc ] && . /etc/workshop-platform.bashrc' >> /etc/bash.bashrc
fi

# Create runtime directory
mkdir -p /workshop/runtime

# Install compile-and-setup helper script
cat > /usr/local/bin/workshop-compile-and-setup << 'HELPER'
#!/bin/bash
set -e

WORKSHOP_PATH="${WORKSHOPPATH:-.}"
STEP="${STEP:-}"

# Compile workshop content to /workshop/
compile-workshop --workshop "$WORKSHOP_PATH" --output-dir /workshop

# If a starting step is specified, apply setup for all steps up to it
if [ -n "$STEP" ]; then
    workshop-setup --step "$STEP"
fi

echo "Workshop compiled and ready."
HELPER
chmod +x /usr/local/bin/workshop-compile-and-setup

# Install setup helper (applies steps up to a given step ID)
cat > /usr/local/bin/workshop-setup << 'SETUP'
#!/bin/bash
set -e

STEP=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --step) STEP="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [ -z "$STEP" ]; then
    echo "No step specified, starting at step 1."
    exit 0
fi

# Read step order from workshop.json
STEPS=$(cat /workshop/workshop.json | python3 -c "
import json, sys
data = json.load(sys.stdin)
for s in data.get('steps', []):
    print(s['id'])
" 2>/dev/null || cat /workshop/workshop.json | jq -r '.steps[].id' 2>/dev/null)

# Apply setup for each step up to and including the target
FOUND=0
for S in $STEPS; do
    SETUP_FILE="/workshop/steps/${S}/setup.json"
    if [ -f "$SETUP_FILE" ]; then
        echo "Applying setup for ${S}..."

        # Copy staged files
        python3 -c "
import json, shutil, os
setup = json.load(open('$SETUP_FILE'))
for f in setup.get('files', []):
    src = '/workshop/steps/${S}/stage/' + f['source']
    tgt = f['target']
    os.makedirs(os.path.dirname(tgt), exist_ok=True)
    shutil.copy2(src, tgt)
" 2>/dev/null || true

        # Run commands
        python3 -c "
import json, subprocess
setup = json.load(open('$SETUP_FILE'))
for cmd in setup.get('commands', []):
    subprocess.run(cmd, shell=True, check=True)
" 2>/dev/null || true
    fi

    if [ "$S" = "$STEP" ]; then
        FOUND=1
        break
    fi
done

if [ "$FOUND" -eq 0 ]; then
    echo "Warning: step '${STEP}' not found in workshop.json"
    exit 1
fi

echo "Setup applied through step ${STEP}."
SETUP
chmod +x /usr/local/bin/workshop-setup

echo "Workshop platform tools installed successfully."
```

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
check ttyd
check goss
check workshop-compile-and-setup
check workshop-setup

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

Add three functions:

#### `BuildCompileWorkshop`

Same pattern as `BuildCLI` — cross-compile the `cmd/compile-workshop` binary:

```go
func (m *WorkshopBuilder) BuildCompileWorkshop(
    ctx context.Context,
    src *dagger.Directory,
    targetOS string,    // "linux"
    targetArch string,  // "amd64" or "arm64"
) *dagger.File {
    return dag.Container().
        From("golang:1.24-alpine").
        WithDirectory("/src", src).
        WithWorkdir("/src").
        WithEnvVariable("CGO_ENABLED", "0").
        WithEnvVariable("GOOS", targetOS).
        WithEnvVariable("GOARCH", targetArch).
        WithExec([]string{"go", "build", "-o", "/out/compile-workshop", "./cmd/compile-workshop"}).
        File("/out/compile-workshop")
}
```

#### Parameterize `BuildBackend` for OS/arch

Currently `BuildBackend` hardcodes linux/amd64. Add optional `targetOS` and `targetArch` parameters (default to linux/amd64 for backward compatibility):

```go
func (m *WorkshopBuilder) BuildBackendBinary(
    ctx context.Context,
    src *dagger.Directory,
    targetOS string,    // default "linux"
    targetArch string,  // default "amd64"
) *dagger.File {
    // Same as BuildBackend but with parameterized GOOS/GOARCH
    // Returns just the binary file, not a container
}
```

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

### 4b. GitHub Actions: Binary Releases

**New file: `.github/workflows/release.yml`**

Triggered by version tag push (`v*`). Builds binaries for linux/amd64 + linux/arm64:

```yaml
name: Release Binaries

on:
  push:
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        arch: [amd64, arm64]
    steps:
      - uses: actions/checkout@v4
      - uses: dagger/dagger-for-github@v7
        with:
          verb: call
          args: |
            build-backend-binary --src . --target-os linux --target-arch ${{ matrix.arch }}
            --output workshop-backend-linux-${{ matrix.arch }}
      - uses: dagger/dagger-for-github@v7
        with:
          verb: call
          args: |
            build-compile-workshop --src . --target-os linux --target-arch ${{ matrix.arch }}
            --output compile-workshop-linux-${{ matrix.arch }}
      - uses: dagger/dagger-for-github@v7
        with:
          verb: call
          args: |
            build-cli --src . --platform linux/${{ matrix.arch }}
            --output workshop-cli-linux-${{ matrix.arch }}
      - uses: softprops/action-gh-release@v2
        with:
          files: |
            workshop-backend-linux-${{ matrix.arch }}
            compile-workshop-linux-${{ matrix.arch }}
            workshop-cli-linux-${{ matrix.arch }}
```

### 4c. GitHub Actions: Publish Feature

**New file: `.github/workflows/publish-feature.yml`**

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

### 4d. Makefile Targets

**File: `Makefile`**

Add:

```makefile
build-feature:
	dagger call build-feature --src . --output /tmp/workshop-feature.tgz

build-compile-workshop:
	dagger call build-compile-workshop --src . --target-os linux --target-arch amd64 --output ./compile-workshop
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
2. **`postCreateCommand` runs as the container user** — may need `sudo` for writing to `/workshop/` if it's owned by root. The `install.sh` should `chmod 777 /workshop/runtime` or create with appropriate permissions.
3. **Python fallback** — the setup helper scripts use python3 for JSON parsing. If python3 isn't available, fall back to `jq`. Most devcontainer base images have one or the other.
4. **Binary size** — `workshop-backend` includes the embedded frontend (~5MB gzipped). Total download is ~15-20MB for all binaries. Acceptable for a one-time container build.
5. **Architecture detection** — `uname -m` returns `x86_64` or `aarch64`. Map to `amd64`/`arm64` for GitHub release asset names.
6. **Feature version vs tool version** — the feature's `version` field in `devcontainer-feature.json` is the feature format version. The `version` _option_ controls which binary release to download.
