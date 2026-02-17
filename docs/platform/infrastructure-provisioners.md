# Infrastructure Provisioners

## Purpose

External tools orchestrated by the [CLI](./cli.md) and [Operator](./operator.md) to provision nested Kubernetes clusters when workspaces require them (`cluster.mode == per-workspace`). These are not owned by the platform but are critical dependencies.

## Tools

### k3d

- **Used when:** Docker backend + `cluster.mode == per-workspace`
- **What it does:** Creates lightweight K3s clusters inside Docker containers
- **Managed by:** CLI (local mode)

### k3s

- **Used when:** Direct local installation is preferred over containerized clusters
- **What it does:** Lightweight Kubernetes distribution
- **Managed by:** CLI (local mode)

TODO: Clarify when k3s is used vs k3d. Is k3s a fallback or a separate configuration option?

### vcluster

- **Used when:** Kubernetes backend + `cluster.mode == per-workspace`
- **What it does:** Creates virtual Kubernetes clusters within an existing cluster
- **Managed by:** Operator (cluster mode)

## Provisioning Flow

### Local Mode (k3d)

```
CLI detects cluster.mode == per-workspace
  → CLI provisions k3d cluster
  → CLI extracts kubeconfig
  → CLI injects kubeconfig into workload container
  → Student kubectl commands target the k3d cluster
```

### Cluster Mode (vcluster)

```
Operator detects cluster.mode == per-workspace in WorkspaceInstance
  → Operator deploys vcluster into workspace namespace
  → Operator waits for vcluster ready
  → Operator extracts kubeconfig secret
  → Operator mounts kubeconfig into workload pod
  → Student kubectl commands target the vcluster
```

## Key Constraint

Cluster provisioning is **never** expressed in `docker-compose.yml`. It is infrastructure orchestration logic that lives in the CLI and Operator.

## Version Management

TODO: Define how provisioner versions (k3d, vcluster) are managed and how Kubernetes version selection (`cluster.version` in workspace.yaml) is implemented.

## Cleanup

TODO: Define cleanup procedures for provisioned clusters — how are they torn down during workspace deletion and step transitions?

## Resource Overhead

TODO: Document the resource overhead of each provisioner to inform resource class sizing.
