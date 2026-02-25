# Workspace Backend Service

## Purpose

A Go binary embedded in every workshop container image. It is the runtime engine of each workspace — serving the student web UI, proxying terminal access, managing asciinema recording, tracking student progress via event-sourced state, providing instructor monitoring APIs, and mediating LLM help interactions.

## Role in the System

The backend binary is pre-installed in [base images](./base-images.md) or injected by the [Dagger compilation pipeline](../artifact/compilation.md) when using custom base images. It is platform infrastructure, not workshop content — authors do not configure it directly.

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
        ├── Create /workshop/runtime/ directory
        ├── Replay /workshop/runtime/state-events.jsonl → reconstruct state
        ├── Spawn ttyd → asciinema rec → /bin/bash  (terminal + recording)
        ├── Start HTTP server (web UI + student API + instructor API)
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

### State Derivation from Event Log

State is derived entirely from `/workshop/runtime/state-events.jsonl`. On startup:

1. If the file exists, replay events line by line
2. Last `step_start` event → current active step (if none, default to first step)
3. All `goss_result` events where `passed: true` → completed step set
4. Last `connected`/`disconnected` → connection state

During the session, every state change appends a new event:
- `step_start` — student navigates to a step
- `goss_result` — validation executed (student-triggered or periodic)
- `connected` / `disconnected` — WebSocket connect/disconnect

This event-sourcing approach provides:
- No state corruption from partial writes (append-only)
- Full audit trail of every state change
- Simple crash recovery — replay the file
- Natural shipping to Postgres via Vector in K8s mode

### Terminal Access + Asciinema Recording

The backend spawns ttyd wrapping the shell in asciinema:

```
ttyd <options> -- asciinema rec --stdin --append /workshop/runtime/session.cast -c /bin/bash
```

- `--stdin` captures input for full replay fidelity
- `--append` allows recording to survive ttyd restarts (reconnections)
- The resulting `session.cast` is in [asciicast v2 format](https://docs.asciinema.org/manual/asciicast/v2/)

The backend:
- Monitors ttyd; restarts it if it exits unexpectedly
- Proxies all browser WebSocket connections to ttyd through a single origin — no CORS issues
- Serves `session.cast` with HTTP Range support for player seeking
- No nsenter or shared process namespace required — ttyd runs inside the same container

### Command Log Watching

The shell's `PROMPT_COMMAND` hook (from `/etc/workshop-platform.bashrc`) writes commands to `/workshop/runtime/command-log.jsonl`. The backend watches this file using fsnotify (or periodic tail-read) and:

- Maintains an in-memory buffer of recent commands
- Pushes new commands to SSE subscribers (instructor view)
- Serves command history via the instructor API

See [Instrumentation](./instrumentation.md) for the shell hook implementation.

### Web UI Serving

- Serves the student-facing web application (HTML, JS, CSS) as embedded static assets
- The student's browser connects only to this backend — there is no separate frontend server

### Goss Validation

When a student clicks Validate (or on periodic auto-validation):

1. Read the goss spec from `/workshop/steps/<id>/goss.yaml`
2. Execute `goss validate -g /workshop/steps/<id>/goss.yaml --format json`
3. Return per-test pass/fail results to the frontend
4. Append a `goss_result` event to `state-events.jsonl`

Because every step image contains ALL steps' goss specs, the backend can validate any step regardless of which step image the container was built from. This enables non-linear navigation.

#### Optional Periodic Validation

The backend can optionally run goss validation periodically (configurable interval, default off). Results from periodic validation are NOT shown to the student — they are written only to `state-events.jsonl` for instructor monitoring. This lets instructors see progress without students explicitly clicking Validate.

TODO: Define the configuration mechanism for periodic validation interval. Environment variable (e.g., `WORKSHOP_GOSS_INTERVAL=30s`)? Or a field in `workshop.yaml`? Currently undocumented.

### Non-Linear Navigation

The backend enforces navigation rules based on `workshop.json`:

| Mode | Behavior |
|---|---|
| `linear` | Only next/prev allowed. Steps must be completed in order. |
| `free` | Any step accessible at any time. |
| `guided` | Free within unlocked groups. Groups unlock when previous group is completed or via `requires`. |

Progress is tracked as a **completion set** — the set of step IDs where goss validation has passed. The frontend shows a completion matrix rather than a linear progress bar.

**Important distinction:** "Navigating" to a step (viewing its tutorial content, checking its goss spec) does NOT require an image swap. Every image contains all steps' metadata. An image swap only occurs when the student explicitly requests to switch their working environment (reset/transition). See [Workshop Spec — Navigation vs Image Swap](../definition/workshop.md#navigation-vs-image-swap).

TODO: Define the API contract for "view step content" vs "transition workspace to step". Currently `POST /api/steps/:id/navigate` is ambiguous — does it trigger an image swap or just change the viewed step? Likely needs two separate actions: one for viewing (no restart) and one for transitioning (container restart).

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
- Push events to SSE subscribers (instructor view)

## API Surface

### Student API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/steps` | List all steps with titles, groups, completion status, accessibility |
| `GET` | `/api/steps/:id/content` | Get step tutorial markdown |
| `POST` | `/api/steps/:id/navigate` | Navigate to a step (enforces navigation rules) |
| `POST` | `/api/steps/:id/validate` | Run goss validation, return results |
| `GET` | `/api/state` | Current state: active step, completed set, navigation mode |
| `POST` | `/api/llm/help` | Request LLM help (streaming response) |
| `GET` | `/api/llm/history` | Get LLM interaction history for current step |

### Instructor API

All instructor endpoints require bearer token authentication (token from `WORKSHOP_INSTRUCTOR_TOKEN` env var).

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/instructor/status` | Current state: active step, completed set, connected, last active |
| `GET` | `/api/instructor/commands` | Recent command history (with pagination) |
| `GET` | `/api/instructor/commands/stream` | SSE stream of new commands |
| `GET` | `/api/instructor/events` | SSE stream of state events (step changes, goss results, connect/disconnect) |
| `GET` | `/api/instructor/goss/history` | Goss validation history for all steps |
| `GET` | `/api/instructor/recording` | Serve `session.cast` with HTTP Range support |

### Static Assets

| Path | Description |
|---|---|
| `/` | Student web UI (embedded SPA) |
| `/instructor/` | Instructor dashboard (embedded, separate SPA or sub-route) |
| `/ws/terminal` | WebSocket proxy to ttyd |

## Instructor Dashboard (Docker Mode)

In Docker mode (single-user, no sidecar), the backend serves a simple instructor view at `/instructor/`:

- Reads local JSONL files directly — no Postgres, no Vector
- SSE endpoint tails local files and pushes events
- Bearer token auth on all `/api/instructor/*` endpoints
- Same container, same process — just a different web view

In Kubernetes mode, the instructor view is served by a [separate dashboard service](./instructor-dashboard.md) that aggregates data from multiple workspaces via Postgres.

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

When authors use a custom `base.image` or `base.containerFile`, the Dagger pipeline injects the platform layer automatically. See [Compilation](../artifact/compilation.md) for details.

## What It Does NOT Do

- Does not parse `workshop.yaml` (build-time concern only)
- Does not manage other workspaces — scoped entirely to the container it runs in
- Does not interact with the Kubernetes API
- Does not pull images or manage Deployments (that is the operator's job)
- Does not write to any database (state is file-based)
- Does not know whether a Vector sidecar exists (it just writes local files)

## Step Transitions

### Cluster Mode

Step transitions are driven by the [Operator](./operator.md) via Deployment image swap. The old pod is replaced by a new pod running the next step's image. The new backend starts fresh, reads flat files, replays state events, and begins serving.

The student's browser reconnects to the new backend. The terminal WebSocket session restarts with the new container's shell.

### Local Mode

Step transitions are driven by the [CLI](./cli.md): stop current container, start new container from next step image. The backend behavior is identical to cluster mode — it always starts fresh.

TODO: Define the mechanism by which the student's browser (→ backend API) triggers a step transition in Docker local mode. The backend runs inside the container and cannot stop/start its own container. Either: (a) the CLI polls a backend API for transition requests, (b) the backend has Docker socket access (security concern), or (c) the student runs CLI commands from the host. This is the most critical design gap for the single-user milestone.

### State Persistence Across Transitions

The `/workshop/runtime/` directory is ephemeral to the container. When a step transition creates a new container:
- State events are lost (fresh start) in default mode
- In K8s mode, the Vector sidecar has already shipped events to Postgres before the transition
- In Docker mode, the CLI can optionally mount a volume for `/workshop/runtime/` to preserve history

TODO: Define whether state persistence across step transitions is required for Docker mode. If state-events.jsonl is preserved via volume mount, the completed set carries over but the workspace filesystem is replaced. Is this the desired behavior? If so, document the volume mount convention. If not, how does the student's completion progress survive step transitions in Docker mode?

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Base Images](./base-images.md) | Backend binary pre-installed in platform base images |
| [Dagger Pipeline](../artifact/compilation.md) | Injects backend binary when using custom base images; bakes flat file metadata |
| [Flat File Artifact](../artifact/flat-file-artifact.md) | `/workshop/` directory is the read-only metadata source |
| [Instrumentation](./instrumentation.md) | Shell bashrc writes command log; asciinema records terminal |
| [LLM Help](./llm-help.md) | Backend handles LLM API calls and context assembly |
| [Instructor Dashboard](./instructor-dashboard.md) | In K8s mode, aggregates data from multiple backends via Postgres |
| [Aggregation](./aggregation.md) | Vector sidecar ships JSONL files to Postgres/S3 |
| [Operator](./operator.md) | Provisions the workspace pod; backend runs inside it |
| [CLI](./cli.md) | In local mode, manages the container lifecycle |
| [Frontend / Student UI](../presentation/frontend.md) | Served by this backend; communicates via HTTP and WebSocket |
