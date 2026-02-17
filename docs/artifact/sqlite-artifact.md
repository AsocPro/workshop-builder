# SQLite Artifact — Portable Workshop Distribution

## Purpose

The SQLite database file is the **primary distributed artifact** for a workshop. A single `.db` file contains everything needed to run a workshop — content, compiled state, and runtime data.

## Why SQLite

- Single file — trivially portable and distributable
- No server process — embedded in the runtime
- Handles multi-megabyte blobs comfortably
- Queryable — can inspect contents with standard tooling
- Transactional — safe concurrent reads, atomic writes

## Database Sections

### 1. Workshop Definition

| Data | Storage |
|---|---|
| Step metadata (title, order, identifiers) | Rows in steps table |
| Markdown content | Text blobs (stored directly, not externalized) |
| Navigation structure | Step ordering and relationships |
| Validation rules | Per-step validation configuration |

### 2. Compiled Step Artifacts

For each step:

| Artifact | Format |
|---|---|
| Kubernetes manifest bundle | Serialized YAML/JSON blob |
| File state archive | tar blob (or similar) |
| Educational state snapshot | Serialized metadata |

Each step is **fully self-contained**. Runtime never computes diffs between steps.

### 3. Runtime State

| Data | Purpose |
|---|---|
| Current active step | Track student position |
| Student progress markers | Track completion |
| Snapshot history | Resume support |
| Per-student workspace state | Optional student-specific data |

Runtime snapshots are **logical** (educational state), not infrastructure diffs.

## Schema

TODO: Define the concrete SQLite schema (tables, columns, types, indexes).

## Size Expectations

TODO: Provide size estimates for typical workshops. Consider: number of steps, manifest sizes, file archive sizes, markdown content sizes.

## Distribution

TODO: Define how SQLite artifacts are distributed — direct download? Registry? Git LFS? Container image embedding?

## YAML Export/Import

Some users prefer Git-based declarative workflows. The platform supports:

- **Export:** Workshop DB → YAML files (for Git version control)
- **Import:** YAML files → rebuild DB
- **Decompile:** Compiled snapshots → human-readable files

However, the **production runtime artifact is always SQLite**. Git workflows are tooling conveniences, not the primary distribution format.

TODO: Define the YAML export format and directory structure.

## Integrity

TODO: Define integrity verification — checksums? Signatures? Schema version validation on load?

## Migration

TODO: Define how schema migrations work when the platform evolves (new fields, new tables, format changes).
