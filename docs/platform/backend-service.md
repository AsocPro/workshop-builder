# Workspace Backend Service

## Purpose

A Go binary embedded in every workshop container image. It is the runtime engine of each workspace — serving the student web UI, proxying terminal access, tracking student progress, and mediating all frontend API interactions.

## Role in the System

The backend binary is injected into every step image by the [Dagger compilation pipeline](../artifact/compilation.md). It is platform infrastructure, not workshop content — authors do not configure it directly.

The backend is PID 2 in the workspace container, launched by tini:

```
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

tini (as PID 1) handles zombie process reaping and clean signal forwarding. The backend owns its child processes (primarily ttyd).

## Container Startup Sequence

```
tini (PID 1)
  └── workshop-backend (PID 2)
        ├── Initialize SQLite working copy
        ├── Spawn ttyd subprocess (terminal access to container shell)
        ├── Start HTTP server (web UI + API)
        └── Supervise ttyd (restart on exit)
```

## Responsibilities

### Terminal Access

- Spawns ttyd as a child process pointing at the container's default shell
- Monitors ttyd; restarts it if it exits unexpectedly
- Proxies all browser WebSocket connections to ttyd through a single origin — no CORS issues, no direct browser-to-ttyd connection
- No nsenter or shared process namespace required — ttyd runs inside the same container as the backend

### Web UI Serving

- Serves the student-facing web application (HTML, JS, CSS) as embedded static assets
- The student's browser connects only to this backend — there is no separate frontend server

### API Surface

- **Step content** — serve markdown for the current step from SQLite
- **Step navigation** — accept next/previous/jump-to requests; update progress in SQLite
- **Progress tracking** — read and write `runtime_state` in the per-instance SQLite database
- **Status** — expose current step, workspace phase, and metadata
- **Validation / Completion** — evaluate whether the student has completed a step

TODO: Define the full API surface (REST vs WebSocket vs both), route structure, and authentication model.

TODO: Define the step completion and validation mechanism. This requires dedicated design work — validation rules are stored in SQLite `step_metadata` but the evaluation language, trigger model, and feedback format are not yet defined.

### SQLite Lifecycle

The backend is the sole writer to the per-instance SQLite database. No other component writes to it directly.

**At startup:**
1. Locate the distribution SQLite file (mounted from a ConfigMap in cluster mode; provided by the CLI in local mode)
2. Copy it to a writable location in ephemeral container storage
3. Use the working copy as the per-instance database for all reads and writes

The distribution SQLite contains the workshop definition (steps, markdown, image tags, navigation). The working copy adds runtime state (`runtime_state`, `custom_state`) written during the student session.

Because each workspace pod has its own container and its own working SQLite copy, there are no concurrent cross-instance writes. SQLite's single-writer model is appropriate.

## How the Binary Gets into Images

The Dagger compilation pipeline adds a platform layer on top of each step's content layers before pushing:

```
Step N content layers
  (files, env, commands from workshop.yaml)
          │
          ▼
Platform layer (injected by Dagger):
  - /sbin/tini
  - /usr/local/bin/workshop-backend  (embedded web UI assets)
  ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

Every step image — regardless of base image — contains the backend and the correct entrypoint. Authors do not need to include or configure the backend in `workshop.yaml`.

## What It Does NOT Do

- Does not parse `workshop.yaml` (build-time concern only)
- Does not manage other workspaces — scoped entirely to the container it runs in
- Does not interact with the Kubernetes API
- Does not pull images or manage Deployments (that is the operator's job)

## Step Transitions

### Cluster Mode

Step transitions are driven by the [Operator](./operator.md) via Deployment image swap. The old pod is replaced by a new pod running the next step's image. The new backend starts fresh, initializes its SQLite working copy from the distribution file, and begins serving.

The student's browser reconnects to the new backend. The terminal WebSocket session restarts with the new container's shell.

### Local Mode

Step transitions are driven by the [CLI](./cli.md): stop current container, start new container from next step image. The backend behavior is identical to cluster mode — it always starts fresh.

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Dagger Pipeline](../artifact/compilation.md) | Injects backend binary and tini into every step image at compile time |
| [SQLite Artifact](../artifact/sqlite-artifact.md) | Distribution SQLite is the source; backend copies it to a writable working instance at startup |
| [Operator](./operator.md) | Provisions the workspace pod; backend runs inside it |
| [CLI](./cli.md) | In local mode, manages the container lifecycle; provides the distribution SQLite |
| [Frontend / Student UI](../presentation/frontend.md) | Served by this backend; communicates via HTTP and WebSocket |
