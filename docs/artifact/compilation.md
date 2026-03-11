# Compilation Layer — Dagger Build Pipeline

## Purpose

Transform a [workshop definition](../definition/workshop.md) — the `workshop.yaml` manifest and per-step directories — into a set of tagged OCI images. Each step becomes one immutable image representing the **completed reference state** of that step. All workshop metadata — step definitions, tutorial content, goss specs, LLM configuration — is compiled into JSON and baked into each image as flat files under `/workshop/`.

There is no separate distribution artifact. The container image IS the workshop.

## Input

- `workshop.yaml` manifest (validated by [Shared Go Library](../platform/shared-go-library.md))
- `prompts/` directory — LLM system prompt overrides (if present)
- Per-step directories under `steps/`:
  - `step.yaml` — build recipe and metadata
  - `content.md` — tutorial markdown
  - `goss.yaml` — validation spec (if present)
  - `hints.md`, `explain.md`, `solve.md` — static help content (if present)
  - `files/` — content files referenced by file mappings
  - `llm-docs/` — LLM reference documents (if present)
- Registry credentials (for pushing images)

## Output

| Artifact | Description |
|---|---|
| Tagged OCI images | One image per step, pushed to a container registry |

The workshop source files are **not** distributed — only the images in the registry are needed at runtime. Each image contains the complete workshop metadata as compiled JSON flat files.

### Why JSON for Compiled Artifacts

The build pipeline compiles author-facing YAML into JSON for the baked `/workshop/` metadata. This is intentional:

- **Zero runtime dependencies** — the backend reads metadata with Go's `encoding/json` (stdlib). No YAML parser needed in the backend binary.
- **Unambiguous types** — JSON has explicit types, avoiding YAML's implicit type coercion issues.
- **Clear boundary** — YAML is for humans (author writes), JSON is compiled output (backend reads). Different format signals "don't hand-edit this."
- **`goss.yaml` stays YAML** — goss expects YAML, but goss consumes it directly. The backend never parses goss specs — it shells out to `goss validate` and reads JSON results from stdout.

## Key Properties

- **No diffs.** Each step image contains complete cumulative state.
- **No patch chains.** No step depends on a previous step at runtime — the image is the state.
- **No separate metadata artifact.** Workshop metadata is baked into the image, not distributed separately.
- **Self-contained.** The image contains everything needed to run the workshop — no separate config files or database. The CLI is the required entry point to start it.
- **Deterministic.** Building from the same workshop source produces the same image content (modulo base image changes).
- **Incrementally cacheable.** Dagger layer caching skips unchanged steps automatically.

## Base Images

The pipeline builds on top of [platform base images](../platform/base-images.md) that include all platform tooling:

| Base Image | Use Case |
|---|---|
| `workshop-base:alpine` | Lightweight workshops |
| `workshop-base:ubuntu` | General Linux workshops |
| `workshop-base:centos` | RHEL-ecosystem workshops |

Base images include: tini, workshop-backend binary (with embedded web UI), goss, asciinema, and shell instrumentation (`/etc/workshop-platform.bashrc`). Authors `FROM workshop-base:<distro>` and layer on their content.

When the author specifies a custom `base.image` or `base.containerFile` (not a `workshop-base:*` image), the author must install the required platform components themselves. The pipeline validates their presence before building. See [Base Images — Custom Base Image Requirements](../platform/base-images.md#custom-base-image-requirements).

## Dagger Pipeline

The CLI invokes a Dagger pipeline that reads the manifest and per-step directories, building steps sequentially:

```
workshop-base:<distro> (or custom base.image / base.containerFile)
      │
      ▼
Step 1 build:
  FROM base
  → validate platform components present (if custom base)
  → COPY files from steps/<id>/files/ to targets (per step.yaml mappings)
  → ENV (from step.yaml env map)
  → RUN commands (from step.yaml commands[])
  → Compile and bake /workshop/ metadata directory:
      /workshop/workshop.json           (compiled from workshop.yaml + all step.yaml files)
      /workshop/prompts/*.md            (copied from prompts/, if present)
      /workshop/steps/<id>/meta.json    (compiled from step.yaml)
      /workshop/steps/<id>/content.md   (copied from steps/<id>/content.md)
      /workshop/steps/<id>/goss.yaml    (copied from steps/<id>/goss.yaml, if present)
      /workshop/steps/<id>/hints.md     (copied from steps/<id>/hints.md, if present)
      /workshop/steps/<id>/explain.md   (copied from steps/<id>/explain.md, if present)
      /workshop/steps/<id>/solve.md     (copied from steps/<id>/solve.md, if present)
      /workshop/steps/<id>/llm.json     (compiled from step.yaml llm config, if present)
      /workshop/steps/<id>/llm-docs/*   (copied from steps/<id>/llm-docs/, if present)
  → verify ENTRYPOINT is set (custom bases must set this themselves)
  → push as <workshop.image>:<step-1-id>
      │
      ▼
Step 2 build:
  FROM <workshop.image>:<step-1-id>
  → COPY files
  → ENV
  → RUN commands
  → push as <workshop.image>:<step-2-id>
      │
      ▼
      ...
      │
      ▼
Step N pushed
```

### Metadata Baking

The `/workshop/` directory is baked into the first step image and inherited by all subsequent steps. It contains the **complete** workshop definition — ALL steps' metadata, not just the current step. This enables:

- Non-linear navigation — the backend can render any step's tutorial content
- Cross-step validation — goss specs for any step are available at any time
- LLM context assembly — the help system can reference any step's configuration
- Progress tracking — the backend knows the full step graph for completion tracking

The metadata is written once (in the step 1 build) and inherited unchanged through OCI layers. Subsequent steps only add their `/workspace/` content changes.

### Compilation: YAML Source → JSON Artifacts

The pipeline compiles the author's YAML source files into JSON artifacts for the backend:

**`workshop.yaml` + all `step.yaml` files → `/workshop/workshop.json`**:
```json
{
  "name": "explore-kubernetes",
  "image": "myorg/explore-kubernetes",
  "navigation": "free",
  "infrastructure": {
    "cluster": {
      "enabled": true,
      "provider": "k3d"
    },
    "extraContainers": [
      {
        "name": "app",
        "image": "myorg/sample-app:latest",
        "ports": [{"port": 3000, "description": "App server"}]
      },
      {
        "name": "db",
        "image": "postgres:16",
        "ports": [{"port": 5432, "description": "Postgres"}],
        "env": {"POSTGRES_PASSWORD": "workshop"}
      }
    ]
  },
  "steps": [
    {"id": "step-pods", "title": "Working with Pods", "group": "basics", "position": 0},
    {"id": "step-services", "title": "Services & Networking", "group": "basics", "position": 1},
    {"id": "step-configmaps", "title": "ConfigMaps & Secrets", "group": "configuration", "position": 2},
    {"id": "step-rbac", "title": "RBAC", "group": "security", "requires": ["step-pods"], "position": 3}
  ]
}
```

The `infrastructure` block is compiled from `workshop.yaml` and included in `workshop.json` so that the CLI can determine what to provision by reading the image alone — no source `workshop.yaml` needed at runtime.

**Each `step.yaml` + convention files → `/workshop/steps/<id>/meta.json`**:
```json
{
  "id": "step-pods",
  "title": "Working with Pods",
  "group": "basics",
  "position": 0,
  "hasGoss": true,
  "hasLlm": true,
  "hasHints": true,
  "hasExplain": false,
  "hasSolve": true
}
```

**Each `step.yaml` with `llm` config → `/workshop/steps/<id>/llm.json`**:
```json
{
  "context": "Common mistake: students forget the -n namespace flag.",
  "hasDocs": true
}
```

Files that are already in their final format are copied directly:
- `prompts/*.md` → `/workshop/prompts/*.md` (if present)
- `steps/<id>/content.md` → `/workshop/steps/<id>/content.md`
- `steps/<id>/goss.yaml` → `/workshop/steps/<id>/goss.yaml` (if present)
- `steps/<id>/hints.md` → `/workshop/steps/<id>/hints.md` (if present)
- `steps/<id>/explain.md` → `/workshop/steps/<id>/explain.md` (if present)
- `steps/<id>/solve.md` → `/workshop/steps/<id>/solve.md` (if present)
- `steps/<id>/llm-docs/*` → `/workshop/steps/<id>/llm-docs/*` (if present)

## Custom Base Image Validation

When building from a `workshop-base:*` image, all platform components are pre-installed — no validation needed.

When building from a custom base image (`base.image` or `base.containerFile`), the pipeline validates that required platform components are present before building any steps. If a required binary is missing, the build fails immediately with a clear error. The pipeline does **not** attempt to inject the platform layer automatically — authors are responsible for installing components in their custom base. See [Base Images — Custom Base Image Requirements](../platform/base-images.md#custom-base-image-requirements) for the full list.

## Incremental Rebuilds

Dagger's cache provides incremental builds natively: if a step's inputs (files, env, commands, and parent image) are unchanged, Dagger skips the rebuild of that layer.

## Validation During Compilation

Before the Dagger build starts, the shared library validates the workshop structure:

- Manifest schema validation (required fields, type correctness, URL-safe IDs)
- Step directory existence (every listed step has a `steps/<id>/` directory)
- Convention file existence (`step.yaml` and `content.md` present in each step directory)
- Source file existence (all `files[].source` entries exist in `steps/<id>/files/`)
- LLM docs validation (`llm-docs/` not empty if present)
- Navigation consistency (`group`/`requires` not used in `linear` mode)
- Step ordering (IDs are unique, at least one step present)
- Prerequisite graph (no cycles in `requires` references)

Validation errors abort the pipeline before any images are built.

## Recompilation

Recompilation is triggered manually by running `workshop build compile`. It is not automatic.

Use `--from-step <id>` for incremental rebuilds when only later steps have changed.

## Size Expectations

Because each step image is a complete layer stack (not a diff), storage in the registry grows with step count and base image size. For a typical workshop with 10 steps on a `workshop-base:ubuntu` base:

- Each step adds only its changed files and build outputs as new layers
- OCI layer deduplication in the registry minimizes actual storage
- Unchanged layers are shared across all step images
- The `/workshop/` metadata directory adds minimal overhead (typically under 1MB)
- Multiple workshops sharing the same base image benefit from shared base layers
