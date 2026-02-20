# Workspace Metadata — `workspace.yaml`

## Purpose

Defines platform-level semantics that govern how a workspace is provisioned, isolated, lifecycled, and accessed. This is the counterpart to `step-spec.yaml` — the step spec says **what the container images contain**, `workspace.yaml` says **how workspaces run**.

## Conceptual Structure

```yaml
version: v1

lifecycle:
  mode: persistent | ephemeral
  ttl: 2h
  idleSuspend: true

isolation:
  mode: individual | team

cluster:
  mode: none | per-workspace | shared
  version: "1.29"

resources:
  cpu: "500m"
  memory: "512Mi"

access:
  webTerminal:
    enabled: true
```

## Field Definitions

### `lifecycle`

| Field | Type | Description |
|---|---|---|
| `mode` | `persistent \| ephemeral` | Whether the workspace survives beyond a session |
| `ttl` | duration string | Time-to-live before automatic cleanup |
| `idleSuspend` | bool | Whether idle workspaces are suspended to save resources |

### `isolation`

| Field | Type | Description |
|---|---|---|
| `mode` | `individual \| team` | Whether the workspace is per-user or shared among a team |

The default and primary mode for v1 is `individual` — each student gets their own isolated workspace. This covers the vast majority of workshop use cases.

`team` mode is reserved for future implementation. It is a multitenancy concern (assigning multiple students to a shared workspace) that adds meaningful complexity around membership management, access control, and resource sharing. The field is present in the schema to avoid a breaking change later, but the implementation is TBD.

TODO: Define team membership model — how are teams defined and assigned, how membership is enforced, and what the operator does differently for team vs individual workspaces.

### `cluster`

| Field | Type | Description |
|---|---|---|
| `mode` | `none \| per-workspace \| shared` | Whether the workspace gets its own Kubernetes cluster |
| `version` | string | Kubernetes version for provisioned clusters |

- `none` — No nested cluster. Workload runs directly.
- `per-workspace` — Each workspace gets its own cluster (vcluster or k3d depending on backend).
- `shared` — Workspaces share a cluster with namespace isolation.

### `resources`

Authors specify CPU and memory directly rather than picking a named tier. This gives workshop owners full control over resource allocation based on their workload's actual needs.

```yaml
resources:
  cpu: "500m"
  memory: "512Mi"
```

These values apply per container in the workspace. The platform enforces a hard default if no values are specified — leaving resources unset is not allowed, as containers without limits can consume unbounded resources and starve other workspaces on the same node.

TODO: Define the hard default values the platform uses when `resources` is omitted, and whether omitting resources is a validation error or silently applies the defaults.

TODO: Define whether limits are applied at the container level, the namespace level (ResourceQuota), or both.

This section is deferred until core functionality is complete. The field is present in the schema now to avoid a breaking change later.

### `access`

| Field | Type | Description |
|---|---|---|
| `webTerminal.enabled` | bool | Enable browser-based terminal via ttyd |

```yaml
access:
  webTerminal:
    enabled: true
```

The terminal always attaches to the current step's running container. No `target` field is needed — there is only one container per step (the step's OCI image). When the student advances to the next step, the terminal reconnects to the new container automatically.

`webTerminal` is the only access surface for v1. SSH access and browser-based code editors (e.g. code-server) are out of scope for v1 and intentionally omitted to avoid premature complexity.

### Terminal Implementation

[ttyd](https://github.com/tsl0922/ttyd) is used as the terminal backend. It provides the full terminal frontend (xterm.js), pty management, WebSocket protocol, and terminal resize handling. The platform backend proxies all browser WebSocket connections through to ttyd — the browser never connects to ttyd directly, which avoids CORS issues and keeps a single origin for the web UI.

**Cluster mode:** ttyd runs as a sidecar injected into the workspace pod by the operator. The platform backend proxies WebSocket connections to it.

**Local mode:** ttyd and the platform backend run as native processes spawned by the CLI binary — not as containers. This avoids any container socket dependency and ensures compatibility with both Docker and Podman. Podman is daemonless and does not expose a socket by default, so mounting a socket into a container is not a portable solution. The CLI detects which container runtime is available and invokes the appropriate exec command (`docker exec` or `podman exec`).

TODO: Determine the exact mechanism for ttyd to access the current step container shell in cluster mode — sidecar with shared process namespace, nsenter, or other approach. Defer to operator implementation.

TODO: Define how the CLI detects and selects between Docker and Podman runtimes in local mode.

## Relationship to step-spec.yaml

`workspace.yaml` is logically paired with `step-spec.yaml`. Together they form a complete workshop definition:

- `step-spec.yaml` = what the step container images contain (files, env, commands)
- `workspace.yaml` = platform behavior (lifecycle, isolation, access)

They are separate files to maintain separation of concerns.

## Consumers

| Consumer | Usage |
|---|---|
| [CLI](../platform/cli.md) | Reads and validates; drives provisioning decisions |
| [Operator](../platform/operator.md) | Receives as part of CRD spec; enforces semantics |
| [Shared Go Library](../platform/shared-go-library.md) | Parses, validates, generates CRD objects |

## Validation Rules

Validation is intentionally minimal for v1. The shared library enforces the following:

| Rule | Error Message |
|---|---|
| `lifecycle.mode` is not `persistent` or `ephemeral` | `lifecycle.mode: must be "persistent" or "ephemeral"` |
| `isolation.mode` is not `individual` or `team` | `isolation.mode: must be "individual" or "team"` |
| `cluster.mode` is not `none`, `per-workspace`, or `shared` | `cluster.mode: must be "none", "per-workspace", or "shared"` |
| `lifecycle.ttl` is set but not a valid duration string | `lifecycle.ttl: invalid duration format` |
All validation rules are self-contained within `workspace.yaml`. There is no cross-file validation against `step-spec.yaml`.

## Schema Versioning

A top-level `version` field is required in every `workspace.yaml`. The current version is `v1`.

```yaml
version: v1
```

The shared library validates that the version field is present and known. An unrecognized version is a hard validation error. This gives the platform a hook to handle schema migrations as the format evolves without breaking existing files silently.
