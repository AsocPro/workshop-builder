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

## Relationship to CLI

TODO: Define whether the GUI calls CLI commands as subprocesses, imports CLI logic as Go packages, or shares a common service layer.

## Features

TODO: Define the specific feature set for v1 of the GUI.

## Frontend Technology

TODO: Define the frontend framework used within Wails (React, Svelte, Vue, etc.).

## Distribution

TODO: Define how the GUI is packaged and distributed (platform-specific binaries, installer, etc.).
