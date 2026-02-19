# Documentation Build-Out Plan

Work through this list top to bottom. Each item is ordered by dependency — earlier docs inform later ones. Check off items as you complete them.

---

## Phase 1: Define the Inputs

These are the foundation. Everything downstream — types, CRDs, operator behavior, compilation — is shaped by what you decide here. Get these right first.

---

### 1. Workload Layer (`definition/workload-layer.md`)

The Compose subset you support is the single most consequential input decision. It determines what the shared library parses, what translation rules exist, what the CRDs contain, and what the operator deploys.

- [x] Define the exact subset of Compose features the platform supports. Document which features are intentionally excluded and why.
- [x] Document how Compose services map to Kubernetes objects (Deployments, Services, PVCs, etc.) during translation in the shared library.
- [x] Define specific validation rules and error messages.
- [x] Add example `docker-compose.yml` files for common workshop patterns (single container, multi-service, with volumes, etc.).

**Think about:** What's the simplest Compose subset that covers 90% of workshop needs? What Compose features would create translation nightmares (networks, build, deploy, configs/secrets)?

---

### 2. Workspace Metadata (`definition/workspace-metadata.md`)

This defines every platform behavior knob. The CRD schema is essentially a Kubernetes-native representation of this file, so it needs to be locked down before CRD work.

- [x] Define team membership model — how are teams defined and assigned? (deferred; individual mode only for v1, team mode schema reserved for future)
- [x] Define the available resource classes and their concrete quota/limit values. (replaced with explicit cpu/memory fields; defaults deferred)
- [x] Define how access surfaces are implemented (sidecars? ingress rules? services?). (webTerminal via ttyd, proxied through backend; cluster injection details deferred to operator)
- [x] Define validation rules (e.g., team mode requires cluster backend, ttl requires ephemeral mode, etc.). (minimal validation defined)
- [x] Define how the workspace.yaml schema will be versioned over time. (top-level `version: v1` field; unrecognized version is a hard validation error)

**Think about:** What's the minimal set of fields for v1? Which fields can be added later without breaking the schema? Are resource classes just T-shirt sizes or something more flexible?

---

## Phase 2: Define the Domain Model & API Surface

With inputs defined, you can now lock down the types, the Kubernetes API objects, and how they relate.

---

### 3. Shared Go Library (`platform/shared-go-library.md`)

This is the glue. Every component imports it. Its package structure and type definitions set the shape of the entire codebase.

- [ ] Define the Go package layout (e.g., `pkg/types`, `pkg/compose`, `pkg/validate`, `pkg/translate`).
- [ ] Define testing approach — unit tests for validation, integration tests for translation, golden file tests for manifest generation.
- [ ] Define how this library is versioned relative to the CRD versions and CLI releases.

**Think about:** What's the import boundary? Should the library produce raw K8s object structs, or its own intermediate representation? How do you keep `client-go` out while still generating K8s types?

---

### 4. CRDs (`platform/crds.md`)

The CRDs are the Kubernetes API contract. They encode the workspace metadata, translated workload spec, and step management into cluster-native objects. This must be solid before operator work.

- [ ] Finalize the CRD schema. Define which fields are overridable at instance level vs locked at template level.
- [ ] Define how compiled step artifacts are referenced — inline in CRD? ConfigMap references? External storage references?
- [ ] Define the full status subresource (conditions, phase transitions, error reporting).
- [ ] Define how step transitions are requested — update to `spec.currentStep`? A separate sub-resource? An API call?
- [ ] Define CRD versioning strategy (v1alpha1 → v1beta1 → v1) and conversion webhook requirements.
- [ ] Define admission webhook validation rules (beyond OpenAPI schema validation).
- [ ] Define which roles can create/read/update/delete Templates vs Instances.

**Think about:** Step artifacts could be huge (tar blobs). Inline in CRDs has etcd size limits (~1.5MB). ConfigMaps have the same limit. You likely need an external storage reference (S3, PVC, OCI artifact). This decision ripples into compilation, operator, and CLI.

---

## Phase 3: Define the Artifact Pipeline

Now that you know what the inputs look like and what the CRDs/operator expect, define how authoring state becomes runtime artifacts.

---

### 5. Compilation (`artifact/compilation.md`)

The bridge between messy authoring and clean runtime. Needs to know what authoring captures (input) and what the operator consumes (output).

- [ ] Define the serialization format for each artifact type (manifest bundle format, archive format, educational snapshot format).
- [ ] Define what validation occurs during compilation — manifest validity, file completeness, step ordering, etc.
- [ ] Define when and how recompilation is triggered — manual only? Automatic on step save? Incremental recompilation of changed steps only?
- [ ] Provide rough size estimates for typical workshops (e.g., 10 steps, 5 services, moderate file state).

**Think about:** The serialization format choice here directly affects SQLite schema design and operator consumption. Pick something the operator can apply without complex deserialization.

---

### 6. SQLite Artifact (`artifact/sqlite-artifact.md`)

The portable distribution format. Its schema is shaped by what compilation produces and what the runtime needs to read.

- [ ] Define the concrete SQLite schema (tables, columns, types, indexes).
- [ ] Provide size estimates for typical workshops.
- [ ] Define how SQLite artifacts are distributed — direct download? Registry? Git LFS? Container image embedding?
- [ ] Define the YAML export format and directory structure.
- [ ] Define integrity verification — checksums? Signatures? Schema version validation on load?
- [ ] Define how schema migrations work when the platform evolves.

**Think about:** How does the SQLite file get into the cluster? Is it baked into a container image? Mounted as a volume? Downloaded by an init container? This affects operator design.

---

### 7. Authoring (`definition/authoring.md`)

Now that you know what compilation needs as input, you can precisely define what authoring must capture and how.

- [ ] Define exactly what is captured during a snapshot — full namespace dump? Specific resource types? How are system-level resources excluded?
- [ ] Define how the instructor specifies which resources and files belong to a step vs platform internals.
- [ ] Are authoring snapshots preserved for re-editing after compilation, or are they discarded? If preserved, where?
- [ ] Define how instructors edit or re-order existing steps. Can they insert between steps? Delete? What happens downstream?
- [ ] Define whether multiple instructors can collaborate on authoring simultaneously.

**Think about:** The snapshot capture scope is the hardest question here. Too broad and you capture platform noise. Too narrow and instructors have to manually tag everything. Maybe a label/annotation convention?

---

## Phase 4: Define the Runtime Engine

With inputs, API surface, and artifact pipeline defined, you can now fully specify the runtime behavior.

---

### 8. Operator (`platform/operator.md`)

The biggest component. Depends on almost everything above. Has the most TODOs because it's where all the design decisions converge.

- [ ] Define what "platform system components" are excluded from namespace cleanup during step transitions.
- [ ] Define the reconciliation flow for WorkspaceTemplate and WorkspaceInstance controllers. How are step transitions detected and processed?
- [ ] Define how the operator handles partial failures (e.g., namespace created but deployment failed, manifest applies but PVC restore fails).
- [ ] Define acceptable step transition times. What optimizations are available (parallel apply, pre-staging)?
- [ ] Define retention policy for runtime snapshots — how many kept? Per student? Per step?
- [ ] Define operator scaling strategy — single replica with leader election? Sharded by namespace?
- [ ] Define metrics, events, and logging strategy.
- [ ] Define how the operator is deployed (Helm chart, OLM, raw manifests).

**Think about:** The reconciliation flow is the core of the system. Sketch it as a state machine. What are the transitions? What triggers them? What happens on failure at each stage?

---

### 9. CLI (`platform/cli.md`)

With the shared library, CRDs, and operator defined, the CLI becomes a relatively thin orchestration layer.

- [ ] Define the CLI command tree (e.g., `workshop create`, `workshop run`, `workspace list`, `workspace delete`).
- [ ] Define CLI configuration model (config file location, environment variables, kubeconfig discovery).
- [ ] Define how batch workshop provisioning works (provision N workspaces for a class, assign to users).
- [ ] Define how the CLI authenticates to Kubernetes clusters and any platform-level auth requirements.
- [ ] Define error handling strategy — how validation errors, provisioning failures, and partial states are reported.

**Think about:** The CLI has two very different personas — an author building locally and an admin provisioning a 30-person workshop. The command tree should reflect both without confusion.

---

### 10. Backend Capabilities (`platform/backend-capabilities.md`)

With operator and CLI defined, you can finalize exactly what each backend supports.

- [ ] Fill in step transitions / reset row for both backends in the capability matrix.
- [ ] Define the exact validation error messages for each unsupported capability per backend.
- [ ] Define how step transitions work in local/Docker mode — simplified version of the operator's reset flow run by the CLI?

---

### 11. Infrastructure Provisioners (`platform/infrastructure-provisioners.md`)

- [ ] Clarify when k3s is used vs k3d. Is k3s a fallback or a separate configuration option?
- [ ] Define how provisioner versions (k3d, vcluster) are managed and how Kubernetes version selection works.
- [ ] Define cleanup procedures for provisioned clusters during workspace deletion and step transitions.
- [ ] Document the resource overhead of each provisioner to inform resource class sizing.

---

## Phase 5: Define the Presentation Layer

Everything else is defined. Now specify how users interact with it.

---

### 12. Frontend (`presentation/frontend.md`)

- [ ] Define the student-facing API surface — REST? WebSocket? gRPC?
- [ ] Define the cluster status panel — what information is shown? Real-time or polled?
- [ ] Define how builder mode connects to the authoring namespace — direct K8s API? Through the CLI? Through a backend service?
- [ ] Define the frontend framework (React, Svelte, Vue, etc.).
- [ ] Define whether the frontend is a standalone SPA, embedded in the Wails GUI, or served by the runtime platform.
- [ ] Define markdown rendering capabilities — standard CommonMark? Extensions?
- [ ] Define how students and instructors authenticate to the frontend.
- [ ] Define target devices — desktop only? Tablet? Mobile?

---

### 13. GUI (`presentation/gui.md`)

- [ ] Define whether the GUI calls CLI commands as subprocesses, imports CLI logic as Go packages, or shares a common service layer.
- [ ] Define the specific feature set for v1 of the GUI.
- [ ] Define the frontend framework used within Wails.
- [ ] Define how the GUI is packaged and distributed.

---

## Phase 6: Finalize

### 14. Overview (`overview.md`)

- [ ] Update with current implementation status and phase roadmap.
- [ ] Review all cross-links and ensure consistency across docs.
- [ ] Verify the system flow diagram still matches the detailed docs.

---

## Summary

| Phase | Focus | Docs | TODOs |
|---|---|---|---|
| 1 | Define the Inputs | workload-layer, workspace-metadata | 9 |
| 2 | Domain Model & API | shared-go-library, crds | 10 |
| 3 | Artifact Pipeline | compilation, sqlite-artifact, authoring | 15 |
| 4 | Runtime Engine | operator, cli, backend-capabilities, provisioners | 16 |
| 5 | Presentation | frontend, gui | 12 |
| 6 | Finalize | overview | 3 |
| | **Total** | **14 docs** | **65** |

Work top to bottom. Each phase builds on the previous. Resist the temptation to jump ahead — decisions in Phase 1 cascade through everything.
