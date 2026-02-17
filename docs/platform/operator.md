# Kubernetes Operator — Enforcement Layer

## Purpose

The authoritative multi-tenant lifecycle engine. The Operator is the only component that creates and manages Kubernetes resources for workspaces in cluster mode. It also owns **step transitions and reset semantics** — these are infrastructure operations, not content concerns.

## What It Creates (Per Workspace Instance)

| Resource | Purpose |
|---|---|
| Namespace | Isolation boundary |
| ResourceQuota | Enforce resource limits per resource class |
| LimitRange | Default container resource constraints |
| RBAC bindings | Scope user/service account permissions |
| NetworkPolicy | Optional network isolation between workspaces |
| PVCs | Persistent storage for workspace data |
| Deployments | Workload containers (from translated Compose) |
| Services | Internal and external networking |
| Access sidecars | SSH, web terminal, code-server containers |
| vcluster | Nested Kubernetes cluster (if `cluster.mode == per-workspace`) |

## Lifecycle Management

- **TTL deletion** — automatically clean up expired workspaces
- **Idle suspend** — scale down idle workspaces to save resources
- **Resume** — bring suspended workspaces back up
- **Status reporting** — phase, access endpoints, resource consumption

## Step Transitions & Reset

The operator owns the step transition flow. When a student moves to Step N:

```
1. Delete namespace contents (excluding platform system components)
2. Recreate or clean the namespace
3. Apply compiled manifest bundle for Step N
4. Replace PVC/file contents with compiled file state
5. Update educational state
6. Update WorkspaceInstance status (currentStep = N)
```

### Reset Rules

- **Jumping to any step = full clean reset.** Always.
- **All previous student mutations are erased.** No residual state.
- **No incremental patching.** No diffs applied.
- **No mutation replay.** No step history interpreted.
- **Only declarative reconciliation.** Apply desired state from [compiled artifacts](../artifact/compilation.md).

This guarantees:

- Deterministic student experience
- Simplified debugging (state is always known)
- Simplified validation (expected state = compiled state)
- No cascading corruption from student mistakes

### Kubernetes State Strategy

**Rejected approaches:**
- Full cluster backups (Velero-level restore)
- Patch replay chains
- Manifest diffs at runtime
- Layered infrastructure replay

**Chosen approach:**
- Store full desired state per step (from compilation)
- Runtime deletes and reapplies
- Determinism over cleverness

### File and PVC Strategy

- Each compiled step contains full file state
- Runtime replaces PVC contents completely
- No incremental mutation at runtime

### Runtime Snapshots

The operator tracks student progress as logical checkpoints:

- Current active step
- Completion markers
- Optional student-specific notes or answers

These are **educational state only** — not infrastructure diffs. Cluster state at any point is reconstructed from compiled artifacts, never from snapshots.

TODO: Define retention policy for runtime snapshots — how many are kept? Per student? Per step?

## What It Does NOT Do

- Parse Compose files directly (receives parsed/translated spec via [CRD](./crds.md))
- Handle local mode (that's [CLI](./cli.md)-only)
- Implement GUI or frontend logic

## Platform System Components

TODO: Define what "platform system components" are excluded from namespace cleanup during step transitions — the runtime agent? monitoring sidecars? access proxies?

## Reconciliation Loop

TODO: Define the reconciliation flow for WorkspaceTemplate and WorkspaceInstance controllers. How are step transitions detected and processed?

## Error Handling

TODO: Define how the operator handles partial failures (e.g., namespace created but deployment failed), and what happens if a step transition partially fails (manifest applies but PVC restore fails).

## Transition Performance

TODO: Define acceptable step transition times. What optimizations are available (parallel apply, pre-staging)?

## Scaling

TODO: Define operator scaling strategy — single replica with leader election? Sharded by namespace?

## Monitoring & Observability

TODO: Define metrics, events, and logging strategy for the operator.

## Deployment

TODO: Define how the operator is deployed (Helm chart, OLM, raw manifests).
