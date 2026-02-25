# Shared Go Library — Core Domain Model

## Purpose

Holds the canonical types and validation logic shared across all platform components. This library is the single source of truth for domain semantics and prevents drift between the CLI, Operator, and GUI.

## What It Contains

- `workshop.yaml` parser (`pkg/workshop`)
- `WorkspaceTemplate` structs
- `WorkspaceInstance` structs
- Validation rules
- Capability matrix logic (what each backend supports)
- CRD generation logic (workshop.yaml step image tags → CRD objects)

## What It Must NOT Contain

- Kubernetes client logic (no `client-go` imports)
- Docker execution logic (no Docker SDK imports)
- Dagger SDK imports
- GUI logic
- CLI argument parsing
- HTTP/API server logic

This keeps the library portable and testable without infrastructure dependencies.

## Key Responsibilities

### Workshop Spec Parsing

- Parse `workshop.yaml` into internal representation
- Validate against schema (required fields, URL-safe step IDs, mutually exclusive file entry fields, local source file existence, markdown field mutual exclusion)
- Reject invalid specs with structured errors that include the field path and a human-readable message

### CRD Generation

- Convert parsed workshop step data into Kubernetes CRD objects
- Populate `WorkspaceTemplate.spec.steps` with image tags derived from `workshop.yaml` (`<workshop.image>:<step-id>`)
- Generate `WorkspaceInstance` specs
- Normalize field values and apply defaults

Note: WorkspaceTemplate operator config fields (lifecycle, isolation, cluster mode, resources, access) are authored directly in the CRD by operators — the library provides the types and validation for them but does not generate them from any author-facing file.

### Capability Matrix

- Define what each backend (Docker, Kubernetes) supports
- Provide clear errors when a workspace configuration requires capabilities unavailable on the target backend

## Consumers

| Consumer | How It Uses the Library |
|---|---|
| [CLI](./cli.md) | workshop.yaml parsing, validation, CRD step population, capability checks |
| [Operator](./operator.md) | Domain structs, validation |
| [GUI](../presentation/gui.md) | Parsing, validation, status interpretation |

## Package Structure

TODO: Define the Go package layout. Proposed structure:

```
pkg/
  workshop/     # workshop.yaml parser, types, and validation
  crd/          # WorkspaceTemplate and WorkspaceInstance types and generation
  capability/   # backend capability matrix and enforcement
  types/        # shared domain types used across packages
```

`pkg/workshop` replaces the former `pkg/compose` and `pkg/stepspec` packages. There is no `pkg/translate` package — Compose-to-Kubernetes translation is removed. There is no `pkg/workspace` package — workspace deployment config lives in the CRD, authored directly by operators.

## Testing Strategy

TODO: Define testing approach — unit tests for validation, integration tests for CRD generation, golden file tests for generated manifests.

## Versioning

TODO: Define how this library is versioned relative to the CRD versions and CLI releases.
