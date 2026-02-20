# SQLite Artifact — Workshop Metadata Distribution

## Purpose

The SQLite database file serves two related roles:

1. **Distribution artifact** — produced by the compilation pipeline, contains educational content (markdown, goss validation specs), step image references, and navigation. Distributed alongside the OCI images (via Git, download, etc.). Read-only.

2. **Per-instance working database** — at runtime, the [backend service](../platform/backend-service.md) copies the distribution SQLite into ephemeral container storage and uses the copy as the per-instance working database, writing student progress (`runtime_state`, `custom_state`) into it.

Each workspace instance has its own working copy. There is no shared SQLite across instances — a namespace is a single workspace with a single backend process writing to a single database file. SQLite's single-writer model is appropriate for this scope.

Images live in a container registry and are never stored in SQLite.

## Why SQLite

- Single file — trivially portable and distributable
- No server process — embedded in the runtime
- Queryable — inspect contents with standard tooling
- Transactional — safe concurrent reads, atomic writes
- Small — metadata-only, no blobs

## Schema

```sql
-- Workshop identity
CREATE TABLE workshop (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT NOT NULL DEFAULT 'v1',
    created_at  DATETIME NOT NULL
);

-- Step definitions and image references
CREATE TABLE steps (
    id              TEXT PRIMARY KEY,
    workshop_id     TEXT NOT NULL REFERENCES workshop(id),
    position        INTEGER NOT NULL,
    title           TEXT NOT NULL,
    markdown        TEXT,
    image_tag       TEXT NOT NULL,   -- e.g. myorg/kubernetes-101:step-1-intro
    goss_spec       TEXT             -- goss YAML spec for validating step completion; NULL = no validation
);

-- Per-step metadata: hints, unlock conditions, extensible key/value pairs
CREATE TABLE step_metadata (
    step_id  TEXT NOT NULL REFERENCES steps(id),
    key      TEXT NOT NULL,
    value    TEXT NOT NULL,
    PRIMARY KEY (step_id, key)
);

-- Step navigation graph
CREATE TABLE navigation (
    step_id       TEXT PRIMARY KEY REFERENCES steps(id),
    next_step_id  TEXT REFERENCES steps(id),
    prev_step_id  TEXT REFERENCES steps(id),
    unlock_condition TEXT   -- optional expression; NULL means always unlocked
);

-- Step validation uses goss specs stored in `steps.goss_spec`. At runtime, the backend
-- service writes the current step's spec to /workshop/.goss/goss.yaml inside the container
-- on each step transition. When the student clicks Validate, the backend runs
-- `goss validate -g /workshop/.goss/goss.yaml` and returns results to the frontend.
-- A NULL goss_spec means no validation — the student can advance freely.
--
-- TODO: Unlock conditions (navigation.unlock_condition), hint systems, and the
-- step_metadata key/value schema are not yet fully designed.

-- Per-workspace runtime state
CREATE TABLE runtime_state (
    workspace_id  TEXT NOT NULL,
    step_id       TEXT NOT NULL REFERENCES steps(id),
    status        TEXT NOT NULL,   -- pending | active | completed
    started_at    DATETIME,
    completed_at  DATETIME,
    PRIMARY KEY (workspace_id, step_id)
);

-- Arbitrary per-student state
CREATE TABLE custom_state (
    workspace_id  TEXT NOT NULL,
    key           TEXT NOT NULL,
    value         TEXT NOT NULL,
    PRIMARY KEY (workspace_id, key)
);
```

There are no blob columns. No manifest bundles, no file archives, no tar blobs. Step state is fully represented by the OCI image referenced in `steps.image_tag`.

## Database Sections

### 1. Workshop Definition

| Table | Contents |
|---|---|
| `workshop` | Workshop identity: name, version, creation timestamp |
| `steps` | Per-step metadata: title, position, markdown content, OCI image tag, goss validation spec |
| `step_metadata` | Arbitrary per-step key/value pairs: hints, unlock conditions |
| `navigation` | Step ordering and unlock conditions |

### 2. Step Image Registry

For each step, `steps.image_tag` records the built OCI image tag (e.g. `myorg/kubernetes-101:step-1-intro`). This is written by the [Compilation Layer](./compilation.md) after a successful Dagger build.

The operator and CLI read `image_tag` to perform step transitions — no manifest bundles or file archives are needed.

Note: Digest-pinned image references (e.g. hash-suffixed tags or `@sha256:` references) are not used in v1. Workshop versioning and image pinning strategies will be designed as a future workstream.

### 3. Runtime State

| Table | Contents |
|---|---|
| `runtime_state` | Per-workspace, per-step progress tracking |
| `custom_state` | Arbitrary per-student key/value state (answers, notes, etc.) |

Runtime state is **educational progress only** — not infrastructure state. The cluster state at any step is reconstructed from the OCI image, never from runtime snapshots.

## Size Expectations

A typical workshop with 10 steps:

| Component | Approximate Size |
|---|---|
| SQLite file | **< 5 MB** |
| Per-step markdown | 1–20 KB each |
| Image tags and digests | < 1 KB each |
| Per-step goss specs | < 5 KB each |
| Other metadata | < 1 KB per step |

The previous architecture stored Kubernetes manifest bundles and tar blobs of file state in SQLite — a typical workshop was hundreds of MB. The new schema drops all blobs. Image data lives in the registry with OCI layer deduplication; SQLite carries only references.

## Distribution

SQLite and images are distributed separately:

| Artifact | Distribution Method |
|---|---|
| SQLite file | Git repository, direct download, or package registry |
| OCI images | Container registry (Docker Hub, GHCR, ECR, etc.) |

A workshop is fully portable with:
1. The `.db` SQLite file (contains all metadata and image references)
2. Access to the container registry where images are pushed

There is no need to co-locate the SQLite file with the images — the `image_tag` column contains the full registry reference.

## YAML Export/Import

Some authors prefer Git-based declarative workflows. The platform supports:

- **Export:** Workshop DB → YAML files (for version control and review)
- **Import:** YAML files → rebuild DB (for CI/CD pipelines)

The YAML export mirrors the SQLite schema: one file per table or one directory per workshop. The `image_tag` field is included, so an exported+imported workshop is ready to run without recompilation.

TODO: Define the exact YAML export directory structure.

## Integrity

TODO: Define integrity verification — checksums? Signatures? Schema version validation on load?

## Migration

TODO: Define how schema migrations work when the platform evolves (new fields, new tables, format changes).
