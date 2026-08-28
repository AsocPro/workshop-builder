# Build Status

**What actually runs, vs. what's only designed.** This is the reality counterpart to
the spec (`docs/`) and the doc-build tracker (`docs/plan.md`). The docs stay the goal;
this table is the honest front page for "is it built yet."

- The **`docs/` spec is the design of record.** When code and spec diverge, that's a
  signal to reconcile — either update the code toward the spec or update the spec toward
  a deliberate change. This table just tells you where each capability currently stands.
- Grain is **one row per real capability**, not per doc section or per endpoint.

**Legend** — ✅ Built (works end-to-end) · 🟡 Partial (some of it real, rest stubbed/designed) · ⬜ Designed only (spec exists, no code)

**Keeping this honest:** updating the relevant row is the *last step of finishing a
milestone* (that's the only place build state actually changes). Keep states coarse —
detail lives in the `plans/mNN` file, this table just rolls up.

---

## Authoring & Build

| Capability | Designed | Built | Milestone | Notes |
|---|---|---|---|---|
| `workshop.yaml` parse / validate / compile | [compilation](docs/artifact/compilation.md), [workshop](docs/definition/workshop.md) | ✅ | [m2](plans/m2-shared-library.md) | Single compiler: `pkg/workshop`. Incremental rebuild (`--from-step`) documented; not surfaced as a flag yet |
| Flat-file `/workshop/` artifact | [flat-file-artifact](docs/artifact/flat-file-artifact.md) | ✅ | [m2](plans/m2-shared-library.md)/[m4](plans/m4-dagger-pipeline.md) | JSON metadata baked per step |
| Dagger per-step OCI image build | [compilation](docs/artifact/compilation.md) | ✅ | [m4](plans/m4-dagger-pipeline.md) | One tagged image per step, `FROM` base |
| Base images (`ubuntu`/`rocky`/`debian`) | [base-images](docs/platform/base-images.md) | ✅ | [m12](plans/m12-base-images.md) | `build-base-images` + `publish-base-images` |
| Release binary pipeline | — | ✅ | [m13](plans/m13-devcontainer-feature.md) | `build-release`: binaries + vendored ttyd/goss + bashrc + installer |
| `compile-workshop` / `workshop-setup` helpers | [devcontainer](docs/platform/devcontainer-feature.md) | ✅ | [m13](plans/m13-devcontainer-feature.md) | Thin wrappers over `pkg/workshop` for build-from-source modes |

## Runtime (backend — one binary, embedded UI)

| Capability | Designed | Built | Milestone | Notes |
|---|---|---|---|---|
| Backend serves embedded SPA | [frontend](docs/presentation/frontend.md), [backend-service](docs/platform/backend-service.md) | ✅ | [m5](plans/m5-frontend.md)/[m6](plans/m6-m7-embed-terminal.md) | Svelte built into the Go binary via `go:embed` |
| Step list + markdown content | [frontend](docs/presentation/frontend.md) | ✅ | [m5](plans/m5-frontend.md)/[m6](plans/m6-m7-embed-terminal.md) | |
| Navigation enforcement (linear / free / guided + `requires`) | [backend-service](docs/platform/backend-service.md) | ✅ | — | `backend/store/state.go` |
| Web terminal (ttyd) | [instrumentation](docs/platform/instrumentation.md) | ✅ | [m7](plans/m6-m7-embed-terminal.md) | ttyd proxy + WS |
| goss validation → completion | [backend-service](docs/platform/backend-service.md) | ✅ | [m8](plans/m8-goss-validation.md) | Marks step complete on pass |
| Command logging | [instrumentation](docs/platform/instrumentation.md) | ✅ | [m10](plans/m10-m11-commands-help.md) | PROMPT_COMMAND → `command-log.jsonl` → `/api/commands` |
| Help panel (static hints/explain/solve) | [llm-help](docs/platform/llm-help.md) | 🟡 | [m11](plans/m10-m11-commands-help.md) | Serves baked markdown over SSE; **not** a live LLM |
| State event log | [flat-file-artifact](docs/artifact/flat-file-artifact.md) | ✅ | — | Append-only; state in-memory, no persistence |
| In-place step transition (`activate`) | [standalone](docs/platform/standalone-mode.md) | ✅ | [m13](plans/m13-devcontainer-feature.md)/[m14](plans/m14-standalone-mode.md) | Standalone/devcontainer modes |
| Session recording (asciinema) | [instrumentation](docs/platform/instrumentation.md) | 🟡 | — | `/api/recordings*` handlers return `notImplemented` — playback API stubbed |

## Delivery Modes

| Capability | Designed | Built | Milestone | Notes |
|---|---|---|---|---|
| Docker/podman + CLI (`workshop run`) | [cli](docs/platform/cli.md) | ✅ | [m9](plans/m9-cli.md) | Host management panel + **image-swap** step transitions |
| Standalone (`--serve` + systemd) | [standalone](docs/platform/standalone-mode.md) | ✅ | [m14](plans/m14-standalone-mode.md) | Compiles from source, runs on the real host |
| DevContainer feature | [devcontainer](docs/platform/devcontainer-feature.md) | ✅ | [m13](plans/m13-devcontainer-feature.md) | Feature downloads release binaries, compiles from source |
| Cluster / multi-tenant (operator) | [operator](docs/platform/operator.md) | ⬜ | — | Designed only |
| CRDs (WorkspaceTemplate / WorkspaceInstance) | [crds](docs/platform/crds.md) | ⬜ | — | Designed only |
| Infrastructure provisioners (k3d / vcluster) | [provisioners](docs/platform/infrastructure-provisioners.md) | ⬜ | — | Designed only |
| `infrastructure` block / extraContainers (runtime) | [workshop](docs/definition/workshop.md) | ⬜ | — | Schema documented; no runtime provisioning |

## Multi-Tenant / Aggregation (cluster mode — designed)

| Capability | Designed | Built | Milestone | Notes |
|---|---|---|---|---|
| Aggregation (Vector → Postgres/S3) | [aggregation](docs/platform/aggregation.md) | ⬜ | — | Designed only |
| Instructor dashboard (real-time, SSE, OIDC) | [instructor-dashboard](docs/platform/instructor-dashboard.md) | ⬜ | — | Designed only; instructor mode explicitly removed from single-user Docker |

## Other (designed)

| Capability | Designed | Built | Milestone | Notes |
|---|---|---|---|---|
| Live LLM help (streaming API, context assembly) | [llm-help](docs/platform/llm-help.md) | ⬜ | — | Static-file version is the 🟡 "Help panel" row above |
| Builder GUI (Wails desktop app) | [gui](docs/presentation/gui.md) | ⬜ | — | Authoring is CLI + edit-YAML today |
| Unify tools into one `workshop` command | — | ⬜ | [m15](plans/m15-cli-unification.md) | Planned; backend already partially unified |

---

## The designed-but-not-built gap (at a glance)

Everything for **single-user, local, take-home** is ✅. The ⬜ rows are the
**multi-tenant / cluster half** of the plan — deliberate core scope per the design,
not yet implemented:

- Cluster operator + CRDs + provisioners
- Aggregation (Vector/Postgres) + instructor dashboard
- Live LLM help, Builder GUI

Two partials to be aware of: **help panel** is static markdown (not a live LLM), and
**session recording** capture may run but the playback API is stubbed.
