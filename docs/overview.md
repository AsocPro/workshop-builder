# Workshop Platform — Architecture Overview

## What This Is

A Kubernetes-native platform for building and running technical Linux-based workshops. Workshop content is packaged as **tagged OCI images** in a container registry, one per workshop step. Each image is self-contained — all metadata (step definitions, tutorial markdown, goss specs, LLM configuration) is baked in as flat files. A workshop runs with just:

```bash
docker run -p 8080:8080 myorg/kubernetes-101:step-1-intro
```

No CLI required, no external database, no separate configuration.

## Architecture Sections

The system has four layers, each with clear responsibilities and boundaries.

---

### 1. [Workshop Definition](./definition/) — The Inputs

Everything an author creates to define a workshop.

| Doc | What It Covers |
|---|---|
| [Workshop Spec](./definition/workshop.md) | `workshop.yaml` — per-step container image build recipes, tutorial content, navigation, LLM config |
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
| [Backend Service](./platform/backend-service.md) | Go binary embedded in every step image — serves web UI, proxies terminal, reads flat files, writes JSONL, manages recording and LLM help |
| [Base Images](./platform/base-images.md) | Platform foundation layers (`workshop-base:{alpine,ubuntu,centos}`) with all tooling pre-installed |
| [Instrumentation](./platform/instrumentation.md) | Shell command logging (PROMPT_COMMAND) and asciinema terminal recording |
| [LLM Help](./platform/llm-help.md) | Contextual student assistance — reads command history + goss results + docs, gives hints |
| [Instructor Dashboard](./platform/instructor-dashboard.md) | Real-time instructor visibility — Docker (local) and Kubernetes (aggregated) modes |
| [Aggregation](./platform/aggregation.md) | Vector sidecar ships JSONL to Postgres/S3 in Kubernetes mode |
| [Infrastructure Provisioners](./platform/infrastructure-provisioners.md) | k3d, vcluster orchestration |
| [Backend Capabilities](./platform/backend-capabilities.md) | Docker vs Kubernetes feature matrix |

The operator owns step transitions — these are image swaps: update Deployment spec → rollout → done. No namespace teardown or PVC restoration.

The backend service runs inside every workspace container. It is the runtime bridge between the student browser and the workspace — serving the web UI, proxying terminal WebSocket connections, managing asciinema recording, and reading the flat file metadata baked into the image.

---

### 3. [Artifact & Distribution](./artifact/) — The Package

How workshops are compiled into portable, distributable artifacts.

| Doc | What It Covers |
|---|---|
| [Compilation](./artifact/compilation.md) | Dagger build pipeline — `workshop.yaml` → OCI images with baked-in metadata |
| [Flat File Artifact](./artifact/flat-file-artifact.md) | In-image metadata format (`/workshop/` filesystem layout) |

Compilation transforms `workshop.yaml` into a set of tagged OCI images. Each image contains the complete workshop metadata as flat files under `/workshop/` — no separate database or distribution artifact.

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
6. **The container image IS the workshop** — no separate distribution artifact.
7. **Storage cost is acceptable; operational complexity is not.**
8. **Feature parity is NOT required across backends** — semantics must be clear.
9. **The student container is identical in Docker and Kubernetes mode** — aggregation is bolted on via sidecar.

## System Flow

```
Author creates:
  workshop.yaml
  (written via CLI proxy or directly)
            │
    workshop build compile
    (Dagger pipeline)
            │
     OCI images pushed to registry
     (one per step, self-contained)
     Each image contains:
       - /workshop/ metadata (ALL steps)
       - /workspace/ content (THIS step)
       - Platform tooling (backend, goss, asciinema, bashrc)
            │
            │
   ┌────────┴──────────────────┐
   │                           │
 Local mode                 Cluster mode
 (docker run step image)    (WorkspaceTemplate + WorkspaceInstance CRDs)
   │                           │
 Backend starts:            Operator reconciles:
   - reads /workshop/*        - provisions namespace
   - replays state events     - deploys step image with Vector sidecar
   - spawns ttyd+asciinema    - manages lifecycle
   - serves student web UI    - step transitions via image swap
   - serves instructor view       │
   - writes JSONL files        Backend starts (same image, same behavior)
                                 - writes JSONL files
                                 - Vector sidecar ships to Postgres/S3
                                 - Instructor dashboard aggregates
```

## Image Layer Structure

```
workshop-base:ubuntu (maintained by platform team)
  ├── tini
  ├── workshop-backend binary (embedded web UI assets)
  ├── asciinema
  ├── goss
  ├── /etc/workshop-platform.bashrc (PROMPT_COMMAND instrumentation)
  └── ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
         │
         ▼ author layers (via workshop.yaml build)
myorg/kubernetes-101:step-1-intro
  ├── apt-get install kubectl helm ...     (author's packages)
  ├── /workshop/workshop.json              (workshop identity + step list)
  ├── /workshop/steps/*/                   (ALL steps' metadata)
  │   ├── meta.json, content.md, goss.yaml, llm.json
  └── /workspace/...                       (step-specific content files)
```

## What This System Is NOT

- A container layer simulator
- A Velero-style cluster backup system
- A diff-replay engine or patch-chain executor
- A system where Compose is the control plane
- A system that simulates namespaces in Docker
- A system requiring feature parity across backends
- A system that stores metadata in SQLite
- A system with a separate distribution artifact
- A system with an author-facing file for deployment/operator configuration

## Open Architectural Questions

The following questions must be resolved before implementation. They are tracked as TODOs in the relevant docs:

1. **Step transition mechanism in Docker local mode.** The backend runs inside the container and cannot restart itself. How does a student's browser action (via backend API) trigger the CLI on the host to swap containers? See [CLI](./platform/cli.md) and [Backend Service](./platform/backend-service.md).

2. **Navigation vs image swap UX.** In free/guided navigation, students need to view any step's content without disrupting their terminal session. The distinction between "view step" (no restart) and "switch workspace to step" (image swap) must be clear in the API and UI. See [Workshop Spec](./definition/workshop.md#navigation-vs-image-swap) and [Frontend](./presentation/frontend.md).

3. **State persistence across Docker mode step transitions.** When a container is replaced, `/workshop/runtime/` is lost unless a volume is mounted. Should completion progress persist? If so, what's the volume mount convention? See [Backend Service](./platform/backend-service.md).

4. **LLM API key distribution for Docker mode.** The student runs `docker run -e WORKSHOP_LLM_API_KEY=...`. Who provides the key? Is this an instructor responsibility? See [LLM Help](./platform/llm-help.md).
