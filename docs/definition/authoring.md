# Authoring — CLI Proxy Model

## Purpose

Provide instructors with a natural, incremental workflow for building workshops. Authoring produces a `step-spec.yaml` that records *intent* — the files, environment variables, and commands that define each step — rather than capturing live infrastructure state.

## Environment

- Uses a **local container** (Docker or Podman)
- The `workshop build proxy` command wraps the container shell
- The proxy observes changes and writes them to `step-spec.yaml`
- No live Kubernetes cluster required for authoring

## Authoring Workflow

```
1. Run: workshop build proxy
   → Container launched from base image (or last step image)

2. Work inside the container:
   → Create/edit files
   → Set environment variables
   → Run commands

3. CLI proxy observes and records to step-spec.yaml:
   → File diffs → files: entries
   → Env changes → env: entries
   → Commands run → commands: entries

4. Run: workshop build step save
   → Current step finalized in step-spec.yaml
   → Proxy opens next step container (built on top of current)

5. Continue for all steps

6. Run: workshop build compile
   → Dagger builds one OCI image per step
   → Images pushed to registry
   → SQLite updated with image tags
```

## Key Properties

- **Records intent, not observed state.** The proxy records what the author *did* — files written, commands run — not a snapshot of the full filesystem. This keeps `step-spec.yaml` human-readable and version-controllable.
- **Incremental.** Each step's proxy session builds on top of the previous step's image. The instructor works forward naturally.
- **No live cluster required.** Authoring is purely local container work. Kubernetes is not involved.
- **Version-controllable.** `step-spec.yaml` and local source files are the complete authoring artifact. Commit them to Git like any other source code.

## What the Proxy Records

| Author Action | Recorded In step-spec.yaml |
|---|---|
| Create or edit a file | `files:` entry with `path` and `content` (or `source` if from local disk) |
| Set an environment variable | `env:` key/value entry |
| Run a shell command | `commands:` list entry |
| Delete a file | `files:` entry with `content: ""` (empty file) or explicit deletion command |

The proxy does not record platform internals, system files, or ephemeral process state — only changes the author makes explicitly.

## Step Editing

To edit a step after it has been finalized:

1. Edit `step-spec.yaml` directly — add, remove, or change `files`, `env`, or `commands` entries
2. Run `workshop build compile --from-step <id>` to rebuild from the edited step forward

Steps are YAML text. No special tooling is needed to edit them.

## Step Reordering

To reorder steps, edit the `steps:` list in `step-spec.yaml` and reorder the entries. Run `workshop build compile` to rebuild all affected steps.

## Version Control Integration

`step-spec.yaml` is the version-controllable source of truth for a workshop. Commit it to Git along with any local source files referenced by `files[].source` entries.

Unlike the previous snapshot-based model, there is no binary authoring state to manage. The full workshop definition is human-readable text. Branching, diffing, pull requests, and code review work naturally.

## Collaboration

Multiple instructors can collaborate by editing `step-spec.yaml` in a shared Git repository. The normal Git workflow applies: branch, commit, review, merge.

TODO: Define whether simultaneous proxy sessions from multiple authors are supported — i.e., can two authors build proxy sessions concurrently against different steps?
