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

---

### 2. Workspace Metadata (`definition/workspace-metadata.md`)

**DISSOLVED.** Lifecycle, isolation, cluster mode, resources, and access are now operator configuration in the WorkspaceTemplate CRD. There is no author-facing `workspace.yaml` file. `workspace-metadata.md` is now a tombstone pointing to `crds.md`.

- [x] Dissolve workspace-metadata.md — fields moved to WorkspaceTemplate CRD.
- [x] Remove workspace.yaml as an author-facing file entirely.
- [ ] Define team membership model — `individual` only for v1, `team` schema-reserved in WorkspaceTemplate.
- [ ] Define default resource values, validation behavior, and quota level — post-v1 robustness work.
- [ ] Define how the CLI detects and selects between Docker and Podman runtimes in local mode.

---

## Phase 2: Artifact Pipeline (Mostly Complete)

The authoring and build pipeline. These produce the two artifacts consumed by the runtime — OCI images and the SQLite file.

---

### 3. Authoring (`definition/authoring.md`)

The CLI proxy model for recording workshop steps.

- [x] Rewrite for CLI proxy model (no K8s snapshots).
- [x] Document what the proxy records (files, env, commands → workshop.yaml).
- [x] Document step editing (edit YAML directly) and step reordering.
- [x] Document version control integration (commit workshop.yaml).
- [x] Document collaboration model (Git-based).
- [ ] Define whether simultaneous proxy sessions from multiple authors are supported.

---

### 4. Compilation (`artifact/compilation.md`)

The Dagger build pipeline.

- [x] Reframe as Dagger pipeline (not snapshot flattening).
- [x] Document sequential step building and OCI layer inheritance.
- [x] Document incremental rebuild strategy (`--from-step`).
- [x] Document SQLite update after successful build.
- [x] Document size expectations (under 5MB for SQLite).
- [x] Document platform layer injection (tini + backend binary added to every step image).
- [x] Document markdown compilation (step markdown → SQLite, not image contents).
- [x] Remove digest-pinned tag output from pipeline.
- [ ] Define what validation occurs during the Dagger build (e.g., RUN command failure behavior).
- [ ] Provide size estimates for typical OCI image stacks.
- [ ] Define how the backend binary version is pinned / sourced at compile time.

---

### 5. SQLite Artifact (`artifact/sqlite-artifact.md`)

The portable metadata distribution format.

- [x] Define the concrete SQLite schema (no blob columns).
- [x] Document size expectations (under 5MB vs previous hundreds of MB).
- [x] Document distribution model (SQLite separate from images).
- [x] Add "Step Image Registry" section replacing "Compiled Step Artifacts."
- [x] Clarify dual role: distribution artifact (read-only) vs per-instance working copy (backend writes runtime_state here).
- [x] Remove image_digest column — no digest-pinned tags in v1.
- [x] Add TODO placeholder for completion/validation/unlock system (dedicated design workstream).
- [ ] Define the YAML export format and directory structure.
- [ ] Define integrity verification — checksums? Signatures? Schema version validation on load?
- [ ] Define how schema migrations work when the platform evolves.
- [ ] Design the step completion, validation, and unlock condition system.

---

## Phase 3: Backend Runtime (Single-User Mode Core)

The runtime engine running inside each workspace container. This is what makes a step image an interactive learning environment rather than just a container.

---

### 6. Backend Service (`platform/backend-service.md`)

The Go binary embedded in every step image. **The heart of single-user local mode.**

- [x] Document purpose and role — runtime engine inside each workspace container.
- [x] Document container startup sequence (tini → backend → ttyd).
- [x] Document SQLite lifecycle (distribution artifact → per-instance working copy).
- [x] Document how the binary gets injected by the Dagger pipeline.
- [x] Document step transition behavior (starts fresh on each new container).
- [ ] Define the full API surface (routes, REST vs WebSocket, auth).
- [ ] Define step completion and validation mechanism — dedicated design workstream needed.
- [ ] Define how the distribution SQLite is provided to the container in local mode (CLI mounts it? env var pointing to registry? baked in?).
- [ ] Define frontend framework and asset embedding strategy.

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

---

## Phase 5: CLI — Local Mode

Focus on the local mode command surface. Cluster-specific CLI work (batch provisioning, K8s auth) is deferred to Phase 8.

---

### 8. CLI (`platform/cli.md`)

- [x] Update Local Mode flow (step image pull + run, not Compose).
- [x] Update Cluster Mode (CRD generation uses image tags from SQLite).
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

*Phases 1–6 define and implement everything required for a single user to author a workshop, compile it, and run it locally. Before proceeding to Phase 7, the single-user flow should be fully documented and working end-to-end.*

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
- [ ] Define exact validation error messages for each unsupported capability per backend.

---

## Phase 9: Finalize

---

### 16. Overview (`overview.md`)

- [x] Update "What This Is" (SQLite + registry together form distribution).
- [x] Update layer 1 table (step-spec.md, authoring).
- [x] Update core principles (2, 5, 6).
- [x] New system flow diagram.
- [x] Add "A system that stores file state in SQLite" to "What This Is NOT".
- [x] Add backend service to platform layer table.
- [x] Update presentation layer description (student UI vs builder GUI).
- [x] Update system flow diagram to include backend startup in each mode.
- [x] Add open architectural decisions section (file naming/boundary question).
- [ ] Update with current implementation status and phase roadmap.
- [ ] Review all cross-links and ensure consistency across docs.

---

## Summary

| Phase | Focus | Docs | Status |
|---|---|---|---|
| 1 | Workshop Specification | workshop-spec, workspace-metadata | Complete |
| 2 | Artifact Pipeline | authoring, compilation, sqlite-artifact | Mostly done |
| 3 | Backend Runtime | backend-service | Needs API surface + SQLite delivery |
| 4 | Student UI | frontend | Needs design |
| 5 | CLI — Local Mode | cli (local) | Partially done |
| 6 | Builder GUI | gui | Needs design |
| **—** | **Single-User Milestone** | | |
| 7 | Domain Model & API | shared-go-library, crds | Partially done |
| 8 | Cluster Mode | operator, cli (cluster), provisioners, backend-capabilities | Partially done |
| 9 | Finalize | overview | Mostly done |

Work top to bottom through Phase 6, then validate the single-user mode end-to-end before continuing.
