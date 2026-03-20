# M12 — Proper Base Images via Dagger

## Goal

Real `workshop-base:ubuntu`, `workshop-base:rocky`, and `workshop-base:debian` images built
reproducibly via Dagger. Removes the ad-hoc binary injection from the workshop build pipeline (M4).
Images are published to `ghcr.io/asocpro/workshop-base` for production use.

## Prerequisites

- M4 complete (Dagger pipeline working)
- M9 complete (CLI working end-to-end)
- All previous milestones complete (backend embeds frontend, goss, ttyd, etc.)

## Working Directory

`/home/zach/workshop-builder`

## Acceptance Test

```bash
# Build locally
make base-images

docker run --rm workshop-base:ubuntu  which goss           # → /usr/local/bin/goss
docker run --rm workshop-base:ubuntu  which ttyd           # → /usr/local/bin/ttyd
docker run --rm workshop-base:ubuntu  which workshop-backend # → /usr/local/bin/workshop-backend
docker run --rm workshop-base:ubuntu  cat /etc/workshop-platform.bashrc # → PROMPT_COMMAND hook

docker run --rm workshop-base:rocky   which goss
docker run --rm workshop-base:debian  which goss

# Publish to ghcr.io (requires GITHUB_TOKEN)
make publish-base-images

# After base images exist, rebuild workshop images
make build-workshop
./workshop run localhost/hello-linux:step-1-intro
# Full end-to-end works with proper base images
```

---

## Overview

```
Before M12:
  workshop build pipeline: ubuntu:24.04 → [install apt packages] → [download tools] → step images

After M12:
  base image pipeline: ubuntu:24.04 → [install everything] → workshop-base:ubuntu
                       rockylinux:9 → [install everything] → workshop-base:rocky
                       debian:bookworm-slim → [install everything] → workshop-base:debian
  workshop build pipeline: workshop-base:ubuntu → step images  (much simpler)
```

All three distros use **bash**, so the same `PROMPT_COMMAND` hook works everywhere.

---

## Directory Structure

```
base-images/
  bashrc          (shared PROMPT_COMMAND hook — works in bash on all three distros)
```

One bashrc file shared by Ubuntu, Rocky, and Debian. The Dagger pipeline for base images is
integrated into the main `dagger/main.go` module (no separate module).

---

## `base-images/bashrc`

```bash
# Workshop Platform Shell Instrumentation
# /etc/workshop-platform.bashrc — sourced by /etc/bash.bashrc (Ubuntu/Debian)
# or /etc/bashrc (Rocky)

__workshop_log_command() {
    local exit_code=$?
    local cmd
    # Get last command from history (remove leading number and spaces)
    cmd=$(history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
    if [ -n "$cmd" ] && [ -d /workshop/runtime ]; then
        local escaped_cmd
        escaped_cmd=$(printf '%s' "$cmd" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g')
        printf '{"ts":"%s","cmd":"%s","exit":%d}\n' \
            "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            "$escaped_cmd" \
            "$exit_code" \
            >> /workshop/runtime/command-log.jsonl 2>/dev/null || true
    fi
    return $exit_code
}

PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND; }__workshop_log_command"
export PROMPT_COMMAND
```

### Per-distro sourcing

| Distro | Interactive bash config | Append line |
|--------|------------------------|-------------|
| Ubuntu | `/etc/bash.bashrc` | `source /etc/workshop-platform.bashrc` |
| Debian | `/etc/bash.bashrc` | `source /etc/workshop-platform.bashrc` |
| Rocky  | `/etc/bashrc` | `source /etc/workshop-platform.bashrc` |

---

## tini Strategy

Use a downloaded static binary for all three distros — consistent, no package manager variation.

```
/sbin/tini  (all three distros)
```

Entrypoint for all: `["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]`

---

## Dagger: `BuildBaseImages` and `PublishBaseImages`

Add to `dagger/main.go`:

```go
// BuildBaseImages builds workshop-base:{ubuntu,rocky,debian} and publishes them
// to the local Podman/Docker daemon (no registry auth required).
func (m *WorkshopBuilder) BuildBaseImages(
    ctx context.Context,
    // +defaultPath="/"
    src *dagger.Directory,
) error {
    backendBin := m.BuildBackend(ctx, src)

    for _, variant := range []string{"ubuntu", "rocky", "debian"} {
        img, err := m.buildBaseImage(ctx, src, variant, backendBin)
        if err != nil {
            return fmt.Errorf("building %s base: %w", variant, err)
        }
        tag := "workshop-base:" + variant
        fmt.Printf("Publishing %s locally\n", tag)
        if _, err := img.Publish(ctx, tag); err != nil {
            return fmt.Errorf("publishing %s: %w", tag, err)
        }
    }
    return nil
}

// PublishBaseImages builds all three base images and pushes them to ghcr.io.
// Requires a GitHub token with write:packages scope.
// Usage: dagger call publish-base-images --src . --token env:GITHUB_TOKEN
func (m *WorkshopBuilder) PublishBaseImages(
    ctx context.Context,
    // +defaultPath="/"
    src *dagger.Directory,
    // GitHub token with write:packages scope
    token *dagger.Secret,
    // Registry repo prefix (default: ghcr.io/asocpro/workshop-base)
    // +optional
    // +default="ghcr.io/asocpro/workshop-base"
    repo string,
) error {
    if repo == "" {
        repo = "ghcr.io/asocpro/workshop-base"
    }
    backendBin := m.BuildBackend(ctx, src)

    for _, variant := range []string{"ubuntu", "rocky", "debian"} {
        img, err := m.buildBaseImage(ctx, src, variant, backendBin)
        if err != nil {
            return fmt.Errorf("building %s base: %w", variant, err)
        }
        tag := repo + ":" + variant
        fmt.Printf("Publishing %s\n", tag)
        img = img.WithRegistryAuth("ghcr.io", "x-access-token", token)
        if _, err := img.Publish(ctx, tag); err != nil {
            return fmt.Errorf("publishing %s: %w", tag, err)
        }
    }
    return nil
}

// buildBaseImage builds a single workshop base image variant.
func (m *WorkshopBuilder) buildBaseImage(
    ctx context.Context,
    src *dagger.Directory,
    variant string,
    backendBin *dagger.File,
) (*dagger.Container, error) {
    tini := m.downloadTini(ctx)
    goss := m.downloadGoss(ctx)
    ttyd := m.downloadTtyd(ctx)
    bashrc := src.File("base-images/bashrc")

    switch variant {
    case "ubuntu":
        return m.buildUbuntuBase(ctx, bashrc, backendBin, tini, goss, ttyd), nil
    case "rocky":
        return m.buildRockyBase(ctx, bashrc, backendBin, tini, goss, ttyd), nil
    case "debian":
        return m.buildDebianBase(ctx, bashrc, backendBin, tini, goss, ttyd), nil
    default:
        return nil, fmt.Errorf("unknown variant: %s", variant)
    }
}

func (m *WorkshopBuilder) buildUbuntuBase(
    ctx context.Context,
    bashrc *dagger.File,
    backendBin, tini, goss, ttyd *dagger.File,
) *dagger.Container {
    return dag.Container().
        From("ubuntu:24.04").
        WithExec([]string{
            "sh", "-c",
            "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " +
                "bash curl ca-certificates jq && rm -rf /var/lib/apt/lists/*",
        }).
        WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/etc/workshop-platform.bashrc", bashrc).
        WithExec([]string{"sh", "-c", `echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc`}).
        WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
        WithEntrypoint([]string{"/sbin/tini", "--", "/usr/local/bin/workshop-backend"})
}

func (m *WorkshopBuilder) buildDebianBase(
    ctx context.Context,
    bashrc *dagger.File,
    backendBin, tini, goss, ttyd *dagger.File,
) *dagger.Container {
    return dag.Container().
        From("debian:bookworm-slim").
        WithExec([]string{
            "sh", "-c",
            "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " +
                "bash curl ca-certificates jq && rm -rf /var/lib/apt/lists/*",
        }).
        WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/etc/workshop-platform.bashrc", bashrc).
        WithExec([]string{"sh", "-c", `echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc`}).
        WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
        WithEntrypoint([]string{"/sbin/tini", "--", "/usr/local/bin/workshop-backend"})
}

func (m *WorkshopBuilder) buildRockyBase(
    ctx context.Context,
    bashrc *dagger.File,
    backendBin, tini, goss, ttyd *dagger.File,
) *dagger.Container {
    return dag.Container().
        From("rockylinux:9").
        WithExec([]string{
            "sh", "-c",
            "dnf install -y bash curl ca-certificates jq && dnf clean all",
        }).
        WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/etc/workshop-platform.bashrc", bashrc).
        WithExec([]string{"sh", "-c", `echo 'source /etc/workshop-platform.bashrc' >> /etc/bashrc`}).
        WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
        WithEntrypoint([]string{"/sbin/tini", "--", "/usr/local/bin/workshop-backend"})
}
```

---

## Update Workshop Build Pipeline (`BuildWorkshop`)

After M12, `buildStepImage` no longer installs tools inline — it uses the base image:

```go
func (m *WorkshopBuilder) buildStepImage(
    ctx context.Context,
    src *dagger.Directory,
    workshopPath string,
    compiled *compileOutput,
    step stepOutput,
    position int,
    prev *dagger.Container,
) *dagger.Container {
    var ctr *dagger.Container
    if prev == nil {
        // First step: FROM workshop-base:ubuntu (or compiled.BaseImage)
        ctr = dag.Container().From(compiled.BaseImage)
        ctr = m.bakeWorkshopMetadata(ctx, ctr, src, workshopPath, compiled)
        ctr = ctr.WithExec([]string{"mkdir", "-p", "/workshop/runtime"})
    } else {
        ctr = prev
    }
    // ... same file mappings, commands, env, entrypoint as before
}
```

Add `BaseImage` to `compileOutput`:
```go
type compileOutput struct {
    WorkshopJSON  string
    WorkshopImage string
    BaseImage     string   // from workshop.yaml base.image, default "workshop-base:ubuntu"
    Steps         []stepOutput
}
```

Populate in `cmd/compile-workshop/main.go`:
```go
out.BaseImage = loaded.Manifest.Base.Image
if out.BaseImage == "" {
    out.BaseImage = "workshop-base:ubuntu"
}
```

Also remove `backendBin`, `tini`, `goss`, `ttyd` parameters from `buildStepImage` and
their download calls from `BuildWorkshop` — they're now in the base image.

---

## `Makefile` Update

```makefile
.PHONY: test build-backend build-workshop base-images publish-base-images build-cli

test:
	dagger call test --src .

build-backend:
	dagger call build-backend --src .

base-images:
	dagger call build-base-images --src .

publish-base-images:
	dagger call publish-base-images --src . --token env:GITHUB_TOKEN

build-workshop: base-images
	dagger call build-workshop --src . --workshop-path examples/hello-linux

build-cli:
	dagger call build-cli --src . -o workshop
	chmod +x workshop
```

---

## ghcr.io Setup

ghcr.io (GitHub Container Registry) is **free for public repositories** under a GitHub org or user.
Images published as public packages have no storage/bandwidth limits.

Published image references:
```
ghcr.io/asocpro/workshop-base:ubuntu
ghcr.io/asocpro/workshop-base:rocky
ghcr.io/asocpro/workshop-base:debian
```

### Authentication

The `GITHUB_TOKEN` passed to `publish-base-images` must have `write:packages` scope.
For CI (GitHub Actions), the built-in `GITHUB_TOKEN` works automatically.
For local use, create a PAT at https://github.com/settings/tokens with `write:packages`.

### Making images public

After first push, go to:
`https://github.com/orgs/asocpro/packages` → package settings → change visibility to Public.

---

## Tool Versions — Check Latest at Implementation Time

Always use the latest stable release at time of implementation:

| Tool | Check at |
|------|----------|
| tini | https://github.com/krallin/tini/releases |
| goss | https://github.com/goss-org/goss/releases |
| ttyd | https://github.com/tsl0922/ttyd/releases |
| Ubuntu | https://hub.docker.com/_/ubuntu |
| Debian | https://hub.docker.com/_/debian |
| Rocky Linux | https://hub.docker.com/_/rockylinux |
| golang | https://hub.docker.com/_/golang |
| node | https://hub.docker.com/_/node |

The plan shows specific versions as examples — replace with actual latest stable when implementing.

---

## Key Decisions

- **Three distros, all bash**: Ubuntu, Debian, Rocky. All use `bash` → identical `PROMPT_COMMAND`
  hook. Rocky sources from `/etc/bashrc`; Ubuntu/Debian source from `/etc/bash.bashrc`.
- **Alpine dropped**: ash shell lacks `PROMPT_COMMAND`; command logging is a core feature.
  Alpine may be revisited post-MVP if there's a workaround.
- **tini static binary for all**: downloaded via `dag.HTTP()` for consistency — no package manager
  variation.
- **Local vs published**: `make base-images` publishes locally (no auth). `make publish-base-images`
  pushes to `ghcr.io/asocpro/workshop-base` with a GitHub token.
- **No version tags on base images for MVP**: Just `:ubuntu`, `:rocky`, `:debian`. Add
  versioning (`:v1.0`, `:latest`) post-MVP.
- **Backend binary includes embedded frontend**: `BuildBaseImages` calls `BuildBackend` which
  already builds the frontend first (M6). The base image contains the full binary.
- **M12 does NOT remove M4 tooling download logic immediately**: Implement the base images first,
  then update `BuildWorkshop` to `FROM workshop-base:ubuntu`. M4 download logic is removed
  as part of this milestone since the base image replaces it.
- **`workshop.yaml` default base image**: `workshop-base:ubuntu`. Workshop authors can specify
  `workshop-base:rocky` or `workshop-base:debian` via `base.image`.
