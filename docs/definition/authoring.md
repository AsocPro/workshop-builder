# Authoring — CLI Proxy Model

## Purpose

Provide instructors with a natural, incremental workflow for building workshops. Authoring produces a `workshop.yaml` that records *intent* — the files, environment variables, and commands that define each step — rather than capturing live infrastructure state.

## Environment

- Uses a **local container** (Docker or Podman)
- The `workshop build proxy` command wraps the container shell
- The proxy observes changes and writes them to `workshop.yaml`
- No live Kubernetes cluster required for authoring

## Authoring Workflow

```
1. Run: workshop build proxy
   → Container launched from base image (or last step image)

2. Work inside the container:
   → Create/edit files
   → Set environment variables
   → Run commands

3. CLI proxy observes and records to workshop.yaml:
   → File diffs → files: entries
   → Env changes → env: entries
   → Commands run → commands: entries

4. Run: workshop build step save
   → Current step finalized in workshop.yaml
   → Proxy opens next step container (built on top of current)

5. Continue for all steps

6. Run: workshop build compile
   → Dagger builds one OCI image per step
   → Each image contains all metadata as flat files under /workshop/
   → Images pushed to registry
```

## Key Properties

- **Records intent, not observed state.** The proxy records what the author *did* — files written, commands run — not a snapshot of the full filesystem. This keeps `workshop.yaml` human-readable and version-controllable.
- **Incremental.** Each step's proxy session builds on top of the previous step's image. The instructor works forward naturally.
- **No live cluster required.** Authoring is purely local container work. Kubernetes is not involved.
- **Version-controllable.** `workshop.yaml` and local source files are the complete authoring artifact. Commit them to Git like any other source code.

## What the Proxy Records

| Author Action | Recorded In workshop.yaml |
|---|---|
| Create or edit a file | `files:` entry with `path` and `content` (or `source` if from local disk) |
| Set an environment variable | `env:` key/value entry |
| Run a shell command | `commands:` list entry |
| Delete a file | `commands:` entry: `rm /path/to/file` |

File deletions must be represented as explicit `rm` commands, not as empty `files:` entries. An empty `files:` entry creates an empty file — it does not remove the file from the image layer.

The proxy does not record platform internals, system files, or ephemeral process state — only changes the author makes explicitly.

## Step Editing

To edit a step after it has been finalized:

1. Edit `workshop.yaml` directly — add, remove, or change `files`, `env`, or `commands` entries
2. Run `workshop build compile --from-step <id>` to rebuild from the edited step forward

Steps are YAML text. No special tooling is needed to edit them.

## Step Reordering

To reorder steps, edit the `steps:` list in `workshop.yaml` and reorder the entries. Run `workshop build compile` to rebuild all affected steps.

## Version Control Integration

`workshop.yaml` is the version-controllable source of truth for a workshop. Commit it to Git along with any local source files referenced by `files[].source` entries.

Unlike the previous snapshot-based model, there is no binary authoring state to manage. The full workshop definition is human-readable text. Branching, diffing, pull requests, and code review work naturally.

## Collaboration

Multiple instructors can collaborate by editing `workshop.yaml` in a shared Git repository. The normal Git workflow applies: branch, commit, review, merge.

**Builds are local-only**: Each author builds locally with `workshop build compile`. There is no shared build coordination required — all builds happen on individual machines.
