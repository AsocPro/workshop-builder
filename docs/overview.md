# Workshop Platform — Architecture Overview

## What This Is

A Kubernetes-native platform for building and running technical Linux-based workshops. Workshop content and infrastructure state are packaged as a single portable SQLite artifact, providing deterministic, resettable learning environments.

## Architecture Sections

The system has four layers, each with clear responsibilities and boundaries.

---

### 1. [Workshop Definition](./definition/) — The Inputs

Everything an author creates to define a workshop.

| Doc | What It Covers |
|---|---|
| [Workload Layer](./definition/workload-layer.md) | `docker-compose.yml` — container topology |
| [Workspace Metadata](./definition/workspace-metadata.md) | `workspace.yaml` — lifecycle, isolation, cluster mode, access |
| [Authoring](./definition/authoring.md) | Builder mode — incremental step creation, snapshot capture |

An author works with Compose files, workspace config, and a live builder environment. Their output feeds into compilation.

---

### 2. [Core Platform](./platform/) — The Engine

Shared logic, orchestration, and runtime enforcement. This is the system that actually provisions, manages, resets, and tears down workspaces.

| Doc | What It Covers |
|---|---|
| [Shared Go Library](./platform/shared-go-library.md) | Canonical types, validation, translation — used by everything |
| [CLI](./platform/cli.md) | Administration surface — local mode, cluster mode, provisioning |
| [CRDs](./platform/crds.md) | WorkspaceTemplate + WorkspaceInstance — cluster API objects |
| [Operator](./platform/operator.md) | Multi-tenant enforcement, step transitions, reset semantics, lifecycle |
| [Infrastructure Provisioners](./platform/infrastructure-provisioners.md) | k3d, k3s, vcluster orchestration |
| [Backend Capabilities](./platform/backend-capabilities.md) | Docker vs Kubernetes feature matrix |

The operator owns step transitions and reset logic — these aren't a separate "content" concern, they're core platform behavior that involves namespace management, manifest application, and PVC replacement.

---

### 3. [Artifact & Distribution](./artifact/) — The Package

How workshops are compiled into portable, distributable artifacts.

| Doc | What It Covers |
|---|---|
| [Compilation](./artifact/compilation.md) | Flattening authoring snapshots into deterministic per-step artifacts |
| [SQLite Artifact](./artifact/sqlite-artifact.md) | Single-file distribution format, schema, YAML export/import |

Compilation transforms messy incremental authoring state into clean, self-contained step bundles. The SQLite file is the portable source of truth.

---

### 4. [Presentation](./presentation/) — The Interface

How users interact with the platform.

| Doc | What It Covers |
|---|---|
| [Frontend](./presentation/frontend.md) | Student mode (step navigation, tutorials) + Builder mode (step editing, compilation) |
| [GUI](./presentation/gui.md) | Wails desktop app for workshop administration |

Presentation layers are thin — they trigger backend operations and display results.

---

## Core Principles

1. **Kubernetes is the authoritative multi-tenant control plane.**
2. **Docker Compose defines workload topology only** — never lifecycle or isolation.
3. **Runtime must be simpler than authoring.**
4. **Reset must be deterministic** — jumping to any step produces identical state.
5. **Compilation flattens mutation history** — no diffs or patch chains at runtime.
6. **The SQLite file is the portable source of truth.**
7. **Storage cost is acceptable; operational complexity is not.**
8. **Feature parity is NOT required across backends** — semantics must be clear.

## System Flow

```
Author creates:
  docker-compose.yml + workspace.yaml + step content (via builder mode)
                            |
                       Compilation
                            |
                     SQLite Artifact
                            |
            CLI submits to cluster (or runs locally)
                            |
                   CRDs created in K8s
                            |
                 Operator reconciles:
                   - provisions namespace
                   - applies manifests
                   - manages lifecycle
                   - handles step transitions & resets
                            |
                 Frontend displays to student
```

## What This System Is NOT

- A container layer simulator
- A Velero-style cluster backup system
- A diff-replay engine or patch-chain executor
- A system where Compose is the control plane
- A system that simulates namespaces in Docker
- A system requiring feature parity across backends

## Project Status

TODO: Add current implementation status and phase roadmap.
