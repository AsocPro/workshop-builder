# Help System — Static Content and LLM Assistance

## Purpose

A help system that provides students with hints, explanations, and solutions for each step. Works at two levels: **static help content** (authored markdown, no infrastructure required) and **LLM-generated help** (contextual, personalized, requires API key). Both are optional and complement each other.

## Help Modes

| Mode | Behavior | Use Case |
|---|---|---|
| `hints` | Nudges and leading questions. Never gives the answer directly. | Default. The learning is in the discovery. |
| `explain` | Explains concepts and shows related examples, but not the exact solution. | When understanding "why" matters more than "how". |
| `solve` | Provides direct solutions with explanation. | When the learning is in understanding the solution, not finding it. |

## Static Help Content

Authors provide per-step help as convention-named markdown files in the step directory:

```
steps/step-pods/
  hints.md       # static hints content
  explain.md     # static explanation content
  solve.md       # static solution content
```

These files are baked into the image at `/workshop/steps/<id>/hints.md`, `/workshop/steps/<id>/explain.md`, `/workshop/steps/<id>/solve.md`. When a student clicks a help mode button, the backend serves the corresponding file directly — no API call, no LLM infrastructure needed.

Static help works everywhere: Docker local mode without an API key, air-gapped environments, workshops where LLM costs aren't justified. Every workshop can have a help system.

## LLM Help (Optional Layer)

When an operator configures an LLM provider, the help system becomes dynamic and contextual.

### Operator-Level Configuration (Required for LLM to be active)

LLM provider configuration is an operator concern, set in the [WorkspaceTemplate CRD](./crds.md):

```yaml
spec:
  defaults:
    llm:
      provider: anthropic
      model: claude-sonnet-4-20250514
      apiKeyEnv: WORKSHOP_LLM_API_KEY
      maxTokens: 1024
      defaultMode: hints
```

If no `llm` config is present in the WorkspaceTemplate, LLM features are disabled. Static help content still works.

### Per-Step LLM Context (Author-Configured)

Authors configure per-step LLM context in `step.yaml`:

```yaml
llm:
  context: |
    Common mistake: students forget the -n namespace flag.
    The correct namespace for this exercise is "workshop".
```

Reference docs are placed in the step's `llm-docs/` directory — no configuration needed. If `llm-docs/` exists and contains files, they are included in LLM context assembly.

### Workshop-Level Prompt Overrides

Authors can override the default system prompts for each help mode by placing markdown files in the `prompts/` directory at the workshop root:

```
my-workshop/
  prompts/
    hints.md       # overrides the default hints system prompt
    explain.md     # overrides the default explain system prompt
    solve.md       # overrides the default solve system prompt
```

These are baked into the image at `/workshop/prompts/`. The backend ships sensible defaults; these files replace them entirely when present. Use this for workshops that need a different pedagogical style (e.g., a security workshop that wants a more Socratic approach).

## Help Behavior Matrix

The backend resolves help requests based on what's available:

| Static File | LLM Configured | Behavior |
|---|---|---|
| No | No | Help button disabled for this mode |
| Yes | No | Static content rendered directly — no API call |
| No | Yes | LLM generates help using step context |
| Yes | Yes | Static content included as reference in LLM context; LLM builds on the author's content |

The help button for each mode is shown if either a static file exists or LLM is configured. The UI indicates whether the response is static or LLM-generated.

## Context Assembly

When the student clicks Help and LLM is configured, the backend assembles a prompt context from multiple sources:

```
┌─────────────────────────────────────────────────┐
│ System prompt                                    │
│   - Default or overridden (from /workshop/prompts/) │
│   - Role: workshop teaching assistant            │
│   - Mode: hints / explain / solve                │
│   - Rules: don't give answers in hints mode, etc │
├─────────────────────────────────────────────────┤
│ Step context                                     │
│   - Step title and tutorial markdown (content.md)│
│   - Instructor-provided context (llm.json)       │
│   - Static help content (hints.md/explain.md/solve.md) │
│   - Reference docs (llm-docs/*)                  │
├─────────────────────────────────────────────────┤
│ Student state                                    │
│   - Recent commands (last 20 from command-log)   │
│   - Latest goss validation results               │
│   - Which checks are passing/failing             │
├─────────────────────────────────────────────────┤
│ Student question (free-text from help panel)      │
└─────────────────────────────────────────────────┘
```

### Context Sources

| Source | File | What's Included |
|---|---|---|
| System prompt override | `/workshop/prompts/<mode>.md` | Custom system prompt for this mode (if present) |
| Tutorial content | `/workshop/steps/<id>/content.md` | Full step markdown |
| Instructor context | `/workshop/steps/<id>/llm.json` → `context` | Author-written hints about common mistakes |
| Static help content | `/workshop/steps/<id>/hints.md` (or `explain.md`, `solve.md`) | Author-curated help for the active mode — included as reference material |
| Reference docs | `/workshop/steps/<id>/llm-docs/*` | Cheat sheets, guides, API references |
| Recent commands | `/workshop/runtime/command-log.jsonl` | Last 20 commands with exit codes |
| Goss results | Most recent goss validation result (in-memory) | Pass/fail per check |

### Token Budget

Context is assembled with a token budget in mind:

- Tutorial markdown: included in full (typically 1–20 KB)
- Instructor context: included in full (typically < 1 KB)
- Reference docs: included in full, up to a configurable limit (default 50 KB total)
- Recent commands: last 20 commands (capped)
- Goss results: last result only

If the total context exceeds the model's context window, reference docs are truncated first (oldest/largest removed), then command history is reduced.

## API

### Request Help

```
POST /api/llm/help
Content-Type: application/json

{
  "step": "step-pods",
  "question": "I ran kubectl apply but it says the file was not found"
}
```

Response is streamed using Server-Sent Events:

```
event: token
data: {"text": "It looks like "}

event: token
data: {"text": "you might be "}

event: token
data: {"text": "in the wrong directory. "}

event: done
data: {"totalTokens": 156}
```

### Get History

```
GET /api/llm/history?step=step-pods
```

Returns previous LLM interactions for the current step:

```json
{
  "interactions": [
    {
      "question": "I ran kubectl apply but it says the file was not found",
      "response": "It looks like you might be in the wrong directory...",
      "timestamp": "2025-03-15T14:30:00.000Z"
    }
  ]
}
```

## LLM Client

The backend includes an LLM client that calls the Anthropic Messages API:

- Streaming responses via SSE
- Configurable model and max tokens from operator config
- API key read from environment variable (specified by `apiKeyEnv` in WorkspaceTemplate)
- Retry with exponential backoff on transient errors
- Timeout after 30 seconds (configurable)

### Provider Support

v1 supports Anthropic only. The provider abstraction is minimal — just enough to swap implementations later if needed. No over-engineered plugin system.

## Rate Limiting

Default: 5 requests per minute per workspace. Configurable via environment variable:

```bash
docker run -e WORKSHOP_LLM_RATE_LIMIT=10 ...
```

When rate-limited, the API returns `429 Too Many Requests` with a `Retry-After` header.

## History Storage

LLM interactions are appended to `/workshop/runtime/llm-history.jsonl`:

```jsonl
{"ts":"2025-03-15T14:30:00.000Z","step":"step-pods","question":"I ran kubectl apply but it says the file was not found","response":"It looks like you might be in the wrong directory...","model":"claude-sonnet-4-20250514","tokens":156}
```

In K8s mode, the [Vector sidecar](./aggregation.md) ships this file to Postgres (`llm_interactions` table).

## Security

- **API key is never baked into the image.** It is injected via environment variable at runtime.
- **No key in workshop.yaml.** The `apiKeyEnv` field specifies the *name* of the env var, not the key itself.
- **Student cannot see the key.** The env var is read by the backend process; it is not exported to the student's shell.
- **No key in JSONL logs.** The LLM history records the question, response, and model — never the API key.

## Help Panel UI

The help panel is a chat-like interface in the tutorial sidebar:

- Help button visible on every step when LLM is configured
- Free-text input for student questions
- Streaming response display (tokens appear as they arrive)
- History of previous interactions for the current step

## API Key Distribution

TODO: Define who provides the LLM API key in Docker local mode. The student runs `docker run -e WORKSHOP_LLM_API_KEY=sk-...`, but students typically don't have API keys. Options: (a) instructor provides a pre-configured `docker run` command, (b) the CLI wraps the run command and injects the key from its own config, (c) LLM help is instructor-only in Docker mode. This affects both UX and security.

## When LLM Is Not Configured

If no `llm` config is present in the WorkspaceTemplate:

- LLM-powered help is disabled — no API calls are made
- The `/api/llm/*` endpoints return `404 Not Found`
- No LLM-related files appear in `/workshop/runtime/`
- **Static help content still works** — if `hints.md`, `explain.md`, or `solve.md` exist for a step, those help buttons are shown and the content is served directly
- Steps with no static help files and no LLM have no help buttons

LLM is purely additive. Static help content ensures every workshop can offer assistance regardless of infrastructure.

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Workshop Spec](../definition/workshop.md) | Per-step `llm` config in step.yaml, static help files, workshop-level prompt overrides |
| [WorkspaceTemplate CRD](./crds.md) | LLM provider config (provider, model, apiKeyEnv) |
| [Backend Service](./backend-service.md) | Serves static help content and handles LLM API calls |
| [Instrumentation](./instrumentation.md) | Command log provides context for LLM prompts |
| [Flat File Artifact](../artifact/flat-file-artifact.md) | `llm.json`, `llm-docs/`, `hints.md`, `explain.md`, `solve.md`, and `prompts/` baked into image |
| [Aggregation](./aggregation.md) | Vector ships LLM history to Postgres in K8s mode |
