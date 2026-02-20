# Compilation Layer — Dagger Build Pipeline

## Purpose

Transform a [`step-spec.yaml`](../definition/step-spec.md) and local source files into a set of tagged OCI images and an updated [SQLite artifact](./sqlite-artifact.md). Each step becomes one immutable image. The SQLite file records educational metadata and image references.

## Input

- `step-spec.yaml` (validated by [Shared Go Library](../platform/shared-go-library.md))
- Local source files referenced by `files[].source` entries
- Registry credentials (for pushing images)

## Output

| Artifact | Description |
|---|---|
| Tagged OCI images | One image per step, pushed to a container registry |
| Updated SQLite | Educational metadata + image tags/digests per step |

The `step-spec.yaml` and local source files are **not** distributed — only the SQLite file and the images in the registry are needed at runtime.

## Key Properties

- **No diffs.** Each step image contains complete cumulative state.
- **No patch chains.** No step depends on a previous step at runtime — the image is the state.
- **Deterministic.** Building from the same `step-spec.yaml` produces the same image content (modulo base image changes).
- **Incrementally cacheable.** Dagger layer caching skips unchanged steps automatically.

## Dagger Pipeline

The CLI invokes a Dagger pipeline that builds steps sequentially:

```
base.image (from step-spec.yaml)
      │
      ▼
Step 1 build:
  FROM base.image
  → COPY / write files (from files[] entries)
  → ENV (from env map)
  → RUN commands (from commands[])
  → push as <workshop.image>:<step-1-id>
  → push as <workshop.image>:<step-1-id>-<short-digest>
      │
      ▼
Step 2 build:
  FROM <workshop.image>:<step-1-id>
  → COPY / write files
  → ENV
  → RUN commands
  → push as <workshop.image>:<step-2-id>
  → push as <workshop.image>:<step-2-id>-<short-digest>
      │
      ▼
      ...
      │
      ▼
Step N pushed → SQLite updated with all image tags and digests
```

OCI layer inheritance replaces snapshot flattening. Each step image is the complete accumulated state at that point — no runtime computation of diffs between steps is needed.

## Incremental Rebuilds

The `--from-step <id>` flag starts the pipeline at a specific step, treating all preceding step images as already built. Only the specified step and all steps after it are rebuilt.

```
workshop build compile --from-step step-3-advanced
```

Steps before `step-3-advanced` retain their existing tags in SQLite. Steps from `step-3-advanced` onward are rebuilt and their SQLite rows updated.

Dagger's own layer cache provides a second level of incrementality: if a step's inputs (files, env, commands, and parent image) are unchanged, Dagger skips the rebuild entirely even without `--from-step`.

## SQLite Update

After all images are pushed, the pipeline writes to the SQLite artifact:

- For each step: `image_tag` and `image_digest` columns in the `steps` table
- Workshop-level metadata row in the `workshop` table (if not already present)

Educational metadata (step titles, markdown, validation rules) is written separately — either authored manually in the YAML export format and imported, or set via the CLI/GUI authoring tools.

## Validation During Compilation

Before the Dagger build starts, the shared library validates the `step-spec.yaml`:

- Schema validation (required fields, type correctness, URL-safe IDs)
- Source file existence (all `files[].source` paths exist on disk)
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
