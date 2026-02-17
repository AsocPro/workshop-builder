# Custom Resource Definitions — WorkspaceTemplate & WorkspaceInstance

## Overview

Two CRDs define the cluster-level control plane for workspace management. These are the API objects that the [CLI](./cli.md) creates and the [Operator](./operator.md) reconciles.

---

## WorkspaceTemplate

### Purpose

Cluster-level reusable workspace definition. Represents a workshop's infrastructure blueprint that can be instantiated multiple times.

### Contains

- Parsed Compose topology (translated to K8s specs)
- Workspace metadata defaults (from `workspace.yaml`)
- Resource class assignment
- Access surface configuration
- Workshop step definitions (compiled artifact references)

### Lifecycle

- **Created by:** CLI (or future API)
- **Consumed by:** Operator
- **Scope:** Cluster-scoped (available across namespaces)

### Example Structure

```yaml
apiVersion: workshop.platform/v1alpha1
kind: WorkspaceTemplate
metadata:
  name: kubernetes-101
spec:
  workload:
    containers: [...]
    volumes: [...]
    services: [...]

  defaults:
    lifecycle:
      mode: ephemeral
      ttl: 2h
    isolation:
      mode: individual
    cluster:
      mode: none
    resources:
      class: workshop-small
    access:
      ssh: true
      webTerminal: true

  steps:
    - name: step-1-intro
      manifestBundle: <ref or inline>
      fileArchive: <ref or inline>
    - name: step-2-deploy
      manifestBundle: <ref or inline>
      fileArchive: <ref or inline>
```

TODO: Finalize the CRD schema. Define which fields are overridable at instance level vs locked at template level.

TODO: Define how compiled step artifacts are referenced — inline in CRD? ConfigMap references? External storage references?

---

## WorkspaceInstance

### Purpose

Represents a single active workspace. This is the runtime object that the [Operator](./operator.md) reconciles.

### Contains

- Template reference
- Owner(s) / assignee(s)
- Isolation mode (inherited or overridden)
- Lifecycle mode
- TTL
- Cluster mode
- Current step
- Status fields

### Lifecycle

- **Created by:** CLI (or future API)
- **Consumed by:** Operator
- **Scope:** Namespaced

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
    ssh: ws-kubernetes-101-student-42.example.com:22
    webTerminal: https://ws-kubernetes-101-student-42.example.com/terminal
  conditions: [...]
```

TODO: Define the full status subresource (conditions, phase transitions, error reporting).

TODO: Define how step transitions are requested — update to `spec.currentStep`? A separate sub-resource? An API call?

---

## CRD Versioning

TODO: Define CRD versioning strategy (v1alpha1 → v1beta1 → v1) and conversion webhook requirements.

## Validation

TODO: Define admission webhook validation rules (beyond OpenAPI schema validation).

## RBAC

TODO: Define which roles can create/read/update/delete Templates vs Instances.
