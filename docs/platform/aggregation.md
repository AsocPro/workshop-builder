# Aggregation — Vector Sidecar + Postgres (Kubernetes Mode)

## Purpose

In Kubernetes mode, a Vector sidecar ships JSONL files from each student container to a central Postgres database and S3-compatible object storage. This enables the [instructor dashboard](./instructor-dashboard.md) to aggregate data from all active workspaces.

The student container doesn't know or care whether a sidecar exists. It always writes the same local files regardless of deployment mode.

## Architecture

```
Student container (identical image, identical behavior)
  │
  ├── /workshop/runtime/command-log.jsonl ──┐
  ├── /workshop/runtime/state-events.jsonl ─┤ shared volume
  ├── /workshop/runtime/session.cast ───────┤
  └── /workshop/runtime/llm-history.jsonl ──┘
                                            │
                                     Vector sidecar
                                            │
                     ┌──────────────────────┼──────────────────────┐
                     │                      │                      │
              HTTP POST to              HTTP POST to          S3/MinIO
              Dashboard Service         Dashboard Service     Object Storage
              /ingest/commands          /ingest/events
                     │                      │                      │
                     └──────────────────────┼──────────────────────┘
                                            │
                                  Instructor Dashboard Service
                                     │              │
                                  Postgres        SSE broadcast
                                  (write)         (to browsers)
```

## Sidecar Isolation

The student container has **no database credentials** and **no direct access** to Postgres or S3. The sidecar reads JSONL files from the shared volume and owns all external credentials. Even if a student inspects the container environment, there's nothing to find.

| Container | Has Access To |
|---|---|
| Student container | Local files only (`/workshop/runtime/`) |
| Vector sidecar | Shared volume (read), Postgres credentials, S3 credentials |

## Four Pipelines

Vector runs four independent pipelines, one per file type:

### 1. Command Log Pipeline

| Source | Destination |
|---|---|
| `command-log.jsonl` | HTTP POST → Dashboard Service → Postgres `command_log` table |

```toml
[sources.command_log]
type = "file"
include = ["/workshop/runtime/command-log.jsonl"]
read_from = "beginning"

[transforms.add_workspace_id]
type = "remap"
inputs = ["command_log"]
source = '.workspace_id = "${WORKSPACE_ID}"'

[sinks.command_log_http]
type = "http"
inputs = ["add_workspace_id"]
uri = "${DASHBOARD_SERVICE_URL}/ingest/commands"
encoding.codec = "json"
method = "post"
batch.max_events = 100
batch.timeout_secs = 5
```

### 2. State Events Pipeline

| Source | Destination |
|---|---|
| `state-events.jsonl` | HTTP POST → Dashboard Service → Postgres `workspace_status` (upsert) + `state_timeline` (append) |

```toml
[sources.state_events]
type = "file"
include = ["/workshop/runtime/state-events.jsonl"]
read_from = "beginning"

[transforms.add_workspace_id_state]
type = "remap"
inputs = ["state_events"]
source = '.workspace_id = "${WORKSPACE_ID}"'

[sinks.state_events_http]
type = "http"
inputs = ["add_workspace_id_state"]
uri = "${DASHBOARD_SERVICE_URL}/ingest/events"
encoding.codec = "json"
method = "post"
batch.max_events = 10
batch.timeout_secs = 2
```

### 3. Recording Pipeline

| Source | Destination |
|---|---|
| `session.cast` | S3/MinIO object storage |

```toml
[sources.recording]
type = "file"
include = ["/workshop/runtime/session.cast"]
read_from = "beginning"

[sinks.recording_s3]
type = "aws_s3"
inputs = ["recording"]
bucket = "${RECORDING_BUCKET}"
key_prefix = "recordings/${WORKSPACE_ID}/"
encoding.codec = "text"
```

### 4. LLM History Pipeline

| Source | Destination |
|---|---|
| `llm-history.jsonl` | HTTP POST → Dashboard Service → Postgres `llm_interactions` table |

```toml
[sources.llm_history]
type = "file"
include = ["/workshop/runtime/llm-history.jsonl"]
read_from = "beginning"

[transforms.add_workspace_id_llm]
type = "remap"
inputs = ["llm_history"]
source = '.workspace_id = "${WORKSPACE_ID}"'

[sinks.llm_history_http]
type = "http"
inputs = ["add_workspace_id_llm"]
uri = "${DASHBOARD_SERVICE_URL}/ingest/llm"
encoding.codec = "json"
method = "post"
batch.max_events = 10
batch.timeout_secs = 5
```

## Cursor Tracking & Restart Recovery

Vector tracks its read cursor offset in a checkpoint file. On sidecar restart:

1. Vector reads its checkpoint file to find the last synced byte offset per file
2. Resumes reading from the last known position — no duplicate data
3. Catches up on any events written while the sidecar was down

This is built-in Vector behavior (file source with `read_from = "beginning"` and checkpointing). No custom cursor logic needed.

## Graceful Shutdown

A `preStop` hook on the sidecar container ensures data is flushed before pod termination:

```yaml
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 5 && kill -SIGTERM 1"]
```

The 5-second sleep allows Vector to flush its internal buffers and complete in-flight HTTP requests. Vector handles `SIGTERM` gracefully — it finishes pending writes before exiting.

## Postgres Schema

The dashboard service owns the Postgres schema. It creates/migrates tables on startup.

```sql
-- Current state per workspace (one row, upserted on each state event)
CREATE TABLE workspace_status (
    workspace_id    TEXT PRIMARY KEY,
    workshop_name   TEXT NOT NULL,
    current_step    TEXT,
    connected       BOOLEAN NOT NULL DEFAULT false,
    last_active_at  TIMESTAMP,
    steps_completed TEXT[],
    last_goss_passed BOOLEAN,
    updated_at      TIMESTAMP NOT NULL
);

-- Append-only state transition timeline
CREATE TABLE state_timeline (
    id              SERIAL PRIMARY KEY,
    workspace_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    step_id         TEXT,
    data            JSONB,
    timestamp       TIMESTAMP NOT NULL
);

-- Append-only command history
CREATE TABLE command_log (
    id              SERIAL PRIMARY KEY,
    workspace_id    TEXT NOT NULL,
    command         TEXT NOT NULL,
    exit_code       INTEGER NOT NULL,
    timestamp       TIMESTAMP NOT NULL
);

-- Asciinema recording references
CREATE TABLE recording_refs (
    workspace_id    TEXT PRIMARY KEY,
    storage_path    TEXT NOT NULL,
    last_sync_byte  BIGINT NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

-- LLM interaction history
CREATE TABLE llm_interactions (
    id              SERIAL PRIMARY KEY,
    workspace_id    TEXT NOT NULL,
    step_id         TEXT NOT NULL,
    student_question TEXT,
    prompt_context  JSONB NOT NULL,
    response        TEXT NOT NULL,
    model           TEXT NOT NULL,
    requested_at    TIMESTAMP NOT NULL
);

CREATE INDEX idx_command_log_ws ON command_log(workspace_id, timestamp);
CREATE INDEX idx_state_timeline_ws ON state_timeline(workspace_id, timestamp);
CREATE INDEX idx_llm_ws ON llm_interactions(workspace_id, step_id);
```

### Table Responsibilities

| Table | Written By | Read By | Pattern |
|---|---|---|---|
| `workspace_status` | Dashboard service (on state events) | Dashboard UI | Upsert (one row per workspace) |
| `state_timeline` | Dashboard service (on state events) | Dashboard UI | Append-only |
| `command_log` | Dashboard service (on command events) | Dashboard UI | Append-only |
| `recording_refs` | Dashboard service (on S3 sync) | Dashboard UI | Upsert |
| `llm_interactions` | Dashboard service (on LLM events) | Dashboard UI | Append-only |

## Pod Specification

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: workshop-workspace-abc123
spec:
  containers:
    - name: workspace
      image: myorg/kubernetes-101:step-1-intro
      ports:
        - containerPort: 8080
      volumeMounts:
        - name: runtime
          mountPath: /workshop/runtime

    - name: vector
      image: timberio/vector:latest-alpine
      volumeMounts:
        - name: runtime
          mountPath: /workshop/runtime
          readOnly: true
        - name: vector-config
          mountPath: /etc/vector
      env:
        - name: WORKSPACE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: DASHBOARD_SERVICE_URL
          value: "http://instructor-dashboard.workshop-system.svc.cluster.local"
        - name: RECORDING_BUCKET
          value: "workshop-recordings"
      envFrom:
        - secretRef:
            name: vector-credentials
      lifecycle:
        preStop:
          exec:
            command: ["/bin/sh", "-c", "sleep 5 && kill -SIGTERM 1"]

  volumes:
    - name: runtime
      emptyDir: {}
    - name: vector-config
      configMap:
        name: vector-config
```

## What Docker Mode Does NOT Have

In Docker mode (single-user, local):
- No Vector sidecar
- No Postgres
- No S3/MinIO
- The backend reads local files directly
- The instructor view is served by the same backend process
- Everything works without any aggregation infrastructure

The container image is identical. The only difference is whether a sidecar is present.

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Backend Service](./backend-service.md) | Writes JSONL files that Vector reads — no direct interaction |
| [Instrumentation](./instrumentation.md) | Shell hook writes command-log.jsonl; asciinema writes session.cast |
| [Instructor Dashboard](./instructor-dashboard.md) | Receives events from Vector, writes to Postgres, serves UI |
| [Operator](./operator.md) | Configures pod spec with sidecar; manages shared volume |
