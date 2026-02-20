# Documentation Build-Out Plan

Work through this list top to bottom. Each item is ordered by dependency — earlier docs inform later ones. Check off items as you complete them.

---

## Phase 1: Define the Inputs

These are the foundation. Everything downstream — types, CRDs, operator behavior, compilation — is shaped by what you decide here. Get these right first.

---

### 1. Step Spec (`definition/step-spec.md`)

The step-spec.yaml format is the single most consequential input decision. It determines what the shared library parses, what Dagger builds, what the CRDs contain, and what the operator deploys.

- [x] Define the step-spec.yaml schema (version, workshop, base, steps).
- [x] Document how Dagger consumes the spec (sequential builds, layer inheritance).
- [x] Define validation rules and error messages.
- [x] Add example step-spec.yaml files for common workshop patterns.
- [x] Document the image tagging convention (`<workshop.image>:<step-id>`).
- [x] Document incremental rebuild strategy (`--from-step`).

---

### 2. Workspace Metadata (`definition/workspace-metadata.md`)

This defines every platform behavior knob. The CRD schema is essentially a Kubernetes-native representation of this file.

- [x] Remove `webTerminal.target` — terminal always attaches to the current step container.
- [x] Remove cross-file validation against Compose services.
- [x] Update relationship section to reference step-spec.yaml.
- [ ] Define team membership model — how are teams defined and assigned? (deferred; individual mode only for v1, team mode schema reserved for future)
- [ ] Define the hard default values for `resources` when omitted, and whether omitting resources is a validation error.
- [ ] Define whether limits apply at container level, namespace level (ResourceQuota), or both.

---

## Phase 2: Define the Domain Model & API Surface

With inputs defined, you can now lock down the types, the Kubernetes API objects, and how they relate.

---

### 3. Shared Go Library (`platform/shared-go-library.md`)

This is the glue. Every component imports it.

- [x] Replace Compose parser with step-spec parser (`pkg/stepspec`).
- [x] Remove `pkg/translate` (no Compose-to-K8s translation needed).
- [x] Document CRD generation responsibility (workspace metadata + image tags → CRD objects).
- [ ] Define the Go package layout (`pkg/stepspec`, `pkg/workspace`, `pkg/crd`, `pkg/capability`, `pkg/types`).
- [ ] Define testing approach — unit tests for validation, integration tests for CRD generation, golden file tests.
- [ ] Define how this library is versioned relative to CRD versions and CLI releases.

---

### 4. CRDs (`platform/crds.md`)

The CRDs are the Kubernetes API contract. They encode workspace metadata and step image references.

- [x] Replace `workload.*` fields with `steps[].imageTag`, `steps[].imageDigest`, `imagePullSecrets`.
- [x] Remove manifest bundle and file archive references.
- [ ] Finalize the CRD schema. Define which fields are overridable at instance level vs locked at template level.
- [ ] Define the full status subresource (conditions, phase transitions, error reporting).
- [ ] Define how step transitions are requested — update to `spec.currentStep`? Separate sub-resource?
- [ ] Define CRD versioning strategy (v1alpha1 → v1beta1 → v1) and conversion webhook requirements.
- [ ] Define admission webhook validation rules.
- [ ] Define which roles can create/read/update/delete Templates vs Instances.

---

## Phase 3: Define the Artifact Pipeline

---

### 5. Compilation (`artifact/compilation.md`)

The Dagger build pipeline.

- [x] Reframe as Dagger pipeline (not snapshot flattening).
- [x] Document sequential step building and OCI layer inheritance.
- [x] Document incremental rebuild strategy (`--from-step`).
- [x] Document SQLite update after successful build.
- [x] Document size expectations (under 5MB for SQLite).
- [ ] Define what validation occurs during the Dagger build (e.g., RUN command failure behavior).
- [ ] Provide size estimates for typical OCI image stacks.

---

### 6. SQLite Artifact (`artifact/sqlite-artifact.md`)

The portable metadata distribution format.

- [x] Define the concrete SQLite schema (no blob columns).
- [x] Document size expectations (under 5MB vs previous hundreds of MB).
- [x] Document distribution model (SQLite separate from images).
- [x] Add "Step Image Registry" section replacing "Compiled Step Artifacts."
- [ ] Define the YAML export format and directory structure.
- [ ] Define integrity verification — checksums? Signatures? Schema version validation on load?
- [ ] Define how schema migrations work when the platform evolves.

---

### 7. Authoring (`definition/authoring.md`)

The CLI proxy model.

- [x] Rewrite for CLI proxy model (no K8s snapshots).
- [x] Document what the proxy records (files, env, commands → step-spec.yaml).
- [x] Document step editing (edit YAML directly) and step reordering.
- [x] Document version control integration (commit step-spec.yaml).
- [x] Document collaboration model (Git-based).
- [ ] Define whether simultaneous proxy sessions from multiple authors are supported.

---

## Phase 4: Define the Runtime Engine

---

### 8. Operator (`platform/operator.md`)

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

### 9. CLI (`platform/cli.md`)

- [x] Update Local Mode flow (step image pull + run, not Compose).
- [x] Update Cluster Mode (CRD generation uses image tags from SQLite).
- [x] Add full "Build Commands" section (proxy, compile, step save/new, status).
- [ ] Define the full CLI command tree beyond the `build` group.
- [ ] Define CLI configuration model.
- [ ] Define how batch workshop provisioning works.
- [ ] Define how the CLI authenticates to Kubernetes clusters.
- [ ] Define error handling strategy.

---

### 10. Backend Capabilities (`platform/backend-capabilities.md`)

- [x] Step transitions row: Docker = "Image swap (CLI-managed)"; K8s = "Image swap (Operator-managed)".
- [x] Add "Dagger build pipeline" row.
- [x] Remove PVC/manifest TODO references.
- [ ] Define exact validation error messages for each unsupported capability per backend.

---

### 11. Infrastructure Provisioners (`platform/infrastructure-provisioners.md`)

- [x] Update key constraint note (step-spec.yaml, not docker-compose.yml).
- [ ] Clarify when k3s is used vs k3d.
- [ ] Define how provisioner versions are managed and how Kubernetes version selection works.
- [ ] Define cleanup procedures for provisioned clusters.
- [ ] Document resource overhead of each provisioner.

---

## Phase 5: Define the Presentation Layer

---

### 12. Frontend (`presentation/frontend.md`)

- [x] Update builder mode (CLI proxy commands, not K8s namespace snapshots).
- [ ] Define the student-facing API surface.
- [ ] Define the cluster status panel.
- [ ] Define how builder mode connects to the CLI.
- [ ] Define the frontend framework.
- [ ] Define whether the frontend is a standalone SPA, embedded in Wails, or served by the runtime.
- [ ] Define markdown rendering capabilities.
- [ ] Define authentication.
- [ ] Define target devices.

---

### 13. GUI (`presentation/gui.md`)

- [x] Update local mode client description (step image pull + swap, not Compose).
- [ ] Confirm whether the Wails app is the local mode client, a standalone admin GUI, or both.
- [ ] Define whether the GUI calls CLI commands as subprocesses, imports CLI logic as Go packages, or shares a common service layer.
- [ ] Define the specific feature set for v1 of the GUI.
- [ ] Define the frontend framework used within Wails.
- [ ] Define how the GUI is packaged and distributed.

---

## Phase 6: Finalize

### 14. Overview (`overview.md`)

- [x] Update "What This Is" (SQLite + registry together form distribution).
- [x] Update layer 1 table (step-spec.md, authoring).
- [x] Update core principles (2, 5, 6).
- [x] New system flow diagram.
- [x] Add "A system that stores file state in SQLite" to "What This Is NOT".
- [ ] Update with current implementation status and phase roadmap.
- [ ] Review all cross-links and ensure consistency across docs.

---

## Summary

| Phase | Focus | Docs | Completed TODOs | Remaining TODOs |
|---|---|---|---|---|
| 1 | Define the Inputs | step-spec, workspace-metadata | 9 | 3 |
| 2 | Domain Model & API | shared-go-library, crds | 5 | 8 |
| 3 | Artifact Pipeline | compilation, sqlite-artifact, authoring | 14 | 6 |
| 4 | Runtime Engine | operator, cli, backend-capabilities, provisioners | 11 | 17 |
| 5 | Presentation | frontend, gui | 3 | 14 |
| 6 | Finalize | overview | 5 | 2 |
| | **Total** | **14 docs** | **47** | **50** |

Work top to bottom. Each phase builds on the previous.
