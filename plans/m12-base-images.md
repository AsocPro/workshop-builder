# M12 — Proper Base Images via Dagger

## Goal

Real `workshop-base:ubuntu` and `workshop-base:alpine` images built reproducibly. Removes the ad-hoc binary injection from the workshop build pipeline (M4). Clean separation: workshop pipeline just does `FROM workshop-base:ubuntu`.

## Prerequisites

- M4 complete (Dagger pipeline working)
- M9 complete (CLI working end-to-end)
- All previous milestones complete (backend embeds frontend, goss, ttyd, etc.)

## Working Directory

`/home/zach/workshop-builder`

## Acceptance Test

```bash
make base-images
docker run --rm workshop-base:ubuntu which goss
# → /usr/local/bin/goss
docker run --rm workshop-base:ubuntu which ttyd
# → /usr/local/bin/ttyd
docker run --rm workshop-base:ubuntu which workshop-backend
# → /usr/local/bin/workshop-backend
docker run --rm workshop-base:ubuntu cat /etc/workshop-platform.bashrc
# → PROMPT_COMMAND hook content

# After base images exist, rebuild workshop images
make build-workshop
./workshop run localhost/hello-linux:step-1-intro
# Full end-to-end works with proper base images
```

---

## Overview

M12 extracts the base image logic from the workshop build pipeline into standalone reusable images:

```
Before M12:
  workshop build pipeline: ubuntu:24.04 → [install apt packages] → [download tools] → step images

After M12:
  base image pipeline: ubuntu:24.04 → [install everything] → workshop-base:ubuntu
  workshop build pipeline: workshop-base:ubuntu → step images  (much simpler)
```

---

## Directory Structure

```
base-images/
  ubuntu/
    bashrc          (PROMPT_COMMAND hook for Ubuntu/bash)
  alpine/
    bashrc          (PROMPT_COMMAND hook for Alpine/ash)
```

The Dagger pipeline for base images is integrated into the main `dagger/main.go` module (no separate module).

---

## `base-images/ubuntu/bashrc`

```bash
# Workshop Platform Shell Instrumentation
# /etc/workshop-platform.bashrc — sourced by /etc/bash.bashrc

__workshop_log_command() {
    local exit_code=$?
    local cmd
    # Get last command from history (remove leading number and spaces)
    cmd=$(history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
    if [ -n "$cmd" ] && [ -d /workshop/runtime ]; then
        # Escape for JSON using printf (handles basic cases)
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

Note: Preserve the exit code — `__workshop_log_command` captures `$?` at the top and returns it at the end so the user's prompt still shows the correct exit code.

## `base-images/alpine/bashrc`

Alpine uses `ash` (BusyBox shell), not bash. `PROMPT_COMMAND` doesn't exist in ash. Use a different hook mechanism.

For Alpine, use a custom `$ENV` file or source from `/etc/profile`:

```sh
# Workshop Platform Shell Instrumentation
# /etc/workshop-platform.ashrc

# Note: ash doesn't support PROMPT_COMMAND directly
# Use trap DEBUG or a wrapper — ash has limited options
# For MVP: use PS1 trick with command logging disabled in Alpine
# This is a known limitation; use Ubuntu base for workshops needing command logging

# Minimal stub — define the variable to avoid errors
WORKSHOP_PLATFORM_INSTRUMENTED=1
export WORKSHOP_PLATFORM_INSTRUMENTED
```

**MVP decision**: Command logging works in Ubuntu base only. Alpine base is supported for lightweight workshops that don't need command logging. Document this limitation.

A more complete Alpine approach (post-MVP) would use a wrapper script around ash that logs commands.

---

## Dagger: `BuildBaseImages` function

Add to `dagger/main.go`:

```go
// BuildBaseImages builds workshop-base:ubuntu and workshop-base:alpine.
// Both images are published to the local registry.
func (m *WorkshopBuilder) BuildBaseImages(
    ctx context.Context,
    // +defaultPath="/"
    src *dagger.Directory,
) error {
    // Build backend binary (includes embedded frontend)
    backendBin := m.BuildBackend(ctx, src)

    // Build Ubuntu base
    if err := m.buildUbuntuBase(ctx, src, backendBin); err != nil {
        return fmt.Errorf("building ubuntu base: %w", err)
    }

    // Build Alpine base
    if err := m.buildAlpineBase(ctx, src, backendBin); err != nil {
        return fmt.Errorf("building alpine base: %w", err)
    }

    return nil
}

func (m *WorkshopBuilder) buildUbuntuBase(
    ctx context.Context,
    src *dagger.Directory,
    backendBin *dagger.File,
) error {
    tini := m.downloadTini(ctx)
    goss := m.downloadGoss(ctx)
    ttyd := m.downloadTtyd(ctx)

    img := dag.Container().
        From("ubuntu:24.04").
        // Install system packages
        WithExec([]string{
            "sh", "-c",
            "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " +
                "bash curl ca-certificates tini && " +
                "rm -rf /var/lib/apt/lists/*",
        }).
        // Install platform binaries
        WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
        // Install bashrc instrumentation
        WithFile("/etc/workshop-platform.bashrc",
            src.File("base-images/ubuntu/bashrc")).
        WithExec([]string{
            "sh", "-c",
            `echo '\n# Workshop Platform\nif [ -f /etc/workshop-platform.bashrc ]; then\n    . /etc/workshop-platform.bashrc\nfi' >> /etc/bash.bashrc`,
        }).
        // Create runtime directory template
        WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
        // Set entrypoint
        WithEntrypoint([]string{"/usr/bin/tini", "--", "/usr/local/bin/workshop-backend"})

    tag := "workshop-base:ubuntu"
    fmt.Printf("Publishing %s\n", tag)
    _, err := img.Publish(ctx, tag)
    return err
}

func (m *WorkshopBuilder) buildAlpineBase(
    ctx context.Context,
    src *dagger.Directory,
    backendBin *dagger.File,
) error {
    tini := m.downloadTini(ctx)
    goss := m.downloadGossAlpine(ctx)
    ttydBin := m.downloadTtyd(ctx) // ttyd has a static binary that works on alpine

    img := dag.Container().
        From("alpine:3.21").
        WithExec([]string{
            "sh", "-c",
            "apk add --no-cache bash curl ca-certificates",
        }).
        WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/ttyd", ttydBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
        WithFile("/etc/workshop-platform.ashrc",
            src.File("base-images/alpine/bashrc")).
        WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
        WithEntrypoint([]string{"/sbin/tini", "--", "/usr/local/bin/workshop-backend"})

    tag := "workshop-base:alpine"
    fmt.Printf("Publishing %s\n", tag)
    _, err := img.Publish(ctx, tag)
    return err
}

// downloadGossAlpine downloads the musl-linked goss binary for Alpine.
func (m *WorkshopBuilder) downloadGossAlpine(ctx context.Context) *dagger.File {
    // goss provides a musl-linked binary for Alpine
    return dag.HTTP("https://github.com/goss-org/goss/releases/download/v0.4.9/goss-linux-amd64")
    // Note: goss ships a statically-linked binary that works on musl too
    // If not, use the -musl variant: goss-linux-amd64-musl
}
```

---

## Update Workshop Build Pipeline (`BuildWorkshop`)

After M12, `buildStepImage` no longer installs tools inline — it just uses the base image:

```go
func (m *WorkshopBuilder) buildStepImage(
    ctx context.Context,
    src *dagger.Directory,
    workshopPath string,
    compiled *compileOutput,
    step stepOutput,
    position int,
    backendBin *dagger.File,   // no longer used (in base image)
    tini, goss, ttyd *dagger.File, // no longer used
    prev *dagger.Container,
) *dagger.Container {
    var base *dagger.Container
    if prev == nil {
        // First step: FROM workshop-base:ubuntu (or whatever base.image says)
        base = dag.Container().From(compiled.BaseImage)
        // Bake /workshop/ metadata (ALL steps)
        base = m.bakeWorkshopMetadata(ctx, base, src, workshopPath, compiled)
    } else {
        base = prev
    }

    // Apply this step's file mappings, commands, env
    // (same as before)
    // ...

    return base
}
```

Add `BaseImage` to `compileOutput`:
```go
type compileOutput struct {
    WorkshopJSON  string
    WorkshopImage string
    BaseImage     string   // from workshop.yaml base.image
    Steps         []stepOutput
}
```

And populate it in `cmd/compile-workshop/main.go`:
```go
out.BaseImage = loaded.Manifest.Base.Image
if out.BaseImage == "" {
    out.BaseImage = "workshop-base:ubuntu" // default
}
```

---

## `Makefile` Update

```makefile
.PHONY: test build-backend build-workshop base-images build-cli

test:
	dagger call test --src .

build-backend:
	dagger call build-backend --src .

base-images:
	dagger call build-base-images --src .

build-workshop: base-images
	dagger call build-workshop --src . --workshop-path examples/hello-linux

build-cli:
	dagger call build-cli --src . -o workshop
	chmod +x workshop
```

---

## Tool Versions — Check Latest at Implementation Time

Always use the latest stable release at time of implementation:

| Tool | Check at |
|------|----------|
| tini | https://github.com/krallin/tini/releases |
| goss | https://github.com/goss-org/goss/releases |
| ttyd | https://github.com/tsl0922/ttyd/releases |
| Ubuntu | https://hub.docker.com/_/ubuntu |
| Alpine | https://hub.docker.com/_/alpine |
| golang | https://hub.docker.com/_/golang |
| node | https://hub.docker.com/_/node |

The plan shows specific versions as examples — replace with actual latest stable when implementing.

---

## tini in Ubuntu vs Alpine

- Ubuntu: Install tini via apt (`apt-get install -y tini`) → lands at `/usr/bin/tini`
- Alpine: Download static binary → install at `/sbin/tini`

Make sure the entrypoint path matches:
- Ubuntu entrypoint: `["/usr/bin/tini", "--", "/usr/local/bin/workshop-backend"]`
- Alpine entrypoint: `["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]`

---

## Key Decisions

- **M12 does NOT remove M4 tooling download logic immediately** — implement the base images, then update `BuildWorkshop` to use `FROM workshop-base:ubuntu` instead of `FROM ubuntu:24.04`. The M4 fallback logic can be kept as a code path for custom base images.
- **Alpine command logging**: Limited in MVP — documented limitation. Ubuntu is the primary target.
- **Base images published to local registry**: `workshop-base:ubuntu` and `workshop-base:alpine`. For production, these would be pushed to a real registry (e.g., `ghcr.io/asocpro/workshop-base:ubuntu`).
- **Backend binary includes embedded frontend**: `BuildBaseImages` calls `BuildBackend` which already builds the frontend first (M6). So the base image contains the full binary.
- **No version tags on base images for MVP**: Just `workshop-base:ubuntu` (no `:latest` or `:v1.0` distinction). Add versioning post-MVP.
