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
| [Workshop Spec](./definition/workshop.md) | `workshop.yaml` — per-step container image build recipes and tutorial content |
| [Authoring](./definition/authoring.md) | CLI proxy model — recording commands and files to `workshop.yaml` |

An author writes a single `workshop.yaml` (via the CLI proxy or directly). Deployment behavior (lifecycle, isolation, cluster mode, resources, access) is operator configuration — it lives in the [WorkspaceTemplate CRD](./platform/crds.md), not in any author-facing file.

---

### 2. [Core Platform](./platform/) — The Engine

Shared logic, orchestration, and runtime enforcement. This is the system that actually provisions, manages, resets, and tears down workspaces.

| Doc | What It Covers |
|---|---|
| [Shared Go Library](./platform/shared-go-library.md) | Canonical types, validation, CRD generation — used by everything |
| [CLI](./platform/cli.md) | Administration surface — local mode, cluster mode, build commands |
| [CRDs](./platform/crds.md) | WorkspaceTemplate + WorkspaceInstance — cluster API objects and operator config |
| [Operator](./platform/operator.md) | Multi-tenant enforcement, step transitions (image swap), lifecycle |
| [Backend Service](./platform/backend-service.md) | Go binary embedded in every step image — serves web UI, proxies terminal, owns SQLite |
| [Infrastructure Provisioners](./platform/infrastructure-provisioners.md) | k3d, vcluster orchestration |
| [Backend Capabilities](./platform/backend-capabilities.md) | Docker vs Kubernetes feature matrix |

The operator owns step transitions — these are image swaps: update Deployment spec → rollout → done. No namespace teardown or PVC restoration.

The backend service runs inside every workspace container. It is the runtime bridge between the student browser and the workspace — serving the web UI, proxying terminal WebSocket connections, and reading/writing the per-instance SQLite database.

---

### 3. [Artifact & Distribution](./artifact/) — The Package

How workshops are compiled into portable, distributable artifacts.

| Doc | What It Covers |
|---|---|
| [Compilation](./artifact/compilation.md) | Dagger build pipeline — `workshop.yaml` → OCI images + SQLite |
| [SQLite Artifact](./artifact/sqlite-artifact.md) | Metadata-only distribution format, schema, YAML export/import |

Compilation transforms `workshop.yaml` into a set of tagged OCI images and an updated SQLite file. The SQLite file contains no blobs — only metadata and image references.

---

### 4. [Presentation](./presentation/) — The Interface

How users interact with the platform. There are two separate interfaces for two distinct audiences.

| Doc | What It Covers |
|---|---|
| [Frontend](./presentation/frontend.md) | Student-facing web UI — served by the backend inside the workspace container |
| [GUI](./presentation/gui.md) | Wails desktop app for workshop authors — builder mode, compilation, workstation-side tools |

The student UI is served directly from inside the workspace container by the backend service. The builder GUI is a separate binary that runs on the author's workstation and interacts with local tools (Docker, Dagger).

---

## Core Principles

1. **Kubernetes is the authoritative multi-tenant control plane.**
2. **`workshop.yaml` defines what container images contain** — never lifecycle or isolation.
3. **Runtime must be simpler than authoring.**
4. **Reset must be deterministic** — jumping to any step produces identical state.
5. **Each step is a complete OCI image** — no diffs or patch chains at runtime.
6. **SQLite + registry together form the portable distribution.**
7. **Storage cost is acceptable; operational complexity is not.**
8. **Feature parity is NOT required across backends** — semantics must be clear.

## System Flow

```
Author creates:
  workshop.yaml
  (written via CLI proxy or directly)
            │
    workshop build compile
    (Dagger pipeline)
            │
     OCI images pushed          SQLite updated       Backend binary
     to registry                (metadata +          injected into
     (one per step)              image tags)         each step image
            │                         │                    │
            └──────────┬──────────────┘                    │
                       │              (backend embedded in images)
          CLI reads SQLite artifact
                       │
          ┌────────────┴─────────────────┐
          │                              │
    Local mode                    Cluster mode
    (docker run step image)       (WorkspaceTemplate + WorkspaceInstance CRDs)
          │                              │
    Backend starts in container   Operator reconciles:
      - serves student web UI       - provisions namespace
      - spawns ttyd                 - deploys step image
      - reads/writes SQLite         - manages lifecycle
      - step transitions via          - step transitions via
        CLI stop/start                  image swap
                                        │
                                Backend starts in new pod
                                  - serves student web UI
                                  - spawns ttyd
                                  - reads/writes SQLite
```

## What This System Is NOT

- A container layer simulator
- A Velero-style cluster backup system
- A diff-replay engine or patch-chain executor
- A system where Compose is the control plane
- A system that simulates namespaces in Docker
- A system requiring feature parity across backends
- A system that stores file state in SQLite
- A system with an author-facing file for deployment/operator configuration

## Project Status

TODO: Add current implementation status and phase roadmap.
