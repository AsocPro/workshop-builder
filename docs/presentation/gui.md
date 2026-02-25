# Builder GUI — Wails Desktop Application

## Purpose

The workshop authoring and administration tool. A Wails desktop app that runs on the instructor's workstation and provides a graphical interface for the full build workflow — writing steps, running the proxy session, compiling images, and managing workshop deployments.

Builder mode is a **separate binary** from the student-facing frontend. Students use a browser to access the web UI served by the backend inside their workspace container. Authors use this Wails app on their own machine.

## Technology

Built with [Wails](https://wails.io/) — Go backend with an embedded web frontend. The Go backend has direct access to the local filesystem, Docker daemon, and Dagger SDK — all of which are required for the build workflow.

## Why Wails for Builder Mode

The builder needs to run on the author's workstation and interact with local tooling (Docker for proxy sessions, Dagger for compilation). A web-based builder would require running a local server, exposing a socket, and managing cross-origin access. A Wails app runs natively and can import the [Shared Go Library](../platform/shared-go-library.md) directly — no subprocess calls needed for domain logic.

## Architecture Constraint

The GUI **must**:

- Call into the [Shared Go Library](../platform/shared-go-library.md) for all domain logic (parsing, validation, CRD generation)
- NOT duplicate business logic from the [CLI](../platform/cli.md)
- Share the same validation, parsing, and type definitions as the CLI

The GUI is a presentation layer over the same core logic the CLI uses.

## Responsibilities

### Workshop Authoring

- Step list editor — add, remove, reorder steps in `workshop.yaml`
- Markdown editor — write or import tutorial content per step
- Start proxy session — launch `workshop build proxy` for interactive authoring
- Save step — trigger `workshop build step save`

### Compilation

- Compile workshop — trigger `workshop build compile` via Dagger pipeline
- Show build progress and logs
- Display step build status (built / not built / error)

### Distribution

- Push compiled images to registry
- YAML import/export

### Cluster Administration

- Provision workspaces (batch or individual)
- View workspace status
- Tear down workspaces

## Relationship to CLI

TODO: Define whether the GUI invokes CLI commands as subprocesses or imports CLI logic as Go packages directly. The latter is preferred to avoid subprocess management and enable shared error handling.

## Features

TODO: Define the specific feature set for v1 of the GUI — which of the above are in scope for the first release.

## Frontend Technology

TODO: Define the frontend framework used within Wails (React, Svelte, Vue, etc.).

## Distribution

TODO: Define how the GUI binary is packaged and distributed (platform-specific binaries, installer, Homebrew tap, etc.).
