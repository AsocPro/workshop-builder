# Backend Capability Model

## Purpose

The platform supports two container backends with intentionally different capability sets, plus a containerless standalone mode for single-operator server runbooks. This asymmetry is a design decision, not a limitation to be resolved.

## Capability Matrix

| Capability | Docker Backend | Kubernetes Backend | Standalone Mode (server) |
|---|---|---|---|
| Single-user workspaces | Yes | Yes | Yes (single user per server) |
| Multi-user / team mode | No | Yes | No |
| Namespace isolation | No | Yes | No — runs on the real host |
| Resource quotas | No | Yes | No |
| RBAC | No | Yes | Host accounts (runs as invoking user) |
| TTL enforcement | Basic (CLI-managed) | Full (Operator-managed) | No |
| Idle suspend | No | Yes | No |
| Network policies | No | Yes | No |
| Nested cluster (k3d) | Yes | No | No — infrastructure must pre-exist |
| Nested cluster (vcluster) | No | Yes | No |
| OIDC integration | No | Yes | No (optional basic auth) |
| Batch provisioning | Limited | Full | No |
| Step transitions / reset | Image swap (CLI-managed) | Image swap (Operator-managed) | In-place setup; no reset |
| Dagger build pipeline | Yes (local Docker) | Yes (local Docker, pre-deploy) | Not used — compile-on-serve |
| Non-linear navigation | Yes | Yes | Yes |
| Command logging | Yes (local JSONL) | Yes (JSONL → Vector → Postgres) | Yes (local JSONL, scoped to workshop terminals) |
| Terminal recording | Yes (local session.cast) | Yes (session.cast → Vector → S3) | Yes (local session.cast) |
| Goss validation | Yes | Yes | Yes — against the live host |
| LLM help | Yes (direct API call) | Yes (direct API call) | Yes (direct API call) |
| Instructor view | No (student UI + CLI) | Yes (aggregated dashboard service) | No |
| Real-time monitoring (SSE) | No | Yes (Vector → Dashboard → SSE) | No |
| Multi-workspace aggregation | No | Yes (Postgres + Dashboard service) | No |
| Asciinema playback | Yes (local file) | Yes (S3 storage) | Yes (local file) |

Standalone mode's "No" column is by design — see [Standalone Mode](./standalone-mode.md) for the scope constraints (single user per server, server state authoritative, private networks only). Workshops declaring `infrastructure.cluster` or `infrastructure.extraContainers` fail standalone startup with an explicit error.

## Design Rationale

- **Docker backend** exists for local development, authoring, and simple single-user workshops
- **Kubernetes backend** is the production runtime for multi-tenant workshop delivery
- Attempting feature parity would either bloat the Docker backend with hacks or weaken the Kubernetes backend

## Monitoring Symmetry

The student container is **identical** in both modes. It always writes the same JSONL files and session.cast regardless of deployment mode. The difference is in how that data is consumed:

| Concern | Docker Mode | Kubernetes Mode |
|---|---|---|
| Data source | Backend reads local files | Vector sidecar ships to Postgres/S3 |
| Instructor view | Not applicable (student UI + CLI) | Separate dashboard service |
| Real-time updates | Not applicable | Vector → Dashboard → SSE |
| Aggregation | Single workspace only | All workspaces in Postgres |
| Recording storage | Local `session.cast` file | S3/MinIO object storage |

## Capability Enforcement

The [Shared Go Library](./shared-go-library.md) contains capability matrix logic that:

1. Checks workspace requirements against the target backend
2. Produces clear errors when requirements exceed backend capabilities
3. Prevents submission of unsupported configurations

Example: A WorkspaceTemplate with `isolation.mode: team` will fail validation when targeting the Docker backend with a message explaining that team mode requires the Kubernetes backend.

## Feature Parity Non-Goals

The platform explicitly does NOT attempt to:

- Simulate namespaces in Docker
- Implement quotas in Docker
- Add RBAC to local mode
- Make Docker behave like Kubernetes
- Run a Postgres instance for single-user Docker mode
- Run a Vector sidecar in Docker mode
