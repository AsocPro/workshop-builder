# Shared Go Library — Core Domain Model

## Purpose

Holds the canonical types and validation logic shared across all platform components. This library is the single source of truth for domain semantics and prevents drift between the CLI, Operator, and GUI.

## What It Contains

- Compose subset parser
- `WorkspaceTemplate` structs
- `WorkspaceInstance` structs
- Validation rules
- Capability matrix logic (what each backend supports)
- Translation logic (Compose → Kubernetes objects)

## What It Must NOT Contain

- Kubernetes client logic (no `client-go` imports)
- Docker execution logic (no Docker SDK imports)
- GUI logic
- CLI argument parsing
- HTTP/API server logic

This keeps the library portable and testable without infrastructure dependencies.

## Key Responsibilities

### Compose Parsing

- Parse `docker-compose.yml` into internal representation
- Validate against supported subset
- Reject unsupported features with clear errors

### Workspace Metadata Parsing

- Parse `workspace.yaml` into internal structs
- Validate field values and cross-field constraints
- Enforce [capability matrix](./backend-capabilities.md) (e.g., team mode requires Kubernetes backend)

### Translation

- Convert parsed Compose + workspace metadata into Kubernetes object specs
- Generate CRD objects ([WorkspaceTemplate, WorkspaceInstance](./crds.md))
- Normalize manifests (strip runtime-generated fields)

### Capability Matrix

- Define what each backend (Docker, Kubernetes) supports
- Provide clear errors when a workspace definition requires capabilities unavailable on the target backend

## Consumers

| Consumer | How It Uses the Library |
|---|---|
| [CLI](./cli.md) | Parsing, validation, translation, capability checks |
| [Operator](./operator.md) | Domain structs, validation (ideally) |
| [GUI](../presentation/gui.md) | Parsing, validation, status interpretation |

## Package Structure

TODO: Define the Go package layout (e.g., `pkg/types`, `pkg/compose`, `pkg/validate`, `pkg/translate`).

## Testing Strategy

TODO: Define testing approach — unit tests for validation, integration tests for translation, golden file tests for manifest generation.

## Versioning

TODO: Define how this library is versioned relative to the CRD versions and CLI releases.
