# Standalone Mode — Workshop Backend Directly on a Server

## Purpose

Standalone mode runs the workshop platform directly on a Linux server — no container, no CLI orchestration. The backend, terminal, and goss validation operate against the real host. The server state IS the workshop state.

This is the **interactive runbook / onboarding delivery mode**. The motivating use case: a team operates a server-based platform (e.g., an internal Kubernetes-based platform) and wants a unified docs-and-run interface for onboarding engineers who have never touched the server, or for refreshing engineers who have. Workshop steps are living documentation — content rendered next to a terminal on the actual machine, with goss validation checking the real environment. Docs that execute and verify cannot silently rot: when the platform drifts from the docs, validation fails.

This is the third delivery mode:

| Mode | Audience | Isolation | Step Transitions |
|---|---|---|---|
| Container mode (`workshop run`) | Classroom, instructor-led | OCI image per step | External swap (CLI/Operator) |
| [DevContainer mode](./devcontainer-feature.md) | Self-paced, async students | Devcontainer | In-place (`setup.json`) |
| **Standalone mode** | Single operator on a real server | **None — intentionally** | In-place (`setup.json`), often none |

## Scope Constraints

These are hard constraints, not roadmap items. They are what keep standalone mode simple:

1. **Single user per server.** One person owns the server at a time. Completion state, command logs, and terminal sessions all assume exactly one operator. There is no per-user state, no session isolation, no identity model.
2. **Server state is authoritative.** Goss validation reflects the state of the machine, not the progress of a person. If the environment already satisfies a step, that step validates as complete. This is the desired semantic: the workshop describes the server, and the server answers.
3. **Private networks only.** Basic auth over HTTP is the ceiling for v1. No TLS machinery, no rate limiting, no lockout. Deployment on anything reachable from an untrusted network is out of scope and documented as such.
4. **Linux only.** Same constraint as every other mode — ttyd and goss are Linux tools.
5. **No reset.** Step transitions mutate the real host and are one-directional. "Reset" means re-provisioning the server, which is outside the platform's responsibility.

## Why This Is Not the Rejected "Passthrough Mode"

[DevContainer Feature — Why Not a Native Binary?](./devcontainer-feature.md#why-not-a-native-binary) evaluated and rejected running the binary on a bare host **as the self-paced student delivery mode**. Every rejection reason was specific to that audience, and every one inverts here:

| Rejection reason (student passthrough) | Standalone mode (server runbook) |
|---|---|
| No Windows support | Irrelevant — the target is a Linux server |
| Step transitions are destructive on a real host | That's the point — the workshop configures the real machine |
| Workshop commands run with the user's full permissions | Intended — the operator owns the server and wants those commands run |
| WSL2 negates the value | Not the scenario |

The passthrough rejection stands for self-paced student delivery. Standalone mode is a different audience with different trade-offs.

## How It Works End-to-End

1. The team keeps a workshop definition (`workshop.yaml` + steps) in a git repo — typically alongside or inside the platform's own repo
2. The operator installs the platform tools on the server (same release assets the devcontainer feature uses)
3. The operator serves the workshop straight from a checkout:

```bash
git clone git@internal:platform/onboarding-workshop
workshop-backend --serve ./onboarding-workshop
```

4. `--serve` compiles `workshop.yaml` to a runtime directory (invoking the same logic as `compile-workshop --output-dir`), then starts the backend against it
5. The operator opens the UI — via SSH port-forward, or directly on the private network
6. Steps render content, the embedded terminal is a shell on the real server, Validate runs goss against the real environment

No image build in the loop. Content iteration is `git pull` and restart — the workflow of docs, not of release engineering.

## What Gets Installed

The same single-version release bundle as the [devcontainer feature](./devcontainer-feature.md#what-the-feature-installs), delivered by an installer script instead of `install.sh`:

| Component | Purpose |
|---|---|
| `workshop-backend` | Serves UI, API, proxies terminal, compiles on `--serve` |
| `workshop-setup` | Pre-applies setup for skip-ahead (optional) |
| `ttyd` | Terminal-over-HTTP |
| `goss` | Step validation against the live host |

`compile-workshop` is not separately required — `--serve` embeds compilation — but is available for pre-compiling or CI validation of workshop content.

The command-logging bashrc is not installed at all: it is embedded in the `workshop-backend` binary via `go:embed` (same single source of truth as the base images and the devcontainer feature) and written as an rcfile under `WORKSHOP_ROOT` on `--serve`. The installer's only real job is fetching the binaries; everything else — instrumentation rcfile, runtime directory, systemd unit — the binary sets up on its own, with no root required for the ad-hoc flow.

### Instrumentation Is Scoped to Workshop Sessions

In base images, the bashrc is sourced globally via `/etc/bash.bashrc` — fine inside a container that exists only for the workshop. On a real server, instrumenting every shell on the box is unacceptable. In standalone mode:

- The installer does **not** touch `/etc/bash.bashrc` or any global shell config — nothing is written to `/etc` at all
- The backend writes the embedded instrumentation as a generated rcfile under `WORKSHOP_ROOT` and launches the ttyd shell with it explicitly (`bash --rcfile` — the generated rcfile sources the user's own `~/.bashrc` first, so their aliases, prompt, and kubeconfig exports survive)
- The instrumentation honors `WORKSHOP_ROOT` for the command-log path instead of assuming `/workshop`

SSH sessions, cron, and every other shell on the server are untouched. Only terminals opened through the workshop UI are logged.

## Backend: Standalone Mode

`WORKSHOP_MODE=standalone` (set by `--serve`). Behavior is the in-place transition model shared with devcontainer mode, plus server-appropriate adjustments:

- **No management URL** — nothing manages containers; there are no containers
- **Step activation** — in-place setup (`setup.json`: copy staged files, run commands), identical code path to devcontainer mode. Steps with no setup (the common case for runbooks) simply update the active step
- **Configurable root** — `WORKSHOP_ROOT` points at the compiled output directory (e.g., `~/.workshop/<name>/` or a path under the checkout); never assumes `/workshop`
- **Listen address** — defaults to `127.0.0.1:8080` (SSH port-forward friendly). `--listen 0.0.0.0:8080` opts into network exposure and logs a prominent warning if auth is not enabled
- **Runs as the invoking user** — the terminal is a shell as whoever started the backend. Their kubeconfig, their RBAC, their permissions. For platform onboarding this is exactly right: the operator works as themselves

### Basic Auth

Optional, for the network-exposed variant:

```bash
workshop-backend --serve ./onboarding-workshop \
  --listen 0.0.0.0:8080 \
  --auth-user admin --auth-password-file ./pass
```

- HTTP Basic auth on every route. The terminal WebSocket upgrade is an HTTP request through the same backend proxy, so it is gated with no extra mechanism
- Credentials via flags/env/file — never baked into workshop content
- **This protects a shell on the server.** The docs are opinionated: private networks only; prefer SSH port-forward (`ssh -L 8080:localhost:8080 dev-server`) which needs no auth at all because SSH already authenticated the operator

## Step Transitions and Validation Semantics

Same in-place model as [devcontainer mode](./devcontainer-feature.md#step-transitions-in-place-linear-progression), with the server-state framing:

- **Most runbook steps have no setup.** The platform already exists; steps are "read, run, validate." Setup is reserved for steps that genuinely stage files or scaffold examples
- **Validation is a statement about the server.** A fresh backend process starts with an empty completion set; the operator re-validates and the live environment answers. There is no progress to persist because the server *is* the progress
- **Authoring guideline: idempotent steps.** Setup commands and instructed commands should be safe to re-run, since the same server may host the workshop repeatedly across its lifetime (refreshers, re-onboarding after re-provisioning)

## Deployment Patterns

### Ad-Hoc (Recommended)

The operator runs the backend on demand from a checkout, accesses it via SSH port-forward, and stops it when done. Zero installation beyond the tool binaries, zero auth configuration, naturally fresh state per session.

### Persistent Service

For a long-lived "this server's docs live at :8080" setup, the binary installs itself as a systemd service — no hand-written unit files:

```bash
workshop-backend service install \
  --serve /opt/onboarding-workshop \
  --listen 0.0.0.0:8080 \
  --auth-user admin --auth-password-file /etc/workshop/pass
```

`service install`:

1. Resolves its own absolute path (`os.Executable()`) for `ExecStart`
2. Bakes the provided serve flags into the unit verbatim — the unit is a frozen invocation of the same command the operator would run ad-hoc
3. Writes `/etc/systemd/system/workshop.service` (or `~/.config/systemd/user/workshop.service` with `--user`, for running under the operator's own account without root)
4. Prints the follow-up commands rather than running them: `systemctl daemon-reload && systemctl enable --now workshop`

`--print` writes the generated unit to stdout instead of installing it, for review or for piping through `sudo tee` when the backend itself isn't running as root. `service uninstall` removes the unit.

Generated unit:

```ini
[Unit]
Description=Workshop runbook (onboarding-workshop)
After=network.target

[Service]
ExecStart=/usr/local/bin/workshop-backend --serve /opt/onboarding-workshop --listen 0.0.0.0:8080 --auth-user admin --auth-password-file /etc/workshop/pass
Restart=on-failure
User=platform-admin

[Install]
WantedBy=multi-user.target
```

`User=` defaults to the invoking user (via sudo, the user who ran sudo) — the service must run as a real operator account, never as a system nobody-user, because the terminal shell and kubeconfig semantics depend on it.

Basic auth required for this variant — `service install` refuses a non-loopback `--listen` without auth flags. Still single-user semantics: the auth gate controls *access*, not *identity* — whoever holds the credential is the operator.

## What It Does NOT Do

- Does not provision infrastructure — no k3d, no `extraContainers`. Workshops declaring `infrastructure.cluster` or `infrastructure.extraContainers` fail validation with a clear message (per the [backend capabilities](./backend-capabilities.md) pattern); the server's environment is assumed to already exist
- Does not support multiple users — no sessions, no per-user state, no identity
- Does not provide TLS — private networks only (v1)
- Does not reset or isolate anything — the host is mutated for real
- Does not run on Windows or macOS
- Does not replace devcontainer mode for self-paced student delivery — that rejection stands

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Backend Service](./backend-service.md) | Same binary; `WORKSHOP_MODE=standalone` selects in-place transitions + auth/listen flags |
| [DevContainer Feature](./devcontainer-feature.md) | Shares the in-place transition model, `setup.json`, compile-to-directory, and release assets |
| [Compilation](../artifact/compilation.md) | `compile-workshop --output-dir` logic embedded in `--serve`; no OCI images involved |
| [Instrumentation](./instrumentation.md) | Same bashrc, scoped to workshop terminal sessions only; honors `WORKSHOP_ROOT` |
| [Base Images](./base-images.md) | Not used — binaries installed directly on the host |
| [CLI](./cli.md) | Not used — no container lifecycle to manage |
| [Backend Capabilities](./backend-capabilities.md) | Standalone column: no infrastructure provisioning, no aggregation, no instructor dashboard |
