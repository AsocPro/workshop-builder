# Instructor Dashboard

## Purpose

Give instructors real-time visibility into student progress across all active workspaces. This is a Kubernetes-mode-only component — in Docker local mode, progress is visible directly in the student UI and step management is handled by the CLI.

## Architecture

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

## Dashboard Service

A separate Go binary deployed as a Kubernetes Deployment:

- **HTTP receiver**: Accepts events from Vector sidecars via HTTP POST
- **Postgres writer**: Writes events to the [Postgres schema](./aggregation.md)
- **SSE broadcaster**: Pushes events to connected instructor browsers
- **Web UI server**: Serves the aggregated instructor dashboard
- **CRD watcher**: Watches WorkspaceInstance CRDs to discover active workspaces

The dashboard service is a thin HTTP receiver + SSE broadcaster + Postgres writer. Vector handles buffering, retry, and backpressure — no custom collection logic.

## Views

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

## Authentication

Bearer token for simple deployments (`INSTRUCTOR_DASHBOARD_TOKEN` env var), or OIDC integration for production deployments (shared with cluster auth).

## API Surface

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
| [Backend Service](./backend-service.md) | Student containers write JSONL files; Vector ships them here |
| [Aggregation](./aggregation.md) | Vector ships data to the dashboard service |
| [Instrumentation](./instrumentation.md) | Source of command logs and recordings |
| [Operator](./operator.md) | Dashboard watches WorkspaceInstance CRDs for workspace discovery |
