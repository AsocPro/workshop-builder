# Workshop Definition — `workshop.yaml`

## Purpose

The sole author-facing configuration file. Defines the container image build recipe for every workshop step, tutorial content, validation specs, navigation structure, and LLM help configuration. This is what authors write, commit to Git, and hand to the build pipeline.

`workshop.yaml` describes **what each step looks like when completed** (files, environment, commands), **what students read** (markdown tutorial content), **how students navigate** (linear, free, or guided), and **how the LLM help system behaves** (provider, model, per-step context). Deployment behavior — lifecycle, isolation, cluster mode, resources, access — is operator configuration and lives in the [WorkspaceTemplate CRD](../platform/crds.md), not here.

## What It Contains

- Workshop identity and base image reference
- Navigation mode (`linear`, `free`, or `guided`)
- Per-step file contents (inline or sourced from local paths)
- Per-step environment variables
- Per-step shell commands (run during the image build)
- Per-step tutorial markdown (inline or sourced from local files)
- Per-step goss validation specs (inline or sourced from local files)
- Per-step LLM help configuration (mode, context, reference docs)
- Step identifiers, titles, ordering, groups, and prerequisites
- Workshop-level LLM configuration (provider, model, API key env var)

## What It Does NOT Contain

- TTL or lifecycle mode
- Isolation or team semantics
- Cluster provisioning config
- Quotas or resource classes
- Access surface configuration
- API keys or secrets (injected via env vars at runtime)

These are operator concerns configured in the [WorkspaceTemplate CRD](../platform/crds.md).

## Design Rationale

By encoding each step as an OCI image build recipe:

- Step state is reproducible from source — no live cluster snapshots required
- Step transitions become a single image swap, not a multi-step namespace teardown
- Version control is natural — commit `workshop.yaml` and local source files
- Incremental builds are handled by Dagger layer caching — unchanged steps are skipped automatically
- Local and cluster mode use the same images with no translation gap

## The Workshop Is a Container Image

There is no separate distribution artifact. All metadata — step definitions, markdown, goss specs, LLM config — is baked into the image as flat files under `/workshop/`. A workshop runs with just:

```bash
docker run -p 8080:8080 myorg/kubernetes-101:step-1-intro
```

No CLI required, no SQLite, no external configuration. The [compilation pipeline](../artifact/compilation.md) transforms `workshop.yaml` into images built on top of [base images](../platform/base-images.md) that include all platform tooling.

Every step image contains ALL steps' metadata (tutorial content, goss specs, LLM config). Only the `/workspace/` content differs per step. This enables:
- Validating any step at any time (non-linear navigation)
- Showing tutorial content for any step
- Tracking progress as a completion set, not a linear cursor

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
  name: <workshop-name>          # workshop identifier
  image: <org/repo>              # base image name; step ID appended as tag
  navigation: <linear|free|guided>  # navigation mode (default: linear)
  llm:                           # workshop-level LLM config (optional)
    provider: <anthropic>        # LLM provider
    model: <model-id>            # model identifier
    apiKeyEnv: <ENV_VAR_NAME>    # env var containing API key (injected at runtime)
    maxTokens: <number>          # max response tokens (default: 1024)
    defaultMode: <hints|explain|solve>  # default help mode (default: hints)

base:
  image: <registry/image:tag>    # starting layer for step 1
  # OR
  containerFile: <path to container file>

steps:
  - id: <step-id>                # URL-safe identifier; used as image tag suffix
    title: "<display title>"
    group: <group-name>          # step group for guided navigation (optional)
    requires:                    # prerequisite step IDs (optional)
      - <step-id>
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
    llm:                         # per-step LLM config override (optional)
      mode: <hints|explain|solve>  # help mode for this step
      context: |                   # instructor-provided context for the LLM
        <hints about common mistakes, gotchas, etc.>
      docs:                        # reference docs to include in LLM context
        - <relative local path>    # baked into image at /workshop/steps/<id>/llm-docs/
```

### Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | string | Yes | Schema version; must be `v1` |
| `workshop.name` | string | Yes | Workshop identifier |
| `workshop.image` | string | Yes | Base image name for tag generation (e.g. `myorg/kubernetes-101`) |
| `workshop.navigation` | string | No | Navigation mode: `linear` (default), `free`, or `guided` |
| `workshop.llm` | object | No | Workshop-level LLM help configuration |
| `base.image` | string | No | Starting image for step 1 build (e.g. `workshop-base:ubuntu`) |
| `base.containerFile` | string | No | Path to a Containerfile/Dockerfile for building the base layer (relative to `workshop.yaml`) |
| `steps` | list | Yes | Ordered list of step build specs |

`base.image` and `base.containerFile` are mutually exclusive — specify one or the other, not both. Exactly one must be present.

When using a [base image](../platform/base-images.md) (e.g. `workshop-base:ubuntu`), all platform tooling (tini, backend binary, goss, asciinema, shell instrumentation) is pre-installed. When using a custom `base.image` or `base.containerFile`, the compilation pipeline injects the platform layer automatically.

### Navigation Modes

| Mode | Behavior |
|---|---|
| `linear` | Strict ordering — next/prev only. Default mode. |
| `free` | Any step in any order. All steps accessible from the start. |
| `guided` | Free within groups; groups unlock in order or via `requires` prerequisites. |

Progress is tracked as a **completion set** (which steps have been validated) rather than a linear cursor. This enables non-linear navigation: completing step 5 before step 3 is valid in `free` mode.

In `guided` mode, a step is accessible if:
1. All steps in its `requires` list are completed, AND
2. Its group is unlocked (groups unlock when all steps in the previous group are completed)

If neither `requires` nor `group` is set on a step, it is always accessible in `guided` mode.

### Step Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | Yes | URL-safe step identifier; used as the image tag (e.g. `step-1-intro`) |
| `title` | string | Yes | Human-readable step title for UI display |
| `group` | string | No | Group name for `guided` navigation mode |
| `requires` | list | No | Step IDs that must be completed before this step is accessible |
| `markdown` | string | No | Inline Markdown tutorial content for this step |
| `markdownFile` | string | No | Relative path to a `.md` file (relative to `workshop.yaml`) |
| `files` | list | No | Files to write into the image layer |
| `env` | map | No | Environment variables set in the image layer |
| `commands` | list | No | Shell commands run during the image build |
| `goss` | string | No | Inline [goss](https://github.com/goss-org/goss) YAML spec for validating step completion |
| `gossFile` | string | No | Relative path to a goss YAML file (relative to `workshop.yaml`) |
| `llm` | object | No | Per-step LLM help configuration |

`markdown` and `markdownFile` are mutually exclusive — specify one or the other, not both. At compile time, the content is written to `/workshop/steps/<id>/content.md` in the image.

`goss` and `gossFile` are mutually exclusive — specify one or the other, not both. At compile time, the resolved spec is written to `/workshop/steps/<id>/goss.yaml` in the image.

### LLM Configuration

Workshop-level `llm` fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `provider` | string | Yes | LLM provider (`anthropic`) |
| `model` | string | Yes | Model identifier (e.g. `claude-sonnet-4-20250514`) |
| `apiKeyEnv` | string | Yes | Environment variable name containing the API key |
| `maxTokens` | number | No | Maximum response tokens (default: 1024) |
| `defaultMode` | string | No | Default help mode: `hints` (default), `explain`, or `solve` |

Per-step `llm` fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `mode` | string | No | Override help mode for this step |
| `context` | string | No | Instructor-provided context (common mistakes, hints) included in LLM prompts |
| `docs` | list | No | Relative paths to reference doc files; baked into image at `/workshop/steps/<id>/llm-docs/` |

LLM help modes:
- `hints` — nudges and leading questions, never gives the answer directly
- `explain` — explains concepts and shows related examples, but not the exact solution
- `solve` — provides direct solutions (use sparingly, for steps where the learning is in understanding, not discovering)

The API key is **never baked into the image** — it is injected via environment variable at runtime (`docker run -e WORKSHOP_LLM_API_KEY=... <image>`).

See [LLM Help](../platform/llm-help.md) for full details on context assembly and the help API.

### Goss Validation

[Goss](https://github.com/goss-org/goss) specs validate the live state of the student's running workspace container. The goss spec for a step defines what "completed" looks like — it tests the same state that the step's `files`, `env`, and `commands` would produce. When a student clicks the **Validate** button in the UI, the backend service:

1. Reads the step's `goss.yaml` from `/workshop/steps/<id>/goss.yaml`
2. Executes `goss validate --format documentation`
3. Returns per-test pass/fail results to the frontend
4. Writes a goss result event to `/workshop/runtime/state-events.jsonl`

Students can see exactly which checks are failing so they know what to fix. Steps without a goss spec have no validation — the student can mark them complete freely.

Because every step image contains ALL steps' goss specs (under `/workshop/steps/`), the backend can validate any step at any time — enabling non-linear navigation where students complete steps in any order.

#### Goss binary installation

The goss binary is pre-installed in all [base images](../platform/base-images.md) at `/usr/local/bin/goss`. When using a custom base image, the Dagger pipeline installs goss automatically if any step declares a `goss` or `gossFile` field.

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
| `<workshop.image>:<step-id>` | `myorg/kubernetes-101:step-1-intro` | Completed reference state for this step |

When a student needs to start step N, the platform pulls `step-(N-1)` — the image where the previous step is already completed. For step 1, the base image is used.

Note: Digest-pinned tags and workshop versioning strategies are deferred to a future workstream. The `<step-id>` tag is sufficient for v1.

## Dagger Build Pipeline

The Dagger pipeline processes steps in order, with each step building on the previous:

```
workshop-base:<distro> (or base.image / base.containerFile)
    │
    ▼
Step 1: FROM base
        → write files
        → set env vars
        → run commands
        → bake /workshop/ metadata (ALL steps' content, goss, LLM config)
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

The `/workshop/` metadata directory is baked into the first step image and inherited by all subsequent steps. It contains the full workshop definition — all steps' tutorial content, goss specs, and LLM config — so every image can render any step's content.

See [Compilation](../artifact/compilation.md) for full pipeline details.

### Incremental Rebuilds

Dagger's layer cache handles unchanged steps automatically — only steps with changed inputs are rebuilt. No extra arguments or special logic is needed to handle incremental rebuilds.

## Consumers

| Consumer | Source | How It Uses It |
|---|---|---|
| CLI build proxy | Writes to `workshop.yaml` | Records file diffs, env changes, and commands from interactive authoring sessions |
| Dagger build pipeline | Reads `workshop.yaml` + local source files | Builds and pushes one OCI image per step; bakes metadata as flat files |
| [Shared Go Library](../platform/shared-go-library.md) | Reads `workshop.yaml` | Parses and validates the spec; provides types consumed by CLI and compilation |
| [CLI](../platform/cli.md) | Image tags | In local mode, pulls step images and runs containers |
| [Operator](../platform/operator.md) | Image tags | Reads image tags per step; updates Deployment spec during step transitions |

`workshop.yaml` is consumed at build time only. All runtime consumers read from the flat files baked into the image under `/workshop/`.

## Validation Rules

The [Shared Go Library](../platform/shared-go-library.md) validates `workshop.yaml` before any build occurs.

| Rule | Error Message |
|---|---|
| `version` is missing or not `v1` | `version: must be "v1"` |
| `workshop.name` is missing | `workshop.name: required` |
| `workshop.image` is missing | `workshop.image: required` |
| `workshop.navigation` is not `linear`, `free`, or `guided` | `workshop.navigation: must be one of: linear, free, guided` |
| Both `base.image` and `base.containerFile` are missing | `base: exactly one of image or containerFile is required` |
| Both `base.image` and `base.containerFile` are set | `base: image and containerFile are mutually exclusive` |
| `base.containerFile` path does not exist | `base.containerFile: file not found: <path>` |
| `steps` list is empty | `steps: at least one step is required` |
| A step `id` is missing | `steps[<n>]: id is required` |
| A step `id` contains invalid characters (not URL-safe) | `steps[<n>].id: must be lowercase alphanumeric and hyphens only` |
| A step `id` is duplicated | `steps[<n>].id: "<id>" is already used by step <m>` |
| A step `title` is missing | `steps[<n>]: title is required` |
| A step `requires` references a non-existent step ID | `steps[<n>].requires: unknown step "<id>"` |
| A step `requires` creates a cycle | `steps[<n>].requires: circular dependency detected` |
| `workshop.navigation` is `linear` but steps have `group` or `requires` | `steps[<n>].group: not allowed in linear navigation mode` |
| `workshop.llm.apiKeyEnv` is missing when `llm` is configured | `workshop.llm.apiKeyEnv: required when llm is configured` |
| A step `llm.docs` path does not exist | `steps[<n>].llm.docs[<m>]: file not found: <path>` |
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

Minimal case — one step, one file, linear navigation (default).

```yaml
version: v1

workshop:
  name: hello-world
  image: myorg/hello-world

base:
  image: workshop-base:ubuntu

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
  image: workshop-base:ubuntu

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

### Free Navigation Workshop

Students can complete steps in any order. Progress is tracked as a completion set.

```yaml
version: v1

workshop:
  name: explore-kubernetes
  image: myorg/explore-kubernetes
  navigation: free

base:
  image: workshop-base:ubuntu

steps:
  - id: step-pods
    title: "Working with Pods"
    markdownFile: ./docs/pods.md
    gossFile: ./goss/pods.yaml

  - id: step-services
    title: "Services & Networking"
    markdownFile: ./docs/services.md
    gossFile: ./goss/services.yaml

  - id: step-configmaps
    title: "ConfigMaps & Secrets"
    markdownFile: ./docs/configmaps.md
    gossFile: ./goss/configmaps.yaml
```

### Guided Navigation with Groups and Prerequisites

Groups unlock in order. Steps within a group can be completed in any order. The `requires` field adds cross-group dependencies.

```yaml
version: v1

workshop:
  name: kubernetes-deep-dive
  image: myorg/kubernetes-deep-dive
  navigation: guided

base:
  image: workshop-base:ubuntu

steps:
  - id: step-pods
    title: "Working with Pods"
    group: basics
    markdownFile: ./docs/pods.md
    gossFile: ./goss/pods.yaml

  - id: step-services
    title: "Services & Networking"
    group: basics
    markdownFile: ./docs/services.md
    gossFile: ./goss/services.yaml

  - id: step-configmaps
    title: "ConfigMaps & Secrets"
    group: configuration
    markdownFile: ./docs/configmaps.md
    gossFile: ./goss/configmaps.yaml

  - id: step-rbac
    title: "RBAC"
    group: security
    requires:
      - step-pods
    markdownFile: ./docs/rbac.md
    gossFile: ./goss/rbac.yaml
```

### Workshop with LLM Help

```yaml
version: v1

workshop:
  name: kubernetes-101
  image: myorg/kubernetes-101
  llm:
    provider: anthropic
    model: claude-sonnet-4-20250514
    apiKeyEnv: WORKSHOP_LLM_API_KEY
    maxTokens: 1024
    defaultMode: hints

base:
  image: workshop-base:ubuntu

steps:
  - id: step-pods
    title: "Working with Pods"
    markdownFile: ./docs/pods.md
    gossFile: ./goss/pods.yaml
    llm:
      mode: hints
      context: |
        Common mistake: students forget the -n namespace flag.
        The correct namespace for this exercise is "workshop".
      docs:
        - ./docs/kubectl-cheatsheet.md

  - id: step-services
    title: "Services & Networking"
    markdownFile: ./docs/services.md
    gossFile: ./goss/services.yaml
    llm:
      mode: explain
      context: |
        Students often confuse ClusterIP and NodePort service types.
      docs:
        - ./docs/kubectl-cheatsheet.md
        - ./docs/networking-guide.md
```

### Workshop with Goss Validation

The goss binary is pre-installed in base images — authors only need to provide the spec.

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

### Workshop with Custom Base Container

Use a Containerfile to customize the base layer instead of starting from a platform base image.

```yaml
version: v1

workshop:
  name: custom-base-workshop
  image: myorg/custom-base-workshop

base:
  containerFile: ./Containerfile.base

steps:
  - id: step-1-intro
    title: "Introduction"
    markdownFile: ./docs/step-1.md
    files:
      - path: /workspace/README.md
        content: "Welcome to the workshop!"
    commands:
      - echo "Step 1 complete" > /workspace/status.txt

  - id: step-2-advanced
    title: "Advanced Topics"
    markdownFile: ./docs/step-2.md
    files:
      - path: /workspace/advanced.sh
        source: ./step-2/advanced.sh
    commands:
      - chmod +x /workspace/advanced.sh
    goss: |
      file:
        /workspace/advanced.sh:
          exists: true
          mode: "0755"
```

When using a custom base image (not a `workshop-base:*` image), the Dagger pipeline automatically injects the platform layer (tini, backend binary, goss, asciinema, shell instrumentation).
