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
- [x] Add per-step `content.md` convention file for tutorial markdown.
- [x] Clarify file deletion — explicit `rm` commands only.
- [x] Rename workshop.yaml → workshop.yaml (done; step-spec.md removed).
- [x] Add navigation modes (`linear`, `free`, `guided`) and step `group`/`requires` fields.
- [x] Add workshop-level and per-step LLM configuration.
- [x] Drop all SQLite references — metadata baked as flat files into image.
- [x] Document base image usage (`workshop-base:{alpine,ubuntu,centos}`).
- [x] Add `infrastructure` block to workshop.yaml — `cluster` (enabled, provider) and `extraContainers` (name, image, ports, env).
- [x] Add validation rules for infrastructure block.
- [x] Add infrastructure examples (cluster workshop, multi-container web workshop).
- [x] Remove "Cluster provisioning config" from What It Does NOT Contain — it now does contain it.
- [x] Fix "Workshop Is a Container Image" section — CLI required, no bare `docker run`.

---

### 2. Workspace Metadata (REMOVED)

Fields moved to the [WorkspaceTemplate CRD](../platform/crds.md). No author-facing `workspace.yaml` file.

- [x] Dissolve workspace-metadata.md — fields moved to WorkspaceTemplate CRD.
- [x] Remove workspace.yaml as an author-facing file entirely.
- [ ] Define team membership model — `individual` only for v1, `team` schema-reserved in WorkspaceTemplate.
- [ ] Define default resource values, validation behavior, and quota level — post-v1 robustness work.
- [ ] Define how the CLI detects and selects between Docker and Podman runtimes in local mode.

### 2b. Navigation vs Image Swap (CRITICAL — Cross-cutting)

- [x] **Define the distinction between "view step content" and "switch workspace to step".** Content fetching (`GET /api/steps/:id/content`) is view-only — no restart. Step transitions are driven externally (CLI in local mode, Operator in K8s mode). No navigate endpoint needed.
- [x] **Progress tracking uses goss results, not navigation events.** The completion set (steps with passing goss) is the authoritative progress signal.
- [x] **Define the step transition mechanism in Docker local mode.** CLI runs a local management server on the host; passes its URL to the container via `WORKSHOP_MANAGEMENT_URL`. Backend renders it as a link in the UI. Management server handles all container lifecycle operations and survives container replacements.
- [x] **Define the student-initiated step transition surface in cluster mode.** Operator sets `WORKSHOP_MANAGEMENT_URL` pointing to an operator-hosted management endpoint. Same backend pattern as local mode — backend just renders the link.
- [x] **State persistence across step transitions — not needed.** Each container starts fresh. Students re-validate to re-establish completion status. No volume mount, no save/restore.
- [x] **Browser reconnection after step transitions.** No auto-reconnect. Management server or CLI notifies the student when the new container is ready; student reloads manually.
- [x] **Student UI does not manage lifecycle.** Step transitions, resets, and image swaps are exclusively CLI (local) and operator/instructor tooling (K8s) responsibilities. The student UI has no such controls.

---

## Phase 2: Artifact Pipeline (COMPLETE)

The authoring and build pipeline. These produce the artifacts consumed by the runtime — OCI images with baked-in metadata.

---

### 3. Authoring (`definition/authoring.md`)

- [x] Rewrite for CLI proxy model (no K8s snapshots).
- [x] Document what the proxy records (files, env, commands → workshop.yaml).
- [x] Document step editing (edit YAML directly) and step reordering.
- [x] Document version control integration (commit workshop.yaml).
- [x] Document collaboration model (Git-based).
- [x] Define whether simultaneous proxy sessions from multiple authors are supported.
- [x] Update references from SQLite to flat file compilation.

---

### 4. Compilation (`artifact/compilation.md`)

- [x] Reframe as Dagger pipeline (not snapshot flattening).
- [x] Document sequential step building and OCI layer inheritance.
- [x] Document incremental rebuild strategy (`--from-step`).
- [x] Document size expectations.
- [x] Document platform layer injection (tini + backend + goss + asciinema + bashrc).
- [x] Document metadata baking — flat files under `/workshop/` instead of SQLite.
- [x] Document workshop.json generation and per-step metadata files.
- [x] Document base image usage and custom base image injection.
- [x] Remove all SQLite references.
- [x] Include `infrastructure` block in compiled workshop.json output.
- [x] Update "Self-contained" description — CLI required, no bare `docker run`.
- [ ] Define what validation occurs during the Dagger build (e.g., RUN command failure behavior).
- [ ] Provide size estimates for typical OCI image stacks.
- [ ] Define how the backend binary version is pinned / sourced at compile time.

---

### 5. Flat File Artifact (`artifact/flat-file-artifact.md`)

- [x] Replace SQLite schema with flat file filesystem layout.
- [x] Document `/workshop/` read-only metadata directory structure.
- [x] Document `/workshop/runtime/` ephemeral runtime data directory.
- [x] Define workshop.json schema — including `infrastructure` block (cluster, extraContainers).
- [x] Define meta.json, content.md, goss.yaml, llm.json schemas.
- [x] Define JSONL runtime file formats (command-log, state-events, session.cast, llm-history).
- [x] Document state event log format (append-only; in-memory state, no startup replay).
- [x] Add `step_viewed` event to state-events schema and example — enables timestamp-based command correlation.
- [x] Document migration path from SQLite design.
- [x] Document distribution model — CLI required; image read via `docker run --rm <image> cat /workshop/workshop.json`.
- [x] Document infrastructure schema (cluster object, extraContainers array with ports and env).

---

## Phase A: Base Images + Shell Instrumentation (COMPLETE)

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

## Phase B: Instructor Monitoring + LLM Help (COMPLETE)

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

## Phase C: Aggregation (K8s Only) (COMPLETE)

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

---

### 6. Backend Service (`platform/backend-service.md`)

- [x] Document purpose and role — runtime engine inside each workspace container.
- [x] Document container startup sequence (tini → backend → ttyd/asciinema).
- [x] Document flat file metadata reading (replaces SQLite lifecycle).
- [x] Document state event log (append-only; state is in-memory, no startup replay).
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
- [x] Define frontend framework and asset embedding — Svelte 5 + Vite, embedded via `//go:embed`.
- [x] State persistence across step transitions — not needed.
- [x] Browser auto-reconnection after step transitions — not needed.
- [x] `WORKSHOP_MANAGEMENT_URL` — always set in local mode (CLI required); may be set in cluster mode.
- [x] Add `step_viewed` event to state event log on `GET /api/steps/:id/content`.
- [x] LLM help API is step-scoped — `POST /api/steps/:id/llm/help`, `GET /api/steps/:id/llm/history`.
- [ ] Define how the backend signals cluster health to the frontend (part of `/api/state` or separate endpoint).
- [ ] Define LLM capability flag — how does the frontend know LLM is configured? Include in `/api/state` or a `/api/capabilities` endpoint.
- [ ] Define command history polling behavior — periodic poll or on-demand fetch?

---

## Phase 4: Student UI

---

### 7. Frontend / Student UI (`presentation/frontend.md`)

- [x] Establish that student UI is served by the backend binary inside the workspace container.
- [x] Establish that builder mode is a separate binary (Wails app), not a mode of this frontend.
- [x] Establish that student UI has no lifecycle controls — no step transitions, no resets.
- [x] Define the frontend framework — Svelte 5 + Vite, static assets embedded in Go binary via `//go:embed`.
- [x] Define styling — Tailwind CSS, desktop-first, responsive breakpoints for tablet/mobile.
- [x] Define markdown rendering — markdown-it + highlight.js + Mermaid.js + plugins (task lists, admonitions, heading anchors).
- [x] Define authentication — none in Docker mode; OAuth2 Proxy + Authentik in cluster mode.
- [x] Define target devices — desktop-first; tablet and mobile supported.
- [x] Define the student-facing API surface (REST endpoints + WebSocket).
- [x] Define terminal embed — ttyd in an `<iframe>`, no xterm.js.
- [x] Define cluster status indicator — small colored button (green/red), shown when backend reports cluster mode enabled.
- [x] Define step management link — always shown in local mode (CLI always provides URL); conditional in cluster mode.
- [x] Define validation locking — completed steps show a static indicator; Validate button hidden to prevent false failures after step transitions.
- [x] Define `step_viewed` logging for timestamp-based command correlation.
- [x] Define LLM help as step-scoped — `POST /api/steps/:id/llm/help`.
- [x] Define UX distinction between view step content (no restart) and switch workspace (image swap + restart) — student UI is view-only.
- [ ] Design the LLM help panel component (layout, streaming render, history display).
- [ ] Design the non-linear navigation UI (step list sidebar, completion indicators, group display).
- [ ] Define command history display — shown in UI or LLM-only context?
- [ ] Define session recordings UI — how does the student access and play recordings?

---

## Phase 5: CLI — Local Mode

---

### 8. CLI (`platform/cli.md`)

- [x] Update Local Mode flow (step image pull + run, not Compose).
- [x] Update Cluster Mode (CRD generation uses image tags).
- [x] Add full "Build Commands" section (proxy, compile, step save/new, status).
- [x] Establish CLI as required entry point for local mode — bare `docker run` not supported.
- [x] Document pre-flight `workshop.json` read from image (`docker run --rm <image> cat /workshop/workshop.json`) before starting any infrastructure.
- [ ] Document `extraContainers` lifecycle — start before workspace container, stop after, handle step transitions (which containers get replaced vs kept).
- [ ] Document port auto-assignment for `extraContainers` — how host ports are selected, how mappings are surfaced in the management UI.
- [ ] Define the full CLI command tree — focus on local mode first (`workshop run`, `workspace reset`, `workspace list`).
- [ ] Define CLI configuration model (config file, env vars, registry credentials).
- [ ] Define error handling strategy — validation errors, image pull failures, partial states.
- [ ] *(defer to Phase 8)* Define how batch workshop provisioning works.
- [ ] *(defer to Phase 8)* Define how the CLI authenticates to Kubernetes clusters.

---

## Phase 6: Builder GUI

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

---

### 10. Shared Go Library (`platform/shared-go-library.md`)

- [x] Replace Compose parser with step-spec parser (`pkg/stepspec`).
- [x] Remove `pkg/translate` (no Compose-to-K8s translation needed).
- [x] Document CRD generation responsibility (workspace metadata + image tags → CRD objects).
- [ ] Define the Go package layout (`pkg/stepspec`, `pkg/workspace`, `pkg/crd`, `pkg/capability`, `pkg/types`).
- [ ] Add packages for new features: `pkg/commandlog`, `pkg/recording`, `pkg/instructor`, `pkg/llm`, `pkg/state`.
- [ ] Add `pkg/infrastructure` — parsing and validation for the `infrastructure` block (cluster, extraContainers).
- [ ] Define testing approach — unit tests for validation, integration tests for CRD generation, golden file tests.
- [ ] Define how this library is versioned relative to CRD versions and CLI releases.

---

### 11. CRDs (`platform/crds.md`)

- [x] Replace `workload.*` fields with `steps[].imageTag`, `steps[].imageDigest`, `imagePullSecrets`.
- [x] Remove manifest bundle and file archive references.
- [ ] Finalize the CRD schema. Define which fields are overridable at instance level vs locked at template level.
- [ ] Define the full status subresource (conditions, phase transitions, error reporting).
- [ ] Define how step transitions are requested — update to `spec.currentStep`? Separate sub-resource?
- [ ] Define CRD versioning strategy (v1alpha1 → v1beta1 → v1) and conversion webhook requirements.
- [ ] Define admission webhook validation rules.
- [ ] Define which roles can create/read/update/delete Templates vs Instances.
- [ ] Define how `infrastructure.extraContainers` from workshop.json maps to pod spec (sidecars vs separate pods).

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
| 1 | Workshop Specification | workshop | Complete |
| 2 | Artifact Pipeline | authoring, compilation, flat-file-artifact | Complete |
| A | Base Images + Instrumentation | base-images, instrumentation | Complete |
| B | Instructor Monitoring + LLM | instructor-dashboard, llm-help | Complete |
| C | Aggregation (K8s) | aggregation | Complete |
| 3 | Backend Runtime | backend-service | Mostly complete — capability flag + cluster health endpoint TBD |
| 4 | Student UI | frontend | Core decisions complete — LLM panel + nav UI design TBD |
| 5 | CLI — Local Mode | cli (local) | Partially complete — extraContainers lifecycle + port mapping + command tree TBD |
| 6 | Builder GUI | gui | Needs design |
| **—** | **Single-User Milestone** | | Unblocked — ready to proceed once open items above closed |
| 7 | Domain Model & API | shared-go-library, crds | Partially done — `pkg/infrastructure` needed |
| 8 | Cluster Mode | operator, cli (cluster), provisioners, backend-capabilities | Partially done |
| 9 | Finalize | overview | Mostly done |

**Remaining open items before single-user milestone:**
- Backend: cluster health signal + LLM capability flag in `/api/state` or `/api/capabilities`
- Backend: command history poll vs on-demand
- CLI: `extraContainers` lifecycle across step transitions
- CLI: port auto-assignment and display in management UI
- CLI: full command tree (`workshop run`, `workspace reset`, etc.)
- Frontend: LLM help panel component design
- Frontend: step navigation sidebar + completion UI design
- Frontend: recordings UI
