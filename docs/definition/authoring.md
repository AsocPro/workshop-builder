# Authoring — Builder Mode

## Purpose

Provide instructors with a natural, incremental workflow for building workshops. This is the "messy" side of the system — mutation, iteration, and experimentation happen here so that the [compiled artifacts](../artifact/compilation.md) and runtime can remain clean and deterministic.

## Environment

- Uses a **live Kubernetes namespace**
- Uses **real PVCs and file systems**
- Allows direct `kubectl` and file manipulation
- Supports step-based snapshot capture
- State is mutable and layered

## Authoring Workflow

```
1. Instructor begins Step 1
2. Mutates cluster state and files
3. Clicks "Save Step" → authoring snapshot captured
4. Continues to Step 2
5. Mutates further
6. Saves again → snapshot captured
7. Repeat for all steps
```

Each step snapshot implicitly inherits all previous state during authoring. The instructor works incrementally — they don't need to rebuild from scratch for each step.

## Key Properties

- **Incremental:** Each step builds on the previous one
- **Mutable:** The instructor can freely modify the environment
- **Live:** Real cluster, real files, real tools
- **Non-deterministic:** The authoring environment carries forward all accumulated state and side effects

These properties are intentional. Authoring complexity is absorbed here so that [compilation](../artifact/compilation.md) can flatten it into clean runtime artifacts.

## Authoring Snapshots

When the instructor saves a step, the system captures:

- Current Kubernetes manifest state in the namespace
- Current file/PVC contents
- Educational metadata (step title, markdown content, validation rules)

These snapshots are:

- **Internal to builder mode** — never exposed to students
- **Incremental and layered** — each builds on previous state
- **Input to compilation** — consumed by the [Compilation Layer](../artifact/compilation.md)
- **Not included in the final SQLite artifact** — only compiled output ships

TODO: Define exactly what is captured during a snapshot — full namespace dump? Specific resource types? How are system-level resources excluded?

TODO: Define how the instructor specifies which resources and files belong to a step vs platform internals.

TODO: Are authoring snapshots preserved for re-editing after compilation, or are they discarded? If preserved, where are they stored?

## Step Editing

TODO: Define how instructors edit or re-order existing steps. Can they insert a step between existing steps? Delete a step? What happens to downstream steps?

## Collaboration

TODO: Define whether multiple instructors can collaborate on authoring simultaneously.

## Version Control Integration

Authoring snapshots can optionally be exported to a Git-compatible format. See [SQLite Artifact](../artifact/sqlite-artifact.md) for the YAML export/import workflow.
