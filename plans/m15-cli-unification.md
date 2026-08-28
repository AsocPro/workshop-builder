# M15 — Unify the tools into a single `workshop` command

> **Status: PLANNED — not scheduled.** Captured as a note, not next-up work.

## Goal

Collapse the separately-named binaries into **one `workshop` command** with
subcommands, instead of shipping and documenting several tools.

Today (for reference):

| Binary | Source | Role |
|---|---|---|
| `workshop-backend` | `backend/` | Serves the student UI + terminal + goss. Already multi-mode: default container entrypoint, `--serve` (standalone), `service install|uninstall`. |
| `workshop` | `cli/` | Host-side podman orchestrator + management panel + image-swap step transitions (Docker/podman mode). |
| `compile-workshop` / `workshop-setup` | `cmd/` | Thin build-from-source wrappers over `pkg/workshop`. |

Note: the backend binary has *already* partially unified (serve / `--serve` / `service`).
This milestone finishes the job across all the tools.

## Target shape

A single `workshop` binary dispatching by subcommand, e.g.:

- `workshop serve <dir>` — standalone (today's `workshop-backend --serve`)
- `workshop run <image>` — podman runner + image-swap step transitions (today's `workshop` CLI)
- `workshop compile <dir>` — compile to a directory (today's `compile-workshop`)
- `workshop setup` — pre-apply prior steps (today's `workshop-setup`)
- `workshop service install|uninstall` — systemd unit

The in-container entrypoint stays the backend serve path (invoked implicitly, no
subcommand needed) so image ENTRYPOINTs don't change.

## Organizing principle (source layout)

**Keep each capability in its own package so pieces stay easy to find — and easy to
pare down later if usage ever calls for it.** The unification is a thin front-door
(`cmd/workshop/` cobra dispatch); the actual logic stays partitioned:

- `pkg/workshop/` — the one and only compiler (already the single source of truth)
- `backend/` — runtime: serve UI/terminal/goss, standalone, service
- `cli/` — podman orchestration + image-swap + management panel
- `cmd/` — thin subcommand wiring only, no business logic

The umbrella is a dispatcher over cleanly-separated packages, not a merge that
entangles them. Adding or removing a delivery mode should stay a package-level
change plus one line in the dispatcher.

## Acceptance sketch

- `workshop <subcommand>` dispatches to the right mode; old binary names removed or aliased.
- Release pipeline ships one binary (plus vendored ttyd/goss) instead of several.
- Each delivery mode remains an isolated package behind the dispatcher.
