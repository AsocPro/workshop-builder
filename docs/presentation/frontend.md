# Frontend — Student Web UI

## Purpose

The student-facing web application. A thin layer that displays tutorial content, sends navigation requests to the backend, and embeds the terminal. All state management and logic lives in the [backend service](../platform/backend-service.md) — the frontend only triggers backend operations and displays results.

## Delivery

The frontend is served as static assets (HTML, JS, CSS) embedded in the [workshop-backend binary](../platform/backend-service.md). Students access it by navigating to the workspace URL in a browser — there is no separate frontend server and no installation step for students.

This applies in both local mode (browser connects to the backend running inside the Docker container) and cluster mode (browser connects to the backend in the workspace pod, via whatever ingress or port-forward is configured).

## Audience

Workshop participants following a guided tutorial.

## Features

- Step content navigation (view any step's tutorial — no container restart required)
- Step transition / reset (switches workspace to a different step's image — triggers container restart)
- Resume progress from last checkpoint
- Markdown tutorial content display
- Embedded terminal (WebSocket to ttyd, proxied through backend)
- Validation feedback (did the student complete the step correctly?)
- LLM help panel (chat-like interface for contextual assistance, when configured)
- Cluster status panel (optional)

TODO: Define the UX distinction between "view step content" (no image swap) and "switch workspace to step" (image swap + container restart). Students in free navigation mode need to browse step content without disrupting their terminal session. The UI must make it clear which action triggers a restart.

## Key Constraints

All state management, reset logic, and validation lives in the backend. The frontend:

1. Sends requests to the backend API
2. Displays results and content
3. Embeds the terminal WebSocket stream

The frontend does not talk to the Kubernetes API or any external service directly.

---

## Technology

TODO: Define the frontend framework (React, Svelte, Vue, etc.).

## API Surface

TODO: Define the full student-facing API surface — REST for content and navigation, WebSocket for terminal proxying. Define route structure.

## Cluster Status Panel

TODO: Define what information is shown in the cluster status panel (for workspaces with `cluster.mode == per-workspace`). Real-time or polled?

## Markdown Rendering

TODO: Define markdown rendering capabilities — standard CommonMark? Extensions (syntax highlighting, diagrams, admonitions, tabs)?

## Authentication

TODO: Define how students authenticate to access their workspace frontend.

## Responsive Design

TODO: Define target devices — desktop only? Tablet? Mobile?

---

## What This Is NOT

Builder mode is a **separate binary** (the [Wails desktop GUI](./gui.md)), not a mode of this frontend. Authors do not use this frontend to build workshops — they use the Wails app on their workstation.
