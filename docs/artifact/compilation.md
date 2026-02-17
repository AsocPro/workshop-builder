# Compilation Layer

## Purpose

Transform incremental [authoring snapshots](../definition/authoring.md) into deterministic, self-contained runtime artifacts. This is conceptually similar to container image flattening after layered builds.

## Input

- Authoring snapshots (one per step)
- Markdown content
- Workshop metadata

## Output (Per Step)

Each compiled step is **fully self-contained** and includes:

| Artifact | Description |
|---|---|
| Kubernetes manifest bundle | Full desired state, normalized |
| File state archive | Complete file/PVC contents (e.g., tar blob) |
| Educational state snapshot | Step metadata, validation rules, markdown |

## Key Properties

- **No diffs.** Each step contains complete state.
- **No patch chains.** No step depends on a previous step's output.
- **No mutation replay.** Runtime never needs to understand what changed between steps.
- **Deterministic.** Applying a compiled step always produces the same result.

## Manifest Normalization

During compilation, Kubernetes manifests are normalized:

- Strip `status` fields
- Strip generated fields (`resourceVersion`, `uid`, `creationTimestamp`, etc.)
- Strip cluster-specific annotations
- Retain only the desired state

This ensures manifests are portable and reapplicable.

## Flattening Process

```
Authoring snapshot for Step 3
  (which accumulated state from Steps 1 + 2 + 3)
                  |
             Compilation
                  |
         Compiled Step 3 artifact
         (complete, standalone state)
```

The compilation process captures the **total accumulated state** at each step point and packages it as a standalone artifact. The incremental authoring history is discarded.

## Output Destination

Compiled artifacts are stored in the [SQLite Artifact](./sqlite-artifact.md).

TODO: Define the serialization format for each artifact type (manifest bundle format, archive format, educational snapshot format).

## Validation During Compilation

TODO: Define what validation occurs during compilation — manifest validity, file completeness, step ordering, etc.

## Recompilation

TODO: Define when and how recompilation is triggered — manual only? Automatic on step save? Incremental recompilation of changed steps only?

## Size Considerations

Because each step stores complete state (not diffs), storage grows linearly with steps and state size. This is an accepted tradeoff:

> Storage cost is acceptable; operational complexity is not.

TODO: Provide rough size estimates for typical workshops (e.g., 10 steps, 5 services, moderate file state).
