// Workshop Builder Dagger module — build, test, and publish workshop images.
package main

import (
	"context"
	"dagger/workshop-builder/internal/dagger"
	"encoding/json"
	"fmt"
)

type WorkshopBuilder struct{}

// ── Test ─────────────────────────────────────────────────────────────────────

// Test runs all Go tests in the repo root module.
func (m *WorkshopBuilder) Test(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) (string, error) {
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"go", "test", "-v", "./pkg/workshop/...", "./backend/..."}).
		Stdout(ctx)
}

// ── BuildBackend ──────────────────────────────────────────────────────────────

// BuildBackend builds the frontend then cross-compiles the backend binary
// (linux/amd64, no CGO) with frontend assets embedded.
func (m *WorkshopBuilder) BuildBackend(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.File {
	// Step 1: Build frontend
	frontendDist := dag.Container().
		From("node:22-alpine").
		WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
		WithDirectory("/app", src.Directory("frontend")).
		WithWorkdir("/app").
		WithExec([]string{"npm", "ci"}).
		WithExec([]string{"npm", "run", "build"}).
		Directory("/app/dist")

	// Step 2: Inject dist/ into Go source tree at backend/frontend/dist/
	srcWithDist := src.WithDirectory("backend/frontend/dist", frontendDist)

	// Step 3: Compile backend (CGO disabled, linux/amd64)
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/src", srcWithDist).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", "amd64").
		WithExec([]string{
			"go", "build",
			"-ldflags", "-s -w",
			"-o", "/out/workshop-backend",
			"./backend/",
		}).
		File("/out/workshop-backend")
}

// ── BuildWorkshop ─────────────────────────────────────────────────────────────

// BuildWorkshop builds all step OCI images for a workshop.
// Returns a directory of OCI tarballs: one per step, named "<step-id>.tar".
// workshopPath is relative to src (e.g. "examples/hello-linux").
func (m *WorkshopBuilder) BuildWorkshop(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// Path to workshop directory relative to src root (e.g. "examples/hello-linux")
	workshopPath string,
) (*dagger.Directory, error) {
	// Step 1: Compile workshop metadata via Go container
	compileOut, err := m.runCompileWorkshop(ctx, src, workshopPath)
	if err != nil {
		return nil, fmt.Errorf("compile workshop: %w", err)
	}

	// Step 2: Build backend binary
	backendBin := m.BuildBackend(ctx, src)

	// Step 3: Download tool binaries
	tini := m.downloadTini(ctx)
	goss := m.downloadGoss(ctx)
	ttyd := m.downloadTtyd(ctx)

	// Step 4: Build step images sequentially, each layering on the previous
	out := dag.Directory()
	var prev *dagger.Container

	for i, step := range compileOut.Steps {
		fmt.Printf("Building step %d/%d: %s\n", i+1, len(compileOut.Steps), step.ID)

		img := m.buildStepImage(ctx, src, workshopPath, compileOut, step, i, backendBin, tini, goss, ttyd, prev)

		// Add OCI tarball to output directory; caller loads into Podman
		out = out.WithFile(step.ID+".tar", img.AsTarball())

		prev = img
	}
	return out, nil
}

// ── Internal types ────────────────────────────────────────────────────────────

type compileOutput struct {
	WorkshopJSON  string       `json:"workshopJson"`
	WorkshopImage string       // extracted from workshopJson after parsing
	Steps         []stepOutput `json:"steps"`
}

type workshopJSONPartial struct {
	Image string `json:"image"`
}

type stepOutput struct {
	ID         string            `json:"id"`
	MetaJSON   string            `json:"metaJson"`
	LLMJson    string            `json:"llmJson,omitempty"`
	HasGoss    bool              `json:"hasGoss"`
	HasHints   bool              `json:"hasHints"`
	HasExplain bool              `json:"hasExplain"`
	HasSolve   bool              `json:"hasSolve"`
	HasLLMDocs bool              `json:"hasLlmDocs"`
	Files      []fileMapping     `json:"files,omitempty"`
	Commands   []string          `json:"commands,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

type fileMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode,omitempty"`
}

// ── runCompileWorkshop ────────────────────────────────────────────────────────

func (m *WorkshopBuilder) runCompileWorkshop(ctx context.Context, src *dagger.Directory, workshopPath string) (*compileOutput, error) {
	out, err := dag.Container().
		From("golang:1.24-alpine").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{
			"go", "run", "./cmd/compile-workshop/",
			"--workshop", workshopPath,
		}).
		Stdout(ctx)
	if err != nil {
		return nil, err
	}

	var result compileOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parsing compile output: %w", err)
	}

	// Extract image name from workshop.json
	var wj workshopJSONPartial
	if err := json.Unmarshal([]byte(result.WorkshopJSON), &wj); err != nil {
		return nil, fmt.Errorf("parsing workshop.json image: %w", err)
	}
	result.WorkshopImage = wj.Image

	return &result, nil
}

// ── buildStepImage ────────────────────────────────────────────────────────────

func (m *WorkshopBuilder) buildStepImage(
	ctx context.Context,
	src *dagger.Directory,
	workshopPath string,
	compiled *compileOutput,
	step stepOutput,
	position int,
	backendBin *dagger.File,
	tini, goss, ttyd *dagger.File,
	prev *dagger.Container,
) *dagger.Container {
	var ctr *dagger.Container

	if prev == nil {
		// First step: build the full base layer
		ctr = dag.Container().From("ubuntu:24.04")

		// Install system dependencies
		ctr = ctr.WithExec([]string{
			"sh", "-c",
			"apt-get update && apt-get install -y --no-install-recommends " +
				"bash curl ca-certificates jq && rm -rf /var/lib/apt/lists/*",
		})

		// Install platform binaries
		ctr = ctr.
			WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
			WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
			WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
			WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755})

		// Shell instrumentation
		ctr = ctr.WithNewFile("/etc/workshop-platform.bashrc", bashrcContent())
		ctr = ctr.WithExec([]string{
			"sh", "-c",
			`echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc`,
		})

		// Bake /workshop/ metadata (all steps) — only done once in first image
		ctr = m.bakeWorkshopMetadata(ctx, ctr, src, workshopPath, compiled)

		// Runtime state directory
		ctr = ctr.WithExec([]string{"mkdir", "-p", "/workshop/runtime"})
	} else {
		ctr = prev
	}

	// Apply this step's file mappings
	stepSrcDir := src.Directory(workshopPath + "/steps/" + step.ID + "/files")
	for _, fm := range step.Files {
		perms := 0644
		if fm.Mode != "" {
			fmt.Sscanf(fm.Mode, "%o", &perms)
		}
		ctr = ctr.WithFile(fm.Target, stepSrcDir.File(fm.Source), dagger.ContainerWithFileOpts{Permissions: perms})
	}

	// Run step setup commands
	for _, cmd := range step.Commands {
		ctr = ctr.WithExec([]string{"sh", "-c", cmd})
	}

	// Set step environment variables
	for k, v := range step.Env {
		ctr = ctr.WithEnvVariable(k, v)
	}

	// Entrypoint: tini wraps the backend; CMD is the backend so callers can
	// override it (e.g. "cat /file") while still going through tini.
	ctr = ctr.
		WithEntrypoint([]string{"/sbin/tini", "--"}).
		WithDefaultArgs([]string{"/usr/local/bin/workshop-backend"})

	return ctr
}

// ── bakeWorkshopMetadata ──────────────────────────────────────────────────────

func (m *WorkshopBuilder) bakeWorkshopMetadata(
	ctx context.Context,
	ctr *dagger.Container,
	src *dagger.Directory,
	workshopPath string,
	compiled *compileOutput,
) *dagger.Container {
	// workshop.json
	ctr = ctr.WithNewFile("/workshop/workshop.json", compiled.WorkshopJSON)

	for _, step := range compiled.Steps {
		stepBase := "/workshop/steps/" + step.ID + "/"
		stepSrc := src.Directory(workshopPath + "/steps/" + step.ID + "/")

		ctr = ctr.WithNewFile(stepBase+"meta.json", step.MetaJSON)

		if step.LLMJson != "" {
			ctr = ctr.WithNewFile(stepBase+"llm.json", step.LLMJson)
		}

		ctr = ctr.WithFile(stepBase+"content.md", stepSrc.File("content.md"))

		if step.HasGoss {
			ctr = ctr.WithFile(stepBase+"goss.yaml", stepSrc.File("goss.yaml"))
		}
		if step.HasHints {
			ctr = ctr.WithFile(stepBase+"hints.md", stepSrc.File("hints.md"))
		}
		if step.HasExplain {
			ctr = ctr.WithFile(stepBase+"explain.md", stepSrc.File("explain.md"))
		}
		if step.HasSolve {
			ctr = ctr.WithFile(stepBase+"solve.md", stepSrc.File("solve.md"))
		}
		if step.HasLLMDocs {
			ctr = ctr.WithDirectory(stepBase+"llm-docs/", stepSrc.Directory("llm-docs"))
		}
	}

	return ctr
}

// ── bashrcContent ─────────────────────────────────────────────────────────────

func bashrcContent() string {
	return `# Workshop Platform Shell Instrumentation

__workshop_log_command() {
    local exit_code=$?
    local cmd
    cmd=$(history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
    if [ -n "$cmd" ]; then
        local ts
        ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
        jq -n --arg ts "$ts" --arg cmd "$cmd" --argjson exit "$exit_code" \
            '{"ts":$ts,"cmd":$cmd,"exit":$exit}' \
            >> /workshop/runtime/command-log.jsonl 2>/dev/null || true
    fi
}

PROMPT_COMMAND="__workshop_log_command${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
`
}

// ── RunBackend ────────────────────────────────────────────────────────────────

// RunBackend starts a workshop step image as a backend service.
// image is an OCI tarball produced by BuildWorkshop (e.g. ./dist/step-1-intro.tar).
func (m *WorkshopBuilder) RunBackend(
	// OCI tarball of the workshop step image (e.g. ./dist/step-1-intro.tar)
	image *dagger.File,
	// +optional
	managementURL string,
) *dagger.Service {
	ctr := dag.Container().
		Import(image).
		WithExposedPort(8080)
	if managementURL != "" {
		ctr = ctr.WithEnvVariable("WORKSHOP_MANAGEMENT_URL", managementURL)
	}
	return ctr.AsService()
}

// ── Dev ───────────────────────────────────────────────────────────────────────

// viteDevContainer builds a node container with deps installed, ready to run
// the Vite dev server. Deps are cached on package.json + lock only — source
// changes don't invalidate npm ci, so restarts skip the install step entirely.
// node_modules never appear on the host.
func (m *WorkshopBuilder) viteDevContainer(frontend *dagger.Directory) *dagger.Container {
	withDeps := dag.Container().
		From("node:22-alpine").
		WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
		WithFile("/app/package.json", frontend.File("package.json")).
		WithFile("/app/package-lock.json", frontend.File("package-lock.json")).
		WithWorkdir("/app").
		WithExec([]string{"npm", "ci"})

	// Overlay full source on top — node_modules from the step above are preserved
	return withDeps.WithDirectory("/app", frontend)
}

// ── Dev ───────────────────────────────────────────────────────────────────────

// Dev starts the backend and frontend dev server together, wired via service binding.
// Usage: dagger call dev --image ./dist/step-1-intro.tar up --ports 5173:5173
func (m *WorkshopBuilder) Dev(
	ctx context.Context,
	// OCI tarball of the workshop step image (e.g. ./dist/step-1-intro.tar)
	image *dagger.File,
	// +defaultPath="/frontend"
	frontend *dagger.Directory,
	// +optional
	managementURL string,
) *dagger.Service {
	backend := m.RunBackend(image, managementURL)

	return m.viteDevContainer(frontend).
		WithServiceBinding("backend", backend).
		WithEnvVariable("BACKEND_URL", "http://backend:8080").
		WithExposedPort(5173).
		AsService(dagger.ContainerAsServiceOpts{
			Args: []string{"npm", "run", "dev", "--", "--host"},
		})
}

// ── DevExample ────────────────────────────────────────────────────────────────

// DevExample builds the hello-linux example workshop and starts it with the
// frontend dev server in one command.
// Usage: dagger call dev-example up --ports 5173:5173
func (m *WorkshopBuilder) DevExample(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// +defaultPath="/frontend"
	frontend *dagger.Directory,
	// Step ID to run as the backend
	// +optional
	// +default="step-1-intro"
	stepID string,
) (*dagger.Service, error) {
	if stepID == "" {
		stepID = "step-1-intro"
	}
	built, err := m.BuildWorkshop(ctx, src, "examples/hello-linux")
	if err != nil {
		return nil, fmt.Errorf("build example workshop: %w", err)
	}
	image := built.File(stepID + ".tar")
	return m.Dev(ctx, image, frontend, ""), nil
}

// ── DevFrontend ───────────────────────────────────────────────────────────────

// DevFrontend starts a Vite dev server pointing at an existing backend URL.
// Useful when the backend is already running (e.g. podman run -p 8080:8080 ...).
// Usage: dagger call dev-frontend --backend-url http://localhost:8080 up --ports 5173:5173
func (m *WorkshopBuilder) DevFrontend(
	ctx context.Context,
	// +defaultPath="/frontend"
	frontend *dagger.Directory,
	// URL of the already-running backend
	backendURL string,
) *dagger.Service {
	return m.viteDevContainer(frontend).
		WithEnvVariable("BACKEND_URL", backendURL).
		WithExposedPort(5173).
		AsService(dagger.ContainerAsServiceOpts{
			Args: []string{"npm", "run", "dev", "--", "--host"},
		})
}

// ── FrontendLockfile ──────────────────────────────────────────────────────────

// FrontendLockfile runs npm install in the frontend directory and returns the
// generated package-lock.json. Use this to bootstrap the lockfile:
//
//	dagger call frontend-lockfile --frontend ./frontend --output frontend/package-lock.json
func (m *WorkshopBuilder) FrontendLockfile(
	ctx context.Context,
	// +defaultPath="/frontend"
	frontend *dagger.Directory,
) *dagger.File {
	return dag.Container().
		From("node:22-alpine").
		WithDirectory("/app", frontend).
		WithWorkdir("/app").
		WithExec([]string{"npm", "install"}).
		File("/app/package-lock.json")
}

// ── Tool Downloads ─────────────────────────────────────────────────────────────

func (m *WorkshopBuilder) downloadTini(_ context.Context) *dagger.File {
	return dag.HTTP("https://github.com/krallin/tini/releases/download/v0.19.0/tini-amd64")
}

func (m *WorkshopBuilder) downloadGoss(_ context.Context) *dagger.File {
	return dag.HTTP("https://github.com/goss-org/goss/releases/download/v0.4.9/goss-linux-amd64")
}

func (m *WorkshopBuilder) downloadTtyd(_ context.Context) *dagger.File {
	return dag.HTTP("https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.x86_64")
}
