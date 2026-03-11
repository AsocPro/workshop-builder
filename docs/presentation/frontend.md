# Frontend — Student Web UI

## Purpose

The student-facing web application. A thin layer that displays tutorial content, sends navigation requests to the backend, and embeds the terminal. All state management and logic lives in the [backend service](../platform/backend-service.md) — the frontend only triggers backend operations and displays results.

## Delivery

The frontend is served as static assets (HTML, JS, CSS) embedded in the [workshop-backend binary](../platform/backend-service.md). Students access it by navigating to the workspace URL in a browser — there is no separate frontend server and no installation step for students.

This applies in both local mode (browser connects to the backend running inside the Docker container) and cluster mode (browser connects to the backend in the workspace pod, via whatever ingress or port-forward is configured).

## Audience

Workshop participants following a guided tutorial.

## Features

- Step content navigation (view any step's tutorial)
- Resume progress from last checkpoint
- Markdown tutorial content display
- Embedded terminal (WebSocket to ttyd, proxied through backend)
- Validation feedback (did the student complete the step correctly?)
- LLM help panel (chat-like interface for contextual assistance, when configured)
- Cluster status indicator (optional — shown when backend reports cluster mode is enabled)
- Step management link (always shown in local mode; conditionally shown in cluster mode if provided)

## Key Constraints

All state management, reset logic, and validation lives in the backend. The frontend:

1. Sends requests to the backend API
2. Displays results and content
3. Embeds the terminal WebSocket stream

The frontend does not talk to the Kubernetes API or any external service directly.

**Step lifecycle (reset, image swap, container restart) is not managed by the student UI.** That responsibility belongs to the CLI (Docker mode) or the operator/instructor tooling (K8s mode). The student UI is read-only with respect to workspace lifecycle.

---

## Technology

### Framework

**Svelte 5** with **Vite**.

Svelte compiles to vanilla JS — no virtual DOM, small bundle, readable component files. Good choice for keeping the codebase approachable to non-web contributors.

Build output (`vite build` → `dist/`) is embedded in the Go binary using `//go:embed`. The backend serves static assets at `/` with no separate frontend server.

### Styling

**Tailwind CSS** — utility-first, desktop-first layout. Responsive breakpoints (`sm:`, `md:`, `lg:`) handle tablet and mobile without writing custom CSS. Goal is a layout that works well on desktop and is fully usable on tablet/phone (good for on-the-go learners).

### Markdown Rendering

Client-side rendering pipeline:

| Layer | Library | Notes |
|---|---|---|
| Core renderer | **markdown-it** | CommonMark-compliant; what GitLab uses |
| Syntax highlighting | **highlight.js** via `markdown-it-highlightjs` | Simpler than Shiki; good enough for tutorial content |
| Diagrams | **Mermaid.js** | Post-processes fenced `` ```mermaid `` blocks into SVG |
| Task lists | `markdown-it-task-lists` | GFM-style `- [x]` checkboxes |
| Admonitions | `markdown-it-container` | `:::note`, `:::warning`, `:::tip` blocks |
| Heading anchors | `markdown-it-anchor` | Deep links into step content |

Markdown content is fetched from the backend as raw text and rendered entirely in the browser. The Go backend does not parse or transform markdown.

This gives parity with what authors and students expect from GitHub/GitLab-flavored markdown — tables, fenced code blocks, task lists, diagrams, callout blocks.

## API Surface

All communication is to the backend running in the same container. No cross-origin requests — the frontend is served from the same origin as the API.

### REST Endpoints

#### Workshop State

| Method | Path | When called | Notes |
|---|---|---|---|
| `GET` | `/api/state` | On load; after validation | Returns active step ID, completed step set, navigation mode (`linear`/`free`/`guided`) |
| `GET` | `/api/steps` | On load | Returns all steps with titles, group membership, completion flags, and accessibility (locked/unlocked) |

The frontend fetches both on initial load to bootstrap the UI. `/api/state` is re-fetched after validation to update completion indicators without a full page reload.

#### Step Content

| Method | Path | When called | Notes |
|---|---|---|---|
| `GET` | `/api/steps/:id/content` | When student selects a step | Returns raw markdown text; rendered client-side |

Viewing step content is a lightweight fetch — no workspace change. The backend appends a `step_viewed` event to `state-events.jsonl` on each call, enabling timestamp-based correlation with the command log at query time.

#### Validation

| Method | Path | When called | Notes |
|---|---|---|---|
| `POST` | `/api/steps/:id/validate` | When student clicks Validate | Runs goss; returns per-test pass/fail results |

Response shape (per-test results) drives the validation feedback panel. After a passing result, the frontend re-fetches `/api/state` to update the completion set.

**Completed steps lock validation.** If a step's ID is already in the completed set (from `/api/state`), the Validate button is replaced with a static "Completed" indicator — no re-validation is offered. This prevents confusing false failures: in a linear tutorial, a later step's environment will break earlier steps' goss tests. The completion set is the authoritative record; goss results from an already-transitioned workspace are meaningless.

#### Command History

| Method | Path | When called | Notes |
|---|---|---|---|
| `GET` | `/api/commands` | Periodically or on demand | Recent shell commands from the terminal session; used for display and LLM context |

Supports pagination. The frontend may poll this or display it on demand — TBD.

#### Session Recordings

| Method | Path | When called | Notes |
|---|---|---|---|
| `GET` | `/api/recordings` | When student opens recordings view | Lists available `.cast` files with start timestamps |
| `GET` | `/api/recordings/:filename` | When student plays a recording | Streams asciicast v2 file with HTTP Range support for seeking |

#### LLM Help

| Method | Path | When called | Notes |
|---|---|---|---|
| `POST` | `/api/steps/:id/llm/help` | When student clicks Help | Streaming response (SSE or chunked); renders incrementally in the help panel |
| `GET` | `/api/steps/:id/llm/history` | When help panel opens | Previous interactions for this step |

The step ID is explicit in the path. The student may be browsing step 3's content while their workspace is on step 5 — the backend must assemble context for the step being viewed, not whichever image is currently running.

The backend context assembly for a help request includes: the step's `content.md`, its `llm.json` instructor hints, reference docs from `llm-docs/`, the latest goss results for that step, and recent commands from the command log. Commands are not tagged with a step ID in the command log, but `step_viewed` events in `state-events.jsonl` provide timestamp anchors that allow correlation at query time. The LLM help path uses recent commands as-is — the step content and instructor hints provide enough context without per-step command filtering.

The LLM help panel is only shown when the backend has LLM configured (indicated in `/api/state` or a capability flag — TBD).

### WebSocket

| Path | Purpose |
|---|---|
| `/ws/terminal` | Proxied connection to ttyd. The frontend embeds the ttyd web UI in an `<iframe>`. |

### Static Assets

| Path | Description |
|---|---|
| `/` | The Svelte SPA (index.html + hashed JS/CSS bundles) |
| `/assets/*` | JS, CSS, fonts emitted by Vite build |

## Cluster Status Indicator

When the backend reports that cluster mode is enabled (via `/api/state` or a capabilities field — TBD), the UI shows a small colored status button — green for up, red for down. The frontend polls the backend for cluster health and updates the indicator. No detail panel, no diagnostics — just a quick health signal so the student knows if their cluster target is reachable.

Cluster mode is baked into `workshop.json` at image build time, so the backend always knows whether to expose this. No env var needed — unlike `WORKSHOP_MANAGEMENT_URL`, which is runtime-injected because the management server address changes per invocation.

## Authentication

- **Docker mode (single-user)**: No authentication. The backend is only accessible to whoever can reach the container's port — no login required.
- **Cluster mode (multi-tenant)**: Authentication is handled externally via [OAuth2 Proxy](https://oauth2-proxy.github.io/oauth2-proxy/) in front of the workspace ingress, integrated with [Authentik](https://goauthentik.io/) as the identity provider. The backend itself does not implement auth — it sits behind the proxy and trusts the network boundary.

## Responsive Design

Desktop-first layout. Tablet and mobile are supported — the goal is a usable experience on any device, since students may want to follow along on a tablet or phone.

Layout priorities:
- **Desktop**: sidebar for step navigation + main content pane + embedded terminal below or beside
- **Tablet**: collapsible sidebar, stacked content and terminal
- **Mobile**: single-column, terminal accessible via tab/toggle

---

## What This Is NOT

Builder mode is a **separate binary** (the [Wails desktop GUI](./gui.md)), not a mode of this frontend. Authors do not use this frontend to build workshops — they use the Wails app on their workstation.

Step lifecycle management (reset, step transitions, image swaps, container restarts) is **not** a student UI responsibility. The student cannot trigger a workspace transition from this UI. That is handled by the CLI in Docker mode and the operator/instructor tooling in K8s mode.
