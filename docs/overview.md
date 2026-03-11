# Workshop Platform — Architecture Overview

## What This Is

A Kubernetes-native platform for building and running technical Linux-based workshops. Workshop content is packaged as **tagged OCI images** in a container registry, one per workshop step. Each image is self-contained — all metadata (step definitions, tutorial markdown, goss specs, LLM configuration, infrastructure requirements) is baked in as flat files under `/workshop/`.

The CLI is the required entry point for running workshops. It reads the workshop's runtime specification directly from the image before starting anything:

```bash
workshop run myorg/kubernetes-101
```

No external database, no separate configuration artifact. The image IS the workshop.

## Architecture Sections

The system has four layers, each with clear responsibilities and boundaries.

---

### 1. [Workshop Definition](./definition/) — The Inputs

Everything an author creates to define a workshop.

| Doc | What It Covers |
|---|---|
| [Workshop Spec](./definition/workshop.md) | `workshop.yaml` manifest + per-step directories — build recipes, tutorial content, navigation, LLM config |
| [Authoring](./definition/authoring.md) | CLI proxy model — recording commands and files to step directories |

An author writes a `workshop.yaml` manifest (step ordering, identity, navigation) and per-step directories with `step.yaml`, `content.md`, and content files (via the CLI proxy or directly). Deployment behavior (lifecycle, isolation, cluster mode, resources, access) is operator configuration — it lives in the [WorkspaceTemplate CRD](./platform/crds.md), not in any author-facing file.

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
| [Instructor Dashboard](./platform/instructor-dashboard.md) | Real-time instructor visibility across all workspaces — Kubernetes mode only |
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
| [Compilation](./artifact/compilation.md) | Dagger build pipeline — workshop definition → OCI images with baked-in JSON metadata |
| [Flat File Artifact](./artifact/flat-file-artifact.md) | In-image metadata format (`/workshop/` filesystem layout) |

Compilation transforms the workshop definition (manifest + per-step directories) into a set of tagged OCI images. Author-facing YAML is compiled to JSON. Each image contains the complete workshop metadata as flat files under `/workshop/` — no separate database or distribution artifact.

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
2. **The workshop definition defines what container images contain** — never lifecycle or isolation.
3. **Runtime must be simpler than authoring.**
4. **Reset must be deterministic** — jumping to any step produces identical state.
5. **Each step is a complete OCI image** — no diffs or patch chains at runtime.
6. **The container image IS the workshop** — no separate distribution artifact. The CLI reads everything it needs from the image.
7. **The CLI is the required entry point for local mode** — bare `docker run` is not supported. Consistent orchestration regardless of workshop complexity.
8. **Storage cost is acceptable; operational complexity is not.**
9. **Feature parity is NOT required across backends** — semantics must be clear.
10. **The student container is identical in Docker and Kubernetes mode** — aggregation is bolted on via sidecar.

## System Flow

```
Author creates:
  workshop.yaml (manifest — steps, navigation, infrastructure requirements)
  steps/<id>/step.yaml + content.md + files/
  (written via CLI proxy or directly)
            │
    workshop build compile
    (Dagger pipeline — compiles YAML → JSON, builds OCI images)
            │
     OCI images pushed to registry
     (one per step, self-contained)
     Each image contains:
       - /workshop/workshop.json  (identity, steps, infrastructure spec)
       - /workshop/steps/*/       (ALL steps' metadata)
       - /workspace/...           (THIS step's content files)
       - Platform tooling         (backend, goss, asciinema, bashrc)
            │
            │
   ┌────────┴──────────────────────┐
   │                               │
 Local mode                   Cluster mode
 (workshop run — CLI required)  (WorkspaceTemplate + WorkspaceInstance CRDs)
   │                               │
 CLI reads workshop.json        Operator reconciles:
   from first step image          - provisions namespace
 CLI provisions infrastructure:  - deploys step image with Vector sidecar
   - k3d cluster (if needed)     - manages lifecycle
   - extraContainers              - step transitions via image swap
   - port mappings                    │
 CLI starts management server   Backend starts (same image, same behavior)
 CLI runs workspace container     - reads /workshop/*
   │                               - spawns ttyd+asciinema
 Backend starts:                  - serves student web UI
   - reads /workshop/*             - writes JSONL files
   - spawns ttyd+asciinema         - Vector sidecar ships to Postgres/S3
   - serves student web UI         - Instructor dashboard aggregates
   - writes JSONL files
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
- A system where workshops can be run with bare `docker run` — the CLI is always required

## Open Architectural Questions

The following questions must be resolved before implementation. They are tracked as TODOs in the relevant docs:

1. **LLM API key distribution for Docker mode.** Who provides the key when a student runs a workshop locally? Is this an instructor responsibility? See [LLM Help](./platform/llm-help.md).

2. **LLM capability flag and cluster health signal.** How does the frontend know whether LLM help is configured and whether cluster mode is enabled? Should `/api/state` include a capabilities object, or is a separate `/api/capabilities` endpoint cleaner? See [Backend Service](./platform/backend-service.md) and [Frontend](./presentation/frontend.md).

3. **`extraContainers` lifecycle across step transitions.** When a student transitions to a new step, which extra containers are replaced alongside the workspace container and which persist? See [CLI](./platform/cli.md).

4. **Port auto-assignment for `extraContainers`.** The CLI maps container ports to available host ports at startup. How are these mappings surfaced to the student (management UI, stdout, env vars injected into workspace)? See [CLI](./platform/cli.md).

5. **Command history display.** Is the command history visible in the student UI, or is it only used as LLM context? If shown, is it polled periodically or fetched on demand? See [Frontend](./presentation/frontend.md).

6. **`extraContainers` in cluster mode.** How does the `infrastructure.extraContainers` block from `workshop.json` map to Kubernetes — sidecars in the workspace pod, or separate pods? See [CRDs](./platform/crds.md) and [Operator](./platform/operator.md).
