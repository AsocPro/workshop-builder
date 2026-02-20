# GUI — Wails Desktop Application

## Purpose

Friendly admin interface for managing workshops and workspaces. Wraps CLI functionality in a graphical interface to simplify onboarding and batch management.

## Technology

Built with [Wails](https://wails.io/) — Go backend with a web frontend.

## Responsibilities

- Workshop batch management (provision/tear down multiple workspaces)
- Workspace status display
- Simplified onboarding for new administrators
- Visual feedback for long-running operations

## Architecture Constraint

The GUI **must**:

- Call into the [Shared Go Library](../platform/shared-go-library.md) for all domain logic
- NOT duplicate business logic from the [CLI](../platform/cli.md)
- Share the same validation, parsing, and translation code

The GUI is a presentation layer over the same core logic the CLI uses.

## Local Mode Client

The Wails app is a strong candidate for the local mode client — the environment students use to run workshops on their own machine without a cluster. In this role the Wails app would:

- Pull step images from the registry (using image tags from SQLite)
- Spawn and manage workshop containers (Docker or Podman) running step images
- Perform step transitions by stopping the current container and starting the next step image
- Spawn ttyd as a subprocess for terminal access to the current step container
- Run an HTTP/WebSocket proxy server in the Go backend, proxying all browser connections (including ttyd WebSocket) through a single origin to avoid CORS issues
- Serve the web UI from the embedded WebView

This keeps local mode as a single distributable binary with no "open a browser and navigate to localhost" step for students. The Go backend in the Wails app has direct access to the container runtime and can manage the full workshop lifecycle.

TODO: Confirm whether the Wails app is the local mode client, a standalone admin GUI, or both. Decide before frontend implementation begins.

## Relationship to CLI

TODO: Define whether the GUI calls CLI commands as subprocesses, imports CLI logic as Go packages, or shares a common service layer.

## Features

TODO: Define the specific feature set for v1 of the GUI.

## Frontend Technology

TODO: Define the frontend framework used within Wails (React, Svelte, Vue, etc.).

## Distribution

TODO: Define how the GUI is packaged and distributed (platform-specific binaries, installer, etc.).
