# Workload Layer — `docker-compose.yml`

## Purpose

Defines container topology. This is purely a workload description — it describes **what containers run**, not how they are managed, isolated, or lifecycled.

## What It Contains

- `services`
- `image`
- `ports`
- `volumes`
- `environment`
- `command`
- Minimal `depends_on`

## What It Does NOT Contain

- TTL
- Lifecycle mode (persistent/ephemeral)
- Team isolation
- Namespace semantics
- RBAC
- Cluster provisioning config
- Quotas
- Resource classes

These concerns belong in [`workspace.yaml`](./workspace-metadata.md).

## Design Rationale

Compose is an industry-standard format for describing container topology. By limiting it to this role:

- Workshop authors use familiar tooling
- No custom extensions or x-fields are required for platform semantics
- The same Compose file works in local Docker mode and cluster mode
- Compose files remain portable and testable outside the platform

## Consumers

| Consumer | How It Uses Compose |
|---|---|
| CLI (local mode) | Runs `docker compose up` directly |
| CLI (cluster mode) | Parses and translates via [shared library](../platform/shared-go-library.md) |
| [Operator](../platform/operator.md) | Receives parsed/translated spec (never reads Compose directly) |

## Supported Compose Subset

TODO: Define the exact subset of Compose features that the platform supports. Document which Compose features are intentionally excluded and why.

## Translation Rules

TODO: Document how Compose services map to Kubernetes objects (Deployments, Services, PVCs, etc.) during translation in the shared library.

## Validation

The [Shared Go Library](../platform/shared-go-library.md) validates Compose files against the supported subset before any execution or translation occurs.

TODO: Define specific validation rules and error messages.

## Examples

TODO: Add example `docker-compose.yml` files for common workshop patterns (single container, multi-service, with volumes, etc.).
