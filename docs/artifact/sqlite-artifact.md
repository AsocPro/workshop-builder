# Flat File Artifact — In-Image Workshop Metadata

*This document replaces the previous SQLite artifact design. Workshop metadata is now baked into the container image as flat files — there is no separate distribution artifact.*

## Purpose

Workshop metadata — step definitions, tutorial content, goss specs, LLM configuration — is stored as flat files in a read-only `/workshop/` directory baked into every step image at build time. Runtime state — command logs, state events, recordings — is written to an ephemeral `/workshop/runtime/` directory during the student session.

There is no SQLite database, no separate distribution artifact, and no external configuration. The container image IS the workshop.

## Why Flat Files

- **No separate artifact to manage** — the image is the complete package
- **`docker run` just works** — no CLI, no config mount, no database delivery
- **Human-readable** — inspect with `cat`, `ls`, standard tooling
- **OCI layer efficiency** — metadata directory is one layer shared across all step images
- **Simple backend** — read files from disk, no database driver or query layer
- **Non-linear navigation** — all steps' metadata available in every image

## Filesystem Layout

### Build-Time Metadata (`/workshop/` — read-only)

```
/workshop/
  ├── workshop.json                     # identity, navigation mode, step list, LLM config
  ├── steps/
  │   ├── step-pods/
  │   │   ├── meta.json                 # title, position, group, requires
  │   │   ├── content.md                # tutorial markdown
  │   │   ├── goss.yaml                 # validation spec (optional)
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
  ├── state-events.jsonl                # state transitions (IS the state — replayed on startup)
  ├── session.cast                      # asciinema recording (asciicast v2)
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
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "apiKeyEnv": "WORKSHOP_LLM_API_KEY",
    "maxTokens": 1024,
    "defaultMode": "hints"
  },
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
| `llm` | object | No | Workshop-level LLM configuration (omitted if LLM not configured) |
| `llm.provider` | string | Yes* | LLM provider |
| `llm.model` | string | Yes* | Model identifier |
| `llm.apiKeyEnv` | string | Yes* | Env var name for API key |
| `llm.maxTokens` | number | No | Max response tokens |
| `llm.defaultMode` | string | No | Default help mode |
| `steps` | array | Yes | Ordered list of step references |
| `steps[].id` | string | Yes | Step identifier (matches directory name under `steps/`) |
| `steps[].title` | string | Yes | Display title |
| `steps[].group` | string | No | Group name for guided navigation |
| `steps[].requires` | array | No | Prerequisite step IDs |
| `steps[].position` | number | Yes | Display order (0-indexed) |

*Required when `llm` object is present.

## Schema: meta.json

Per-step metadata at `/workshop/steps/<id>/meta.json`.

```json
{
  "id": "step-pods",
  "title": "Working with Pods",
  "group": "basics",
  "position": 0,
  "hasGoss": true,
  "hasLlm": true
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

## Schema: content.md

Tutorial markdown at `/workshop/steps/<id>/content.md`. Raw markdown content — resolved from either the `markdown` or `markdownFile` field in `workshop.yaml` at build time. Rendered by the frontend.

## Schema: goss.yaml

Goss validation spec at `/workshop/steps/<id>/goss.yaml`. Raw goss YAML — resolved from either the `goss` or `gossFile` field in `workshop.yaml`. Present only for steps that have validation.

## Schema: llm.json

Per-step LLM configuration at `/workshop/steps/<id>/llm.json`.

```json
{
  "mode": "hints",
  "context": "Common mistake: students forget the -n namespace flag.",
  "hasDocs": true
}
```

| Field | Type | Description |
|---|---|---|
| `mode` | string | Help mode: `hints`, `explain`, or `solve` |
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

Append-only NDJSON file written by the backend on state transitions. This file IS the state — the backend replays it on startup to reconstruct current progress.

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
| `step_start` | `step` | Student navigated to a step |
| `goss_result` | `step`, `passed`, `checks` | Validation result (student-triggered or periodic) |

State reconstruction on startup:
- Last `step_start` event → active step
- All `goss_result` events where `passed: true` → completed set
- Last `connected`/`disconnected` → connection state

### session.cast

Asciinema recording in [asciicast v2 format](https://docs.asciinema.org/manual/asciicast/v2/). Written by `asciinema rec` wrapping the terminal shell. See [Instrumentation](../platform/instrumentation.md) for details.

### llm-history.jsonl

Append-only NDJSON file recording LLM interactions. See [LLM Help](../platform/llm-help.md) for schema details.

## State Derivation (No Separate State File)

There is no `state.json` file. The backend derives current state entirely from `state-events.jsonl`:

1. On startup, read `state-events.jsonl` line by line
2. Replay events to reconstruct: active step, completed steps, connection state
3. Continue appending new events during the session

This event-sourcing approach means:
- No state corruption from partial writes
- Full audit trail of every state change
- Simple recovery — just replay the file
- In K8s mode, the same file is shipped to Postgres by Vector for aggregation

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
| Per-step `meta.json` | < 1 KB each |
| Per-step `content.md` | 1–20 KB each |
| Per-step `goss.yaml` | < 5 KB each |
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
| `runtime_state` table | `state-events.jsonl` (event-sourced) |
| `custom_state` table | Removed (not needed — state is event-sourced) |
| Distribution file | Eliminated — metadata baked into image |
| Per-instance working copy | Eliminated — no database to copy |
