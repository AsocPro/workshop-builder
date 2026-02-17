# Frontend Architecture

## Purpose

The frontend serves two distinct modes with different audiences and capabilities. In both modes, the frontend is a **thin layer** — it triggers backend operations and displays results. It does not compute state.

---

## Student Mode

### Audience

Workshop participants following a guided tutorial.

### Features

- Step navigation (next, previous, jump to step)
- Reset to step (triggers full clean reset via [Operator](../platform/operator.md))
- Resume progress from last checkpoint
- Markdown tutorial display
- Validation feedback (did the student complete the step correctly?)
- Cluster status panel (optional)

### Key Constraint

All state management, reset logic, and validation happens server-side. The frontend only:

1. Triggers backend transitions
2. Displays results
3. Shows tutorial content

TODO: Define the student-facing API surface — REST? WebSocket? gRPC?

TODO: Define the cluster status panel — what information is shown? Real-time or polled?

---

## Builder Mode

### Audience

Workshop instructors creating or editing workshops.

### Features

- Step list editor (add, remove, reorder steps)
- Save step snapshot (trigger [authoring snapshot](../definition/authoring.md) capture)
- Compile workshop (trigger [compilation](../artifact/compilation.md))
- Export [SQLite artifact](../artifact/sqlite-artifact.md)
- Optional YAML import/export
- Optional diff viewer (show changes between steps)

### Key Constraint

Builder mode may interact with live Kubernetes namespaces. It does NOT change the runtime architecture — builder mode is a client of the [Authoring Layer](../definition/authoring.md).

TODO: Define how builder mode connects to the authoring namespace — direct K8s API? Through the CLI? Through a backend service?

---

## Technology

TODO: Define the frontend framework (React, Svelte, Vue, etc.).

TODO: Define whether the frontend is a standalone SPA, embedded in the [Wails GUI](./gui.md), or served by the runtime platform.

## Markdown Rendering

TODO: Define markdown rendering capabilities — standard CommonMark? Extensions (diagrams, code highlighting, tabs, admonitions)?

## Authentication

TODO: Define how students and instructors authenticate to the frontend.

## Responsive Design

TODO: Define target devices — desktop only? Tablet? Mobile?
