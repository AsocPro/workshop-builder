# Flat File Artifact — In-Image Workshop Metadata

*This document replaces the previous SQLite artifact design. Workshop metadata is now baked into the container image as flat files — there is no separate distribution artifact.*

## Purpose

Workshop metadata — step definitions, tutorial content, goss specs, LLM configuration — is stored as flat files in a read-only `/workshop/` directory baked into every step image at build time. Runtime state — command logs, state events, recordings — is written to an ephemeral `/workshop/runtime/` directory during the student session.

There is no SQLite database, no separate distribution artifact, and no external configuration. The container image IS the workshop.

## Why Flat Files

- **No separate artifact to manage** — the image is the complete package
- **`docker run` just works** — no CLI, no config mount, no database delivery
- **Human-inspectable** — inspect with `cat`, `ls`, standard tooling
- **OCI layer efficiency** — metadata directory is one layer shared across all step images
- **Simple backend** — read files from disk, no database driver or query layer
- **Non-linear navigation** — all steps' metadata available in every image

## Why JSON (Not YAML)

The author writes YAML (`workshop.yaml`, per-step `step.yaml`). The build pipeline compiles metadata into JSON for the `/workshop/` directory. This is intentional:

- **Zero runtime dependencies** — the backend reads metadata with Go's `encoding/json` (stdlib). No YAML parser needed in the backend binary.
- **Unambiguous types** — JSON has explicit types, avoiding YAML's implicit type coercion.
- **Clear boundary** — different format from the author-facing YAML signals "compiled output, don't hand-edit."
- **`goss.yaml` stays YAML** — goss expects YAML and consumes it directly. The backend never parses goss specs — it shells out to `goss validate` and reads JSON results from stdout. The mixed formats in `/workshop/` are fine because they have different consumers.

## Filesystem Layout

### Build-Time Metadata (`/workshop/` — read-only)

```
/workshop/
  ├── workshop.json                     # identity, navigation mode, step list
  ├── prompts/                          # LLM system prompt overrides (optional)
  │   ├── hints.md
  │   ├── explain.md
  │   └── solve.md
  ├── steps/
  │   ├── step-pods/
  │   │   ├── meta.json                 # title, position, group, requires
  │   │   ├── content.md                # tutorial markdown
  │   │   ├── goss.yaml                 # validation spec (optional)
  │   │   ├── hints.md                  # static hints content (optional)
  │   │   ├── explain.md                # static explanation content (optional)
  │   │   ├── solve.md                  # static solution content (optional)
  │   │   ├── llm.json                  # LLM config (optional)
  │   │   └── llm-docs/                 # reference docs for LLM context (optional)
  │   │       ├── kubectl-cheatsheet.md
  │   │       └── ...
  │   ├── step-services/
  │   │   ├── meta.json
  │   │   ├── content.md
  │   │   └── goss.yaml
  │   └── ...
```

### Runtime Data (`/workshop/runtime/` — ephemeral)

```
/workshop/runtime/                      # created at runtime, ephemeral
  ├── command-log.jsonl                 # every command + timestamp + exit code
  ├── state-events.jsonl                # state transitions (append-only event log)
  ├── session-<timestamp>.cast          # asciinema recording per connection (asciicast v2)
  └── llm-history.jsonl                 # LLM interactions
```

The `/workshop/runtime/` directory is created by the backend on first startup. Its contents are ephemeral — they exist only for the lifetime of the container (or the shared volume in Kubernetes mode where a [Vector sidecar](./aggregation.md) ships them to Postgres).

## Schema: workshop.json

The top-level workshop identity and step manifest.

```json
{
  "name": "explore-kubernetes",
  "image": "myorg/explore-kubernetes",
  "navigation": "free",
  "steps": [
    {
      "id": "step-pods",
      "title": "Working with Pods",
      "group": "basics",
      "position": 0
    },
    {
      "id": "step-services",
      "title": "Services & Networking",
      "group": "basics",
      "position": 1
    },
    {
      "id": "step-rbac",
      "title": "RBAC",
      "group": "security",
      "requires": ["step-pods"],
      "position": 2
    }
  ]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Workshop identifier |
| `image` | string | Yes | Image name used for tag generation |
| `navigation` | string | Yes | `linear`, `free`, or `guided` |
| `steps` | array | Yes | Ordered list of step references |
| `steps[].id` | string | Yes | Step identifier (matches directory name under `steps/`) |
| `steps[].title` | string | Yes | Display title |
| `steps[].group` | string | No | Group name for guided navigation |
| `steps[].requires` | array | No | Prerequisite step IDs |
| `steps[].position` | number | Yes | Display order (0-indexed) |

LLM provider configuration (provider, model, API key, max tokens, default mode) is not baked into the image — it is an operator concern configured in the [WorkspaceTemplate CRD](../platform/crds.md) and injected at runtime.

## Schema: meta.json

Per-step metadata at `/workshop/steps/<id>/meta.json`.

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

| Field | Type | Description |
|---|---|---|
| `id` | string | Step identifier |
| `title` | string | Display title |
| `group` | string | Group for guided navigation (omitted if none) |
| `position` | number | Display order |
| `requires` | array | Prerequisite step IDs (omitted if none) |
| `hasGoss` | boolean | Whether `/workshop/steps/<id>/goss.yaml` exists |
| `hasLlm` | boolean | Whether `/workshop/steps/<id>/llm.json` exists |
| `hasHints` | boolean | Whether `/workshop/steps/<id>/hints.md` exists |
| `hasExplain` | boolean | Whether `/workshop/steps/<id>/explain.md` exists |
| `hasSolve` | boolean | Whether `/workshop/steps/<id>/solve.md` exists |

## Schema: content.md

Tutorial markdown at `/workshop/steps/<id>/content.md`. Copied directly from the step's `content.md` source file at build time. Rendered by the frontend.

## Schema: hints.md / explain.md / solve.md

Static help content at `/workshop/steps/<id>/hints.md`, `explain.md`, and `solve.md`. Copied directly from the step's source files at build time. Each is optional — presence determines which help modes are available for the step. Rendered by the frontend when the student clicks the corresponding help button. See [Help System](../platform/llm-help.md) for the full behavior matrix.

## Schema: goss.yaml

Goss validation spec at `/workshop/steps/<id>/goss.yaml`. Copied directly from the step's `goss.yaml` source file at build time. Present only for steps that have a `goss.yaml` in their step directory.

## Schema: llm.json

Per-step LLM configuration at `/workshop/steps/<id>/llm.json`.

```json
{
  "context": "Common mistake: students forget the -n namespace flag.",
  "hasDocs": true
}
```

| Field | Type | Description |
|---|---|---|
| `context` | string | Instructor-provided context for LLM prompts |
| `hasDocs` | boolean | Whether `llm-docs/` directory exists with reference files |

## Runtime Files

### command-log.jsonl

Append-only NDJSON file written by the [shell instrumentation](../platform/instrumentation.md) (`PROMPT_COMMAND` hook). One line per command executed in the terminal.

```jsonl
{"ts":"2025-03-15T14:22:01.123Z","cmd":"kubectl get pods","exit":0}
{"ts":"2025-03-15T14:22:15.456Z","cmd":"kubectl apply -f deployment.yaml","exit":1}
{"ts":"2025-03-15T14:22:30.789Z","cmd":"kubectl apply -f /workspace/deployment.yaml","exit":0}
```

| Field | Type | Description |
|---|---|---|
| `ts` | string | ISO 8601 UTC timestamp |
| `cmd` | string | Command text (truncated to 1024 chars) |
| `exit` | number | Exit code |

### state-events.jsonl

Append-only NDJSON file written by the backend on state transitions. State is maintained in-memory — this file is **not** replayed on startup. It exists so that in K8s mode, Vector can ship it to Postgres for instructor visibility and analytics.

```jsonl
{"ts":"2025-03-15T14:20:00.000Z","event":"connected"}
{"ts":"2025-03-15T14:20:01.000Z","event":"step_start","step":"step-pods"}
{"ts":"2025-03-15T14:25:00.000Z","event":"goss_result","step":"step-pods","passed":false,"checks":{"total":5,"passed":2}}
{"ts":"2025-03-15T14:28:00.000Z","event":"goss_result","step":"step-pods","passed":true,"checks":{"total":5,"passed":5}}
{"ts":"2025-03-15T14:28:01.000Z","event":"step_start","step":"step-services"}
{"ts":"2025-03-15T14:45:00.000Z","event":"disconnected"}
```

Event types:

| Event | Fields | Description |
|---|---|---|
| `connected` | — | Browser WebSocket connected |
| `disconnected` | — | Browser WebSocket disconnected |
| `goss_result` | `step`, `passed`, `checks` | Validation result (student-triggered) |

### session-&lt;timestamp&gt;.cast

Asciinema recordings in [asciicast v2 format](https://docs.asciinema.org/manual/asciicast/v2/). One file per connection, named with the ISO 8601 compact start time (e.g. `session-20250315T142000Z.cast`). Written by `asciinema rec` wrapping the terminal shell. See [Instrumentation](../platform/instrumentation.md) for details.

### llm-history.jsonl

Append-only NDJSON file recording LLM interactions. See [LLM Help](../platform/llm-help.md) for schema details.

## State Management

There is no `state.json` file. State is maintained in-memory by the backend and always starts fresh — there is no startup replay. Events are appended to `state-events.jsonl` as they occur; in K8s mode, Vector ships that file to Postgres for aggregation and instructor visibility.

**Future consideration:** Flat file state restore — exporting runtime files from a workspace and mounting them into a fresh container to resume where a student left off. Not needed for v1.

## Distribution

There is no separate distribution artifact. A workshop is fully portable with:

1. Access to the container registry where images are pushed
2. That's it.

```bash
# Run a workshop — no CLI, no config, no database
docker run -p 8080:8080 myorg/kubernetes-101:step-1-intro

# With LLM help enabled
docker run -p 8080:8080 -e WORKSHOP_LLM_API_KEY=sk-... myorg/kubernetes-101:step-1-intro
```

## Size Expectations

The `/workshop/` metadata directory is typically under 1MB for a workshop with 10 steps:

| Component | Approximate Size |
|---|---|
| `workshop.json` | < 2 KB |
| `prompts/*.md` | < 5 KB each |
| Per-step `meta.json` | < 1 KB each |
| Per-step `content.md` | 1–20 KB each |
| Per-step `goss.yaml` | < 5 KB each |
| Per-step `hints.md` / `explain.md` / `solve.md` | 1–10 KB each |
| Per-step `llm.json` | < 1 KB each |
| Per-step `llm-docs/` | 1–50 KB each |

The metadata is baked into one OCI layer and shared across all step images via layer deduplication.

## Migration from SQLite

The previous architecture used a SQLite database as a separate distribution artifact. The flat file approach replaces it entirely:

| SQLite Concept | Flat File Replacement |
|---|---|
| `workshop` table | `workshop.json` |
| `steps` table | `steps/<id>/meta.json` + `content.md` + `goss.yaml` |
| `step_metadata` table | `steps/<id>/meta.json` fields |
| `navigation` table | `workshop.json` `steps` array with `group`/`requires` |
| `runtime_state` table | In-memory state + `state-events.jsonl` (append-only event log) |
| `custom_state` table | Removed — not needed |
| Distribution file | Eliminated — metadata baked into image |
| Per-instance working copy | Eliminated — no database to copy |
