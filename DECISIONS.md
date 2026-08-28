# Decisions

A running log of **decisions and the reasoning behind them** — especially the "why,"
which is the part that evaporates between sessions. When you come back after time away
and wonder "why is it set up this way?", read here before re-deriving it from scratch.

Newest first. Each entry: what was decided, why, and what would make us revisit it.

---

## 2026-08 — Keep the full multi-tenant/cluster scope; do not prune to single-user

**Decision:** Keep the cluster/Kubernetes/multi-tenant half of the design (operator,
CRDs, aggregation, instructor dashboard) as core scope, even though none of it is built
yet. Do not prune the project down to the single-user take-home core.

**Why:** The multi-tenant system is the *live workshop* delivery context (running a
workshop for N students with isolation and instructor visibility); the single-user
modes are the *take-home* context after the live session. They're two points in one
workshop's lifecycle, not scope creep plus a core. Both are intended.

**Considered and rejected (for now):** Pruning to a single "repo is the take-home
artifact" model — retiring the per-step OCI image build + `workshop run` CLI +
image-swap, keeping only standalone + devcontainer. Rejected because that only serves
the take-home half and drops the live/multi-tenant half, which is core.

**Revisit when:** Real usage/testing shows a piece is in the way or not worth its
maintenance. Pruning should be driven by empirical friction, not speculation.

---

## 2026-08 — Take-home artifact would be a git repo (leaning, not adopted)

**Decision:** *If/when* we optimize a take-home path, the artifact is a **git repo**
(workshop.yaml + steps + README), not a prebuilt OCI image reference.

**Why:** A repo is a well-known primitive, is what people already do with example code,
carries its own README with run instructions, and removes the registry/push/host burden
from the author. The runtimes that fit it (standalone `--serve`, devcontainer feature)
compile from source on arrival — both already built.

**Status:** A leaning, superseded in priority by the "keep full scope" decision above.
Not acted on. The OCI-image + CLI path stays for now.

---

## 2026-08 — Docs are the goal; STATUS.md is the reality

**Decision:** The `docs/` tree is the **design of record** (the goal). It intentionally
describes more than is built. Build state is tracked separately in
[STATUS.md](STATUS.md) (✅/🟡/⬜). Do **not** put implementation-status notes into the
design docs.

**Why:** Mixing "what it should do" with "what it does" is exactly what caused the
disorientation this convention fixes — `docs/plan.md` marking things "Complete" (meaning
doc-written) read as "built." One reality marker (STATUS.md), one goal (docs/), cleanly
separated. When code and spec diverge, that's a signal to reconcile deliberately.

**Revisit when:** N/A — structural convention. See also `CLAUDE.md`.

---

## 2026-08 — Unify the tools into one `workshop` command (planned)

**Decision:** Fold the separately-named binaries (`workshop-backend`, `workshop` CLI,
`compile-workshop`/`workshop-setup`) into a single `workshop` command with subcommands.
Planned, not scheduled.

**Why:** One tool is simpler to ship, document, and reason about than several. The
backend binary already partially unified (serve / `--serve` / `service`); this finishes
the job. Source stays partitioned by package behind a thin dispatcher so pieces remain
easy to find and to pare down later.

**Details:** [plans/m15-cli-unification.md](plans/m15-cli-unification.md).

---

## (earlier — reconstructed from git history)

- **Removed instructor mode from single-user Docker** (commit `9628084`). The instructor
  dashboard is a cluster-mode concept; it doesn't belong in the local single-user path.
- **Stateless runtime / flat files, no SQLite** (commit `b639965` and around). Runtime
  state is in-memory and ephemeral; workshop metadata is baked as flat files under
  `/workshop/`. Determinism comes from immutable per-step images, not saved state.
- **CLI required for local mode; no bare `docker run`** (design). The CLI adds step
  transitions (image-swap) and infrastructure that a bare run can't do — though a
  self-contained image *can* be run directly for a single step.
