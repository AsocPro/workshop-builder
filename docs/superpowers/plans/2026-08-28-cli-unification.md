# CLI Unification (M15) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the four binaries (`workshop-backend`, `workshop` CLI, `compile-workshop`, `workshop-setup`) into one `workshop` command with subcommands, backed by cleanly-separated packages.

**Architecture:** A thin cobra dispatcher at `cmd/workshop/` routes to existing packages. The bulk of the work is extracting each tool's logic out of `package main` into exported entrypoints so the dispatcher can call them. Bare `workshop` (no subcommand) runs the container serve path, so image ENTRYPOINTs are behaviorally unchanged.

**Tech Stack:** Go 1.24, cobra (CLI), chi (HTTP), Dagger (all builds/tests). Module `github.com/asocpro/workshop-builder`.

## Global Constraints

- **No local Go/Node toolchain.** All builds and tests run through Dagger. The gate for Go tasks is `make test` (→ `dagger call test --src .`). Never run `go test`/`go build` directly on the host.
- Module path: `github.com/asocpro/workshop-builder`. Go 1.24.
- **Hard cutover** — no back-compat aliases. Old binary names are removed, not symlinked.
- CLI dispatch uses **cobra** (the `cli/` package already does).
- The `compile` subcommand MUST emit **byte-identical JSON** to today's `compile-workshop` (the Dagger pipeline parses it). Same struct fields, same tags.
- Do **not** rewrite `workshop run` in docs — the CLI binary is already `workshop`, so that syntax is already correct.
- `cmd/` holds thin subcommand wiring only — no business logic.

## File Structure

- `backend/` (top-level package) — converts from `package main` to library `package backend`; exports `RunContainer`, `RunStandalone`, `StandaloneOptions`. Keeps `go:embed frontend/dist`.
- `backend/servecmd/` — `GenerateUnit` emits the `serve` subcommand form; usage strings updated.
- `backend/setup/` — gains `RunThrough(workshopDir, throughStep string) error` (the old `workshop-setup` loop).
- `pkg/workshop/compilecli/` (new package) — `Run(workshopPath, outputDir string, w io.Writer) error` (the old `compile-workshop` body + its JSON output types).
- `cli/cmd/` — exports `RunCommand() *cobra.Command`; drops its own root.
- `cmd/workshop/` (new package `main`) — cobra root + subcommand wiring + dispatch test.
- Deleted: `backend/main.go`'s `func main`, `cli/main.go`, `cmd/compile-workshop/`, `cmd/workshop-setup/`.
- `dagger/main.go` — one `BuildWorkshopBinary` replaces `BuildBackend`/`BuildCompileWorkshop`/`BuildSetup`/`BuildCLI`; base-image + release + `runCompileWorkshop` repointed; `Test` runs `./cmd/...`.
- `Makefile`, `scripts/`, `devcontainer-feature/`, `docs/`, `STATUS.md`, `CLAUDE.md` — name/subcommand updates.

---

### Task 1: Convert `backend/` top-level to a library package

**Files:**
- Rename: `backend/main.go` → `backend/container.go` (holds `RunContainer`)
- Modify: `backend/container.go`, `backend/standalone.go`, `backend/server.go`, `backend/auth.go`, `backend/embed.go`, `backend/backend_test.go`, `backend/auth_test.go`

**Interfaces:**
- Produces:
  - `func RunContainer()` — container-mode entrypoint (env-configured; blocks on ListenAndServe; `log.Fatal` on error).
  - `func RunStandalone(opts StandaloneOptions) error`
  - `type StandaloneOptions struct { ServeDir, Listen, AuthUser, AuthPasswordFile string }`
  - `func NewServer(...)` and `func BasicAuth(...)` (already exported; now in `package backend`).

- [ ] **Step 1: Rename the file**

```bash
git mv backend/main.go backend/container.go
```

- [ ] **Step 2: Change package declarations to `backend`**

In all seven files (`container.go`, `standalone.go`, `server.go`, `auth.go`, `embed.go`, `backend_test.go`, `auth_test.go`) change the first line `package main` → `package backend`.

- [ ] **Step 3: Delete `func main` and rename `runContainer` → `RunContainer`**

In `backend/container.go`, remove the entire `func main() { … }` block (the `service` dispatch + flag parsing — that logic moves to `cmd/workshop`). Rename `func runContainer()` → `func RunContainer()`. Remove the now-unused `"flag"` import. The `os`, `fmt`, `log`, `net/http`, `path/filepath`, and the backend subpackage imports stay. Result is exactly the old `runContainer` body under the new name:

```go
package backend

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/asocpro/workshop-builder/backend/process"
	"github.com/asocpro/workshop-builder/backend/store"
)

// RunContainer is the container-mode entrypoint (Docker local mode, cluster
// mode, devcontainer mode) — configured entirely by environment variables.
func RunContainer() {
	workshopRoot := os.Getenv("WORKSHOP_ROOT")
	if workshopRoot == "" {
		workshopRoot = "/workshop"
	}
	// … unchanged body of the old runContainer …
}
```

- [ ] **Step 4: Rename `runStandalone`/`standaloneOptions` → exported**

In `backend/standalone.go`: `type standaloneOptions` → `type StandaloneOptions`, `func runStandalone(opts standaloneOptions)` → `func RunStandalone(opts StandaloneOptions)`. Update the internal references (`resolveAuthPassword(opts standaloneOptions)` → `StandaloneOptions`). `guardInfrastructure` and `resolveAuthPassword` stay unexported.

- [ ] **Step 5: Run the test gate to verify the module still compiles and tests pass**

Run: `make test`
Expected: PASS. `go build ./...` succeeds (backend is now a library; `cli/main.go` and the `cmd/*` mains still provide their own `func main`). `go test ./pkg/workshop/... ./backend/...` passes — the backend tests moved with the package rename.

- [ ] **Step 6: Commit**

```bash
git add backend/
git commit -m "Convert backend to a library package with exported entrypoints"
```

---

### Task 2: `servecmd` generates the `serve` subcommand form

The installed systemd unit runs the binary. Standalone is now `workshop serve <dir>`, not `workshop-backend --serve <dir>`, so the generated `ExecStart` must use the `serve` subcommand with a positional dir. `service install`'s own `--serve` flag is unchanged.

**Files:**
- Modify: `backend/servecmd/systemd.go`
- Test: `backend/servecmd/systemd_test.go`

- [ ] **Step 1: Update the test expectations to the new ExecStart form**

In `backend/servecmd/systemd_test.go`, change the exec path argument passed to `GenerateUnit` from `/usr/local/bin/workshop-backend` to `/usr/local/bin/workshop`, and change the expected `ExecStart` line from the `--serve` form to the subcommand form. Example (the first test):

```go
unit := GenerateUnit("/usr/local/bin/workshop", cfg, "platform-admin")
// …
want := `ExecStart=/usr/local/bin/workshop serve /opt/onboarding-workshop --listen 0.0.0.0:8080 --auth-user admin --auth-password-file /etc/workshop/pass`
```

Apply the same `workshop-backend` → `workshop` path change to the other two `GenerateUnit` calls (lines ~34 and ~49).

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — `systemd_test.go` expects `serve <dir>` but `GenerateUnit` still emits `--serve <dir>`.

- [ ] **Step 3: Emit the `serve` subcommand in `GenerateUnit`**

In `backend/servecmd/systemd.go`, change the `exec` slice in `GenerateUnit` so the workshop directory is a positional argument to a `serve` subcommand:

```go
exec := []string{
	unitQuote(exePath),
	"serve", unitQuote(cfg.ServeDir),
	"--listen", unitQuote(cfg.Listen),
}
```

- [ ] **Step 4: Update the usage string and package comment**

Same file: line ~29 `return fmt.Errorf("usage: workshop-backend service install|uninstall [flags]")` → `workshop service install|uninstall`. Line 1 package comment `the `workshop-backend service` subcommand` → `the `workshop service` subcommand`.

- [ ] **Step 5: Run the test to verify it passes**

Run: `make test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/servecmd/
git commit -m "Generate systemd ExecStart using the serve subcommand"
```

---

### Task 3: Extract the setup runner into `backend/setup`

**Files:**
- Modify: `backend/setup/apply.go` (add `RunThrough` + `readStepOrder`)
- Create: `backend/setup/runthrough_test.go`
- Delete: `cmd/workshop-setup/` (whole dir)

**Interfaces:**
- Produces: `func RunThrough(workshopDir, throughStep string) error` — applies setup for every step up to and including `throughStep`; empty `throughStep` prints a notice and returns nil; unknown `throughStep` returns an error.

- [ ] **Step 1: Write the failing test**

Create `backend/setup/runthrough_test.go`:

```go
package setup

import (
	"os"
	"path/filepath"
	"testing"
)

// writeWorkshop builds a minimal compiled workshop dir: workshop.json listing
// the given step IDs, each with an empty setup.json (no files/commands).
func writeWorkshop(t *testing.T, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	steps := ""
	for i, id := range ids {
		if i > 0 {
			steps += ","
		}
		steps += `{"id":"` + id + `"}`
	}
	if err := os.WriteFile(filepath.Join(dir, "workshop.json"), []byte(`{"steps":[`+steps+`]}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		stepDir := filepath.Join(dir, "steps", id)
		if err := os.MkdirAll(stepDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stepDir, "setup.json"), []byte(`{"files":[],"commands":[],"env":{}}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunThroughAppliesThroughStep(t *testing.T) {
	dir := writeWorkshop(t, "step-1", "step-2", "step-3")
	if err := RunThrough(dir, "step-2"); err != nil {
		t.Fatalf("RunThrough: %v", err)
	}
}

func TestRunThroughUnknownStep(t *testing.T) {
	dir := writeWorkshop(t, "step-1", "step-2")
	if err := RunThrough(dir, "nope"); err == nil {
		t.Fatal("expected error for unknown step, got nil")
	}
}

func TestRunThroughEmptyStepIsNoop(t *testing.T) {
	dir := writeWorkshop(t, "step-1")
	if err := RunThrough(dir, ""); err != nil {
		t.Fatalf("RunThrough empty: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — `RunThrough` and `readStepOrder` are undefined.

- [ ] **Step 3: Add `RunThrough` and `readStepOrder` to `backend/setup/apply.go`**

Append to `backend/setup/apply.go` (the loop and `readStepOrder` are moved verbatim from the old `cmd/workshop-setup/main.go`, with `log.Fatalf` replaced by returned errors):

```go
// RunThrough applies each step's setup, in workshop order, up to and including
// throughStep. Empty throughStep is a no-op (start at step 1). It backs the
// devcontainer skip-ahead helper. Errors are returned, not fatal.
func RunThrough(workshopDir, throughStep string) error {
	if throughStep == "" {
		fmt.Println("No step specified — starting at step 1, no setup applied.")
		return nil
	}
	stepIDs, err := readStepOrder(workshopDir)
	if err != nil {
		return fmt.Errorf("reading workshop.json: %w", err)
	}
	found := false
	for _, id := range stepIDs {
		s, err := Load(workshopDir, id)
		if err != nil {
			return fmt.Errorf("loading setup for %s: %w", id, err)
		}
		fmt.Printf("Applying setup for %s...\n", id)
		if err := Apply(workshopDir, id, s); err != nil {
			return fmt.Errorf("applying setup for %s: %w", id, err)
		}
		if id == throughStep {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("step %q not found in workshop", throughStep)
	}
	fmt.Printf("Setup applied through %s.\n", throughStep)
	return nil
}

func readStepOrder(workshopDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(workshopDir, "workshop.json"))
	if err != nil {
		return nil, err
	}
	var wj struct {
		Steps []struct {
			ID string `json:"id"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &wj); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(wj.Steps))
	for _, s := range wj.Steps {
		ids = append(ids, s.ID)
	}
	return ids, nil
}
```

(`encoding/json`, `fmt`, `os`, `path/filepath` are already imported by `apply.go`.)

- [ ] **Step 4: Delete the old CLI**

```bash
git rm -r cmd/workshop-setup
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `make test`
Expected: PASS (the three `RunThrough` tests run under `./backend/...`). `go build ./...` still succeeds — nothing references the deleted `cmd/workshop-setup`.

- [ ] **Step 6: Commit**

```bash
git add backend/setup/ && git rm -r --cached cmd/workshop-setup 2>/dev/null; git commit -m "Extract setup runner into backend/setup.RunThrough"
```

---

### Task 4: Extract the compile runner into `pkg/workshop/compilecli`

A dedicated subpackage keeps the JSON-emitting CLI adapter out of the pure compiler while reusing its `Parse`/`Validate`/`Compile`/`CompileToDir`. The output types live here (not in `pkg/workshop`) to avoid colliding with the compiler's own `FileMapping`.

**Files:**
- Create: `pkg/workshop/compilecli/compilecli.go`
- Create: `pkg/workshop/compilecli/compilecli_test.go`
- Delete: `cmd/compile-workshop/` (whole dir)

**Interfaces:**
- Produces: `func Run(workshopPath, outputDir string, w io.Writer) error` — with `outputDir` set, writes the flat-file layout via `workshop.CompileToDir` (no stdout output); otherwise encodes the `Output` JSON to `w`.

- [ ] **Step 1: Write the failing test**

Create `pkg/workshop/compilecli/compilecli_test.go`:

```go
package compilecli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunEmitsWorkshopJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Run("../../../examples/hello-linux", "", &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var out Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if out.WorkshopJSON == "" {
		t.Fatal("workshopJson is empty")
	}
	if len(out.Steps) == 0 {
		t.Fatal("no steps in output")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — package `compilecli` does not exist. (Note: this test lives under `./pkg/workshop/...`, which `make test` already runs.)

- [ ] **Step 3: Create the package**

Create `pkg/workshop/compilecli/compilecli.go` — the old `cmd/compile-workshop/main.go` body, restructured as `Run` returning errors and writing to `w`. Keep the `Output`/`StepOutput`/`FileMapping` types and their JSON tags **exactly** as they were (the Dagger pipeline parses this):

```go
// Package compilecli is the CLI adapter over pkg/workshop: it compiles a
// workshop source directory and either writes the flat-file layout to a
// directory or emits compiled metadata as JSON. Used by the `workshop compile`
// subcommand and, via that, by the Dagger pipeline.
package compilecli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/asocpro/workshop-builder/pkg/workshop"
)

// Output is the JSON structure emitted to the writer.
type Output struct {
	WorkshopJSON string       `json:"workshopJson"`
	BaseImage    string       `json:"baseImage"`
	Steps        []StepOutput `json:"steps"`
}

// FileMapping mirrors workshop.FileMapping for JSON output.
type FileMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode,omitempty"`
}

// StepOutput holds compiled step data plus the build-time file/command/env specs.
type StepOutput struct {
	ID         string            `json:"id"`
	MetaJSON   string            `json:"metaJson"`
	LLMJson    string            `json:"llmJson,omitempty"`
	HasGoss    bool              `json:"hasGoss"`
	HasHints   bool              `json:"hasHints"`
	HasExplain bool              `json:"hasExplain"`
	HasSolve   bool              `json:"hasSolve"`
	HasLLMDocs bool              `json:"hasLlmDocs"`
	Files      []FileMapping     `json:"files,omitempty"`
	Commands   []string          `json:"commands,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

// Run compiles the workshop at workshopPath. With outputDir set it writes the
// flat-file layout there and produces no JSON. Otherwise it encodes the Output
// JSON to w.
func Run(workshopPath, outputDir string, w io.Writer) error {
	if workshopPath == "" {
		return fmt.Errorf("workshop path is required")
	}
	abs, err := filepath.Abs(workshopPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	if outputDir != "" {
		if _, err := workshop.CompileToDir(abs, outputDir); err != nil {
			return fmt.Errorf("compiling workshop: %w", err)
		}
		return nil
	}

	loaded, err := workshop.Parse(abs)
	if err != nil {
		return fmt.Errorf("parsing workshop: %w", err)
	}
	if err := workshop.Validate(loaded); err != nil {
		return fmt.Errorf("validating workshop: %w", err)
	}
	compiled, err := workshop.Compile(loaded)
	if err != nil {
		return fmt.Errorf("compiling workshop: %w", err)
	}

	baseImage := loaded.Manifest.Base.Image
	if baseImage == "" {
		baseImage = "workshop-base:ubuntu"
	}
	out := Output{
		WorkshopJSON: string(compiled.WorkshopJSON),
		BaseImage:    baseImage,
	}
	for i, cs := range compiled.Steps {
		s := loaded.Steps[i]
		so := StepOutput{
			ID:         cs.ID,
			MetaJSON:   string(cs.MetaJSON),
			HasGoss:    s.HasGoss,
			HasHints:   s.HasHints,
			HasExplain: s.HasExplain,
			HasSolve:   s.HasSolve,
			HasLLMDocs: s.HasLLMDocs,
		}
		if cs.LLMJSON != nil {
			so.LLMJson = string(cs.LLMJSON)
		}
		for _, fm := range s.Spec.Files {
			so.Files = append(so.Files, FileMapping{Source: fm.Source, Target: fm.Target, Mode: fm.Mode})
		}
		if len(s.Spec.Commands) > 0 {
			so.Commands = s.Spec.Commands
		}
		if len(s.Spec.Env) > 0 {
			so.Env = s.Spec.Env
		}
		out.Steps = append(out.Steps, so)
	}

	if err := json.NewEncoder(w).Encode(out); err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}
	return nil
}
```

> If any field name above does not match the current `cmd/compile-workshop/main.go` (e.g. `loaded.Manifest.Base.Image`, `s.Spec.Files`, `cs.LLMJSON`), copy the exact expressions from that file before deleting it — the struct assembly must stay identical.

- [ ] **Step 4: Delete the old CLI**

```bash
git rm -r cmd/compile-workshop
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `make test`
Expected: PASS — `TestRunEmitsWorkshopJSON` produces parseable JSON with steps. `go build ./...` still succeeds.

> Note: `dagger call build-workshop` is temporarily broken here (its `runCompileWorkshop` still calls the deleted `./cmd/compile-workshop/`); Task 7 repoints it. `make test` does not invoke that path.

- [ ] **Step 6: Commit**

```bash
git add pkg/workshop/compilecli && git commit -m "Extract compile runner into pkg/workshop/compilecli"
```

---

### Task 5: Export the `run` command from `cli/cmd`; drop its root

**Files:**
- Modify: `cli/cmd/root.go`
- Delete: `cli/main.go`

**Interfaces:**
- Produces: `func RunCommand() *cobra.Command` — returns the configured `run <image>` cobra command for the umbrella to mount.

- [ ] **Step 1: Replace the standalone root with an exported accessor**

Rewrite `cli/cmd/root.go` to drop `rootCmd`/`Execute` (no longer a standalone binary) and expose the `run` command. `run.go` still declares `var runCmd` and registers its flag in `init()` — leave that untouched:

```go
package cmd

import "github.com/spf13/cobra"

// RunCommand returns the `run <image>` command for mounting under the unified
// `workshop` umbrella.
func RunCommand() *cobra.Command {
	return runCmd
}
```

- [ ] **Step 2: Delete the standalone CLI main**

```bash
git rm cli/main.go
```

- [ ] **Step 3: Run the test gate**

Run: `make test`
Expected: PASS. `go build ./...` succeeds — `cli/cmd` compiles as a library; nothing references the deleted `cli/main.go`.

- [ ] **Step 4: Commit**

```bash
git add cli/ && git rm --cached cli/main.go 2>/dev/null; git commit -m "Export cli/cmd.RunCommand; drop standalone CLI root"
```

---

### Task 6: Create the `cmd/workshop` dispatcher

**Files:**
- Create: `cmd/workshop/main.go`
- Create: `cmd/workshop/dispatch_test.go`
- Modify: `dagger/main.go` (add `./cmd/...` to the `Test` exec)

**Interfaces:**
- Consumes: `backend.RunContainer`, `backend.RunStandalone`, `backend.StandaloneOptions`, `servecmd.RunService`, `setup.RunThrough`, `compilecli.Run`, `clicmd.RunCommand`.
- Produces: `func newRootCmd() *cobra.Command` (assembles root + subcommands; `main` and the test both call it).

- [ ] **Step 1: Add `./cmd/...` to the Dagger test exec**

In `dagger/main.go`, the `Test` function's test exec (line ~28) currently runs `./pkg/workshop/... ./backend/...`. Add `./cmd/...` so the dispatch test runs:

```go
WithExec([]string{"go", "test", "-v", "./pkg/workshop/...", "./backend/...", "./cmd/..."}).
```

- [ ] **Step 2: Write the failing dispatch test**

Create `cmd/workshop/dispatch_test.go`:

```go
package main

import (
	"sort"
	"testing"
)

func TestRootHasAllSubcommands(t *testing.T) {
	root := newRootCmd()
	var names []string
	for _, c := range root.Commands() {
		names = append(names, c.Name())
	}
	sort.Strings(names)
	want := map[string]bool{"serve": false, "service": false, "compile": false, "setup": false, "run": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("missing subcommand %q (have %v)", n, names)
		}
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `make test`
Expected: FAIL — `newRootCmd` is undefined.

- [ ] **Step 4: Write the dispatcher**

Create `cmd/workshop/main.go`:

```go
// Command workshop is the unified entrypoint. With no subcommand it runs the
// container serve path (the in-image ENTRYPOINT); subcommands cover the other
// delivery modes.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	backend "github.com/asocpro/workshop-builder/backend"
	"github.com/asocpro/workshop-builder/backend/servecmd"
	"github.com/asocpro/workshop-builder/backend/setup"
	clicmd "github.com/asocpro/workshop-builder/cli/cmd"
	"github.com/asocpro/workshop-builder/pkg/workshop/compilecli"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "workshop",
		Short:         "Workshop platform — build and run technical Linux workshops",
		SilenceUsage:  true,
		SilenceErrors: true,
		// No subcommand: container-mode entrypoint (env-configured). Blocks.
		Run: func(cmd *cobra.Command, args []string) {
			backend.RunContainer()
		},
	}

	// serve <dir> — standalone mode.
	var opts backend.StandaloneOptions
	serve := &cobra.Command{
		Use:   "serve <dir>",
		Short: "Compile and serve a workshop source directory (standalone mode)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ServeDir = args[0]
			return backend.RunStandalone(opts)
		},
	}
	serve.Flags().StringVar(&opts.Listen, "listen", "127.0.0.1:8080", "listen address")
	serve.Flags().StringVar(&opts.AuthUser, "auth-user", "", "basic auth username")
	serve.Flags().StringVar(&opts.AuthPasswordFile, "auth-password-file", "", "file containing the basic auth password (alternative: WORKSHOP_AUTH_PASSWORD env)")

	// service install|uninstall — systemd unit. servecmd owns its own flag
	// parsing, so pass raw args straight through.
	service := &cobra.Command{
		Use:                "service install|uninstall [flags]",
		Short:              "Install or uninstall the standalone systemd unit",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return servecmd.RunService(args)
		},
	}

	// compile <dir> — compile to a directory or emit JSON.
	var compileOut string
	compile := &cobra.Command{
		Use:   "compile <dir>",
		Short: "Compile a workshop; write a flat-file dir (--output-dir) or emit JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return compilecli.Run(args[0], compileOut, os.Stdout)
		},
	}
	compile.Flags().StringVar(&compileOut, "output-dir", "", "write the compiled flat-file layout here (default: print JSON to stdout)")

	// setup — pre-apply prior steps' setup (devcontainer skip-ahead).
	var setupDir, setupStep string
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Apply step setup through a target step (skip-ahead)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setup.RunThrough(setupDir, setupStep)
		},
	}
	setupCmd.Flags().StringVar(&setupDir, "workshop-dir", "/workshop", "compiled workshop directory")
	setupCmd.Flags().StringVar(&setupStep, "step", "", "apply setup through this step ID (inclusive)")

	root.AddCommand(serve, service, compile, setupCmd, clicmd.RunCommand())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

> The `compile` subcommand keeps the old `--workshop`/`--output-dir` interface as `<dir>` positional + `--output-dir`. Task 7 updates the Dagger caller to `compile --workshop`… — see that task; if you prefer to preserve a `--workshop` flag instead of the positional arg, do so consistently in both places. This plan uses positional `<dir>` and Task 7's Dagger call matches.

- [ ] **Step 5: Run the test to verify it passes**

Run: `make test`
Expected: PASS — `TestRootHasAllSubcommands` finds `serve`, `service`, `compile`, `setup`, `run`. `go build ./...` builds the new `cmd/workshop` main.

- [ ] **Step 6: Commit**

```bash
git add cmd/workshop dagger/main.go && git commit -m "Add unified workshop dispatcher; run cmd tests in Dagger"
```

---

### Task 7: Rewrite the Dagger build/release pipeline and Makefile

Replace the four binary builders with one, repoint base-image bakes and the compile call, collapse the release to one binary, and update Makefile targets. Named `BuildWorkshopBinary` to avoid colliding with the existing `BuildWorkshop` (step-image builder).

**Files:**
- Modify: `dagger/main.go`
- Modify: `Makefile`

- [ ] **Step 1: Add `BuildWorkshopBinary`, remove the four old builders**

In `dagger/main.go`, delete `BuildBackend`, `BuildCompileWorkshop`, `BuildSetup`, and `BuildCLI`. Add one builder that overlays the frontend dist (for the `go:embed`) and builds `./cmd/workshop/`:

```go
// BuildWorkshopBinary builds the frontend then cross-compiles the single
// unified `workshop` binary (frontend embedded, no CGO).
func (m *WorkshopBuilder) BuildWorkshopBinary(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// +optional
	// +default="linux"
	targetOS string,
	// +optional
	// +default="amd64"
	targetArch string,
) *dagger.File {
	srcWithDist := src.WithDirectory("backend/frontend/dist", m.buildFrontendDist(src))
	return m.buildGoBinary(srcWithDist, "./cmd/workshop/", "workshop", targetOS, targetArch)
}
```

- [ ] **Step 2: Repoint base-image and step-image bakes**

Replace every `m.BuildBackend(ctx, src, …)` call (lines ~147, ~177, ~303) with `m.BuildWorkshopBinary(ctx, src, …)`. In `buildUbuntuBase`, `buildDebianBase`, `buildRockyBase`, `buildBaseImage`, `resolveBaseContainer` (and `BuildBaseImages`/`PublishBaseImages`), rename the `backendBin` parameter/variable to `workshopBin`, change each `WithFile("/usr/local/bin/workshop-backend", …)` to `WithFile("/usr/local/bin/workshop", …)`, and each `WithDefaultArgs([]string{"/usr/local/bin/workshop-backend"})` to `WithDefaultArgs([]string{"/usr/local/bin/workshop"})`.

- [ ] **Step 3: Repoint the compile invocation**

In `runCompileWorkshop`, change the exec from the deleted binary to the subcommand:

```go
WithExec([]string{
	"go", "run", "./cmd/workshop/", "compile", workshopPath,
}).
```

(Positional `<dir>` — matches the `compile` subcommand from Task 6. Drop the old `--workshop` flag.)

- [ ] **Step 4: Collapse the release to one binary**

In `BuildRelease`, replace the four `WithFile(...)` binary lines with one, keeping ttyd/goss:

```go
WithFile("workshop-linux-"+arch, m.BuildWorkshopBinary(ctx, src, "linux", arch)).
WithFile("ttyd-linux-"+arch, m.downloadTtydArch(arch)).
WithFile("goss-linux-"+arch, m.downloadGossArch(arch))
```

- [ ] **Step 5: Update the Makefile targets**

In `Makefile`, remove `build-backend`, `build-compile-workshop`, `build-setup`, `build-cli`; add a single binary target (call it `build`, distinct from the existing `build-workshop` step-image target). Update the `.PHONY` line accordingly:

```makefile
build:
	dagger call build-workshop-binary --src . -o workshop
	chmod +x workshop
```

- [ ] **Step 6: Verify the binary builds and dispatches**

Run: `dagger call build-workshop-binary --src . -o /tmp/workshop-bin && /tmp/workshop-bin --help`
Expected: help output listing `serve`, `service`, `compile`, `setup`, `run`. (Host is linux/amd64, so the linux binary runs.)

- [ ] **Step 7: Verify the compile path and step-image build end-to-end**

Run: `dagger call build-workshop --src . --workshop-path examples/hello-linux -o /tmp/wimgs && ls /tmp/wimgs`
Expected: one `<step-id>.tar` per step — exercises the repointed `runCompileWorkshop` and the `workshop`-baked base image.

- [ ] **Step 8: Commit**

```bash
git add dagger/main.go Makefile && git commit -m "Ship one workshop binary from the Dagger pipeline"
```

---

### Task 8: Update downstream scripts and the devcontainer feature

**Files:**
- Modify: `devcontainer-feature/src/workshop/install.sh`
- Modify: `devcontainer-feature/src/workshop/devcontainer-feature.json`
- Modify: `scripts/install-standalone.sh`
- Modify: `scripts/smoke-test.sh`

- [ ] **Step 1: Devcontainer install.sh — one binary + subcommands**

In `devcontainer-feature/src/workshop/install.sh`: change the download loop to `for BINARY in workshop ttyd goss; do`. In the generated `workshop-compile-and-setup` helper, change `compile-workshop --workshop "${WORKSHOP_PATH}" --output-dir /workshop` → `workshop compile "${WORKSHOP_PATH}" --output-dir /workshop`, and `workshop-setup --workshop-dir /workshop --step "${START_STEP}"` → `workshop setup --workshop-dir /workshop --step "${START_STEP}"`.

- [ ] **Step 2: Devcontainer feature JSON — postStart**

In `devcontainer-feature/src/workshop/devcontainer-feature.json`, change `postStartCommand` to `nohup workshop >/tmp/workshop.log 2>&1 &`. (`postCreateCommand: workshop-compile-and-setup` stays — that helper name is internal.)

- [ ] **Step 3: install-standalone.sh — one binary, subcommand examples**

In `scripts/install-standalone.sh`: set `BINARIES="workshop ttyd goss"` and remove the `WITH_COMPILE` branch and its `compile-workshop` mention (compile is now a `workshop` subcommand, always present). Update usage examples: `workshop-backend --serve ./my-workshop` → `workshop serve ./my-workshop`; `workshop-backend service install --serve /opt/my-workshop …` → `workshop service install --serve /opt/my-workshop …` (the `service install --serve` flag is unchanged).

- [ ] **Step 4: smoke-test.sh — binary path**

In `scripts/smoke-test.sh` line ~29, change the check target `/usr/local/bin/workshop-backend` → `/usr/local/bin/workshop`.

- [ ] **Step 5: Verify no stale names remain in these files**

Run: `grep -rn 'workshop-backend\|compile-workshop\|workshop-setup' devcontainer-feature/ scripts/`
Expected: no matches.

- [ ] **Step 6: Commit**

```bash
git add devcontainer-feature/ scripts/ && git commit -m "Point scripts and devcontainer feature at the unified workshop binary"
```

---

### Task 9: Update docs, STATUS.md, CLAUDE.md

Reconcile the design-of-record with the unified binary. **Do not touch `workshop run`** anywhere.

**Files:**
- Modify: `STATUS.md`, `CLAUDE.md`
- Modify (substantive): `docs/platform/backend-service.md`, `docs/platform/standalone-mode.md`, `docs/platform/devcontainer-feature.md`, `docs/platform/base-images.md`
- Modify (trivial): `docs/artifact/compilation.md`, `docs/artifact/flat-file-artifact.md`, `docs/overview.md`, `docs/presentation/frontend.md`

- [ ] **Step 1: STATUS.md — flip the m15 row**

In `STATUS.md`, change the "Unify tools into one `workshop` command" row from `⬜` to `✅`, and adjust the Notes to reflect it's built (single `workshop` binary, subcommands `serve`/`run`/`compile`/`setup`/`service`).

- [ ] **Step 2: CLAUDE.md — architecture section**

Update the "Architecture (the built reality)" section: replace the "Three binaries" framing with the single `workshop` binary and its subcommands; note the source packages (`backend/`, `cli/`, `pkg/workshop/compilecli`, `backend/setup`) still hold the logic behind the `cmd/workshop` dispatcher. Update the `cmd/` bullet in "Source layout" (now `cmd/workshop`).

- [ ] **Step 3: Substantive docs — subcommand rewrites**

Apply these transformations (mind the flag→subcommand change; leave `workshop run` and `service install --serve` alone):
- `workshop-backend --serve <dir>` → `workshop serve <dir>`
- `workshop-backend service install` → `workshop service install`
- bare `workshop-backend` / `/usr/local/bin/workshop-backend` → `workshop` / `/usr/local/bin/workshop`
- `compile-workshop …` → `workshop compile …`; `workshop-setup …` → `workshop setup …`

Files: `docs/platform/backend-service.md` (ENTRYPOINT, process tree, `service install`, `--serve`), `docs/platform/standalone-mode.md` (`--serve`, `workshop-setup`, `compile-workshop`, tool table, systemd `ExecStart` example → `workshop serve <dir> --listen …`), `docs/platform/devcontainer-feature.md` (tool table, install.sh flow, `postStartCommand`, `cmd/workshop-setup` ref → `workshop setup`), `docs/platform/base-images.md` (ENTRYPOINT + tool-table paths).

- [ ] **Step 4: Trivial docs — component renames**

Rename the single `workshop-backend`/`compile-workshop`/`workshop-setup` mention in each of: `docs/artifact/compilation.md`, `docs/artifact/flat-file-artifact.md`, `docs/overview.md` (leave its `workshop run`), `docs/presentation/frontend.md`.

- [ ] **Step 5: Verify only intended references remain**

Run: `grep -rn 'workshop-backend\|compile-workshop\|workshop-setup' docs/ STATUS.md CLAUDE.md | grep -v 'docs/superpowers/'`
Expected: no matches (the spec/plan under `docs/superpowers/` legitimately still name the old binaries as historical context and are excluded).

- [ ] **Step 6: Commit**

```bash
git add STATUS.md CLAUDE.md docs/ && git commit -m "Reconcile docs with the unified workshop command (M15)"
```

---

## Self-Review Notes

- **Spec coverage:** dispatch (Task 6) · backend extraction (Task 1) · setup (Task 3) · compile (Task 4) · run export (Task 5) · service ExecStart (Task 2) · Dagger one-binary + base-image ENTRYPOINT + release (Task 7) · devcontainer/scripts (Task 8) · docs/STATUS/CLAUDE (Task 9). All spec section-4 doc files enumerated in Task 9.
- **Interface consistency:** `RunContainer()`, `RunStandalone(StandaloneOptions) error`, `RunThrough(string,string) error`, `compilecli.Run(string,string,io.Writer) error`, `clicmd.RunCommand() *cobra.Command`, `newRootCmd() *cobra.Command` — used identically in the dispatcher (Task 6) as produced in Tasks 1/3/4/5.
- **Test reachability:** Task 6 adds `./cmd/...` to the Dagger `Test` exec so the dispatch test runs under `make test`; setup/compile tests already fall under `./backend/...` and `./pkg/workshop/...`.
- **Ordering invariant:** `make test` (`go build ./...` + package tests) stays green after every task; the only temporary breakage is `dagger call build-workshop` between Tasks 4 and 7, called out in Task 4.
