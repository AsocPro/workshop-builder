# Workshop Definition — `workshop.yaml`

## Purpose

The sole author-facing configuration file. Defines the container image build recipe for every workshop step and the tutorial content displayed to students. This is what authors write, commit to Git, and hand to the build pipeline.

`workshop.yaml` describes **what each step looks like when completed** (files, environment, commands) and **what students read** (markdown tutorial content). Each step's spec is the reference implementation — the "answer key" — for that step. Deployment behavior — lifecycle, isolation, cluster mode, resources, access — is operator configuration and lives in the [WorkspaceTemplate CRD](../platform/crds.md), not here.

## What It Contains

- Workshop identity and base image reference
- Per-step file contents (inline or sourced from local paths)
- Per-step environment variables
- Per-step shell commands (run during the image build)
- Per-step tutorial markdown (inline or sourced from local files)
- Per-step goss validation specs (inline or sourced from local files)
- Step identifiers, titles, and ordering

## What It Does NOT Contain

- TTL or lifecycle mode
- Isolation or team semantics
- Cluster provisioning config
- Quotas or resource classes
- Access surface configuration

These are operator concerns configured in the [WorkspaceTemplate CRD](../platform/crds.md).

## Design Rationale

By encoding each step as an OCI image build recipe:

- Step state is reproducible from source — no live cluster snapshots required
- Step transitions become a single image swap, not a multi-step namespace teardown
- Version control is natural — commit `workshop.yaml` and local source files
- Incremental builds are handled by Dagger layer caching — unchanged steps are skipped automatically
- Local and cluster mode use the same images with no translation gap

## Step Semantics — Completed State

Each step's spec (`files`, `env`, `commands`) describes the **completed state** of that step — what the container should look like *after* the student has finished the step's objectives. The built image for each step is the reference implementation.

When a student begins step N, they receive the **step N-1 completed image** (or the base image for step 1). The tutorial markdown tells them what to do, the goss spec validates that they did it correctly, and the step N image exists as the known-good reference state they can reset to if needed.

This means:
- `step-1` image = the container after step 1 is completed correctly
- To "start" step 1, the student gets the `base` image
- To "start" step N, the student gets the `step-(N-1)` image
- The CLI and operator manage this N-1 mapping — authors just define steps in order

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
    markdown: |                  # inline tutorial content (optional)
      <markdown content>
    # OR:
    markdownFile: <relative local path>   # path to .md file, relative to workshop.yaml (optional)
    files:
      - path: <absolute container path>
        content: |               # inline content
          <file content>
      - path: <absolute container path>
        source: <relative local path>   # relative to workshop.yaml
    env:
      KEY: value
    commands:
      - <shell command>
    goss: |                      # inline goss YAML spec for step validation (optional)
      <goss spec content>
    # OR:
    gossFile: <relative local path>   # path to goss.yaml file, relative to workshop.yaml (optional)
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
| `markdown` | string | No | Inline Markdown tutorial content for this step |
| `markdownFile` | string | No | Relative path to a `.md` file (relative to `workshop.yaml`) |
| `files` | list | No | Files to write into the image layer |
| `env` | map | No | Environment variables set in the image layer |
| `commands` | list | No | Shell commands run during the image build |
| `goss` | string | No | Inline [goss](https://github.com/goss-org/goss) YAML spec for validating step completion |
| `gossFile` | string | No | Relative path to a goss YAML file (relative to `workshop.yaml`) |

`markdown` and `markdownFile` are mutually exclusive — specify one or the other, not both. At compile time, the content is written to the `steps.markdown` column in SQLite. Neither field affects container image contents.

`goss` and `gossFile` are mutually exclusive — specify one or the other, not both. At compile time, the resolved spec content is written to the `steps.goss_spec` column in SQLite. The spec does not affect container image contents.

### Goss Validation

[Goss](https://github.com/goss-org/goss) specs validate the live state of the student's running workspace container. The goss spec for a step defines what "completed" looks like — it tests the same state that the step's `files`, `env`, and `commands` would produce. When a student clicks the **Validate** button in the UI, the backend service:

1. Writes the step's `goss_spec` from SQLite to `/workshop/.goss/goss.yaml` inside the container
2. Executes `goss validate --format documentation`
3. Returns per-test pass/fail results to the frontend

Students can see exactly which checks are failing so they know what to fix before advancing. Steps without a `goss`/`gossFile` field have no validation; the student can advance freely.

#### Why goss specs live in SQLite, not the image

Students don't always get a new container for each step. The typical flow is: start at step 1, complete the work, validate, advance to step 2, continue in the same running container. Only when a student resets or jumps to a specific step do they get a fresh container from the reference image.

Because the container may persist across multiple steps, the backend service manages the goss spec lifecycle — writing the current step's spec to `/workshop/.goss/goss.yaml` on each step transition and removing it when advancing to a step with no validation.

#### Goss binary installation

The Dagger build pipeline installs the `goss` binary into the base image layer automatically when any step in the workshop declares a `goss` or `gossFile` field. Authors do not need to install goss themselves — the compile step handles it. The binary is available at `/usr/local/bin/goss` in all step images.

### File Entry Fields

| Field | Type | Description |
|---|---|---|
| `path` | string | Absolute path inside the container image |
| `content` | string | Inline file content (YAML literal block scalar) |
| `source` | string | Relative path to a local file (relative to `workshop.yaml`) |

Exactly one of `content` or `source` must be present on each file entry.

## Image Tagging Convention

Each step is built and pushed with a single tag. The image represents the **completed state** of that step.

| Tag | Example | Purpose |
|---|---|---|
| `<workshop.image>:<step-id>` | `myorg/kubernetes-101:step-1-intro` | Completed reference state for this step; stored in SQLite, used by CRD and operator |

This tag is stored in the `steps.image_tag` column in SQLite after a successful build. When a student needs to start step N, the platform pulls `step-(N-1)` — the image where the previous step is already completed. For step 1, the base image is used.

Note: Digest-pinned tags and workshop versioning strategies are deferred to a future workstream. The `<step-id>` tag is sufficient for v1.

## Dagger Build Pipeline

The Dagger pipeline processes steps in order, with each step building on the previous:

```
base.image
    │
    ├─ if any step has goss: install goss binary into base layer
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

Each step image is the **completed reference state** for that step. OCI layer inheritance replaces snapshot flattening — each image contains the complete cumulative state at that point in the workshop.

To start step N, the student receives the step N-1 image. The CLI and operator manage this mapping. Goss specs and markdown are stored in SQLite, not in the images.

### Incremental Rebuilds

The `--from-step <id>` flag instructs the build pipeline to start from a specific step, treating all preceding steps as already built. Dagger's layer cache handles unchanged steps automatically — only steps with changed inputs are rebuilt.

```
workshop build compile --from-step step-3-advanced
```

Steps before `step-3-advanced` are loaded from the registry (or Dagger cache) rather than rebuilt.

## Consumers

| Consumer | Source | How It Uses It |
|---|---|---|
| CLI build proxy | Writes to `workshop.yaml` | Records file diffs, env changes, and commands from interactive authoring sessions |
| Dagger build pipeline | Reads `workshop.yaml` + local source files | Builds and pushes one OCI image per step |
| [Shared Go Library](../platform/shared-go-library.md) | Reads `workshop.yaml` | Parses and validates the spec; provides types consumed by CLI and compilation |
| [CLI](../platform/cli.md) | SQLite (image tags) | In local mode, pulls step images and runs containers; in cluster mode, generates CRDs with image tags |
| [Operator](../platform/operator.md) | SQLite (image tags) | Reads image tags per step; updates Deployment spec during step transitions |

`workshop.yaml` is consumed at build time only. All runtime consumers read exclusively from the [SQLite artifact](../artifact/sqlite-artifact.md).

## Validation Rules

The [Shared Go Library](../platform/shared-go-library.md) validates `workshop.yaml` before any build occurs.

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
| Both `markdown` and `markdownFile` are set | `steps[<n>]: markdown and markdownFile are mutually exclusive` |
| `markdownFile` path does not exist | `steps[<n>].markdownFile: file not found: <path>` |
| Both `goss` and `gossFile` are set | `steps[<n>]: goss and gossFile are mutually exclusive` |
| `gossFile` path does not exist | `steps[<n>].gossFile: file not found: <path>` |

### File Deletion

To delete a file that exists in a parent step's image layer, add an explicit `rm` command:

```yaml
commands:
  - rm /workspace/file-to-remove.txt
```

Do not use an empty `files:` entry with blank content — that creates an empty file, it does not delete the file from the layer.

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
    markdown: |
      Welcome to the workshop! In this step you'll get oriented with the environment.
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
    markdownFile: ./docs/step-1.md
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
    markdownFile: ./docs/step-2.md
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
    markdownFile: ./docs/step-1-scaffold.md
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
    markdownFile: ./docs/step-2-handler.md
    files:
      - path: /workspace/handler.go
        source: ./step-2/handler.go
    commands:
      - cd /workspace && go build ./...

  - id: step-3-tests
    title: "Write Tests"
    markdownFile: ./docs/step-3-tests.md
    files:
      - path: /workspace/handler_test.go
        source: ./step-3/handler_test.go
    commands:
      - cd /workspace && go test ./...
    gossFile: ./goss/step-3.yaml
```

### Step with Inline Goss Validation

The goss binary is installed automatically by the build pipeline — authors only need to provide the spec.

```yaml
steps:
  - id: step-1-scaffold
    title: "Project Scaffold"
    markdownFile: ./docs/step-1-scaffold.md
    files:
      - path: /workspace/go.mod
        source: ./scaffold/go.mod
      - path: /workspace/main.go
        source: ./scaffold/main.go
    commands:
      - cd /workspace && go mod download
    goss: |
      file:
        /workspace/go.mod:
          exists: true
        /workspace/main.go:
          exists: true
      command:
        "cd /workspace && go build ./...":
          exit-status: 0
          title: "Project compiles successfully"
```
