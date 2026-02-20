# Custom Resource Definitions — WorkspaceTemplate & WorkspaceInstance

## Overview

Two CRDs define the cluster-level control plane for workspace management. These are the API objects that the [CLI](./cli.md) creates and the [Operator](./operator.md) reconciles.

The WorkspaceTemplate is where operators configure all deployment behavior — lifecycle, isolation, cluster mode, resources, and access. Workshop authors do not interact with CRDs directly; they write [`workshop.yaml`](../definition/workshop.md) and let the CLI and compilation pipeline produce the artifacts that populate a template.

---

## WorkspaceTemplate

### Purpose

Cluster-level reusable workspace definition. Represents an operator's blueprint for how a workshop runs — instantiated once per workshop, referenced by many WorkspaceInstances.

### Contains

- Operator-configured deployment defaults (lifecycle, isolation, cluster mode, resources, access)
- Workshop step definitions (OCI image tags populated from SQLite by the CLI)
- Image pull secrets

### Lifecycle

- **Created by:** Operator/admin (writes template YAML directly and applies with kubectl or CLI tooling)
- **Step fields populated by:** CLI (reads SQLite image tags; updates template via [Shared Go Library](./shared-go-library.md))
- **Consumed by:** Operator
- **Scope:** Cluster-scoped (available across namespaces)

### Spec Fields

#### `spec.defaults` — Operator Configuration

These fields are operator concerns. They define how workspaces behave at runtime and are not derived from `workshop.yaml`.

| Field | Type | Description |
|---|---|---|
| `lifecycle.mode` | `persistent \| ephemeral` | Whether the workspace survives beyond a session |
| `lifecycle.ttl` | duration string | Time-to-live before automatic cleanup (e.g. `2h`) |
| `lifecycle.idleSuspend` | bool | Whether idle workspaces are suspended to save resources |
| `isolation.mode` | `individual \| team` | Per-user workspaces (`individual`) or shared among a team (`team`, reserved for future) |
| `cluster.mode` | `none \| per-workspace \| shared` | Whether the workspace gets its own nested Kubernetes cluster |
| `cluster.version` | string | Kubernetes version for provisioned nested clusters |
| `resources.cpu` | string | CPU limit per container (e.g. `500m`) — not enforced in v1 |
| `resources.memory` | string | Memory limit per container (e.g. `512Mi`) — not enforced in v1 |
| `access.webTerminal` | bool | Enable browser-based terminal via ttyd |

**`cluster.mode` values:**
- `none` — workload runs directly in the workspace namespace, no nested cluster
- `per-workspace` — each workspace gets its own vcluster
- `shared` — workspaces share a cluster with namespace isolation

**`isolation.mode`:** `individual` is the only implemented mode for v1. `team` mode is schema-reserved for future implementation.

**`resources`:** Present in schema but not enforced in v1. Enforcement (defaults, validation, ResourceQuota vs LimitRange) is deferred post-v1.

#### `spec.steps` — Workshop Content (CLI-Populated)

Step entries are populated by the CLI reading the SQLite artifact after a successful `workshop build compile`. Operators do not write these manually.

| Field | Type | Description |
|---|---|---|
| `steps[].id` | string | Step identifier (matches `workshop.yaml` step ID) |
| `steps[].title` | string | Human-readable step title |
| `steps[].imageTag` | string | Full OCI image tag for this step (e.g. `myorg/kubernetes-101:step-1-intro`) |

#### `spec.imagePullSecrets`

Kubernetes image pull secrets for authenticating to the container registry where step images are stored.

### Example Structure

```yaml
apiVersion: workshop.platform/v1alpha1
kind: WorkspaceTemplate
metadata:
  name: kubernetes-101
spec:
  defaults:
    lifecycle:
      mode: ephemeral
      ttl: 2h
      idleSuspend: true
    isolation:
      mode: individual
    cluster:
      mode: none
    resources:
      cpu: "500m"
      memory: "512Mi"
    access:
      webTerminal: true

  imagePullSecrets:
    - name: registry-credentials

  steps:
    - id: step-1-intro
      title: "Introduction"
      imageTag: myorg/kubernetes-101:step-1-intro

    - id: step-2-deploy
      title: "Deploy the App"
      imageTag: myorg/kubernetes-101:step-2-deploy
```

`steps[].imageTag` is populated from the SQLite artifact by the CLI at template update time. The operator reads these to perform step transitions.

Note: Digest-pinned image references are not used in v1. See [SQLite Artifact](../artifact/sqlite-artifact.md) for context.

TODO: Finalize which fields are overridable at WorkspaceInstance level vs locked at template level.

---

## WorkspaceInstance

### Purpose

Represents a single active workspace for one student (or team, in the future). This is the runtime object the [Operator](./operator.md) reconciles into actual Kubernetes resources.

### Contains

- Reference to a WorkspaceTemplate
- Owner / assignee
- Current step
- Any instance-level overrides of template defaults
- Status fields

### Lifecycle

- **Created by:** CLI (batch provisioning or individual workspace creation)
- **Consumed by:** Operator
- **Scope:** Namespaced (each instance lives in its own namespace)

### Example Structure

```yaml
apiVersion: workshop.platform/v1alpha1
kind: WorkspaceInstance
metadata:
  name: kubernetes-101-student-42
  namespace: ws-kubernetes-101-student-42
spec:
  templateRef:
    name: kubernetes-101
  owner: student-42
  currentStep: step-2-deploy
  overrides:
    lifecycle:
      ttl: 4h
status:
  phase: Running | Suspended | Terminating
  currentStep: step-2-deploy
  startedAt: "2026-01-15T10:00:00Z"
  expiresAt: "2026-01-15T14:00:00Z"
  accessEndpoints:
    webTerminal: https://ws-kubernetes-101-student-42.example.com/terminal
  conditions: [...]
```

TODO: Define the full status subresource (conditions, phase transitions, error reporting).

TODO: Define how step transitions are requested — update to `spec.currentStep`? A separate sub-resource? A backend API call that the operator watches?

---

## CRD Versioning

TODO: Define CRD versioning strategy (v1alpha1 → v1beta1 → v1) and conversion webhook requirements.

## Validation

TODO: Define admission webhook validation rules (beyond OpenAPI schema validation).

## RBAC

TODO: Define which roles can create/read/update/delete Templates vs Instances.
