# LLM Help — Contextual Student Assistance

## Purpose

A help button in the tutorial panel that reads the student's command history, goss validation results, step content, and instructor-provided context to give contextual hints. The LLM never gives away the answer in `hints` mode — it nudges students toward the solution.

## Configuration

LLM help is configured in `workshop.yaml` at two levels:

### Workshop-Level (Required for LLM to be active)

```yaml
workshop:
  llm:
    provider: anthropic
    model: claude-sonnet-4-20250514
    apiKeyEnv: WORKSHOP_LLM_API_KEY
    maxTokens: 1024
    defaultMode: hints
```

### Per-Step (Optional Overrides)

```yaml
steps:
  - id: step-pods
    llm:
      mode: hints
      context: |
        Common mistake: students forget the -n namespace flag.
        The correct namespace for this exercise is "workshop".
      docs:
        - ./docs/kubectl-cheatsheet.md
```

If no workshop-level `llm` config is present, the help button is not shown.

## Help Modes

| Mode | Behavior | Use Case |
|---|---|---|
| `hints` | Nudges and leading questions. Never gives the answer directly. | Default. The learning is in the discovery. |
| `explain` | Explains concepts and shows related examples, but not the exact solution. | When understanding "why" matters more than "how". |
| `solve` | Provides direct solutions with explanation. | When the learning is in understanding the solution, not finding it. |

The mode is set per-step (via `llm.mode`) with a workshop-level default (via `llm.defaultMode`). If neither is set, `hints` is used.

## Context Assembly

When the student clicks Help, the backend assembles a prompt context from multiple sources:

```
┌─────────────────────────────────────────────────┐
│ System prompt                                    │
│   - Role: workshop teaching assistant            │
│   - Mode: hints / explain / solve                │
│   - Rules: don't give answers in hints mode, etc │
├─────────────────────────────────────────────────┤
│ Step context                                     │
│   - Step title and tutorial markdown (content.md)│
│   - Instructor-provided context (llm.json)       │
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
| Tutorial content | `/workshop/steps/<id>/content.md` | Full step markdown |
| Instructor context | `/workshop/steps/<id>/llm.json` → `context` | Author-written hints about common mistakes |
| Reference docs | `/workshop/steps/<id>/llm-docs/*` | Cheat sheets, guides, API references |
| Recent commands | `/workshop/runtime/command-log.jsonl` | Last 20 commands with exit codes |
| Goss results | Last `goss_result` event from `state-events.jsonl` | Pass/fail per check |

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
- Configurable model and max tokens from `workshop.json`
- API key read from environment variable (specified by `apiKeyEnv` in config)
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
- Mode indicator showing current help mode (hints/explain/solve)

## API Key Distribution

TODO: Define who provides the LLM API key in Docker local mode. The student runs `docker run -e WORKSHOP_LLM_API_KEY=sk-...`, but students typically don't have API keys. Options: (a) instructor provides a pre-configured `docker run` command, (b) the CLI wraps the run command and injects the key from its own config, (c) LLM help is instructor-only in Docker mode. This affects both UX and security.

## When LLM Is Not Configured

If `workshop.llm` is not present in `workshop.yaml`:

- No `llm.json` files are generated during compilation
- The help button is not rendered in the UI
- The `/api/llm/*` endpoints return `404 Not Found`
- No LLM-related files appear in `/workshop/runtime/`

The workshop functions exactly as it would without LLM support. LLM is purely additive.

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Workshop Spec](../definition/workshop.md) | `llm` config in workshop.yaml drives compilation |
| [Backend Service](./backend-service.md) | Handles LLM API calls and serves help endpoints |
| [Instrumentation](./instrumentation.md) | Command log provides context for LLM prompts |
| [Flat File Artifact](../artifact/flat-file-artifact.md) | `llm.json` and `llm-docs/` baked into image |
| [Aggregation](./aggregation.md) | Vector ships LLM history to Postgres in K8s mode |
