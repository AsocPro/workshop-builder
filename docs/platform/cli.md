# CLI — Administration Layer

## Purpose

Primary control surface for the platform. Handles workspace orchestration across both local (Docker) and cluster (Kubernetes) backends.

## Modes of Operation

### Local Mode

Single-user execution using Docker as the backend.

1. Parse `docker-compose.yml` + `workspace.yaml`
2. Validate against supported subset via [shared library](./shared-go-library.md)
3. Provision cluster if required (k3d/k3s)
4. Run `docker compose up`
5. Mount kubeconfig if cluster was created
6. Manage workspace lifecycle locally

### Cluster Mode

Multi-tenant execution against a Kubernetes cluster.

1. Parse `docker-compose.yml` + `workspace.yaml`
2. Validate
3. Generate [WorkspaceTemplate / WorkspaceInstance CRDs](./crds.md)
4. Submit to Kubernetes API
5. Watch status
6. Support batch provisioning for workshops

## Cluster Provisioning Logic

When `cluster.mode == per-workspace` in `workspace.yaml`, the CLI handles cluster provisioning differently depending on backend:

### Docker Backend (Local)

1. Provision k3d cluster
2. Extract kubeconfig
3. Inject kubeconfig into workload container

### Kubernetes Backend (Cluster)

Cluster provisioning is delegated to the [Operator](./operator.md) via CRD fields. The CLI just submits the spec.

See [Infrastructure Provisioners](./infrastructure-provisioners.md) for details on k3d and vcluster.

## Dependencies

| Dependency | Required For |
|---|---|
| [Shared Go Library](./shared-go-library.md) | Parsing, validation, translation |
| Docker | Local mode execution |
| Kubernetes API | Cluster mode execution |
| k3d binary | Local cluster provisioning |

The CLI does NOT depend on Operator internals.

## Command Structure

TODO: Define the CLI command tree (e.g., `workshop create`, `workshop run`, `workspace list`, `workspace delete`, etc.).

## Configuration

TODO: Define CLI configuration model (config file location, environment variables, kubeconfig discovery).

## Batch Operations

TODO: Define how batch workshop provisioning works (provision N workspaces for a class, assign to users, etc.).

## Authentication & Authorization

TODO: Define how the CLI authenticates to Kubernetes clusters and any platform-level auth requirements.

## Error Handling

TODO: Define error handling strategy — how validation errors, provisioning failures, and partial states are reported and recovered from.
