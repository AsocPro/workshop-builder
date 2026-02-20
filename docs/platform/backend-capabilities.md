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

## Design Rationale

- **Docker backend** exists for local development, authoring, and simple single-user workshops
- **Kubernetes backend** is the production runtime for multi-tenant workshop delivery
- Attempting feature parity would either bloat the Docker backend with hacks or weaken the Kubernetes backend

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

TODO: Define the exact validation error messages for each unsupported capability per backend.
