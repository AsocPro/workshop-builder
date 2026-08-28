# M15 — CLI Unification Design

> Design of record for [plans/m15-cli-unification.md](../../../plans/m15-cli-unification.md).
> Approved 2026-08-28. Hard-cutover approach (no back-compat aliases).

## Goal

Collapse the four separately-named binaries into **one `workshop` command** with
subcommands. The unification is a thin cobra front-door over cleanly-separated
packages — not a merge that entangles them.

## Current state (what we're replacing)

| Binary | Source | Logic location | Role |
|---|---|---|---|
| `workshop-backend` | `backend/main.go` (`package main`) | in `package main` (`runContainer`, `runStandalone`, `standaloneOptions`, `NewServer`); `servecmd.RunService` already exported | Bare = container entrypoint; `--serve` = standalone; `service install\|uninstall` = systemd |
| `workshop` (CLI) | `cli/main.go` → `cli/cmd.Execute()` | already exported cobra (`run` subcommand) | Host-side podman orchestrator + management panel + image-swap transitions |
| `compile-workshop` | `cmd/compile-workshop/main.go` | in `package main` | Compile source → JSON (stdout) or flat-file dir |
| `workshop-setup` | `cmd/workshop-setup/main.go` | in `package main` | Pre-apply prior steps' setup (devcontainer skip-ahead) |

The core problem: three of four keep their logic *in `package main`*, so a shared
dispatcher can't call them. The bulk of the work is extracting each `main()` into an
exported entrypoint, then wiring them as subcommands.

## Target shape

One binary `workshop` built from a new `cmd/workshop/` package (cobra dispatch):

| Command | Delegates to | Replaces |
|---|---|---|
| `workshop` (no subcommand) | backend container entrypoint | `workshop-backend` (bare) — the in-image ENTRYPOINT |
| `workshop serve <dir>` | backend standalone | `workshop-backend --serve` |
| `workshop service install\|uninstall` | backend servecmd | `workshop-backend service` |
| `workshop run <image>` | cli orchestrator | `workshop` CLI |
| `workshop compile <dir>` | compile logic | `compile-workshop` |
| `workshop setup` | setup logic | `workshop-setup` |

Bare `workshop` (no args) → root `RunE` runs container mode, so the image ENTRYPOINT is
behaviorally identical; only the binary path changes (`workshop-backend` → `workshop`).

## Design

### 1. Dispatch

`cmd/workshop/main.go`: cobra root + subcommand wiring only, **no business logic**. Root
`RunE` (no subcommand) → container entrypoint. Subcommands `serve`, `service`, `run`,
`compile`, `setup`.

### 2. Refactor — make each capability callable

- **`backend/`** — extract `runContainer`, `runStandalone`/`standaloneOptions`, and the
  `NewServer` wiring out of `package main` into exported entrypoints (e.g.
  `RunContainer()`, `RunStandalone(opts)`). `servecmd.RunService` is already exported.
  **`go:embed frontend/dist` stays in the `backend` package** — embedding is unchanged.
- **`cli/`** — already exported; lift its `run` cobra command into the umbrella instead
  of hosting a second root.
- **`cmd/compile-workshop` & `cmd/workshop-setup`** — move `main()` bodies into exported
  funcs the subcommands call.
- Delete the old `main` packages: `backend/main.go`, `cli/main.go`,
  `cmd/compile-workshop/`, `cmd/workshop-setup/`.

Packages stay separated; only the front door merges. Adding/removing a delivery mode
stays a package-level change plus one line in the dispatcher.

**Accepted tradeoff:** the single binary carries the embedded frontend *and* the
podman/cli code, and is baked into every base image. It's one static CGO-free binary —
unused code just adds size. This matches the plan's "one binary" goal.

### 3. Build & release (`dagger/main.go`)

- Replace `BuildBackend`/`BuildCompileWorkshop`/`BuildSetup`/`BuildCLI` with **one
  `BuildWorkshop`** that overlays `frontend/dist` (for the embed) then
  `go build ./cmd/workshop/`. Base-image build + release both use it.
- `BuildRelease` ships one `workshop-linux-<arch>` binary (plus vendored
  ttyd/goss/bashrc/installer) instead of four.
- Base images: ENTRYPOINT → `/usr/local/bin/workshop`.
- Internal Dagger call `go run ./cmd/compile-workshop/` → `go run ./cmd/workshop/ compile`.

### 4. Downstream callers (hard cutover — no aliases)

- `devcontainer-feature/src/workshop/install.sh`: download one `workshop` binary;
  `compile-workshop ...` → `workshop compile ...`; `workshop-setup ...` →
  `workshop setup ...`.
- `devcontainer-feature/src/workshop/devcontainer-feature.json`: postStart → `workshop`.
- `scripts/install-standalone.sh`, `scripts/smoke-test.sh`, `Makefile` → new
  names/subcommands.
- Docs: `STATUS.md` m15 row → ✅; `CLAUDE.md` architecture section; relevant `docs/`.

### 5. Testing

- Existing `pkg/workshop` + `backend` Go tests stay green (`make test`).
- `backend/servecmd/systemd_test.go` hardcodes `/usr/local/bin/workshop-backend` in
  expected unit output → `workshop` (generator takes the exec path as an arg).
- Add a dispatch test for `cmd/workshop/` (subcommands route; `--help` lists them).
- `scripts/smoke-test.sh` exercises the built image entrypoint.

## Acceptance

- `workshop <subcommand>` dispatches to the right mode; old binary names removed.
- Release pipeline ships one binary (plus vendored ttyd/goss) instead of several.
- Each delivery mode remains an isolated package behind the dispatcher.
