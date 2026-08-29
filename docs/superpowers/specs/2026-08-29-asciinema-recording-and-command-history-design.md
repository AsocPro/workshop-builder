# Asciinema Recording + Command-History UI — Design

**Date:** 2026-08-29
**Status:** Approved — ready for implementation planning
**Scope:** Single-user local/take-home experience (container, standalone, devcontainer modes)

## Goal

Finish the two terminal-observability features that feed the read → do → validate loop
and lay the groundwork for the later LLM-help features:

1. **Asciinema recording** — actually capture each terminal session as an asciicast
   file and let the student replay it in the UI. Today nothing is recorded and the
   playback API is stubbed.
2. **Command-history UI** — surface the already-captured command log in the student UI.
   Today the backend records commands end-to-end but no UI consumes them.

Both must work across all three built delivery modes: Docker/podman container,
standalone (`workshop serve`), and devcontainer.

## Reality this builds on (verified in code)

- **Command logging is already built end-to-end:** `workshop-platform.bashrc` logs each
  command via `PROMPT_COMMAND` → `command-log.jsonl` → `store.CommandLog` (fsnotify
  watcher + in-memory ring buffer) → `GET /api/commands`. `api.ts` already has
  `getCommands()`. No UI consumes it.
- **Asciinema is essentially unstarted:**
  - `process/ttyd.go` launches `bash` directly; nothing wraps it in `asciinema rec`, so
    no `.cast` files are ever produced.
  - `ListRecordings` / `GetRecording` handlers both return `notImplemented`. Routes
    `GET /api/recordings` and `GET /api/recordings/{filename}` are registered.
  - No player UI.
  - **The base images do not actually install asciinema** — the Dagger build installs
    only `bash curl ca-certificates jq` plus vendored `goss`/`ttyd`. The docs claim
    asciinema is pre-installed; it is not. Recording would fail everywhere today.

## Decisions locked during brainstorming

- **Scope:** asciinema end-to-end **plus** a command-history UI panel.
- **Recording model:** **per-session timestamped** `session-<ts>.cast` files (matches the
  existing plural `/api/recordings` + `/api/recordings/{filename}` routes and the
  reconnection story), not a single overwritten `session.cast`.
- **asciinema sourcing:** **vendored static binary**, the same mechanism as ttyd/goss.
  asciinema v3 ships `asciinema-x86_64-unknown-linux-musl` (amd64) and
  `asciinema-aarch64-unknown-linux-gnu` (arm64). This is the only way recording can reach
  standalone/devcontainer (which run on the real host, no base image).
- **Record format:** asciicast **v3** (asciinema 3.x default). The web player supports v3
  from **v3.10.0** onward, so the frontend pins asciinema-player ≥3.10 (latest 3.x).
- **Command-history fetch:** **client-driven periodic poll** of the existing on-demand
  `/api/commands` snapshot — no backend change needed. Resolves the open
  `backend-service.md` "poll vs on-demand" item.

## Components

### Component 1 — Asciinema capture

**Vendor the binary (Dagger + release + installer):**

- `dagger/main.go`: add `downloadAsciinemaArch(arch)` mirroring `downloadTtydArch` /
  `downloadGossArch`, pinned to a fixed asciinema release (v3.2.1 at time of writing):
  - amd64 → `asciinema-x86_64-unknown-linux-musl`
  - arm64 → `asciinema-aarch64-unknown-linux-gnu`
- Install into all three base images (`buildUbuntuBase`, `buildDebianBase`,
  `buildRockyBase`) at `/usr/local/bin/asciinema`, `0755`, alongside ttyd/goss.
- Add `asciinema-linux-<arch>` to `BuildRelease` assets.
- `scripts/install-standalone.sh`: place the asciinema binary on `PATH` next to the other
  vendored tools.

**Wrap the shell (`process/ttyd.go`):**

- ttyd currently runs `ttyd <opts> -- <shell...>`. Change `--` to point at a small
  recorder that runs **per ttyd client connection**, so each connection produces a fresh
  timestamped file. The recorder computes the timestamp at connect time (not at backend
  start) so the filename reflects when the session actually began.
- Implementation: a package-level `sh -c` recorder script (kept as a readable const, not
  a giant inline arg), invoked as `ttyd <opts> -- sh -c "$recorder"`. Logic:

  ```sh
  if command -v asciinema >/dev/null 2>&1; then
    ts=$(date -u +%Y%m%dT%H%M%SZ)
    exec asciinema rec --stdin "$WORKSHOP_RUNTIME_DIR/session-$ts.cast" -c "$WORKSHOP_SHELL_CMD"
  else
    exec $WORKSHOP_SHELL_CMD
  fi
  ```

- `TTYDManager` sets `WORKSHOP_RUNTIME_DIR` and `WORKSHOP_SHELL_CMD` in the child env.
  `WORKSHOP_SHELL_CMD` is the exact shell invocation each mode already uses:
  - container mode: `bash --login` (global instrumentation via `/etc/bash.bashrc`)
  - standalone mode: `bash --rcfile <root>/workshop-rcfile -i` (scoped instrumentation)
  - `TTYDManager` needs to know the runtime dir; pass it in at construction
    (`NewTTYDManager` / `NewTTYDManagerWithShell` gain a runtime-dir argument, threaded
    from `container.go` / `standalone.go`).
- **Graceful degradation:** if `asciinema` is not on `PATH`, the recorder `exec`s the
  shell directly. The terminal keeps working; only recording is skipped. Keeps standalone
  robust when the operator hasn't installed the binary.
- Command-logging is orthogonal and unchanged: the bashrc hook runs *inside* the recorded
  shell because the recorder execs the same shell invocation as before.

### Component 2 — Recording serve API

- Add a `RuntimeDir string` field to `handlers.Handlers`, wired through `NewServer` /
  `handlers.New` (threaded from `container.go` / `standalone.go`, which already compute
  the runtime dir).
- `GET /api/recordings` → JSON `[{ "filename", "startTime", "size" }]`:
  - Enumerate `session-*.cast` in the runtime dir.
  - `startTime` parsed from the filename timestamp; `size` from `os.Stat`.
  - Sorted newest-first.
- `GET /api/recordings/{filename}`:
  - Validate `{filename}` against `^session-[0-9TZ]+\.cast$` — reject anything else with
    400 (path-traversal / arbitrary-file-read guard). Never join untrusted input without
    this check.
  - Serve with `http.ServeContent` (opened file + `Stat` modtime) → Range support and
    conditional caching for free.
  - `Content-Type: application/x-asciicast`.
- Remove both `notImplemented` stubs in `handlers/recordings.go`.

### Component 3 — Command-history + recordings UI

- **Right-column layout:** the main content's right half (currently just `<Terminal />`)
  becomes a tabbed panel: **Terminal | Commands | Recordings**.
  - Terminal is the default tab. The terminal `<iframe>` **must stay mounted** when other
    tabs are active — use CSS show/hide (`hidden` class / `display`), **not** `{#if}` —
    otherwise switching tabs tears down the ttyd session (and its recording).
- **`CommandHistory.svelte`** (Commands tab):
  - Polls `api.getCommands()` roughly every 2s while the tab is visible; stops when
    hidden (guard with the active-tab state and clear the interval on teardown).
  - Renders each entry: timestamp, command text, exit code (styled red when non-zero).
- **`Recordings.svelte`** (Recordings tab):
  - Fetches `api.listRecordings()`; shows sessions newest-first (start time + size).
  - Selecting a session mounts **asciinema-player** (`AsciinemaPlayer.create(src, el)`)
    pointed at `api.recordingURL(filename)`; dispose the player on unmount / reselect.
- **`api.ts`:** add `listRecordings(): Promise<RecordingInfo[]>` and
  `recordingURL(filename): string`; add the `RecordingInfo` type.
- **Dependencies:** add `asciinema-player` (≥3.10, pinned latest 3.x) to
  `frontend/package.json`; import its JS API and CSS. Regenerate the lockfile via the
  Dagger `FrontendLockfile` function. Frontend still builds through Dagger/vite and embeds
  into the backend binary via `go:embed`.

## Docs to reconcile ("check docs first")

- **`docs/platform/instrumentation.md`:** resolve the contradiction between line ~91
  (per-connection `session-<timestamp>.cast`) and line ~120 (single `session.cast` at
  `GET /api/recording`). Settle on: per-session `session-<ts>.cast`, listed at
  `GET /api/recordings`, fetched at `GET /api/recordings/{filename}`. Note asciinema is a
  **vendored static binary** (not a distro/Python package) recording **asciicast v3**,
  and playback needs asciinema-player ≥3.10.
- **`docs/platform/base-images.md`:** state that asciinema is vendored and installed by
  the Dagger pipeline; reconcile the "Python package or static build" note.
- **`docs/platform/backend-service.md`:** command history is fetched by client periodic
  poll of `/api/commands`; document the recordings API surface.
- **`docs/presentation/frontend.md`:** command history is shown in the UI (Commands tab);
  recordings UI is the asciinema-player Recordings tab.
- **`docs/plan.md`:** tick the three previously-open boxes (command-history poll-vs-fetch,
  command-history UI, recordings UI).
- **`STATUS.md`:** flip the *Session recording (asciinema)* row 🟡 → ✅ as the last step
  of the milestone.

## Testing

- **Go unit tests** (Dagger `make test`), following the existing `backend_test.go`
  fixture pattern:
  - Recordings-list helper: enumerates/sorts/filters `session-*.cast`, parses start time,
    ignores non-matching files.
  - `GET /api/recordings`: shape + ordering.
  - `GET /api/recordings/{filename}`: serves a fixture `.cast` with a Range request;
    rejects traversal / non-matching filenames.
  - Recorder degradation: with `asciinema` absent, the shell is still exec'd (assert the
    constructed argv / fallback branch).
- **Manual verification** (flagged, not unit-testable): real capture + playback needs a
  live tty/ttyd. Verify a session produces a `.cast` file that the embedded player
  replays. This is a separate manual step, not part of `make test`.

## Out of scope

- Live-follow / tailing playback — serve a static seekable file only.
- The LLM help feature itself — this work is preparatory context plumbing.
- Cluster-mode recording shipping (Vector sidecar → object storage) — designed-only.
- Reworking command logging — it already works end-to-end.

## Risks

- **Player/format compatibility** — mitigated: asciinema v3 records asciicast v3;
  asciinema-player ≥3.10 supports it. Verify during implementation (TDD/manual) with a
  real recorded file.
- **asciinema binary availability in standalone** — mitigated by graceful degradation:
  no binary → no recording, terminal unaffected; the release installer ships the binary
  for the supported path.
