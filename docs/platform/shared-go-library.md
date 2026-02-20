# Shared Go Library — Core Domain Model

## Purpose

Holds the canonical types and validation logic shared across all platform components. This library is the single source of truth for domain semantics and prevents drift between the CLI, Operator, and GUI.

## What It Contains

- Step-spec parser (`pkg/stepspec`)
- `WorkspaceTemplate` structs
- `WorkspaceInstance` structs
- Validation rules
- Capability matrix logic (what each backend supports)
- CRD generation logic (workspace metadata + step image tags → CRD objects)

## What It Must NOT Contain

- Kubernetes client logic (no `client-go` imports)
- Docker execution logic (no Docker SDK imports)
- Dagger SDK imports
- GUI logic
- CLI argument parsing
- HTTP/API server logic

This keeps the library portable and testable without infrastructure dependencies.

## Key Responsibilities

### Step Spec Parsing

- Parse `step-spec.yaml` into internal representation
- Validate against schema (required fields, URL-safe step IDs, mutually exclusive file entry fields, local source file existence)
- Reject invalid specs with structured errors that include the field path and a human-readable message

### Workspace Metadata Parsing

- Parse `workspace.yaml` into internal structs
- Validate field values and cross-field constraints
- Enforce [capability matrix](./backend-capabilities.md) (e.g., team mode requires Kubernetes backend)

### CRD Generation

- Convert parsed workspace metadata + step image tags (from SQLite) into Kubernetes CRD objects
- Generate `WorkspaceTemplate` and `WorkspaceInstance` specs
- Normalize field values and apply defaults

### Capability Matrix

- Define what each backend (Docker, Kubernetes) supports
- Provide clear errors when a workspace definition requires capabilities unavailable on the target backend

## Consumers

| Consumer | How It Uses the Library |
|---|---|
| [CLI](./cli.md) | Step-spec parsing, workspace metadata parsing, validation, CRD generation, capability checks |
| [Operator](./operator.md) | Domain structs, validation |
| [GUI](../presentation/gui.md) | Parsing, validation, status interpretation |

## Package Structure

TODO: Define the Go package layout. Proposed structure:

```
pkg/
  stepspec/     # step-spec.yaml parser, types, and validation
  workspace/    # workspace.yaml parser, types, and validation
  crd/          # WorkspaceTemplate and WorkspaceInstance generation
  capability/   # backend capability matrix and enforcement
  types/        # shared domain types used across packages
```

`pkg/stepspec` replaces the former `pkg/compose` package. There is no `pkg/translate` package — Compose-to-Kubernetes translation is removed. Step images are built by Dagger (outside this library) and referenced by tag in SQLite; the library generates CRD objects that include those tags directly.

## Testing Strategy

TODO: Define testing approach — unit tests for validation, integration tests for CRD generation, golden file tests for generated manifests.

## Versioning

TODO: Define how this library is versioned relative to the CRD versions and CLI releases.
