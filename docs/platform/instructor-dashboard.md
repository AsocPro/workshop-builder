# Instructor Dashboard

## Purpose

Give instructors real-time visibility into student progress. Two modes of operation:

1. **Docker mode** — single-user, local. The backend reads local JSONL files and serves a simple instructor view at `/instructor/`.
2. **Kubernetes mode** — multi-tenant, aggregated. A separate dashboard service receives events from Vector sidecars, writes to Postgres, and serves an aggregated view for all workspaces.

## Docker Mode (Local, Single-User)

In Docker mode, the instructor dashboard is served directly by the [backend service](./backend-service.md) inside the workshop container. No additional infrastructure required.

### Access

```
http://localhost:8080/instructor/
```

Protected by bearer token authentication:

```bash
docker run -p 8080:8080 \
  -e WORKSHOP_INSTRUCTOR_TOKEN=my-secret-token \
  myorg/kubernetes-101:step-1-intro
```

All `/api/instructor/*` endpoints require the `Authorization: Bearer <token>` header.

### Data Source

The backend reads local files directly:
- `/workshop/runtime/command-log.jsonl` — command history
- `/workshop/runtime/state-events.jsonl` — state transitions
- `/workshop/runtime/session.cast` — terminal recording
- `/workshop/runtime/llm-history.jsonl` — LLM interactions

No Postgres, no Vector, no sidecar. Just file reads.

### Real-Time Updates

`GET /api/instructor/events` provides a Server-Sent Events (SSE) stream. The backend tails local files and pushes events:

```
event: command
data: {"ts":"...","cmd":"kubectl get pods","exit":0}

event: state
data: {"ts":"...","event":"goss_result","step":"step-pods","passed":true,"checks":{"total":5,"passed":5}}

event: connection
data: {"ts":"...","event":"connected"}
```

### Views

**Status panel**: Current step, completed steps, connection state, last activity time.

**Command timeline**: Scrollable list of commands with timestamps and exit codes. Failed commands (non-zero exit) highlighted.

**Asciinema player**: Embedded [asciinema-player](https://docs.asciinema.org/manual/player/) component. Clicking a command timestamp in the timeline seeks the player to that moment in the recording.

**Goss history**: Timeline of validation attempts per step with pass/fail counts.

## Kubernetes Mode (Multi-Tenant, Aggregated)

In Kubernetes mode, a separate dashboard service aggregates data from all active workspaces.

### Architecture

```
Student containers (identical to Docker mode)
  │
  ├── /workshop/runtime/*.jsonl ← shared volume → Vector sidecar
  │                                                    │
  │   Vector HTTP sink:                                │
  │   ├── command-log.jsonl → POST /ingest/commands    │
  │   ├── state-events.jsonl → POST /ingest/events     │
  │   └── session.cast → S3/MinIO                      │
  │                                                    │
  └── Backend behavior is IDENTICAL to Docker mode     │
                                                       ▼
                                     Instructor Dashboard Service
                                     (Go binary — K8s Deployment)
                                       │
                                       ├── HTTP receiver (from Vector)
                                       ├── Postgres writer
                                       ├── SSE broadcaster
                                       └── Web UI server
```

### Dashboard Service

A separate Go binary deployed as a Kubernetes Deployment:

- **HTTP receiver**: Accepts events from Vector sidecars via HTTP POST
- **Postgres writer**: Writes events to the [Postgres schema](./aggregation.md)
- **SSE broadcaster**: Pushes events to connected instructor browsers
- **Web UI server**: Serves the aggregated instructor dashboard
- **CRD watcher**: Watches WorkspaceInstance CRDs to discover active workspaces

The dashboard service is a thin HTTP receiver + SSE broadcaster + Postgres writer. Vector handles buffering, retry, and backpressure — no custom collection logic.

### Views

**Student list**: Real-time grid showing all active workspaces with:
- Student/workspace identifier
- Current step
- Connection status (connected/disconnected/idle)
- Completion progress (completion matrix for non-linear workshops)
- Last activity timestamp
- Quick-glance indicators (stuck? failing validation? idle?)

**Student detail**: Click a student to see:
- Command timeline with timestamps and exit codes
- Embedded asciinema player with command-timestamp seeking
- Goss validation history (per step, pass/fail timeline)
- LLM interaction history

**Completion matrix**: For non-linear workshops (`free` or `guided` navigation), shows a matrix of students × steps with completion status. Useful for identifying which steps are causing the most difficulty.

### Authentication

The dashboard service uses a separate auth mechanism from the student containers:

- Bearer token for simple deployments (`INSTRUCTOR_DASHBOARD_TOKEN` env var)
- OIDC integration for production deployments (shared with cluster auth)

TODO: Reconcile the auth token env var naming. Docker mode uses `WORKSHOP_INSTRUCTOR_TOKEN` (set on the student container). K8s mode uses `INSTRUCTOR_DASHBOARD_TOKEN` (set on the dashboard service). These are semantically the same concept (instructor auth) but use different names. Consider standardizing.

### API Surface

| Method | Path | Description |
|---|---|---|
| `POST` | `/ingest/commands` | Receive command log events from Vector |
| `POST` | `/ingest/events` | Receive state events from Vector |
| `GET` | `/api/workspaces` | List all active workspaces with current status |
| `GET` | `/api/workspaces/:id/commands` | Command history for a workspace |
| `GET` | `/api/workspaces/:id/events` | State event history for a workspace |
| `GET` | `/api/workspaces/:id/recording` | Proxy asciinema recording from S3 |
| `GET` | `/api/workspaces/:id/llm` | LLM interaction history for a workspace |
| `GET` | `/api/stream` | SSE stream of all workspace events |
| `GET` | `/` | Dashboard web UI |

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Backend Service](./backend-service.md) | Serves Docker-mode instructor view; K8s mode is separate |
| [Aggregation](./aggregation.md) | Vector ships data to the dashboard service in K8s mode |
| [Instrumentation](./instrumentation.md) | Source of command logs and recordings |
| [Operator](./operator.md) | Dashboard watches WorkspaceInstance CRDs for workspace discovery |
