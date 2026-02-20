# Step Spec — `step-spec.yaml`

## Purpose

Defines the container image build recipe for every workshop step. This is the primary authoring artifact — it describes **what each step's container image contains** (files, environment, commands), not how workspaces are managed or lifecycled.

## What It Contains

- Base image reference
- Per-step file contents (inline or sourced from local paths)
- Per-step environment variables
- Per-step shell commands (run during the image build)
- Step identifiers, titles, and ordering

## What It Does NOT Contain

- TTL or lifecycle mode
- Isolation or team semantics
- Cluster provisioning config
- Quotas or resource classes
- Access surface configuration
- Multi-service topology

These concerns belong in [`workspace.yaml`](./workspace-metadata.md).

## Design Rationale

By encoding each step as an OCI image build recipe:

- Step state is reproducible from source — no live cluster snapshots required
- Step transitions become a single image swap, not a multi-step namespace teardown
- Version control is natural — commit `step-spec.yaml` and local source files
- Incremental builds are handled by Dagger layer caching — unchanged steps are skipped automatically
- Local and cluster mode use the same images with no translation gap

## Schema

```yaml
version: v1

workshop:
  name: <workshop-name>          # used as the SQLite workshop identifier
  image: <org/repo>              # base image name; step ID appended as tag

base:
  image: <registry/image:tag>    # starting layer for step 1

steps:
  - id: <step-id>                # URL-safe identifier; used as image tag suffix
    title: "<display title>"
    files:
      - path: <absolute container path>
        content: |               # inline content
          <file content>
      - path: <absolute container path>
        source: <relative local path>   # relative to step-spec.yaml
    env:
      KEY: value
    commands:
      - <shell command>
```

### Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | string | Yes | Schema version; must be `v1` |
| `workshop.name` | string | Yes | Workshop identifier; stored in SQLite |
| `workshop.image` | string | Yes | Base image name for tag generation (e.g. `myorg/kubernetes-101`) |
| `base.image` | string | Yes | Starting image for step 1 build (e.g. `ubuntu:22.04`) |
| `steps` | list | Yes | Ordered list of step build specs |

### Step Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | Yes | URL-safe step identifier; used as the image tag (e.g. `step-1-intro`) |
| `title` | string | Yes | Human-readable step title for UI display |
| `files` | list | No | Files to write into the image layer |
| `env` | map | No | Environment variables set in the image layer |
| `commands` | list | No | Shell commands run during the image build |

### File Entry Fields

| Field | Type | Description |
|---|---|---|
| `path` | string | Absolute path inside the container image |
| `content` | string | Inline file content (YAML literal block scalar) |
| `source` | string | Relative path to a local file (relative to `step-spec.yaml`) |

Exactly one of `content` or `source` must be present on each file entry.

## Image Tagging Convention

Each step is built and pushed as two tags:

| Tag | Example | Purpose |
|---|---|---|
| `<workshop.image>:<step-id>` | `myorg/kubernetes-101:step-1-intro` | Human-readable; used by CRD and operator |
| `<workshop.image>:<step-id>-<short-digest>` | `myorg/kubernetes-101:step-1-intro-sha256-abc123` | Digest-pinned; used for cache validation |

Both tags are stored in SQLite per step after a successful build.

## Dagger Build Pipeline

The Dagger pipeline processes steps in order, with each step building on the previous:

```
base.image
    │
    ▼
Step 1: FROM base.image
        → write files
        → set env vars
        → run commands
        → push as <workshop.image>:step-1-id
    │
    ▼
Step 2: FROM <workshop.image>:step-1-id
        → write files
        → set env vars
        → run commands
        → push as <workshop.image>:step-2-id
    │
    ▼
Step N: FROM <workshop.image>:step-(N-1)-id
        → write files
        → set env vars
        → run commands
        → push as <workshop.image>:step-N-id
```

OCI layer inheritance replaces snapshot flattening. Each step image contains the complete cumulative state at that point in the workshop.

### Incremental Rebuilds

The `--from-step <id>` flag instructs the build pipeline to start from a specific step, treating all preceding steps as already built. Dagger's layer cache handles unchanged steps automatically — only steps with changed inputs are rebuilt.

```
workshop build compile --from-step step-3-advanced
```

Steps before `step-3-advanced` are loaded from the registry (or Dagger cache) rather than rebuilt.

## Consumers

| Consumer | Source | How It Uses It |
|---|---|---|
| CLI build proxy | Writes to `step-spec.yaml` | Records file diffs, env changes, and commands from interactive authoring sessions |
| Dagger build pipeline | Reads `step-spec.yaml` + local source files | Builds and pushes one OCI image per step |
| [Shared Go Library](../platform/shared-go-library.md) | Reads `step-spec.yaml` | Parses and validates the spec; provides types consumed by CLI and compilation |
| [CLI](../platform/cli.md) | SQLite (image tags) | In local mode, pulls step images and runs containers; in cluster mode, generates CRDs with image tags |
| [Operator](../platform/operator.md) | SQLite (image tags) | Reads image tags per step; updates Deployment spec during step transitions |

`step-spec.yaml` is consumed at build time only. All runtime consumers read exclusively from the [SQLite artifact](../artifact/sqlite-artifact.md).

## Validation Rules

The [Shared Go Library](../platform/shared-go-library.md) validates the step spec before any build occurs.

| Rule | Error Message |
|---|---|
| `version` is missing or not `v1` | `version: must be "v1"` |
| `workshop.name` is missing | `workshop.name: required` |
| `workshop.image` is missing | `workshop.image: required` |
| `base.image` is missing | `base.image: required` |
| `steps` list is empty | `steps: at least one step is required` |
| A step `id` is missing | `steps[<n>]: id is required` |
| A step `id` contains invalid characters (not URL-safe) | `steps[<n>].id: must be lowercase alphanumeric and hyphens only` |
| A step `id` is duplicated | `steps[<n>].id: "<id>" is already used by step <m>` |
| A step `title` is missing | `steps[<n>]: title is required` |
| A file entry has neither `content` nor `source` | `steps[<n>].files[<m>]: exactly one of content or source is required` |
| A file entry has both `content` and `source` | `steps[<n>].files[<m>]: content and source are mutually exclusive` |
| A file entry `source` path does not exist | `steps[<n>].files[<m>].source: file not found: <path>` |
| A file entry `path` is not absolute | `steps[<n>].files[<m>].path: must be an absolute path` |

## Examples

### Single-Step Workshop

Minimal case — one step, one file.

```yaml
version: v1

workshop:
  name: hello-world
  image: myorg/hello-world

base:
  image: ubuntu:22.04

steps:
  - id: step-1-intro
    title: "Introduction"
    files:
      - path: /workspace/README.md
        content: |
          Welcome to the workshop!
```

### Multi-Step Workshop with Local Files

```yaml
version: v1

workshop:
  name: kubernetes-101
  image: myorg/kubernetes-101

base:
  image: ubuntu:22.04

steps:
  - id: step-1-intro
    title: "Introduction"
    files:
      - path: /workspace/README.md
        content: "Welcome."
      - path: /workspace/app/main.go
        source: ./step-1/main.go
    env:
      APP_MODE: development
    commands:
      - go mod download

  - id: step-2-deploy
    title: "Deploy the App"
    files:
      - path: /workspace/app/server.go
        source: ./step-2/server.go
    commands:
      - go build -o /workspace/bin/app ./app
```

### Workshop with Build Dependencies

```yaml
version: v1

workshop:
  name: go-web-app
  image: myorg/go-web-app

base:
  image: golang:1.22

steps:
  - id: step-1-scaffold
    title: "Project Scaffold"
    files:
      - path: /workspace/go.mod
        source: ./scaffold/go.mod
      - path: /workspace/go.sum
        source: ./scaffold/go.sum
      - path: /workspace/main.go
        source: ./scaffold/main.go
    commands:
      - cd /workspace && go mod download

  - id: step-2-handler
    title: "Add HTTP Handler"
    files:
      - path: /workspace/handler.go
        source: ./step-2/handler.go
    commands:
      - cd /workspace && go build ./...

  - id: step-3-tests
    title: "Write Tests"
    files:
      - path: /workspace/handler_test.go
        source: ./step-3/handler_test.go
    commands:
      - cd /workspace && go test ./...
```

## Relationship to workspace.yaml

`step-spec.yaml` is logically paired with `workspace.yaml`. Together they form a complete workshop definition:

- `step-spec.yaml` = what the step container images contain
- `workspace.yaml` = platform behavior (lifecycle, isolation, access)

They are separate files to maintain separation of concerns. The build pipeline reads `step-spec.yaml`; workspace provisioning reads `workspace.yaml`.
