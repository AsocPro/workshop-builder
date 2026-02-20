# CLI — Administration Layer

## Purpose

Primary control surface for the platform. Handles workspace orchestration across both local (Docker) and cluster (Kubernetes) backends, and provides the `workshop build` command group for authoring and compiling workshops.

## Modes of Operation

### Local Mode

Single-user execution using Docker as the backend.

1. Read `step-spec.yaml` + `workspace.yaml`
2. Validate via [Shared Go Library](./shared-go-library.md)
3. Read current step's `image_tag` from SQLite
4. Pull step image from registry
5. Run container from step image (`docker run`)
6. Spawn ttyd subprocess for terminal access
7. Provision nested cluster if required (k3d)
8. Manage workspace lifecycle locally

Step transitions in local mode: stop current container → pull next step image → start new container. No manifest bundles, no PVC operations.

### Cluster Mode

Multi-tenant execution against a Kubernetes cluster.

1. Read `step-spec.yaml` + `workspace.yaml`
2. Validate
3. Read step image tags from SQLite artifact
4. Generate [WorkspaceTemplate / WorkspaceInstance CRDs](./crds.md) (via Shared Go Library)
5. Submit to Kubernetes API
6. Watch status
7. Support batch provisioning for workshops

## Cluster Provisioning Logic

When `cluster.mode == per-workspace` in `workspace.yaml`, the CLI handles cluster provisioning differently depending on backend:

### Docker Backend (Local)

1. Provision k3d cluster
2. Extract kubeconfig
3. Inject kubeconfig into workload container (step image)

### Kubernetes Backend (Cluster)

Cluster provisioning is delegated to the [Operator](./operator.md) via CRD fields. The CLI just submits the spec.

See [Infrastructure Provisioners](./infrastructure-provisioners.md) for details on k3d and vcluster.

## Build Commands

The `workshop build` command group handles the full authoring and compilation lifecycle.

### `workshop build proxy`

Start an interactive authoring session. Launches a container from the current step image (or `base.image` for step 1) and wraps the shell with an observer that records changes to `step-spec.yaml`.

```
$ workshop build proxy
Launching authoring session for step-1-intro...
Container started. Commands you run will be recorded.
Type 'exit' to end the session.
[step-1-intro] $
```

The proxy observes:
- **Filesystem changes** → recorded as `files:` entries in `step-spec.yaml`
- **Environment variable changes** → recorded as `env:` entries
- **Shell commands run** → recorded as `commands:` entries

The proxy records *intent* (what the author did), not *observed state* (a snapshot of the full filesystem). This keeps `step-spec.yaml` human-readable and version-controllable.

### `workshop build step save`

Finalize the current step and advance to the next. Closes the current proxy session, writes the step's accumulated changes to `step-spec.yaml`, and opens a new container session building on top of the current step image.

```
$ workshop build step save
Step step-1-intro finalized and saved to step-spec.yaml.
Opening session for step-2-deploy...
[step-2-deploy] $
```

### `workshop build step new`

Create a new named step slot in `step-spec.yaml` without closing the current session.

```
$ workshop build step new step-3-advanced
Added step slot: step-3-advanced (will follow step-2-deploy)
```

### `workshop build compile`

Invoke the Dagger build pipeline. Reads `step-spec.yaml`, builds one OCI image per step sequentially, pushes images to the registry, and updates SQLite with image tags and digests.

```
$ workshop build compile
Building step-1-intro...     ✓  pushed myorg/kubernetes-101:step-1-intro
Building step-2-deploy...    ✓  pushed myorg/kubernetes-101:step-2-deploy
Building step-3-advanced...  ✓  pushed myorg/kubernetes-101:step-3-advanced
SQLite updated: workshop.db
```

### `workshop build compile --from-step <id>`

Incremental recompilation starting from a specific step. Steps before `<id>` retain their existing image tags in SQLite.

```
$ workshop build compile --from-step step-3-advanced
Skipping step-1-intro (existing tag: myorg/kubernetes-101:step-1-intro)
Skipping step-2-deploy (existing tag: myorg/kubernetes-101:step-2-deploy)
Building step-3-advanced...  ✓  pushed myorg/kubernetes-101:step-3-advanced
SQLite updated: workshop.db
```

### `workshop build status`

Show a summary of the current `step-spec.yaml` and SQLite state.

```
$ workshop build status
Workshop: kubernetes-101
Base:     ubuntu:22.04

Steps:
  [1] step-1-intro       "Introduction"        ✓ built  myorg/kubernetes-101:step-1-intro
  [2] step-2-deploy      "Deploy the App"      ✓ built  myorg/kubernetes-101:step-2-deploy
  [3] step-3-advanced    "Advanced Topics"     ✗ not built

1 step pending compilation. Run: workshop build compile --from-step step-3-advanced
```

## Dependencies

| Dependency | Required For |
|---|---|
| [Shared Go Library](./shared-go-library.md) | Parsing, validation, CRD generation |
| Docker | Local mode execution, build proxy |
| Kubernetes API | Cluster mode execution |
| k3d binary | Local cluster provisioning |
| Dagger SDK | `workshop build compile` pipeline |

The CLI does NOT depend on Operator internals.

## Command Structure

TODO: Define the full CLI command tree beyond the `build` group (e.g., `workshop run`, `workspace list`, `workspace delete`, `workspace reset`).

## Configuration

TODO: Define CLI configuration model (config file location, environment variables, kubeconfig discovery).

## Batch Operations

TODO: Define how batch workshop provisioning works (provision N workspaces for a class, assign to users, etc.).

## Authentication & Authorization

TODO: Define how the CLI authenticates to Kubernetes clusters and any platform-level auth requirements.

## Error Handling

TODO: Define error handling strategy — how validation errors, provisioning failures, and partial states are reported and recovered from.
