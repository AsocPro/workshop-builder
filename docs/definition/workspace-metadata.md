# Workspace Metadata — `workspace.yaml`

## Purpose

Defines platform-level semantics that govern how a workspace is provisioned, isolated, lifecycled, and accessed. This is the counterpart to `docker-compose.yml` — Compose says **what runs**, `workspace.yaml` says **how it runs**.

## Conceptual Structure

```yaml
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
  class: workshop-small | dev-large

access:
  ssh: true
  webTerminal: true
  codeServer: false
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

TODO: Define team membership model — how are teams defined and assigned?

### `cluster`

| Field | Type | Description |
|---|---|---|
| `mode` | `none \| per-workspace \| shared` | Whether the workspace gets its own Kubernetes cluster |
| `version` | string | Kubernetes version for provisioned clusters |

- `none` — No nested cluster. Workload runs directly.
- `per-workspace` — Each workspace gets its own cluster (vcluster or k3d depending on backend).
- `shared` — Workspaces share a cluster with namespace isolation.

### `resources`

| Field | Type | Description |
|---|---|---|
| `class` | string | Named resource tier mapped to quotas and limits |

TODO: Define the available resource classes and their concrete quota/limit values.

### `access`

| Field | Type | Description |
|---|---|---|
| `ssh` | bool | Enable SSH access to the workspace |
| `webTerminal` | bool | Enable browser-based terminal |
| `codeServer` | bool | Enable VS Code in browser |

TODO: Define how access surfaces are implemented (sidecars? ingress rules? services?).

## Relationship to Compose

`workspace.yaml` is logically paired with `docker-compose.yml`. Together they form a complete workspace definition:

- Compose = workload topology
- workspace.yaml = platform behavior

They are separate files to maintain separation of concerns.

## Consumers

| Consumer | Usage |
|---|---|
| [CLI](../platform/cli.md) | Reads and validates; drives provisioning decisions |
| [Operator](../platform/operator.md) | Receives as part of CRD spec; enforces semantics |
| [Shared Go Library](../platform/shared-go-library.md) | Parses, validates, generates CRD objects |

## Validation Rules

TODO: Define validation rules (e.g., team mode requires cluster backend, ttl requires ephemeral mode, etc.).

## Schema Versioning

TODO: Define how the workspace.yaml schema will be versioned over time.
