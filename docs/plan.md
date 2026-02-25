# Documentation Build-Out Plan

Work through this list top to bottom. Each item is ordered by dependency — earlier docs inform later ones. Check off items as you complete them.

The plan is organized around a **single-user local mode milestone first**. Cluster mode (CRDs, operator, provisioners) is deferred until local mode is fully documented and implemented.

---

## Phase 1: Workshop Specification (COMPLETE)

These are the foundation. Everything downstream — types, CRDs, operator behavior, compilation — is shaped by what you decide here.

---

### 1. Workshop Spec (`definition/workshop.md`)

The `workshop.yaml` format is the single most consequential input decision. It determines what the shared library parses, what Dagger builds, what the CRDs contain, and what the operator deploys.

- [x] Define the workshop.yaml schema (version, workshop, base, steps).
- [x] Document how Dagger consumes the spec (sequential builds, layer inheritance).
- [x] Define validation rules and error messages.
- [x] Add example workshop.yaml files for common workshop patterns.
- [x] Document the image tagging convention (`<workshop.image>:<step-id>`).
- [x] Document incremental rebuild strategy (`--from-step`).
- [x] Add `markdown` / `markdownFile` fields per step.
- [x] Clarify file deletion — explicit `rm` commands only.
- [x] Rename workshop.yaml → workshop.yaml (done; step-spec.md marked superseded).
- [x] Add navigation modes (`linear`, `free`, `guided`) and step `group`/`requires` fields.
- [x] Add workshop-level and per-step LLM configuration.
- [x] Drop all SQLite references — metadata baked as flat files into image.
- [x] Document base image usage (`workshop-base:{alpine,ubuntu,centos}`).

---

### 2. Workspace Metadata (`definition/workspace-metadata.md`)

**DISSOLVED.** Lifecycle, isolation, cluster mode, resources, and access are now operator configuration in the WorkspaceTemplate CRD. There is no author-facing `workspace.yaml` file. `workspace-metadata.md` is now a tombstone pointing to `crds.md`.

- [x] Dissolve workspace-metadata.md — fields moved to WorkspaceTemplate CRD.
- [x] Remove workspace.yaml as an author-facing file entirely.
- [ ] Define team membership model — `individual` only for v1, `team` schema-reserved in WorkspaceTemplate.
- [ ] Define default resource values, validation behavior, and quota level — post-v1 robustness work.
- [ ] Define how the CLI detects and selects between Docker and Podman runtimes in local mode.

### 2b. Navigation vs Image Swap (CRITICAL — Cross-cutting)

- [ ] **Define the distinction between "view step content" and "switch workspace to step" in the API and UX.** In free/guided mode, students must browse content without triggering a container restart. The current `POST /api/steps/:id/navigate` is ambiguous. Likely needs two separate actions.
- [ ] **Define the step transition mechanism in Docker local mode.** The backend is inside the container and cannot restart itself. Options: CLI polling, Docker socket access, or host-side CLI commands. This blocks the single-user milestone.
- [ ] **Define state persistence across Docker mode step transitions.** Should `/workshop/runtime/` be volume-mounted to preserve completion progress? What volume mount convention?
- [ ] **Define browser reconnection behavior after step transitions.** The WebSocket drops when the container is replaced. Auto-reconnect? Same port?

---

## Phase 2: Artifact Pipeline (COMPLETE)

The authoring and build pipeline. These produce the artifacts consumed by the runtime — OCI images with baked-in metadata.

---

### 3. Authoring (`definition/authoring.md`)

The CLI proxy model for recording workshop steps.

- [x] Rewrite for CLI proxy model (no K8s snapshots).
- [x] Document what the proxy records (files, env, commands → workshop.yaml).
- [x] Document step editing (edit YAML directly) and step reordering.
- [x] Document version control integration (commit workshop.yaml).
- [x] Document collaboration model (Git-based).
- [x] Define whether simultaneous proxy sessions from multiple authors are supported.
- [x] Update references from SQLite to flat file compilation.

---

### 4. Compilation (`artifact/compilation.md`)

The Dagger build pipeline.

- [x] Reframe as Dagger pipeline (not snapshot flattening).
- [x] Document sequential step building and OCI layer inheritance.
- [x] Document incremental rebuild strategy (`--from-step`).
- [x] Document size expectations.
- [x] Document platform layer injection (tini + backend + goss + asciinema + bashrc).
- [x] Document metadata baking — flat files under `/workshop/` instead of SQLite.
- [x] Document workshop.json generation and per-step metadata files.
- [x] Document base image usage and custom base image injection.
- [x] Remove all SQLite references.
- [ ] Define what validation occurs during the Dagger build (e.g., RUN command failure behavior).
- [ ] Provide size estimates for typical OCI image stacks.
- [ ] Define how the backend binary version is pinned / sourced at compile time.
- [x] Rename `artifact/sqlite-artifact.md` to `artifact/flat-file-artifact.md` — filename was misleading since SQLite was removed.

---

### 5. Flat File Artifact (`artifact/flat-file-artifact.md`)

*Formerly "SQLite Artifact" — replaced by flat file design.*

- [x] Replace SQLite schema with flat file filesystem layout.
- [x] Document `/workshop/` read-only metadata directory structure.
- [x] Document `/workshop/runtime/` ephemeral runtime data directory.
- [x] Define workshop.json schema.
- [x] Define meta.json, content.md, goss.yaml, llm.json schemas.
- [x] Define JSONL runtime file formats (command-log, state-events, session.cast, llm-history).
- [x] Document state derivation from event log (no separate state file).
- [x] Document migration path from SQLite design.
- [x] Document distribution model (image IS the workshop — no separate artifact).

---

## Phase A: Base Images + Shell Instrumentation (NEW)

Foundation for monitoring and recording capabilities.

---

### A1. Base Images (`platform/base-images.md`)

- [x] Define three base images (alpine, ubuntu, centos) and what's included.
- [x] Document OCI layer deduplication benefits.
- [x] Document platform update workflow (rebuild base → rebuild workshops).
- [x] Document custom base image fallback (Dagger injects platform layer).
- [x] Provide example Containerfile for base image build.

---

### A2. Instrumentation (`platform/instrumentation.md`)

- [x] Document `/etc/workshop-platform.bashrc` with PROMPT_COMMAND hook.
- [x] Document command-log.jsonl format and design decisions.
- [x] Document asciinema recording setup (ttyd → asciinema rec → bash).
- [x] Document session.cast format (asciicast v2).
- [x] Document backend file watcher for command log.
- [x] Document reconnection handling and TUI support.
- [x] Document typical data volumes.

---

## Phase B: Instructor Monitoring + LLM Help (NEW)

---

### B1. Instructor Dashboard (`platform/instructor-dashboard.md`)

- [x] Document Docker mode (local, single-user — backend reads local files).
- [x] Document Kubernetes mode (aggregated — separate dashboard service).
- [x] Document SSE real-time event streaming.
- [x] Document authentication (bearer token for Docker, OIDC for K8s).
- [x] Document views: student list, student detail, completion matrix.
- [x] Document dashboard service API surface.

---

### B2. LLM Help (`platform/llm-help.md`)

- [x] Document workshop-level and per-step LLM configuration.
- [x] Document help modes (hints, explain, solve).
- [x] Document context assembly (commands + goss + markdown + instructor context + docs).
- [x] Document streaming API and history endpoint.
- [x] Document rate limiting.
- [x] Document security (API key never baked into image).
- [x] Document help panel UI concept.

---

## Phase C: Aggregation (K8s Only) (NEW)

---

### C1. Aggregation (`platform/aggregation.md`)

- [x] Document Vector sidecar with four pipelines (commands, events, recordings, LLM).
- [x] Document sidecar isolation (student container has no credentials).
- [x] Document cursor tracking and restart recovery.
- [x] Document graceful shutdown (preStop hook).
- [x] Define Postgres schema (5 tables, 3 indexes).
- [x] Provide example pod specification with sidecar.

---

## Phase 3: Backend Runtime (Single-User Mode Core)

The runtime engine running inside each workspace container. This is what makes a step image an interactive learning environment rather than just a container.

---

### 6. Backend Service (`platform/backend-service.md`)

The Go binary embedded in every step image. **The heart of single-user local mode.**

- [x] Document purpose and role — runtime engine inside each workspace container.
- [x] Document container startup sequence (tini → backend → ttyd/asciinema).
- [x] Document flat file metadata reading (replaces SQLite lifecycle).
- [x] Document state derivation from event log replay.
- [x] Document how the binary gets injected (base images or Dagger platform layer).
- [x] Document step transition behavior (starts fresh on each new container).
- [x] Document asciinema recording integration.
- [x] Document command log file watching.
- [x] Document non-linear navigation enforcement.
- [x] Document LLM help integration.
- [x] Document instructor API endpoints (Docker mode).
- [x] Document SSE event streaming.
- [x] Define the full API surface (student + instructor routes).
- [x] Document connection tracking.
- [ ] Define frontend framework and asset embedding strategy.
- [ ] Define the step transition mechanism in Docker local mode (CRITICAL — blocks single-user milestone).
- [ ] Define "view step" vs "transition to step" API contract — `POST /api/steps/:id/navigate` is ambiguous.
- [ ] Define state persistence across step transitions in Docker mode (volume mount convention).
- [ ] Define periodic goss validation configuration mechanism (env var? workshop.yaml field?).
- [ ] Define browser auto-reconnection behavior after container replacement during step transitions.

---

## Phase 4: Student UI

The interface served by the backend service. Defines what students actually see and interact with.

---

### 7. Frontend / Student UI (`presentation/frontend.md`)

- [x] Establish that student UI is served by the backend binary inside the workspace container.
- [x] Establish that builder mode is a separate binary (Wails app), not a mode of this frontend.
- [x] Remove ambiguity about whether frontend is SPA vs Wails-embedded vs separately served.
- [ ] Define the student-facing API surface (routes, REST vs WebSocket).
- [ ] Define the cluster status panel.
- [ ] Define the frontend framework.
- [ ] Define markdown rendering capabilities.
- [ ] Define authentication.
- [ ] Define target devices.
- [ ] Design the LLM help panel component.
- [ ] Design the non-linear navigation UI (completion matrix vs linear progress bar).
- [ ] Design the UX distinction between "view step content" (no restart) and "switch workspace" (image swap + restart). Critical for free/guided navigation modes.

---

## Phase 5: CLI — Local Mode

Focus on the local mode command surface. Cluster-specific CLI work (batch provisioning, K8s auth) is deferred to Phase 8.

---

### 8. CLI (`platform/cli.md`)

- [x] Update Local Mode flow (step image pull + run, not Compose).
- [x] Update Cluster Mode (CRD generation uses image tags).
- [x] Add full "Build Commands" section (proxy, compile, step save/new, status).
- [ ] Define the full CLI command tree — focus on local mode first (`workshop run`, `workspace reset`, `workspace list`).
- [ ] Define CLI configuration model (config file, env vars, registry credentials).
- [ ] Define error handling strategy — validation errors, image pull failures, partial states.
- [ ] *(defer to Phase 8)* Define how batch workshop provisioning works.
- [ ] *(defer to Phase 8)* Define how the CLI authenticates to Kubernetes clusters.

---

## Phase 6: Builder GUI

The Wails desktop app for authors. Logically part of the single-user authoring loop.

---

### 9. Builder GUI (`presentation/gui.md`)

- [x] Establish as builder-mode-only Wails desktop app (not student local mode client).
- [x] Document responsibilities: authoring, compilation, cluster administration.
- [ ] Define whether the GUI imports CLI logic as Go packages or invokes subprocesses.
- [ ] Define the specific feature set for v1.
- [ ] Define the frontend framework used within Wails.
- [ ] Define how the GUI is packaged and distributed.

---

## — MILESTONE: Single-User Local Mode —

*Phases 1–6 define and implement everything required for a single user to author a workshop, compile it, and run it locally with full monitoring, recording, and optional LLM help. Before proceeding to Phase 7, the single-user flow should be fully documented and working end-to-end.*

---

## Phase 7: Domain Model & API Surface (Cluster Mode Prerequisite)

Before building cluster mode, lock down the Go types and Kubernetes API objects.

---

### 10. Shared Go Library (`platform/shared-go-library.md`)

This is the glue. Every component imports it.

- [x] Replace Compose parser with step-spec parser (`pkg/stepspec`).
- [x] Remove `pkg/translate` (no Compose-to-K8s translation needed).
- [x] Document CRD generation responsibility (workspace metadata + image tags → CRD objects).
- [ ] Define the Go package layout (`pkg/stepspec`, `pkg/workspace`, `pkg/crd`, `pkg/capability`, `pkg/types`).
- [ ] Add packages for new features: `pkg/commandlog`, `pkg/recording`, `pkg/instructor`, `pkg/llm`, `pkg/state`.
- [ ] Define testing approach — unit tests for validation, integration tests for CRD generation, golden file tests.
- [ ] Define how this library is versioned relative to CRD versions and CLI releases.

---

### 11. CRDs (`platform/crds.md`)

The Kubernetes API contract. Encodes workspace metadata and step image references.

- [x] Replace `workload.*` fields with `steps[].imageTag`, `steps[].imageDigest`, `imagePullSecrets`.
- [x] Remove manifest bundle and file archive references.
- [ ] Finalize the CRD schema. Define which fields are overridable at instance level vs locked at template level.
- [ ] Define the full status subresource (conditions, phase transitions, error reporting).
- [ ] Define how step transitions are requested — update to `spec.currentStep`? Separate sub-resource?
- [ ] Define CRD versioning strategy (v1alpha1 → v1beta1 → v1) and conversion webhook requirements.
- [ ] Define admission webhook validation rules.
- [ ] Define which roles can create/read/update/delete Templates vs Instances.

---

## Phase 8: Cluster Mode Runtime

---

### 12. Operator (`platform/operator.md`)

- [x] Replace step transition flow with image-swap flow.
- [x] Remove "File and PVC Strategy" subsection.
- [x] Add "restore PVC contents on step transition" to What It Does NOT Do.
- [x] Update reset semantics — determinism guaranteed by OCI image immutability.
- [ ] Define what "platform system components" are excluded from namespace cleanup.
- [ ] Define the reconciliation flow for WorkspaceTemplate and WorkspaceInstance controllers.
- [ ] Define how the operator configures the Vector sidecar in pod specs.
- [ ] Define how the operator handles partial failures (image pull failure, rollout stall, etc.).
- [ ] Define acceptable step transition times.
- [ ] Define retention policy for runtime snapshots.
- [ ] Define operator scaling strategy.
- [ ] Define metrics, events, and logging strategy.
- [ ] Define how the operator is deployed.

---

### 13. CLI — Cluster Mode (`platform/cli.md`, continued)

- [ ] Define how batch workshop provisioning works.
- [ ] Define how the CLI authenticates to Kubernetes clusters.

---

### 14. Infrastructure Provisioners (`platform/infrastructure-provisioners.md`)

- [x] Update key constraint note (workshop.yaml, not docker-compose.yml).
- [x] Remove k3s — k3d only for local nested clusters.
- [ ] Define how provisioner versions are managed and how Kubernetes version selection works.
- [ ] Define cleanup procedures for provisioned clusters.
- [ ] Document resource overhead of each provisioner.

---

### 15. Backend Capabilities (`platform/backend-capabilities.md`)

- [x] Step transitions row: Docker = "Image swap (CLI-managed)"; K8s = "Image swap (Operator-managed)".
- [x] Add "Dagger build pipeline" row.
- [x] Remove PVC/manifest TODO references.
- [x] Remove k3s — k3d is the only local nested cluster tool.
- [x] Add monitoring and recording capabilities.
- [x] Add LLM help and instructor dashboard capabilities.
- [ ] Define exact validation error messages for each unsupported capability per backend.

---

## Phase 9: Finalize

---

### 16. Overview (`overview.md`)

- [x] Update "What This Is" (image-only distribution, no SQLite).
- [x] Update layer 1 table (workshop-spec, authoring).
- [x] Update core principles (image IS the workshop, sidecar-based aggregation).
- [x] New system flow diagram.
- [x] Add monitoring, recording, LLM, dashboard to platform layer table.
- [x] Update image layer structure diagram.
- [x] Update "What This Is NOT" (no SQLite, no separate artifact).
- [x] Add base images, instrumentation, aggregation to architecture sections.
- [x] Review all cross-links and ensure consistency across docs.
- [x] Remove stale SQLite references from cli.md, crds.md, shared-go-library.md, gui.md, frontend.md.
- [x] Fix aggregation.md "three pipelines" → "four pipelines".
- [x] Fix operator.md "access sidecars" → Vector sidecar (ttyd runs inside the container).
- [x] Add open architectural questions section to overview.md.
- [x] Rename `artifact/sqlite-artifact.md` to `artifact/flat-file-artifact.md`.

---

## Summary

| Phase | Focus | Docs | Status |
|---|---|---|---|
| 1 | Workshop Specification | workshop, workspace-metadata | Complete |
| 2 | Artifact Pipeline | authoring, compilation, flat-file-artifact | Complete |
| A | Base Images + Instrumentation | base-images, instrumentation | Complete |
| B | Instructor Monitoring + LLM | instructor-dashboard, llm-help | Complete |
| C | Aggregation (K8s) | aggregation | Complete |
| 3 | Backend Runtime | backend-service | **Blocked** — step transition mechanism in Docker mode undefined; nav vs image swap API ambiguous |
| 4 | Student UI | frontend | Needs design |
| 5 | CLI — Local Mode | cli (local) | **Blocked** — SQLite refs removed but step transition mechanism undefined |
| 6 | Builder GUI | gui | Needs design |
| **—** | **Single-User Milestone** | | **Blocked on: step transition mechanism, nav vs image swap, state persistence** |
| 7 | Domain Model & API | shared-go-library, crds | Partially done (SQLite refs cleaned up) |
| 8 | Cluster Mode | operator, cli (cluster), provisioners, backend-capabilities | Partially done |
| 9 | Finalize | overview | Mostly done |

**Critical path:** Resolve the step transition mechanism (Phase 3/5 blocker) and navigation vs image swap distinction (Phase 3/4 blocker) before continuing. These are cross-cutting decisions that affect backend, CLI, frontend, and workshop spec.

Work top to bottom through Phase 6, then validate the single-user mode end-to-end before continuing.
