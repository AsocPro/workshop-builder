# Backend Capability Model

## Purpose

The platform supports two backends with intentionally different capability sets. This asymmetry is a design decision, not a limitation to be resolved.

## Capability Matrix

| Capability | Docker Backend | Kubernetes Backend |
|---|---|---|
| Single-user workspaces | Yes | Yes |
| Multi-user / team mode | No | Yes |
| Namespace isolation | No | Yes |
| Resource quotas | No | Yes |
| RBAC | No | Yes |
| TTL enforcement | Basic (CLI-managed) | Full (Operator-managed) |
| Idle suspend | No | Yes |
| Network policies | No | Yes |
| Nested cluster (k3d) | Yes | No |
| Nested cluster (vcluster) | No | Yes |
| OIDC integration | No | Yes |
| Batch provisioning | Limited | Full |
| Step transitions / reset | Image swap (CLI-managed) | Image swap (Operator-managed) |
| Dagger build pipeline | Yes (local Docker) | Yes (local Docker, pre-deploy) |
| Non-linear navigation | Yes | Yes |
| Command logging | Yes (local JSONL) | Yes (JSONL → Vector → Postgres) |
| Terminal recording | Yes (local session.cast) | Yes (session.cast → Vector → S3) |
| Goss validation | Yes | Yes |
| LLM help | Yes (direct API call) | Yes (direct API call) |
| Instructor view | Yes (local, single-user at `/instructor/`) | Yes (aggregated dashboard service) |
| Real-time monitoring (SSE) | Yes (local file tailing) | Yes (Vector → Dashboard → SSE) |
| Multi-workspace aggregation | No | Yes (Postgres + Dashboard service) |
| Asciinema playback | Yes (local file) | Yes (S3 storage) |

## Design Rationale

- **Docker backend** exists for local development, authoring, and simple single-user workshops
- **Kubernetes backend** is the production runtime for multi-tenant workshop delivery
- Attempting feature parity would either bloat the Docker backend with hacks or weaken the Kubernetes backend

## Monitoring Symmetry

The student container is **identical** in both modes. It always writes the same JSONL files and session.cast regardless of deployment mode. The difference is in how that data is consumed:

| Concern | Docker Mode | Kubernetes Mode |
|---|---|---|
| Data source | Backend reads local files | Vector sidecar ships to Postgres/S3 |
| Instructor view | Backend serves at `/instructor/` | Separate dashboard service |
| Real-time updates | Backend tails local files → SSE | Vector → Dashboard → SSE |
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
