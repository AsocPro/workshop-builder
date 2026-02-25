# Instrumentation — Command Logging & Terminal Recording

## Purpose

Capture structured data about what happens in the student's terminal session. Two independent mechanisms work together:

1. **Command logging** — a `PROMPT_COMMAND` hook writes every command, timestamp, and exit code as NDJSON
2. **Asciinema recording** — full terminal capture with input/output for replay and seeking

Both are pre-installed in [base images](./base-images.md) and require no author configuration.

## Command Logging

### Shell Hook: `/etc/workshop-platform.bashrc`

```bash
__workshop_log_cmd() {
    local exit_code=$?
    local ts=$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)
    local cmd=$(HISTTIMEFORMAT= history 1 | sed 's/^ *[0-9]* *//')
    [ -z "$cmd" ] && return
    printf '{"ts":"%s","cmd":"%s","exit":%d}\n' \
        "$ts" \
        "$(printf '%s' "$cmd" | head -c 1024 | sed 's/\\/\\\\/g; s/"/\\"/g')" \
        "$exit_code" \
        >> /workshop/runtime/command-log.jsonl
}
PROMPT_COMMAND="__workshop_log_cmd${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
```

### How It Works

1. `PROMPT_COMMAND` fires after every command, before the next prompt is drawn
2. Captures the exit code of the last command (`$?`)
3. Reads the last command from bash history
4. JSON-escapes the command text (backslashes, quotes)
5. Truncates to 1024 characters to prevent log bloat from accidental large outputs
6. Appends one NDJSON line to `/workshop/runtime/command-log.jsonl`

### Output Format

```jsonl
{"ts":"2025-03-15T14:22:01.123Z","cmd":"kubectl get pods","exit":0}
{"ts":"2025-03-15T14:22:15.456Z","cmd":"kubectl apply -f deployment.yaml","exit":1}
{"ts":"2025-03-15T14:22:30.789Z","cmd":"cat /etc/resolv.conf","exit":0}
```

| Field | Type | Description |
|---|---|---|
| `ts` | string | ISO 8601 UTC timestamp with millisecond precision |
| `cmd` | string | Command text from bash history (max 1024 chars) |
| `exit` | number | Exit code of the command |

### Design Decisions

- **`PROMPT_COMMAND` over `trap DEBUG`**: `PROMPT_COMMAND` fires once per prompt, not once per simple command in a pipeline. It also has access to `$?` for the overall command exit code.
- **History-based capture**: Reading from `history` captures the command as the user typed it, including pipes and redirects. It does not capture the expanded form.
- **Append-only**: The file is only ever appended to, never truncated. This ensures no data loss and allows safe concurrent reading by the backend's file watcher.
- **No multiline**: Multiline commands (heredocs, continuation lines) are captured as a single history entry by bash. The NDJSON format handles embedded newlines via JSON escaping.

### Sourcing

The bashrc is sourced automatically for interactive bash sessions via `/etc/bash.bashrc` (added during base image build):

```bash
[ -f /etc/workshop-platform.bashrc ] && . /etc/workshop-platform.bashrc
```

Non-interactive shells (scripts, cron jobs) do not source it — only terminal sessions are instrumented.

## Asciinema Recording

### How It Works

The [backend service](./backend-service.md) spawns ttyd wrapping the shell in asciinema:

```
ttyd <options> -- asciinema rec --stdin --append /workshop/runtime/session.cast -c /bin/bash
```

| Flag | Purpose |
|---|---|
| `--stdin` | Capture keyboard input for full replay fidelity (shows what the student typed) |
| `--append` | Continue recording to the same file on reconnection (ttyd restart) |
| `-c /bin/bash` | The shell command to record |

### Output Format

The recording uses [asciicast v2 format](https://docs.asciinema.org/manual/asciicast/v2/):

```
{"version": 2, "width": 120, "height": 40, "timestamp": 1710511200, "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"}}
[0.5, "o", "$ "]
[1.2, "i", "kubectl get pods\r"]
[1.3, "o", "kubectl get pods\r\n"]
[2.1, "o", "NAME                    READY   STATUS    RESTARTS   AGE\r\n"]
[2.1, "o", "nginx-6b7f6cb5c7-abc   1/1     Running   0          5m\r\n"]
```

- First line: header with terminal dimensions and metadata
- Subsequent lines: `[elapsed_seconds, event_type, data]`
- `"o"` = output (terminal → screen), `"i"` = input (keyboard → terminal)

### File Serving

The backend serves `session.cast` at `GET /api/instructor/recording` with HTTP Range support. This enables:

- The [asciinema-player](https://docs.asciinema.org/manual/player/) web component to seek to any point in the recording
- Efficient streaming — the player doesn't need to download the entire file
- Live tailing — the player can follow the recording as it grows

### Reconnection Handling

When a student's browser disconnects and reconnects:

1. ttyd process may restart (depending on configuration)
2. `--append` flag ensures asciinema continues writing to the same `session.cast` file
3. The recording has a seamless timeline — the gap appears as idle time in playback
4. A `disconnected`/`connected` event pair in `state-events.jsonl` marks the boundary

### TUI Application Support

Asciinema records raw terminal escape sequences, so TUI applications (vim, htop, less, etc.) are captured and replayed with full fidelity. The asciinema-player handles ANSI escape codes, cursor positioning, and alternate screen buffer switching.

## Backend File Watcher

The backend watches `command-log.jsonl` using fsnotify (or periodic tail-read as fallback):

1. On file change notification, read new lines from the last known offset
2. Parse each new NDJSON line
3. Push to in-memory command buffer (ring buffer, configurable size, default 1000 entries)
4. Push to SSE subscribers via the instructor event bus

The watcher is read-only — it never modifies `command-log.jsonl`. The shell hook and the backend are the only two processes that touch this file (write and read, respectively).

## Data Volume

Typical session volumes for a 2-hour workshop:

| File | Typical Size | Growth Rate |
|---|---|---|
| `command-log.jsonl` | 50–200 KB | ~100 bytes per command |
| `session.cast` | 5–50 MB | Depends on output volume (TUI apps increase it) |
| `state-events.jsonl` | 1–5 KB | ~100 bytes per event |

The `session.cast` file is the largest by far. In K8s mode, the [Vector sidecar](./aggregation.md) ships it to S3/MinIO object storage.

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Base Images](./base-images.md) | bashrc and asciinema pre-installed |
| [Backend Service](./backend-service.md) | Watches command log, manages asciinema subprocess, serves recording |
| [Flat File Artifact](../artifact/flat-file-artifact.md) | Command log and recording live in `/workshop/runtime/` |
| [Aggregation](./aggregation.md) | Vector ships JSONL + cast files to Postgres/S3 in K8s mode |
| [Instructor Dashboard](./instructor-dashboard.md) | Displays command timeline, plays back recordings |
