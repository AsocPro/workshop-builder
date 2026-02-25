# Compilation Layer — Dagger Build Pipeline

## Purpose

Transform a [`workshop.yaml`](../definition/workshop.md) and local source files into a set of tagged OCI images. Each step becomes one immutable image representing the **completed reference state** of that step. All workshop metadata — step definitions, tutorial content, goss specs, LLM configuration — is baked into each image as flat files under `/workshop/`.

There is no separate distribution artifact. The container image IS the workshop.

## Input

- `workshop.yaml` (validated by [Shared Go Library](../platform/shared-go-library.md))
- Local source files referenced by `files[].source` entries
- Markdown files referenced by `steps[].markdownFile` entries
- Goss spec files referenced by `steps[].gossFile` entries
- LLM doc files referenced by `steps[].llm.docs` entries
- Registry credentials (for pushing images)

## Output

| Artifact | Description |
|---|---|
| Tagged OCI images | One image per step, pushed to a container registry |

The `workshop.yaml` and local source files are **not** distributed — only the images in the registry are needed at runtime. Each image contains the complete workshop metadata as flat files.

## Key Properties

- **No diffs.** Each step image contains complete cumulative state.
- **No patch chains.** No step depends on a previous step at runtime — the image is the state.
- **No separate metadata artifact.** Workshop metadata is baked into the image, not distributed separately.
- **Self-contained.** `docker run -p 8080:8080 <image>` — no CLI, no config files, no database.
- **Deterministic.** Building from the same `workshop.yaml` produces the same image content (modulo base image changes).
- **Incrementally cacheable.** Dagger layer caching skips unchanged steps automatically.

## Base Images

The pipeline builds on top of [platform base images](../platform/base-images.md) that include all platform tooling:

| Base Image | Use Case |
|---|---|
| `workshop-base:alpine` | Lightweight workshops |
| `workshop-base:ubuntu` | General Linux workshops |
| `workshop-base:centos` | RHEL-ecosystem workshops |

Base images include: tini, workshop-backend binary (with embedded web UI), goss, asciinema, and shell instrumentation (`/etc/workshop-platform.bashrc`). Authors `FROM workshop-base:<distro>` and layer on their content.

When the author specifies a custom `base.image` or `base.containerFile` (not a `workshop-base:*` image), the pipeline injects the platform layer automatically.

## Dagger Pipeline

The CLI invokes a Dagger pipeline that builds steps sequentially:

```
workshop-base:<distro> (or custom base.image / base.containerFile)
      │
      ▼
Step 1 build:
  FROM base
  → if custom base: inject platform layer (tini + backend + goss + asciinema + bashrc)
  → COPY / write files (from files[] entries)
  → ENV (from env map)
  → RUN commands (from commands[])
  → Bake /workshop/ metadata directory:
      /workshop/workshop.json           (workshop identity + step list + navigation)
      /workshop/steps/<id>/meta.json    (per-step metadata)
      /workshop/steps/<id>/content.md   (tutorial markdown)
      /workshop/steps/<id>/goss.yaml    (validation spec, if present)
      /workshop/steps/<id>/llm.json     (LLM config, if present)
      /workshop/steps/<id>/llm-docs/*   (reference docs, if present)
  → ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
  → push as <workshop.image>:<step-1-id>
      │
      ▼
Step 2 build:
  FROM <workshop.image>:<step-1-id>
  → COPY / write files
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

### Workshop.json Generation

The pipeline generates `/workshop/workshop.json` from `workshop.yaml`:

```json
{
  "name": "explore-kubernetes",
  "image": "myorg/explore-kubernetes",
  "navigation": "free",
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "apiKeyEnv": "WORKSHOP_LLM_API_KEY",
    "maxTokens": 1024,
    "defaultMode": "hints"
  },
  "steps": [
    {"id": "step-pods", "title": "Working with Pods", "group": "basics", "position": 0},
    {"id": "step-services", "title": "Services & Networking", "group": "basics", "position": 1},
    {"id": "step-configmaps", "title": "ConfigMaps & Secrets", "group": "configuration", "position": 2},
    {"id": "step-rbac", "title": "RBAC", "group": "security", "requires": ["step-pods"], "position": 3}
  ]
}
```

### Per-Step Metadata Files

For each step, the pipeline writes:

**`/workshop/steps/<id>/meta.json`**:
```json
{
  "id": "step-pods",
  "title": "Working with Pods",
  "group": "basics",
  "position": 0
}
```

**`/workshop/steps/<id>/content.md`**: resolved from `markdown` or `markdownFile` field.

**`/workshop/steps/<id>/goss.yaml`**: resolved from `goss` or `gossFile` field (if present).

**`/workshop/steps/<id>/llm.json`**: resolved from step-level `llm` config (if present):
```json
{
  "mode": "hints",
  "context": "Common mistake: students forget the -n namespace flag.",
  "hasDocs": true
}
```

**`/workshop/steps/<id>/llm-docs/`**: directory containing copies of files referenced by `llm.docs` entries (if present).

## Platform Layer Injection

When building from a `workshop-base:*` image, the platform layer is already present — no injection needed.

When building from a custom base image, the pipeline adds:

```
Custom base image layers
  (author's Containerfile or base.image)
          │
          ▼
Platform layer (injected by Dagger):
  - /sbin/tini
  - /usr/local/bin/workshop-backend  (embedded web UI assets)
  - /usr/local/bin/goss
  - /usr/bin/asciinema
  - /etc/workshop-platform.bashrc    (PROMPT_COMMAND instrumentation)
  ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

## Incremental Rebuilds

The `--from-step <id>` flag starts the pipeline at a specific step, treating all preceding step images as already built. Only the specified step and all steps after it are rebuilt.

```
workshop build compile --from-step step-3-advanced
```

Steps before `step-3-advanced` retain their existing tags. Steps from `step-3-advanced` onward are rebuilt.

Dagger's own layer cache provides a second level of incrementality: if a step's inputs (files, env, commands, and parent image) are unchanged, Dagger skips the rebuild entirely even without `--from-step`.

## Validation During Compilation

Before the Dagger build starts, the shared library validates the `workshop.yaml`:

- Schema validation (required fields, type correctness, URL-safe IDs)
- Source file existence (all `files[].source` paths exist on disk)
- Markdown file existence (all `markdownFile` paths exist on disk)
- Markdown mutual exclusion (`markdown` and `markdownFile` not both set)
- Goss file existence (all `gossFile` paths exist on disk)
- Goss mutual exclusion (`goss` and `gossFile` not both set)
- LLM doc file existence (all `llm.docs` paths exist on disk)
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
