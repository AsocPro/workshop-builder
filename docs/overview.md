# Workshop Platform — Architecture Overview

## What This Is

A Kubernetes-native platform for building and running technical Linux-based workshops. Workshop content is packaged as:

1. A **SQLite artifact** containing educational metadata and OCI image references
2. **Tagged OCI images** in a container registry, one per workshop step

Together these form the portable, distributable workshop. The SQLite file is small (under 5MB); the images live in the registry with OCI layer deduplication.

## Architecture Sections

The system has four layers, each with clear responsibilities and boundaries.

---

### 1. [Workshop Definition](./definition/) — The Inputs

Everything an author creates to define a workshop.

| Doc | What It Covers |
|---|---|
| [Step Spec](./definition/step-spec.md) | `step-spec.yaml` — per-step container image build recipes |
| [Workspace Metadata](./definition/workspace-metadata.md) | `workspace.yaml` — lifecycle, isolation, cluster mode, access |
| [Authoring](./definition/authoring.md) | CLI proxy model — recording commands and files to step-spec.yaml |

An author writes a `step-spec.yaml` (via the CLI proxy or directly) and a `workspace.yaml`. Their output feeds into compilation.

---

### 2. [Core Platform](./platform/) — The Engine

Shared logic, orchestration, and runtime enforcement. This is the system that actually provisions, manages, resets, and tears down workspaces.

| Doc | What It Covers |
|---|---|
| [Shared Go Library](./platform/shared-go-library.md) | Canonical types, validation, CRD generation — used by everything |
| [CLI](./platform/cli.md) | Administration surface — local mode, cluster mode, build commands |
| [CRDs](./platform/crds.md) | WorkspaceTemplate + WorkspaceInstance — cluster API objects |
| [Operator](./platform/operator.md) | Multi-tenant enforcement, step transitions (image swap), lifecycle |
| [Infrastructure Provisioners](./platform/infrastructure-provisioners.md) | k3d, k3s, vcluster orchestration |
| [Backend Capabilities](./platform/backend-capabilities.md) | Docker vs Kubernetes feature matrix |

The operator owns step transitions — these are image swaps: update Deployment spec → rollout → done. No namespace teardown or PVC restoration.

---

### 3. [Artifact & Distribution](./artifact/) — The Package

How workshops are compiled into portable, distributable artifacts.

| Doc | What It Covers |
|---|---|
| [Compilation](./artifact/compilation.md) | Dagger build pipeline — step-spec.yaml → OCI images + SQLite |
| [SQLite Artifact](./artifact/sqlite-artifact.md) | Metadata-only distribution format, schema, YAML export/import |

Compilation transforms `step-spec.yaml` into a set of tagged OCI images and an updated SQLite file. The SQLite file contains no blobs — only metadata and image references.

---

### 4. [Presentation](./presentation/) — The Interface

How users interact with the platform.

| Doc | What It Covers |
|---|---|
| [Frontend](./presentation/frontend.md) | Student mode (step navigation, tutorials) + Builder mode (step editing, compilation) |
| [GUI](./presentation/gui.md) | Wails desktop app for workshop administration and local mode |

Presentation layers are thin — they trigger backend operations and display results.

---

## Core Principles

1. **Kubernetes is the authoritative multi-tenant control plane.**
2. **`step-spec.yaml` defines what container images contain** — never lifecycle or isolation.
3. **Runtime must be simpler than authoring.**
4. **Reset must be deterministic** — jumping to any step produces identical state.
5. **Each step is a complete OCI image** — no diffs or patch chains at runtime.
6. **SQLite + registry together form the portable distribution.**
7. **Storage cost is acceptable; operational complexity is not.**
8. **Feature parity is NOT required across backends** — semantics must be clear.

## System Flow

```
Author creates:
  step-spec.yaml + workspace.yaml
  (written via CLI proxy or directly)
            │
    workshop build compile
    (Dagger pipeline)
            │
     OCI images pushed          SQLite updated
     to registry                (metadata + image tags)
            │                         │
            └──────────┬──────────────┘
                       │
          CLI reads SQLite + workspace.yaml
                       │
          ┌────────────┴─────────────────┐
          │                              │
    Local mode                    Cluster mode
    (docker run                   (CRDs created
     step image)                   in K8s)
                                        │
                               Operator reconciles:
                                 - provisions namespace
                                 - deploys step image
                                 - manages lifecycle
                                 - handles step transitions
                                   (image swap)
                                        │
                              Frontend displays to student
```

## What This System Is NOT

- A container layer simulator
- A Velero-style cluster backup system
- A diff-replay engine or patch-chain executor
- A system where Compose is the control plane
- A system that simulates namespaces in Docker
- A system requiring feature parity across backends
- A system that stores file state in SQLite

## Project Status

TODO: Add current implementation status and phase roadmap.
