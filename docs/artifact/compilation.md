# Compilation Layer — Dagger Build Pipeline

## Purpose

Transform a [`workshop.yaml`](../definition/step-spec.md) and local source files into a set of tagged OCI images and an updated [SQLite artifact](./sqlite-artifact.md). Each step becomes one immutable image representing the **completed reference state** of that step. The SQLite file records educational metadata, image references, tutorial content, and validation specs.

## Input

- `workshop.yaml` (validated by [Shared Go Library](../platform/shared-go-library.md))
- Local source files referenced by `files[].source` entries
- Markdown files referenced by `steps[].markdownFile` entries
- Goss spec files referenced by `steps[].gossFile` entries
- The `workshop-backend` binary, tini, and goss binary (platform binaries — pulled from a platform release or built from source)
- Registry credentials (for pushing images)

## Output

| Artifact | Description |
|---|---|
| Tagged OCI images | One image per step, pushed to a container registry |
| Updated SQLite | Educational metadata + image tags/digests per step |

The `workshop.yaml` and local source files are **not** distributed — only the SQLite file and the images in the registry are needed at runtime.

## Key Properties

- **No diffs.** Each step image contains complete cumulative state.
- **No patch chains.** No step depends on a previous step at runtime — the image is the state.
- **Deterministic.** Building from the same `workshop.yaml` produces the same image content (modulo base image changes).
- **Incrementally cacheable.** Dagger layer caching skips unchanged steps automatically.

## Dagger Pipeline

The CLI invokes a Dagger pipeline that builds steps sequentially:

```
base.image (from workshop.yaml)
      │
      ▼
Step 1 build:
  FROM base.image
  → if any step has goss: install goss binary to /usr/local/bin/goss
  → COPY / write files (from files[] entries)
  → ENV (from env map)
  → RUN commands (from commands[])
  → ADD platform layer: tini + workshop-backend binary
  → SET ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
  → push as <workshop.image>:<step-1-id>
      │
      ▼
Step 2 build:
  FROM <workshop.image>:<step-1-id>
  → COPY / write files
  → ENV
  → RUN commands
  → ADD platform layer: tini + workshop-backend binary  (refreshed to latest)
  → SET ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
  → push as <workshop.image>:<step-2-id>
      │
      ▼
      ...
      │
      ▼
Step N pushed → SQLite updated with all image tags, markdown, and goss specs
```

The platform layer (tini + backend binary) is added to every step image by the pipeline. Authors do not configure this — it is always injected at compile time. The backend binary version used is determined by the platform release in use at compile time.

## Markdown Compilation

For each step that has a `markdown` or `markdownFile` field in `workshop.yaml`, the pipeline:

1. Resolves the content — uses `markdown` directly as inline text, or reads the file at `markdownFile` path
2. Writes the resolved content to the `steps.markdown` column in SQLite

Markdown is compiled into SQLite only — it does not affect the container image contents. This happens as part of the same pipeline run, after images are pushed.

## Goss Spec Compilation

For each step that has a `goss` or `gossFile` field in `workshop.yaml`, the pipeline:

1. Resolves the spec content — uses `goss` directly as inline text, or reads the file at `gossFile` path
2. Writes the resolved spec to the `steps.goss_spec` column in SQLite

Goss specs are stored in SQLite only — they do not affect container image contents. At runtime, the backend service writes the current step's spec to `/workshop/.goss/goss.yaml` inside the container on each step transition. This handles both fresh container starts and in-place step advancement within the same running container.

If any step in the workshop declares a goss spec, the pipeline also installs the `goss` binary into the base image layer at `/usr/local/bin/goss`. This is done once — all subsequent step images inherit it.

Each step image is the complete accumulated **completed reference state** at that point — no runtime computation of diffs between steps is needed.

## Incremental Rebuilds

The `--from-step <id>` flag starts the pipeline at a specific step, treating all preceding step images as already built. Only the specified step and all steps after it are rebuilt.

```
workshop build compile --from-step step-3-advanced
```

Steps before `step-3-advanced` retain their existing tags in SQLite. Steps from `step-3-advanced` onward are rebuilt and their SQLite rows updated.

Dagger's own layer cache provides a second level of incrementality: if a step's inputs (files, env, commands, and parent image) are unchanged, Dagger skips the rebuild entirely even without `--from-step`.

## SQLite Update

After all images are pushed and markdown is resolved, the pipeline writes to the SQLite artifact:

- For each step: `image_tag`, `title`, `markdown` (resolved from `markdown` or `markdownFile`), and `goss_spec` (resolved from `goss` or `gossFile`) in the `steps` table
- Workshop-level metadata row in the `workshop` table (if not already present)

A single compile run produces a complete, ready-to-distribute SQLite file with all step content and image references. No separate authoring step is required for tutorial content.

## Validation During Compilation

Before the Dagger build starts, the shared library validates the `workshop.yaml`:

- Schema validation (required fields, type correctness, URL-safe IDs)
- Source file existence (all `files[].source` paths exist on disk)
- Markdown file existence (all `markdownFile` paths exist on disk)
- Markdown mutual exclusion (`markdown` and `markdownFile` not both set)
- Goss file existence (all `gossFile` paths exist on disk)
- Goss mutual exclusion (`goss` and `gossFile` not both set)
- Step ordering (IDs are unique, at least one step present)

Validation errors abort the pipeline before any images are built.

TODO: Define what additional validation occurs during the build — e.g., whether RUN command failures abort the pipeline or are reported as warnings.

## Recompilation

Recompilation is triggered manually by running `workshop build compile`. It is not automatic.

Use `--from-step <id>` for incremental rebuilds when only later steps have changed.

## Size Expectations

Because each step image is a complete layer stack (not a diff), storage in the registry grows with step count and base image size. For a typical workshop with 10 steps on an `ubuntu:22.04` base:

- Each step adds only its changed files and build outputs as new layers
- OCI layer deduplication in the registry minimizes actual storage
- Unchanged layers are shared across all step images

The SQLite artifact is small regardless of step count — it contains only metadata and image references, not image data. Typical workshops: **under 5MB** for the SQLite file.
