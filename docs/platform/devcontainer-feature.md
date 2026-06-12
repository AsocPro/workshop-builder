# DevContainer Feature — IDE-Native Workshop Delivery

## Purpose

A DevContainer Feature that installs the workshop platform tooling into any devcontainer. Workshop authors add the feature to their `.devcontainer/devcontainer.json` and get the full workshop experience — step UI, web terminal, goss validation — inside VS Code, Cursor, GitHub Codespaces, DevPod, or JetBrains.

This is the **self-paced, async delivery mode**. It complements the existing container-based mode (OCI step images + `workshop run` CLI) which targets classroom/instructor-led delivery.

## Why Not a Native Binary?

A "passthrough mode" (binary on bare host) was evaluated and rejected **as the self-paced student delivery mode**:

- **Windows** — ttyd uses POSIX pty APIs (`forkpty`/`openpty`), no Windows builds exist. goss has no Windows support.
- **Step transitions are destructive** on a real host — no undo, no clean state reset.
- **Security** — workshop `commands:` would run directly on the user's machine with their full permissions.
- **WSL2** works but negates the value — you're already in Linux, might as well use containers.

A DevContainer Feature sidesteps all of these: it runs inside a container (clean state, isolated, Linux), but the user works in their own IDE with their own keybindings, extensions, and terminal.

These rejection reasons are specific to the student audience. For the single-operator, server-targeted use case — interactive runbooks on a machine the operator owns and intends to mutate — every one of them inverts. That mode exists separately: see [Standalone Mode](./standalone-mode.md).

## What the Feature Installs

The `install.sh` script downloads everything from a single GitHub release (one version tag pins all components):

| Binary | Source | Purpose |
|--------|--------|---------|
| `workshop-backend` | Built from this repo, embedded frontend | Serves UI, API, proxies terminal |
| `compile-workshop` | Built from this repo | Compiles workshop.yaml → flat files |
| `workshop-setup` | Built from this repo | Applies step setup (shared logic with backend) |
| `ttyd` | Vendored from upstream in our release | Terminal-over-HTTP |
| `goss` | Vendored from upstream in our release | Step validation |

Plus:
- `/etc/workshop-platform.bashrc` — command logging instrumentation, downloaded from the release (same file as `backend/instrumentation/workshop-platform.bashrc` — single source of truth)
- `/workshop/runtime/` — pre-created runtime directory

## Feature Options

```json
{
  "workshopPath": {
    "type": "string",
    "default": ".",
    "description": "Path to workshop directory (containing workshop.yaml), relative to workspace root"
  },
  "step": {
    "type": "string",
    "default": "",
    "description": "Step ID to start at. All prior steps have their setup (files + commands) pre-applied. Empty = start at step 1."
  },
  "version": {
    "type": "string",
    "default": "latest",
    "description": "Version of workshop platform tools to install"
  }
}
```

## How It Works End-to-End

1. Workshop author writes `workshop.yaml` + steps in their repo
2. They add the feature to `.devcontainer/devcontainer.json`
3. On container build, `install.sh` installs platform tools (backend, ttyd, goss, compile-workshop)
4. `postCreateCommand` runs `compile-workshop --output-dir /workshop` → generates the full `/workshop/` directory with ALL steps' content, plus a `setup.json` per step (file mappings + commands)
5. If `step` option is set, applies setup for all steps up to and including that step (copies files, runs commands in order)
6. `postStartCommand` launches `workshop-backend` in the background
7. VS Code detects port 8080 and auto-opens the browser to the workshop UI

## Step Transitions: In-Place Linear Progression

All steps live in one container. Unlike the OCI-per-step model, there is no container swap. The `step` parameter controls the starting point:

- **Default (no `step`):** Start at step 1. No setup pre-applied.
- **With `step` (e.g., `"step-3-validate"`):** Setup for steps 1→2→3 applied during `postCreateCommand`.

**Forward navigation** — when the student clicks "Next Step", the backend:
1. Copies that step's `files` from staging (`/workshop/steps/{id}/stage/`) to target paths
2. Runs that step's `commands` via `sh -c`
3. Updates active step in state

One-directional — to reset, rebuild the container. This matches the learning model: you build on previous work.

## Startup & Auto-Open

The feature contributes lifecycle hooks via its metadata:

```json
{
  "containerEnv": {
    "WORKSHOP_MODE": "devcontainer"
  },
  "postCreateCommand": "workshop-compile-and-setup",
  "postStartCommand": "nohup workshop-backend >/tmp/workshop-backend.log 2>&1 &"
}
```

| Hook | When | What |
|------|------|------|
| `postCreateCommand` | Once, on container creation | Compiles workshop content, applies setup up to the configured step |
| `postStartCommand` | Every container start (including reopens) | Launches backend in background (log at `/tmp/workshop-backend.log`) |

Feature options (`workshopPath`, `step`) are only available as env vars while `install.sh` runs at build time, so the installer bakes them into the generated `workshop-compile-and-setup` helper rather than reading them at runtime.

Port forwarding is **not** part of the feature — `forwardPorts`/`portsAttributes` are `devcontainer.json` properties, not feature metadata. VS Code auto-detects the listening port regardless; for the auto-open-browser behavior, authors add to their own `devcontainer.json`:

```json
{
  "forwardPorts": [8080],
  "portsAttributes": {
    "8080": { "label": "Workshop UI", "onAutoForward": "openBrowserOnce" }
  }
}
```

In **GitHub Codespaces**, the same `portsAttributes` config works — Codespaces shows the forwarded port in the Ports tab.

For **non-VS-Code runtimes** (DevPod, JetBrains), the terminal output serves as fallback — users see the URL and open it manually.

## Backend: DevContainer Mode

When `WORKSHOP_MODE=devcontainer`, the backend adjusts its behavior:

- **No management URL** — no external CLI managing containers
- **Step activation** — navigate triggers in-place setup (copy files, run commands) instead of requesting a container swap
- **Setup tracking** — per-step `applied` flag prevents re-running setup on revisit
- **Setup metadata** — reads `setup.json` per step from the compiled `/workshop/steps/{id}/` directory

### `setup.json` Per Step

```json
{
  "files": [
    { "source": "deployment.yaml", "target": "/workspace/deployment.yaml" }
  ],
  "commands": [
    "kubectl apply -f /workspace/deployment.yaml"
  ],
  "env": {
    "KUBECONFIG": "/etc/rancher/k3s/k3s.yaml"
  }
}
```

Files listed in `files` are staged under `/workshop/steps/{id}/stage/` at compile time and copied to their target paths when the step is activated.

## Dual-Mode Workshops

A workshop can support both delivery modes from the same source:

```
my-k8s-workshop/
  workshop.yaml                    ← single source of truth
  steps/
    step-1-intro/...
    step-2-deploy/...
  .devcontainer/
    devcontainer.json              ← for self-paced (VS Code/Codespaces)
```

**Container mode** (classroom, instructor-led):
```bash
dagger call build-workshop --src . --workshop-path ./
workshop run myorg/k8s-workshop
```

**DevContainer mode** (self-paced, async):
```bash
# Clone repo → open in VS Code → "Reopen in Container"
# Or: open in GitHub Codespaces directly
```

Same `workshop.yaml`, same steps, same validation — different packaging.

## What Workshop Authors Get

```json
{
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "features": {
    "ghcr.io/asocpro/workshop-builder/workshop:1": {
      "workshopPath": "."
    }
  },
  "customizations": {
    "vscode": {
      "extensions": ["ms-kubernetes-tools.vscode-kubernetes-tools"]
    }
  }
}
```

The workshop platform is one feature among many. Authors compose it with kubectl, helm, python, or any other devcontainer features. Custom base images, custom tools, custom extensions — all works.

For skip-ahead scenarios:
```json
"ghcr.io/asocpro/workshop-builder/workshop:1": {
  "workshopPath": ".",
  "step": "step-3-validate"
}
```

## Publishing

**Binary releases:** Dagger's `BuildRelease` function produces all release assets (our Go binaries, vendored ttyd/goss, bashrc) for both architectures. The CI release workflow is a thin caller: `dagger call build-release` → upload to GitHub Releases. No build logic lives in CI.

**Feature OCI artifact:** Published to `ghcr.io/asocpro/workshop-builder/workshop` using the standard [devcontainers/action](https://github.com/devcontainers/action) GitHub Action. Major version is auto-extracted from the git tag (e.g., `v1.0.0` → `:1`).

## Relationship to Other Components

- **Base Images** — not used in devcontainer mode. The feature installs the same binaries but into whatever container the author specifies. The bashrc file is shared — `backend/instrumentation/workshop-platform.bashrc` is the single source of truth, included in GitHub releases for both the base image pipeline and the feature's `install.sh`.
- **CLI** — not needed. The devcontainer lifecycle hooks handle startup. No step-transition container swaps.
- **Backend** — same binary, different mode. `WORKSHOP_MODE=devcontainer` enables in-place transitions instead of requesting external swap. The step-application logic (`backend/setup/apply.go`) is shared between the backend's activate handler and the standalone `workshop-setup` CLI.
- **Compile-Workshop** — enhanced with `--output-dir` to write the full `/workshop/` directory structure including `setup.json` and `stage/` per step.
- **Dagger Pipeline** — Dagger is the primary build tool. All Go binary builds share a `buildGoBinary()` helper. Tool versions (ttyd, goss) are pinned in Dagger's download functions — one place. `BuildRelease` produces all release assets (both arches, vendored tools, bashrc). CI workflows are thin callers that invoke Dagger and upload results.

## Shared Code & Single Sources of Truth

To avoid duplication between the devcontainer feature and the existing base image / container pipeline:

| Asset | Single Source | Used By |
|-------|-------------|---------|
| Bashrc instrumentation | `backend/instrumentation/workshop-platform.bashrc` | Base image Dagger build, `BuildRelease` → `install.sh` |
| Step application logic | `backend/setup/apply.go` | Backend activate handler, `cmd/workshop-setup` CLI |
| Go cross-compile | `dagger/main.go` → `buildGoBinary()` | All four binary build functions |
| Tool versions (ttyd, goss) | `dagger/main.go` → `downloadTtydArch()` / `downloadGossArch()` | Base image build, `BuildRelease` |
| All release assets | `dagger/main.go` → `BuildRelease()` | `make build-release`, CI release workflow |
