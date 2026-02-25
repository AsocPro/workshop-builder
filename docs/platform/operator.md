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
| Deployment | Workload container (current step's OCI image; ttyd runs inside as a child process of the backend) |
| Service | Internal and external networking |
| Vector sidecar | Ships JSONL runtime data to Postgres/S3 (see [Aggregation](./aggregation.md)) |
| vcluster | Nested Kubernetes cluster (if `cluster.mode == per-workspace`) |

## Lifecycle Management

- **TTL deletion** — automatically clean up expired workspaces
- **Idle suspend** — scale down idle workspaces to save resources
- **Resume** — bring suspended workspaces back up
- **Status reporting** — phase, access endpoints, resource consumption

## Step Transitions & Reset

The operator owns the step transition flow. When a student moves to Step N:

```
1. Resolve image tag for Step N from WorkspaceTemplate spec
2. Update Deployment spec: set image to Step N's imageTag
3. Wait for Deployment rollout to complete
4. Update WorkspaceInstance status (currentStep = N)
5. Update educational state
```

This is an image swap. The running container is replaced by a new container running the Step N image. No namespace teardown, no PVC restoration, no manifest bundle application.

### Reset Rules

- **Jumping to any step = image swap.** Always.
- **All previous student mutations to the container filesystem are replaced** by the immutable step image. No residual state survives.
- **No incremental patching.** No diffs applied.
- **No mutation replay.** No step history interpreted.
- **Determinism is guaranteed by OCI image immutability.** The same image tag always produces the same container state.

This guarantees:

- Deterministic student experience
- Simplified debugging (state is always known)
- Simplified validation (expected state = image contents)
- No cascading corruption from student mistakes

### Kubernetes State Strategy

**Rejected approaches:**
- Full cluster backups (Velero-level restore)
- Patch replay chains
- Manifest diffs at runtime
- Namespace teardown and rebuild per step transition

**Chosen approach:**
- Store complete step state in OCI images (built at compile time)
- Step transition = Deployment image update + rollout wait
- Determinism over cleverness

### Runtime Snapshots

The operator tracks student progress as logical checkpoints:

- Current active step
- Completion markers
- Optional student-specific notes or answers

These are **educational state only** — not infrastructure diffs. Container state at any point is reconstructed from the OCI image, never from snapshots.

TODO: Define retention policy for runtime snapshots — how many are kept? Per student? Per step?

## What It Does NOT Do

- Parse `workshop.yaml` directly (receives image tags via [CRD](./crds.md))
- Handle local mode (that's [CLI](./cli.md)-only)
- Implement GUI or frontend logic
- Restore PVC contents on step transition (there are no per-step PVC archives)

## Platform System Components

TODO: Define what "platform system components" are excluded from namespace cleanup — the runtime agent? monitoring sidecars? access proxies?

## Reconciliation Loop

TODO: Define the reconciliation flow for WorkspaceTemplate and WorkspaceInstance controllers. How are step transitions detected and processed?

## Error Handling

TODO: Define how the operator handles partial failures (e.g., namespace created but Deployment update failed, rollout stalled).

## Transition Performance

TODO: Define acceptable step transition times. A Deployment image update and rollout is typically faster than a full namespace teardown+rebuild. Quantify expected transition latency.

## Scaling

TODO: Define operator scaling strategy — single replica with leader election? Sharded by namespace?

## Monitoring & Observability

TODO: Define metrics, events, and logging strategy for the operator.

## Deployment

TODO: Define how the operator is deployed (Helm chart, OLM, raw manifests).
