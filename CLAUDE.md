# CLAUDE.md

Guidance for Claude Code (and humans) working in this repo.

## What this project is

A tool for building and running technical Linux workshops. An author writes a
`workshop.yaml` + per-step directories (markdown, goss validation, setup files); it
compiles to self-contained artifacts; a student gets a browser UI with step
navigation, an embedded web terminal, and a "Validate" button (goss). The read → do →
validate → complete loop.

## READ THIS FIRST: docs describe more than exists

The `docs/` tree is the **design spec / goal**, and it describes a much larger system
than is built — including a multi-tenant Kubernetes platform (operator, CRDs,
aggregation, instructor dashboard) that has **zero implementing code**. Do not assume a
capability exists because a doc describes it.

- **[STATUS.md](STATUS.md) is the source of truth for what actually runs** (✅ built /
  🟡 partial / ⬜ designed-only). Check it before claiming, using, or modifying a feature.
- **[DECISIONS.md](DECISIONS.md) is why things are the way they are** — decisions +
  reasoning + what would make us revisit. Read it before questioning a design choice or
  re-proposing something (e.g. it records that the full multi-tenant scope is kept on
  purpose, and the single-user prune was considered and deferred — don't re-open unprompted).
- `docs/` = design of record. `plans/mNN-*.md` = implementation milestones (what got
  built, in order). `docs/plan.md` tracks *doc* completeness, not build state.
- When code and spec diverge, that's a signal to reconcile — surface it, don't silently
  pick one.

What's built today: single-user local/take-home (compile → backend serves UI → three
delivery modes). What's designed-only: the cluster/multi-tenant half. See STATUS.md.

## Build & test — everything runs through Dagger

**There is no local Go or Node toolchain assumption.** All compilation and testing
happens inside Dagger containers. Write source directly; run builds/tests via Dagger.
Engine pinned in `dagger.json` (v0.19.11).

| Task | Command |
|---|---|
| Run tests | `make test` → `dagger call test --src .` (Go tests for `pkg/workshop` + `backend`) |
| Build backend binary | `make build-backend` |
| Build workshop step images | `make build-workshop` (loads into podman) |
| Build base images | `make base-images` |
| Build CLI | `make build-cli` |
| Build release assets | `make build-release` |
| Update go.mod/go.sum | `make tidy` (→ `dagger call go-mod-tidy`) |

Other Dagger functions exist (`Dev`, `DevFrontend`, `RunBackend`, `PublishBaseImages`,
`BuildFeature`, etc.) — list with `dagger functions`. The frontend builds via
`npm run build` (vite) *inside* Dagger; `frontend/dist` is overlaid into the backend
binary at build time (source has only a `.gitkeep` placeholder).

## Architecture (the built reality)

Three binaries, one shared compiler:

- **`workshop-backend`** (`backend/`) — serves the student UI (embedded Svelte via
  `go:embed`) + terminal (ttyd) + goss validation. Multi-mode via flags: default
  container entrypoint, `--serve <dir>` (standalone), `service install|uninstall`
  (systemd). This is the always-present core.
- **`workshop`** (`cli/`) — host-side **podman** orchestrator (not the Docker daemon):
  starts the container, runs a separate management panel, does **image-swap** step
  transitions. Docker/podman delivery mode only.
- **`compile-workshop` / `workshop-setup`** (`cmd/`) — thin build-from-source wrappers
  used by standalone and devcontainer modes.
- **`pkg/workshop`** — the ONE compiler (`Compile`, `CompileToDir`). Dagger,
  `compile-workshop`, and standalone all call it. No compile logic lives anywhere else.

Delivery modes (all built): Docker/podman CLI (image-swap transitions) · standalone
(`--serve`, in-place transitions) · DevContainer feature.

Key design facts:
- Each workshop **step is a separate tagged OCI image** (`<image>:<step-id>`), built
  `FROM` a `workshop-base:{ubuntu,rocky,debian}` image, with `/workshop/` flat-file
  metadata baked in.
- **Runtime state is ephemeral** — in-memory, rebuilt each start; no DB, no volume, no
  persistence across restarts. Switching steps via image-swap wipes the container.
- Two transition mechanisms coexist: image-swap (CLI) vs. in-place (standalone/devcontainer).

## Source layout

- `pkg/workshop/` — parse / validate / compile (the single compiler)
- `backend/` — runtime server: handlers, store, process (ttyd), setup, servecmd (systemd), standalone
- `cli/` — podman orchestration + management panel + image-swap
- `cmd/` — `compile-workshop`, `workshop-setup` wrappers
- `dagger/` — build pipeline (its own Go module)
- `frontend/` — Svelte 5 + Vite (built into the backend binary)
- `devcontainer-feature/` — DevContainer Feature (install.sh + metadata)
- `examples/hello-linux/` — the reference workshop, used as the test fixture
- `docs/` — spec · `plans/` — implementation milestones · `STATUS.md` — build tracker · `DECISIONS.md` — decisions + rationale

## Conventions & gotchas

- Module: `github.com/asocpro/workshop-builder`, Go 1.24. HTTP router: chi. CLI: cobra.
- Uses **podman** in local mode, not the Docker daemon directly.
- The **help panel is static markdown** (hints/explain/solve baked into the image),
  **not** a live LLM. Session **recording playback is stubbed** (`/api/recordings*` →
  `notImplemented`). Both are 🟡 in STATUS.md.
- Solo repo; work has gone directly on `master`. Commit/push only when asked.
- When you finish a milestone, update the relevant **STATUS.md** row — that's the
  designated place build state changes.
