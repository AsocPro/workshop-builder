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
		WithExec([]string{"go", "build", "./..."}).
		WithExec([]string{"go", "test", "-v", "./pkg/workshop/...", "./backend/..."}).
		Stdout(ctx)
}

// ── Go Binary Builds ──────────────────────────────────────────────────────────

// buildGoBinary cross-compiles a Go binary from the given package path.
// All Go binary builds go through this single function — Go version, cache
// volumes, and ldflags change in one place.
func (m *WorkshopBuilder) buildGoBinary(
	src *dagger.Directory,
	pkg string, // e.g. "./cli/", "./cmd/compile-workshop/"
	outputName string,
	targetOS string,
	targetArch string,
) *dagger.File {
	if targetOS == "" {
		targetOS = "linux"
	}
	if targetArch == "" {
		targetArch = "amd64"
	}
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", targetOS).
		WithEnvVariable("GOARCH", targetArch).
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{
			"go", "build",
			"-ldflags", "-s -w",
			"-o", "/out/" + outputName,
			pkg,
		}).
		File("/out/" + outputName)
}

// buildFrontendDist builds the Svelte frontend and returns the dist directory.
func (m *WorkshopBuilder) buildFrontendDist(src *dagger.Directory) *dagger.Directory {
	return dag.Container().
		From("node:22-alpine").
		WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
		WithDirectory("/app", src.Directory("frontend")).
		WithWorkdir("/app").
		WithExec([]string{"npm", "ci"}).
		WithExec([]string{"npm", "run", "build"}).
		Directory("/app/dist")
}

// BuildBackend builds the frontend then cross-compiles the backend binary
// (no CGO) with frontend assets embedded.
func (m *WorkshopBuilder) BuildBackend(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// Target OS (default: linux)
	// +optional
	// +default="linux"
	targetOS string,
	// Target arch (default: amd64)
	// +optional
	// +default="amd64"
	targetArch string,
) *dagger.File {
	srcWithDist := src.WithDirectory("backend/frontend/dist", m.buildFrontendDist(src))
	return m.buildGoBinary(srcWithDist, "./backend/", "workshop-backend", targetOS, targetArch)
}

// BuildCompileWorkshop cross-compiles the compile-workshop binary.
func (m *WorkshopBuilder) BuildCompileWorkshop(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// Target OS (default: linux)
	// +optional
	// +default="linux"
	targetOS string,
	// Target arch (default: amd64)
	// +optional
	// +default="amd64"
	targetArch string,
) *dagger.File {
	return m.buildGoBinary(src, "./cmd/compile-workshop/", "compile-workshop", targetOS, targetArch)
}

// BuildSetup cross-compiles the workshop-setup binary (in-place step setup,
// shared logic with the backend's activate handler).
func (m *WorkshopBuilder) BuildSetup(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// Target OS (default: linux)
	// +optional
	// +default="linux"
	targetOS string,
	// Target arch (default: amd64)
	// +optional
	// +default="amd64"
	targetArch string,
) *dagger.File {
	return m.buildGoBinary(src, "./cmd/workshop-setup/", "workshop-setup", targetOS, targetArch)
}

// ── BuildBaseImages ───────────────────────────────────────────────────────────

// BuildBaseImages builds workshop-base:{ubuntu,rocky,debian} and returns a
// directory of OCI tarballs. Load into Podman with:
//
//	dagger call build-base-images --src . --output /tmp/base-images
//	podman load -i /tmp/base-images/ubuntu.tar && podman tag <sha> workshop-base:ubuntu
func (m *WorkshopBuilder) BuildBaseImages(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) (*dagger.Directory, error) {
	backendBin := m.BuildBackend(ctx, src, "", "")
	out := dag.Directory()
	for _, variant := range []string{"ubuntu", "rocky", "debian"} {
		img, err := m.buildBaseImage(ctx, src, variant, backendBin)
		if err != nil {
			return nil, fmt.Errorf("building %s: %w", variant, err)
		}
		out = out.WithFile(variant+".tar", img.AsTarball())
	}
	return out, nil
}

// PublishBaseImages builds all three base images and pushes them to ghcr.io.
// Requires a GitHub token with write:packages scope.
//
//	dagger call publish-base-images --src . --token env:GITHUB_TOKEN
func (m *WorkshopBuilder) PublishBaseImages(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// GitHub token with write:packages scope
	token *dagger.Secret,
	// Registry repo prefix (e.g. ghcr.io/asocpro/workshop-base)
	// +optional
	// +default="ghcr.io/asocpro/workshop-base"
	repo string,
) error {
	if repo == "" {
		repo = "ghcr.io/asocpro/workshop-base"
	}
	backendBin := m.BuildBackend(ctx, src, "", "")
	for _, variant := range []string{"ubuntu", "rocky", "debian"} {
		img, err := m.buildBaseImage(ctx, src, variant, backendBin)
		if err != nil {
			return fmt.Errorf("building %s: %w", variant, err)
		}
		tag := repo + ":" + variant
		fmt.Printf("Publishing %s\n", tag)
		if _, err := img.WithRegistryAuth("ghcr.io", "x-access-token", token).Publish(ctx, tag); err != nil {
			return fmt.Errorf("publishing %s: %w", tag, err)
		}
	}
	return nil
}

// buildBaseImage builds a single workshop base image variant.
func (m *WorkshopBuilder) buildBaseImage(
	ctx context.Context,
	src *dagger.Directory,
	variant string,
	backendBin *dagger.File,
) (*dagger.Container, error) {
	tini := m.downloadTini(ctx)
	goss := m.downloadGoss(ctx)
	ttyd := m.downloadTtyd(ctx)
	bashrc := src.File("backend/instrumentation/workshop-platform.bashrc")

	switch variant {
	case "ubuntu":
		return m.buildUbuntuBase(bashrc, backendBin, tini, goss, ttyd), nil
	case "rocky":
		return m.buildRockyBase(bashrc, backendBin, tini, goss, ttyd), nil
	case "debian":
		return m.buildDebianBase(bashrc, backendBin, tini, goss, ttyd), nil
	default:
		return nil, fmt.Errorf("unknown base image variant: %s", variant)
	}
}

func (m *WorkshopBuilder) buildUbuntuBase(
	bashrc *dagger.File,
	backendBin, tini, goss, ttyd *dagger.File,
) *dagger.Container {
	return dag.Container().
		From("ubuntu:24.04").
		WithExec([]string{
			"sh", "-c",
			"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " +
				"bash curl ca-certificates jq && rm -rf /var/lib/apt/lists/*",
		}).
		WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/etc/workshop-platform.bashrc", bashrc).
		WithExec([]string{"sh", "-c", `echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc`}).
		WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
		WithEntrypoint([]string{"/sbin/tini", "--"}).
		WithDefaultArgs([]string{"/usr/local/bin/workshop-backend"})
}

func (m *WorkshopBuilder) buildDebianBase(
	bashrc *dagger.File,
	backendBin, tini, goss, ttyd *dagger.File,
) *dagger.Container {
	return dag.Container().
		From("debian:bookworm-slim").
		WithExec([]string{
			"sh", "-c",
			"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " +
				"bash curl ca-certificates jq && rm -rf /var/lib/apt/lists/*",
		}).
		WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/etc/workshop-platform.bashrc", bashrc).
		WithExec([]string{"sh", "-c", `echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc`}).
		WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
		WithEntrypoint([]string{"/sbin/tini", "--"}).
		WithDefaultArgs([]string{"/usr/local/bin/workshop-backend"})
}

func (m *WorkshopBuilder) buildRockyBase(
	bashrc *dagger.File,
	backendBin, tini, goss, ttyd *dagger.File,
) *dagger.Container {
	return dag.Container().
		From("rockylinux:9").
		WithExec([]string{
			"sh", "-c",
			"dnf install -y bash curl ca-certificates jq && dnf clean all",
		}).
		WithFile("/sbin/tini", tini, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/goss", goss, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/ttyd", ttyd, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/usr/local/bin/workshop-backend", backendBin, dagger.ContainerWithFileOpts{Permissions: 0755}).
		WithFile("/etc/workshop-platform.bashrc", bashrc).
		WithExec([]string{"sh", "-c", `echo 'source /etc/workshop-platform.bashrc' >> /etc/bashrc`}).
		WithExec([]string{"mkdir", "-p", "/workshop/runtime"}).
		WithEntrypoint([]string{"/sbin/tini", "--"}).
		WithDefaultArgs([]string{"/usr/local/bin/workshop-backend"})
}

// ── BuildWorkshop ─────────────────────────────────────────────────────────────

// BuildWorkshop builds all step OCI images for a workshop.
// Returns a directory of OCI tarballs: one per step, named "<step-id>.tar".
// workshopPath is relative to src (e.g. "examples/hello-linux").
//
// Known workshop-base variants (workshop-base:{ubuntu,rocky,debian} and their
// ghcr.io equivalents) are built inline — no local registry required.
// Custom base images are pulled from the registry via FROM.
func (m *WorkshopBuilder) BuildWorkshop(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// Path to workshop directory relative to src root (e.g. "examples/hello-linux")
	workshopPath string,
) (*dagger.Directory, error) {
	compileOut, err := m.runCompileWorkshop(ctx, src, workshopPath)
	if err != nil {
		return nil, fmt.Errorf("compile workshop: %w", err)
	}

	// Build backend binary (includes embedded frontend)
	backendBin := m.BuildBackend(ctx, src, "", "")

	// Resolve base container for this workshop
	base, err := m.resolveBaseContainer(ctx, src, compileOut.BaseImage, backendBin)
	if err != nil {
		return nil, fmt.Errorf("resolving base %q: %w", compileOut.BaseImage, err)
	}

	// Bake /workshop/ metadata (ALL steps) into the base — done once
	base = m.bakeWorkshopMetadata(ctx, base, src, workshopPath, compileOut)

	// Build step images sequentially, each layering on the previous
	out := dag.Directory()
	prev := base
	for i, step := range compileOut.Steps {
		fmt.Printf("Building step %d/%d: %s\n", i+1, len(compileOut.Steps), step.ID)
		img := m.buildStepImage(src, workshopPath, step, prev)
		out = out.WithFile(step.ID+".tar", img.AsTarball())
		prev = img
	}
	return out, nil
}

// resolveBaseContainer returns the base container for a given base image reference.
// Known workshop-base variants are built inline (no registry pull needed).
// Other references are pulled from the registry via FROM.
func (m *WorkshopBuilder) resolveBaseContainer(
	ctx context.Context,
	src *dagger.Directory,
	baseImage string,
	backendBin *dagger.File,
) (*dagger.Container, error) {
	known := map[string]string{
		"workshop-base:ubuntu":                  "ubuntu",
		"workshop-base:rocky":                   "rocky",
		"workshop-base:debian":                  "debian",
		"ghcr.io/asocpro/workshop-base:ubuntu": "ubuntu",
		"ghcr.io/asocpro/workshop-base:rocky":  "rocky",
		"ghcr.io/asocpro/workshop-base:debian": "debian",
	}
	if variant, ok := known[baseImage]; ok {
		return m.buildBaseImage(ctx, src, variant, backendBin)
	}
	// Custom base image — pull from registry
	return dag.Container().From(baseImage), nil
}

// ── Internal types ────────────────────────────────────────────────────────────

type compileOutput struct {
	WorkshopJSON  string       `json:"workshopJson"`
	WorkshopImage string       // extracted from workshopJson after parsing
	BaseImage     string       `json:"baseImage"`
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
	src *dagger.Directory,
	workshopPath string,
	step stepOutput,
	base *dagger.Container,
) *dagger.Container {
	ctr := base

	// Apply this step's file mappings
	if len(step.Files) > 0 {
		stepSrcDir := src.Directory(workshopPath + "/steps/" + step.ID + "/files")
		for _, fm := range step.Files {
			perms := 0644
			if fm.Mode != "" {
				fmt.Sscanf(fm.Mode, "%o", &perms)
			}
			ctr = ctr.WithFile(fm.Target, stepSrcDir.File(fm.Source), dagger.ContainerWithFileOpts{Permissions: perms})
		}
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

// ── BuildCLI ──────────────────────────────────────────────────────────────────

// BuildCLI cross-compiles the workshop CLI binary for the host platform.
// Usage: dagger call build-cli --src . -o ./workshop && chmod +x ./workshop
func (m *WorkshopBuilder) BuildCLI(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	// Target OS (default: linux)
	// +optional
	// +default="linux"
	targetOS string,
	// Target arch (default: amd64)
	// +optional
	// +default="amd64"
	targetArch string,
) *dagger.File {
	if targetOS == "" {
		targetOS = "linux"
	}
	if targetArch == "" {
		targetArch = "amd64"
	}
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", targetOS).
		WithEnvVariable("GOARCH", targetArch).
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{
			"go", "build",
			"-ldflags", "-s -w",
			"-o", "/out/workshop",
			"./cli/",
		}).
		File("/out/workshop")
}

// ── GoModTidy ─────────────────────────────────────────────────────────────────

// GoModTidy runs go mod tidy and returns a directory containing the updated
// go.mod and go.sum. Use this after adding new dependencies:
//
//	dagger call go-mod-tidy --src . -o .
func (m *WorkshopBuilder) GoModTidy(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.Directory {
	tidied := dag.Container().
		From("golang:1.24-alpine").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "tidy"}).
		Directory("/src")
	return dag.Directory().
		WithFile("go.mod", tidied.File("go.mod")).
		WithFile("go.sum", tidied.File("go.sum"))
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
// Vendored tool versions are pinned here and nowhere else. The base image
// pipeline and BuildRelease both go through these functions.

func (m *WorkshopBuilder) downloadTini(_ context.Context) *dagger.File {
	return dag.HTTP("https://github.com/krallin/tini/releases/download/v0.19.0/tini-amd64")
}

func (m *WorkshopBuilder) downloadGoss(_ context.Context) *dagger.File {
	return m.downloadGossArch("amd64")
}

func (m *WorkshopBuilder) downloadTtyd(_ context.Context) *dagger.File {
	return m.downloadTtydArch("amd64")
}

func (m *WorkshopBuilder) downloadGossArch(arch string) *dagger.File {
	return dag.HTTP("https://github.com/goss-org/goss/releases/download/v0.4.9/goss-linux-" + arch)
}

func (m *WorkshopBuilder) downloadTtydArch(arch string) *dagger.File {
	ttydArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
	return dag.HTTP("https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd." + ttydArch)
}

// ── Release ───────────────────────────────────────────────────────────────────

// BuildRelease builds all release assets for both architectures: our Go
// binaries, vendored tools (ttyd, goss), the canonical bashrc, and the
// standalone installer script. The CI release workflow is a thin caller —
// `dagger call build-release` → upload to a GitHub release. One version tag
// pins everything.
//
//	dagger call build-release --src . -o /tmp/release
func (m *WorkshopBuilder) BuildRelease(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.Directory {
	out := dag.Directory()

	for _, arch := range []string{"amd64", "arm64"} {
		out = out.
			WithFile("workshop-backend-linux-"+arch, m.BuildBackend(ctx, src, "linux", arch)).
			WithFile("compile-workshop-linux-"+arch, m.BuildCompileWorkshop(ctx, src, "linux", arch)).
			WithFile("workshop-setup-linux-"+arch, m.BuildSetup(ctx, src, "linux", arch)).
			WithFile("workshop-cli-linux-"+arch, m.BuildCLI(ctx, src, "linux", arch)).
			WithFile("ttyd-linux-"+arch, m.downloadTtydArch(arch)).
			WithFile("goss-linux-"+arch, m.downloadGossArch(arch))
	}

	return out.
		WithFile("workshop-platform.bashrc", src.File("backend/instrumentation/workshop-platform.bashrc")).
		WithFile("install-standalone.sh", src.File("scripts/install-standalone.sh"))
}

// BuildFeature tarballs the devcontainer feature directory in the format
// expected by the devcontainers spec (devcontainer-feature.json + install.sh
// at the tarball root).
//
//	dagger call build-feature --src . -o /tmp/workshop-feature.tgz
func (m *WorkshopBuilder) BuildFeature(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.File {
	return dag.Container().
		From("alpine:3.21").
		WithDirectory("/feature", src.Directory("devcontainer-feature/src/workshop")).
		WithWorkdir("/feature").
		WithExec([]string{"mkdir", "-p", "/out"}).
		WithExec([]string{"tar", "czf", "/out/workshop-feature.tgz", "."}).
		File("/out/workshop-feature.tgz")
}
