# M14 — Standalone Mode

> **Status note: IMPLEMENTED.** With M13's pipeline now in place, `BuildRelease` includes `install-standalone.sh` and all binaries for both architectures — the installer one-liner works as soon as the first `v*` tag is pushed and the release workflow runs.

## Goal

`workshop-backend --serve <dir>` runs the full workshop experience directly on a Linux server — no container, no CLI orchestration. An operator clones a workshop repo, runs one command, and gets the workshop UI with step navigation, a web terminal on the real host, and goss validation against the live environment. `workshop-backend service install` makes it a persistent systemd service with one more command.

See [Standalone Mode](../docs/platform/standalone-mode.md) for the architecture and scope decisions. The hard constraints: single user per server, server state authoritative, private networks only, Linux only, no reset.

## Prerequisites

- M13 complete (in-place step transitions, `setup.json`, `backend/setup/apply.go`, `WORKSHOP_MODE` detection, `BuildRelease` + GitHub release assets)

## Working Directory

`/home/zach/code/workshop-builder`

## Constraint

Standalone mode must not require root. The default flow (`--serve` + SSH port-forward) runs entirely as the invoking user: compiled output, runtime dir, and instrumentation rcfile all live under `WORKSHOP_ROOT`, never `/etc`. Root (or `--user` units) only enters for `service install`.

## Acceptance Test

```bash
# 1. Build the backend
dagger call build-backend --src . -o ./workshop-backend
chmod +x ./workshop-backend

# 2. Serve straight from a checkout (compile happens implicitly)
./workshop-backend --serve ./examples/hello-linux
# Verify stdout:
#   - compiled to ~/.workshop/hello-linux (or $WORKSHOP_ROOT)
#   - "listening on 127.0.0.1:8080"
#   - mode: standalone

# 3. Open http://localhost:8080
# Verify: step list renders, markdown displays, terminal is a shell on THIS host
# Run a command in the terminal → verify it lands in $WORKSHOP_ROOT/runtime/command-log.jsonl
# Open a regular shell (NOT via the UI) → run a command → verify it is NOT logged

# 4. Validate a step via goss → verify pass/fail reflects real host state

# 5. Step activation (in-place, shared with devcontainer mode)
# Click "Next Step" on a step with setup.json → verify files copied + commands run on the host

# 6. Auth + listen flags
./workshop-backend --serve ./examples/hello-linux --listen 0.0.0.0:8080 \
  --auth-user admin --auth-password-file /tmp/pass
# Verify: unauthenticated request → 401; with credentials → 200
# Verify: terminal WebSocket also requires auth
./workshop-backend --serve ./examples/hello-linux --listen 0.0.0.0:8080
# Verify: prominent warning logged (non-loopback without auth), but serves

# 7. Infrastructure guard
# Serve a workshop with infrastructure.cluster.enabled=true
# Verify: startup fails with a clear "not supported in standalone mode" error

# 8. Systemd self-install
./workshop-backend service install --serve /opt/hello-linux \
  --listen 0.0.0.0:8080 --auth-user admin --auth-password-file /etc/workshop/pass --print
# Verify: valid unit printed to stdout, ExecStart uses the binary's absolute path
sudo ./workshop-backend service install --serve /opt/hello-linux \
  --listen 0.0.0.0:8080 --auth-user admin --auth-password-file /etc/workshop/pass
# Verify: /etc/systemd/system/workshop.service written; follow-up commands printed
# Verify: `service install` with non-loopback --listen and NO auth flags → refuses
./workshop-backend service uninstall
# Verify: unit removed

# 9. Installer script
curl -fsSL https://github.com/asocpro/workshop-builder/releases/latest/download/install-standalone.sh | sh
# Verify: workshop-backend, workshop-setup, ttyd, goss in /usr/local/bin
# Verify: /etc/bash.bashrc NOT modified (unlike the devcontainer install.sh)
```

---

## Directory Structure

```
backend/
  main.go                    ← MODIFY: flag parsing, --serve, mode=standalone, service subcommand dispatch
  server.go                  ← MODIFY: basic auth middleware wrapping all routes
  auth.go                    ← NEW: basic auth middleware
  servecmd/
    serve.go                 ← NEW: --serve orchestration (compile, rcfile, root resolution)
    systemd.go               ← NEW: service install/uninstall, unit generation
  instrumentation/
    bashrc.go                ← NEW: go:embed of the canonical bashrc + rcfile generation
    workshop-platform.bashrc ← MOVED: canonical bashrc (from base-images/bashrc), now WORKSHOP_ROOT-aware
  process/
    ttyd.go                  ← MODIFY: rcfile + env injection for standalone mode

pkg/workshop/
  outputdir.go               ← MODIFY/EXTRACT: compile-to-directory writer as a library function

cmd/compile-workshop/
  main.go                    ← MODIFY: --output-dir delegates to pkg/workshop writer

base-images/
  bashrc                     ← REMOVED (moved to backend/instrumentation/)

dagger/
  main.go                    ← MODIFY: bashrc path references → backend/instrumentation/workshop-platform.bashrc;
                                add install-standalone.sh to BuildRelease

scripts/
  install-standalone.sh      ← NEW: curl|sh installer (release asset)

docs/
  platform/backend-capabilities.md  ← MODIFY: standalone column
```

---

## Step 1: Compile-to-Directory as a Library Function

**Files: `pkg/workshop/outputdir.go`, `cmd/compile-workshop/main.go`**

M13 implements the `--output-dir` writer (workshop.json, per-step meta/content/goss/setup.json/stage/). M14 needs the same logic callable from the backend's `--serve` path. Extract it into the library:

```go
// pkg/workshop/outputdir.go

// CompileToDir parses, validates, and compiles the workshop at srcDir,
// writing the full flat-file structure to outDir.
func CompileToDir(srcDir, outDir string) (*Compiled, error)
```

`cmd/compile-workshop` becomes a thin flag-parsing wrapper around this. If M13 has not yet been implemented when M14 starts, implement the writer here directly — amend the M13 plan's Step 1 to call `workshop.CompileToDir` instead of inlining the writer in `cmd/`.

**Single source of truth:** the directory layout (`setup.json`, `stage/`, etc.) is produced by exactly one function, consumed by the compile-workshop CLI, the devcontainer `postCreateCommand`, and standalone `--serve`.

---

## Step 2: Bashrc — Relocate, Parameterize, Embed

### 2a. Relocate the Canonical File

`go:embed` cannot reach outside the package directory, so the single source of truth moves:

```
base-images/bashrc  →  backend/instrumentation/workshop-platform.bashrc
```

Update every reference:
- `dagger/main.go` base image builds: `WithFile("/etc/workshop-platform.bashrc", src.File("backend/instrumentation/workshop-platform.bashrc"))`
- `BuildRelease`: same path
- M13's `install.sh` is unaffected (downloads from the release)
- `docs/platform/devcontainer-feature.md` and `docs/platform/instrumentation.md` references to `base-images/bashrc`

There is still exactly one copy in the repo. Container modes keep sourcing it globally via `/etc/bash.bashrc` — that behavior is unchanged.

### 2b. Make the Bashrc `WORKSHOP_ROOT`-Aware

The hook currently hardcodes `/workshop/runtime`. Parameterize:

```bash
__workshop_log_command() {
    local exit_code=$?
    local runtime_dir="${WORKSHOP_ROOT:-/workshop}/runtime"
    local cmd
    cmd=$(history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
    if [ -n "$cmd" ] && [ -d "$runtime_dir" ]; then
        # ... unchanged, but >> "$runtime_dir/command-log.jsonl"
    fi
    return $exit_code
}
```

Backward compatible: in containers `WORKSHOP_ROOT` is unset, the default is `/workshop`, behavior identical.

### 2c. Embed in the Backend

**New file: `backend/instrumentation/bashrc.go`**

```go
package instrumentation

import _ "embed"

//go:embed workshop-platform.bashrc
var Bashrc string

// WriteRcfile writes a bash rcfile to <workshopRoot>/workshop-rcfile that
// chains the user's own ~/.bashrc, then the instrumentation hook.
// Used by ttyd in standalone mode via `bash --rcfile`.
func WriteRcfile(workshopRoot string) (string, error) {
    content := "[ -f ~/.bashrc ] && . ~/.bashrc\n" + Bashrc
    path := filepath.Join(workshopRoot, "workshop-rcfile")
    return path, os.WriteFile(path, []byte(content), 0644)
}
```

The chaining matters: `--rcfile` *replaces* `~/.bashrc`, and on a real server the operator's aliases, kubeconfig exports, and prompt must survive. The generated rcfile sources their bashrc first, then adds the `PROMPT_COMMAND` hook.

No `/etc` writes, no root, nothing global. Only ttyd-spawned shells are instrumented — SSH sessions and cron on the server are untouched.

---

## Step 3: TTYDManager — Standalone Shell Invocation

**File: `backend/process/ttyd.go`**

Parameterize the shell invocation and environment:

```go
type Options struct {
    Port    int
    Shell   []string          // default: {"/bin/bash", "--login"}
    Env     map[string]string // appended to os.Environ()
}
```

- **Container modes (unchanged):** `/bin/bash --login` — the global `/etc/bash.bashrc` sourcing handles instrumentation.
- **Standalone mode:** `/bin/bash --rcfile <workshopRoot>/workshop-rcfile -i`, with `WORKSHOP_ROOT=<root>` in the child env so the hook logs to the right place.

ttyd continues to bind `127.0.0.1` with the backend proxying — this is what makes basic auth cover the terminal for free (Step 5).

---

## Step 4: `--serve` — Compile and Run from a Checkout

**Files: `backend/main.go`, `backend/servecmd/serve.go`**

The backend grows flag parsing (env vars remain as fallbacks for container modes, which pass no flags):

```go
serveDir   := flag.String("serve", "", "compile and serve a workshop source directory (standalone mode)")
listen     := flag.String("listen", "127.0.0.1:8080", "listen address")
authUser   := flag.String("auth-user", "", "basic auth username")
authPass   := flag.String("auth-password-file", "", "file containing basic auth password")
```

When `--serve` is set:

1. **Resolve `WORKSHOP_ROOT`** — `$WORKSHOP_ROOT` if set, else `~/.workshop/<workshop-name>/` (name from `workshop.yaml`). Never `/workshop` — that default stays container-only.
2. **Compile** — `workshop.CompileToDir(serveDir, workshopRoot)` (Step 1). Recompiles on every start: `git pull && restart` is the content iteration loop.
3. **Infrastructure guard** — if the compiled `workshop.json` declares `infrastructure.cluster.enabled` or non-empty `infrastructure.extraContainers`, exit with:
   `standalone mode does not provision infrastructure: this workshop requires <X>; provision it on this server manually or use 'workshop run'`
   (follows the [backend capabilities](../docs/platform/backend-capabilities.md) pattern of explicit per-backend errors).
4. **Write the rcfile** — `instrumentation.WriteRcfile(workshopRoot)` (Step 2c).
5. **Set mode** — `WORKSHOP_MODE=standalone`. In-place activation (`/api/steps/{id}/activate`) is enabled exactly as in devcontainer mode:

```go
inPlace := workshopMode == "devcontainer" || workshopMode == "standalone"
```

No new transition code — standalone reuses M13's activate handler and `backend/setup` package wholesale. Steps with no `setup.json` content (the common runbook case) just update the active step.

6. **Listen address rules:**
   - default `127.0.0.1:8080` — SSH port-forward friendly, zero config
   - non-loopback without auth → serve, but log a prominent warning (ad-hoc use on a trusted network is in scope; the operator decides)

---

## Step 5: Basic Auth Middleware

**New file: `backend/auth.go`** — **File: `backend/server.go`**

```go
// BasicAuth wraps a handler with HTTP Basic authentication.
// Constant-time comparison on both fields.
func BasicAuth(user, pass string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        u, p, ok := r.BasicAuth()
        if !ok ||
            subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
            subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
            w.Header().Set("WWW-Authenticate", `Basic realm="workshop"`)
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Wired at the very top in `NewServer` — wrapping the entire mux, not per-route:

```go
var h http.Handler = mux
if authUser != "" {
    h = BasicAuth(authUser, authPass, h)
}
```

- The terminal WebSocket upgrade at `/ws/terminal` is an HTTP request through this same handler chain — gated with zero extra mechanism. Browsers re-send cached Basic credentials on the upgrade request automatically.
- Password from `--auth-password-file` (trailing newline trimmed) or `WORKSHOP_AUTH_PASSWORD` env. No `--auth-password` flag — passwords in argv leak via `ps` and shell history.
- Auth flags without `--serve` are an error: auth is a standalone-mode feature; container modes have their own story (none in Docker, OAuth2 Proxy in cluster).

---

## Step 6: `service install` / `service uninstall`

**New file: `backend/servecmd/systemd.go`**

Subcommand dispatch in `main.go` before flag parsing: `workshop-backend service install [flags]`, `workshop-backend service uninstall`.

`service install` accepts the same flags as `--serve` plus `--user` and `--print`:

```go
func InstallService(opts ServeOptions, userUnit, printOnly bool) error {
    exe, _ := os.Executable()           // absolute path for ExecStart
    exe, _ = filepath.EvalSymlinks(exe)

    unit := generateUnit(exe, opts)     // frozen invocation of the same serve flags

    if printOnly {
        fmt.Print(unit)                  // review, or pipe through `sudo tee`
        return nil
    }
    path := "/etc/systemd/system/workshop.service"
    if userUnit {
        path = "~/.config/systemd/user/workshop.service"  // expanded
    }
    // write unit, then print (NOT run) the follow-ups:
    //   systemctl daemon-reload && systemctl enable --now workshop
    //   (--user variants when userUnit)
}
```

Generated unit:

```ini
[Unit]
Description=Workshop runbook (<workshop name from workshop.yaml>)
After=network.target

[Service]
ExecStart=<abs-path>/workshop-backend --serve <dir> --listen <addr> --auth-user <u> --auth-password-file <f>
Restart=on-failure
User=<invoking user>

[Install]
WantedBy=multi-user.target
```

Rules:

- **`User=`** defaults to the invoking user — under sudo, `$SUDO_USER`, not root. The service must run as a real operator account (terminal shell, kubeconfig, RBAC all depend on it). Omitted entirely for `--user` units.
- **Refusal:** non-loopback `--listen` with no auth flags → hard error. The ad-hoc serve path only warns; a persistent unattended service does not get that leeway.
- **Validation before writing:** the serve dir must exist and contain `workshop.yaml`; the password file must exist. Fail at install time, not at 3am on `Restart=on-failure` loop.
- `service uninstall` removes whichever unit path exists and prints the `daemon-reload` reminder.

No systemd library dependency — the unit is a text template. The `service` subcommand is compiled in unconditionally but is inert in container modes (never invoked).

---

## Step 7: Installer Script + Release Asset

**New file: `scripts/install-standalone.sh`** — **File: `dagger/main.go`** (`BuildRelease`)

Same skeleton as the devcontainer feature's `install.sh` (arch detection, resolve `latest`, download `workshop-backend workshop-setup ttyd goss` from the release to `/usr/local/bin`), with two deliberate differences:

1. **No global shell config changes** — does NOT touch `/etc/bash.bashrc`, does NOT install `/etc/workshop-platform.bashrc`. The backend embeds and writes its own rcfile (Step 2c).
2. **No `/workshop` directory creation** — `--serve` resolves and creates its own root.

`compile-workshop` is not in the default set (`--serve` embeds compilation); a `WITH_COMPILE=1` env opts in for CI/pre-compile use.

Add the script itself to `BuildRelease` output so the install one-liner is version-pinned alongside the binaries:

```go
out = out.WithFile("install-standalone.sh", src.File("scripts/install-standalone.sh"))
```

---

## Step 8: Docs

- `docs/platform/backend-capabilities.md` — add standalone column: in-place transitions, no infrastructure provisioning, no aggregation, no instructor dashboard, basic auth optional.
- `docs/platform/instrumentation.md` — note the standalone scoping (rcfile-per-session instead of global `/etc/bash.bashrc`) and the `WORKSHOP_ROOT` parameterization.
- `docs/platform/standalone-mode.md` — reconcile the install table: the bashrc row becomes "embedded in `workshop-backend`, written under `WORKSHOP_ROOT`" rather than `/etc/workshop-platform.bashrc` (the doc predates the no-root constraint).
- `docs/plan.md` — add M14 section.

---

## Testing

### Unit

- `BasicAuth` — missing/wrong/correct credentials; constant-time paths
- Unit file generation — golden file tests (root unit, `--user` unit, with/without auth)
- `WriteRcfile` — chains `~/.bashrc`, contains the hook, honors root path
- Infrastructure guard — cluster-enabled and extraContainers workshops rejected with the right message
- `CompileToDir` — already covered by M13 golden tests; add a from-`--serve` invocation test

### Integration (on a Linux host or disposable VM)

1. `--serve examples/hello-linux` → full UI walkthrough, terminal on host, goss against host
2. Command logged from UI terminal; NOT logged from a parallel SSH session
3. Auth on `0.0.0.0` → 401 without credentials, terminal WS included
4. `service install` on a throwaway VM → `systemctl enable --now` → reboot → service up

---

## Key Gotchas

1. **`--rcfile` replaces `~/.bashrc`** — the generated rcfile must source the user's own bashrc first or the operator loses their environment (aliases, kubeconfig exports, prompt). This is the difference between "nice runbook" and "why is my shell broken."
2. **`go:embed` cannot use `../`** — hence relocating the canonical bashrc into `backend/instrumentation/`. Do not leave a second copy at `base-images/bashrc`; update Dagger references instead.
3. **Passwords never in argv** — file or env only. `ps` and shell history are shared surfaces on exactly the kind of server this runs on.
4. **`$SUDO_USER` for `User=`** — `service install` typically runs under sudo; the unit must not default to `User=root` silently.
5. **Recompile-on-start overwrites `WORKSHOP_ROOT` content but must not touch `runtime/`** — `CompileToDir` writes metadata and steps; the runtime dir (command log, casts, state events) persists across restarts within the root. Clean step dirs before rewriting; leave `runtime/` alone.
6. **Mode string discipline** — `inPlace := mode == "devcontainer" || mode == "standalone"` lives in one place. Handlers branch on capability (`inPlace`), never on the raw mode string, or the third mode starts the combinatorial mess the design avoids.
7. **WebSocket auth is free only because of the proxy** — if anyone ever exposes ttyd's port directly, auth is bypassed. ttyd stays on `127.0.0.1`; do not add a flag to change that.

## Consolidation Notes

| What | Single Source of Truth | Consumed By |
|------|----------------------|-------------|
| Bashrc | `backend/instrumentation/workshop-platform.bashrc` (moved from `base-images/`) | go:embed (standalone rcfile), Dagger base image build, `BuildRelease` → devcontainer `install.sh` |
| Compile-to-directory writer | `pkg/workshop.CompileToDir()` | `cmd/compile-workshop`, backend `--serve` |
| Step application | `backend/setup/apply.go` (M13, unchanged) | activate handler (devcontainer AND standalone), `workshop-setup` CLI |
| In-place mode behavior | `inPlace` capability flag | devcontainer mode, standalone mode |
| Release assets incl. installer | `dagger/main.go` → `BuildRelease()` | GitHub releases, `install-standalone.sh` one-liner |

**Standalone mode adds no new transition machinery.** Everything between "operator clicks Next Step" and "files copied, commands run" is M13 code. M14 is: flag parsing, one auth middleware, one rcfile writer, one unit-file template, one installer script, and the bashrc relocation that makes the binary self-sufficient.
