# Workspace Backend Service

## Purpose

A Go binary embedded in every workshop container image. It is the runtime engine of each workspace — serving the student web UI, proxying terminal access, managing asciinema recording, tracking student progress, and mediating LLM help interactions.

## Role in the System

The backend binary is pre-installed in [base images](./base-images.md). When using custom base images, the author must install it manually — see [Custom Base Image Requirements](./base-images.md#custom-base-image-requirements). It is platform infrastructure, not workshop content — authors do not configure it directly.

The backend is PID 2 in the workspace container, launched by tini:

```
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

tini (as PID 1) handles zombie process reaping and clean signal forwarding. The backend owns its child processes (ttyd wrapped in asciinema).

## Container Startup Sequence

```
tini (PID 1)
  └── workshop-backend (PID 2)
        ├── Read /workshop/workshop.json + /workshop/steps/*  (flat file metadata)
        ├── Read WORKSHOP_MANAGEMENT_URL env var (optional — link shown in UI if set)
        ├── Create /workshop/runtime/ directory
        ├── Initialize in-memory state (fresh — no replay)
        ├── Spawn ttyd → asciinema rec → /bin/bash  (terminal + recording)
        ├── Start HTTP server (web UI + student API)
        ├── Start file watcher on command-log.jsonl
        └── Supervise ttyd/asciinema (restart on exit)
```

## Responsibilities

### Flat File Metadata (Read-Only)

On startup, the backend reads the baked-in workshop metadata:

1. Parse `/workshop/workshop.json` — workshop identity, navigation mode, step list, LLM config
2. Index `/workshop/steps/*/meta.json` — build step registry with titles, groups, prerequisites
3. Verify step content files exist (`content.md`, optional `goss.yaml`, optional `llm.json`)

No database, no schema migration, no working copy. The files are baked into the image and read directly.

### State Event Log

The backend maintains in-memory state and appends events to `/workshop/runtime/state-events.jsonl` as they occur. State is **not** replayed on startup — the backend always starts fresh.

Events written:
- `goss_result` — validation executed (student-triggered)
- `connected` / `disconnected` — WebSocket connect/disconnect

The event log exists so that in K8s mode, Vector can ship it to Postgres for instructor visibility and analytics. In Docker mode the file accumulates locally but is not read back.

### Terminal Access + Asciinema Recording

The backend spawns ttyd wrapping the shell in asciinema:

```
ttyd <options> -- asciinema rec --stdin /workshop/runtime/session-<timestamp>.cast -c /bin/bash
```

- `--stdin` captures input for full replay fidelity
- Each new connection gets a fresh file named with the ISO 8601 start time (e.g. `session-20250315T142000Z.cast`) — no appending, no timestamp discontinuities
- The resulting files are in [asciicast v2 format](https://docs.asciinema.org/manual/asciicast/v2/)

The backend:
- Monitors ttyd; restarts it if it exits unexpectedly
- On each new connection, generates the timestamp filename before spawning ttyd/asciinema
- Proxies all browser WebSocket connections to ttyd through a single origin — no CORS issues
- Serves session cast files with HTTP Range support for player seeking
- No nsenter or shared process namespace required — ttyd runs inside the same container

### Command Log Watching

The shell's `PROMPT_COMMAND` hook (from `/etc/workshop-platform.bashrc`) writes commands to `/workshop/runtime/command-log.jsonl`. The backend watches this file using fsnotify (or periodic tail-read) and:

- Maintains an in-memory buffer of recent commands for LLM context assembly
- Serves command history via `GET /api/commands` for display in the student UI

See [Instrumentation](./instrumentation.md) for the shell hook implementation.

### Web UI Serving

- Serves the student-facing web application (HTML, JS, CSS) as embedded static assets
- The student's browser connects only to this backend — there is no separate frontend server

### Goss Validation

When a student clicks Validate:

1. Read the goss spec from `/workshop/steps/<id>/goss.yaml`
2. Execute `goss validate -g /workshop/steps/<id>/goss.yaml --format json`
3. Return per-test pass/fail results to the frontend
4. Append a `goss_result` event to `state-events.jsonl`

Because every step image contains ALL steps' goss specs, the backend can validate any step regardless of which step image the container was built from. This enables non-linear navigation.

### Non-Linear Navigation

The backend enforces navigation rules based on `workshop.json`:

| Mode | Behavior |
|---|---|
| `linear` | Only next/prev allowed. Steps must be completed in order. |
| `free` | Any step accessible at any time. |
| `guided` | Free within unlocked groups. Groups unlock when previous group is completed or via `requires`. |

Progress is tracked as a **completion set** — the set of step IDs where goss validation has passed. Goss results are the authoritative progress signal: a passing result adds the step to the completion set and unlocks the next step (in linear/guided modes). The frontend shows a completion matrix rather than a linear progress bar.

Viewing step content (fetching markdown, checking the goss spec) does NOT require an image swap. Every image contains all steps' metadata. An image swap only occurs when the student explicitly requests to switch their working environment. Step transitions are driven externally: by the CLI in single-user mode, and by the Operator in cluster mode. The backend has no API for initiating its own replacement.

### LLM Help

When the student clicks the Help button:

1. Assemble context: recent commands from `command-log.jsonl`, latest goss results, step markdown, instructor-provided context from `llm.json`, reference docs from `llm-docs/`
2. Send to LLM provider (Anthropic Messages API) with streaming
3. Stream response back to the student
4. Append interaction to `/workshop/runtime/llm-history.jsonl`

See [LLM Help](./llm-help.md) for full details.

### Connection Tracking

The backend instruments the WebSocket proxy for terminal connections:

- On WebSocket connect: append `{"event": "connected"}` to `state-events.jsonl`
- On WebSocket disconnect: append `{"event": "disconnected"}` to `state-events.jsonl`

## API Surface

### Student API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/steps` | List all steps with titles, groups, completion status, accessibility |
| `GET` | `/api/steps/:id/content` | Get step tutorial markdown |
| `POST` | `/api/steps/:id/validate` | Run goss validation, return results |
| `GET` | `/api/state` | Current state: active step, completed set, navigation mode |
| `GET` | `/api/commands` | Recent command history (with pagination) |
| `GET` | `/api/recordings` | List session recording files with start timestamps |
| `GET` | `/api/recordings/:filename` | Serve a session cast file with HTTP Range support |
| `POST` | `/api/llm/help` | Request LLM help (streaming response) |
| `GET` | `/api/llm/history` | Get LLM interaction history for current step |

### Static Assets

| Path | Description |
|---|---|
| `/` | Student web UI (embedded SPA) |
| `/ws/terminal` | WebSocket proxy to ttyd |

## How the Binary Gets into Images

### Base Images (Preferred)

[Base images](./base-images.md) (`workshop-base:{alpine,ubuntu,centos}`) include the backend binary pre-installed:

```
workshop-base:ubuntu
  ├── /sbin/tini
  ├── /usr/local/bin/workshop-backend  (embedded web UI assets)
  ├── /usr/local/bin/goss
  ├── /usr/bin/asciinema
  ├── /etc/workshop-platform.bashrc
  └── ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

Authors `FROM workshop-base:ubuntu` and everything is ready.

### Custom Base Images

When authors use a custom `base.image` or `base.containerFile`, they must install the backend binary and other platform components manually. See [Custom Base Image Requirements](./base-images.md#custom-base-image-requirements) for the full list.

## What It Does NOT Do

- Does not parse `workshop.yaml` (build-time concern only)
- Does not manage other workspaces — scoped entirely to the container it runs in
- Does not interact with the Kubernetes API
- Does not pull images or manage Deployments (that is the operator's job)
- Does not write to any database (state is file-based)
- Does not know whether a Vector sidecar exists (it just writes local files)

## Step Transitions

### Cluster Mode

Step transitions are driven by the [Operator](./operator.md) via Deployment image swap. The old pod is replaced by a new pod running the next step's image. The new backend starts fresh, reads flat files, and begins serving.

The management server or CLI notifies the student when the new container is ready. The student reloads their browser manually — no auto-reconnect logic is required.

### Local Mode

Step transitions are driven by the [CLI](./cli.md): stop current container, start new container from next step image. The backend behavior is identical to cluster mode — it always starts fresh.

The backend cannot initiate its own container replacement. The CLI runs a local management server on the host and passes its URL via `WORKSHOP_MANAGEMENT_URL`. The backend renders this as a link in the student UI. The management server survives container replacements and handles all lifecycle operations. See [CLI — Local Management Server](./cli.md#local-management-server).

### State Persistence Across Transitions

The `/workshop/runtime/` directory is ephemeral to the container. Each new container always starts with fresh in-memory state — there is no save/restore mechanism. The student resumes from the step they were on; goss validation re-establishes their completion status when they re-validate. In K8s mode, the Vector sidecar has already shipped events to Postgres before the transition for instructor visibility.

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Base Images](./base-images.md) | Backend binary pre-installed in platform base images |
| [Dagger Pipeline](../artifact/compilation.md) | Injects backend binary when using custom base images; bakes flat file metadata |
| [Flat File Artifact](../artifact/flat-file-artifact.md) | `/workshop/` directory is the read-only metadata source |
| [Instrumentation](./instrumentation.md) | Shell bashrc writes command log; asciinema records terminal |
| [LLM Help](./llm-help.md) | Backend handles LLM API calls and context assembly |
| [Instructor Dashboard](./instructor-dashboard.md) | K8s-only; Vector ships JSONL files from this container to the dashboard service |
| [Aggregation](./aggregation.md) | Vector sidecar ships JSONL files to Postgres/S3 |
| [Operator](./operator.md) | Provisions the workspace pod; backend runs inside it |
| [CLI](./cli.md) | In local mode, manages the container lifecycle |
| [Frontend / Student UI](../presentation/frontend.md) | Served by this backend; communicates via HTTP and WebSocket |
