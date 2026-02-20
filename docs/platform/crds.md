# Custom Resource Definitions — WorkspaceTemplate & WorkspaceInstance

## Overview

Two CRDs define the cluster-level control plane for workspace management. These are the API objects that the [CLI](./cli.md) creates and the [Operator](./operator.md) reconciles.

---

## WorkspaceTemplate

### Purpose

Cluster-level reusable workspace definition. Represents a workshop's infrastructure blueprint that can be instantiated multiple times.

### Contains

- Workspace metadata defaults (from `workspace.yaml`)
- Resource configuration
- Access surface configuration
- Workshop step definitions (OCI image tags from SQLite)
- Image pull secrets

### Lifecycle

- **Created by:** CLI (reads SQLite image tags; generates CRD via [Shared Go Library](./shared-go-library.md))
- **Consumed by:** Operator
- **Scope:** Cluster-scoped (available across namespaces)

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
      imageDigest: myorg/kubernetes-101:step-1-intro-sha256-abc123

    - id: step-2-deploy
      title: "Deploy the App"
      imageTag: myorg/kubernetes-101:step-2-deploy
      imageDigest: myorg/kubernetes-101:step-2-deploy-sha256-def456
```

`steps[].imageTag` and `steps[].imageDigest` are populated from the SQLite artifact by the CLI at template creation time. The operator reads these to perform step transitions — no manifest bundles or file archives are referenced.

TODO: Finalize the CRD schema. Define which fields are overridable at instance level vs locked at template level.

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
