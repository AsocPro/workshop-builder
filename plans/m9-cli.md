# M9 — CLI (`workshop run`)

## Goal

`workshop run localhost/hello-linux:step-1-intro` starts a management server, runs the workshop container, handles step transitions. Full end-to-end single-user Docker flow.

## Prerequisites

- M4 complete (workshop images built and pushed to localhost registry)
- M6 complete (backend embeds frontend — single binary images)
- M7 complete (terminal works)
- M8 complete (goss validation works)

## Working Directory

`/home/zach/workshop-builder`

## Constraint

The CLI binary runs **natively on the workstation** (it uses the Docker socket). It is still **compiled by Dagger** (`dagger call build-cli`). No local Go needed for compilation.

## Acceptance Test

```bash
# Build the CLI
dagger call build-cli --src . --platform linux/amd64 -o ./workshop
chmod +x ./workshop

# Run a workshop
./workshop run localhost/hello-linux:step-1-intro
# Output:
#   Workshop running at http://localhost:XXXXX
#   Management at http://localhost:YYYYY
#   Press Ctrl-C to stop.

# Open workshop URL → full SPA + terminal
# Open management URL → step list, click "Go to step-2-files"
# → container replaces, workshop URL now shows step-2-files content
# Ctrl-C → containers stopped, clean exit
```

---

## Directory Structure

```
cli/
  main.go
  cmd/
    root.go
    run.go
  management/
    server.go
    handlers.go
  docker/
    client.go
```

---

## Go Dependencies (add to root `go.mod`)

```
github.com/spf13/cobra          v1.8.x
github.com/docker/docker        v27.x.x (check pkg.go.dev for latest)
github.com/docker/distribution  v2.8.x
```

Docker client import path (Go module):
```go
import "github.com/docker/docker/client"
import "github.com/docker/docker/api/types"
import "github.com/docker/docker/api/types/container"
import "github.com/docker/docker/api/types/image"
```

---

## `cli/main.go`

```go
package main

import (
    "github.com/asocpro/workshop-builder/cli/cmd"
)

func main() {
    cmd.Execute()
}
```

---

## `cli/cmd/root.go`

```go
package cmd

import (
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "workshop",
    Short: "Workshop platform CLI",
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

func init() {
    rootCmd.AddCommand(runCmd)
}
```

---

## `cli/cmd/run.go`

The core `workshop run` command.

```go
package cmd

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"

    "github.com/spf13/cobra"
    "github.com/asocpro/workshop-builder/cli/docker"
    "github.com/asocpro/workshop-builder/cli/management"
)

var runCmd = &cobra.Command{
    Use:   "run <image>",
    Short: "Run a workshop locally",
    Args:  cobra.ExactArgs(1),
    RunE:  runWorkshop,
}

func runWorkshop(cmd *cobra.Command, args []string) error {
    ctx := context.Background()
    image := args[0]

    // Create Docker client
    dc, err := docker.NewClient()
    if err != nil {
        return fmt.Errorf("connecting to Docker: %w", err)
    }
    defer dc.Close()

    // Find free ports
    workshopPort, err := freePort()
    if err != nil {
        return fmt.Errorf("finding free port for workshop: %w", err)
    }
    mgmtPort, err := freePort()
    if err != nil {
        return fmt.Errorf("finding free port for management: %w", err)
    }

    workshopURL := fmt.Sprintf("http://localhost:%d", workshopPort)
    mgmtURL := fmt.Sprintf("http://localhost:%d", mgmtPort)

    // Start management server (host-side, survives container replacements)
    mgmtSrv, err := management.NewServer(mgmtPort, dc, image)
    if err != nil {
        return fmt.Errorf("creating management server: %w", err)
    }
    mgmtSrv.Start()

    // Run the workshop container
    containerID, err := dc.RunContainer(ctx, docker.RunOptions{
        Image:         image,
        Name:          docker.GenerateName("workshop-workspace"),
        WorkshopPort:  workshopPort,
        ManagementURL: mgmtURL,
    })
    if err != nil {
        return fmt.Errorf("starting workshop container: %w", err)
    }
    mgmtSrv.SetCurrentContainer(containerID)

    fmt.Printf("\nWorkshop running at %s\n", workshopURL)
    fmt.Printf("Management at %s\n", mgmtURL)
    fmt.Println("Press Ctrl-C to stop.")

    // Wait for Ctrl-C
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    fmt.Println("\nStopping…")

    // Stop the container
    currentID := mgmtSrv.CurrentContainer()
    if currentID != "" {
        if err := dc.StopContainer(ctx, currentID); err != nil {
            log.Printf("warning: stopping container %s: %v", currentID, err)
        }
    }

    return nil
}

// freePort finds an available TCP port.
func freePort() (int, error) {
    l, err := net.Listen("tcp", ":0")
    if err != nil {
        return 0, err
    }
    defer l.Close()
    return l.Addr().(*net.TCPAddr).Port, nil
}
```

---

## `cli/docker/client.go`

Docker SDK wrapper.

```go
package docker

import (
    "context"
    "fmt"
    "math/rand"
    "time"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/image"
    dockerclient "github.com/docker/docker/client"
    "github.com/docker/go-connections/nat"
)

// Client wraps the Docker SDK client.
type Client struct {
    dc *dockerclient.Client
}

// NewClient creates a Docker client using the environment (DOCKER_HOST, etc.)
func NewClient() (*Client, error) {
    dc, err := dockerclient.NewClientWithOpts(
        dockerclient.FromEnv,
        dockerclient.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, err
    }
    return &Client{dc: dc}, nil
}

func (c *Client) Close() error {
    return c.dc.Close()
}

// RunOptions configures a workshop container.
type RunOptions struct {
    Image         string
    Name          string
    WorkshopPort  int
    ManagementURL string
}

// RunContainer starts a workshop container and returns its ID.
func (c *Client) RunContainer(ctx context.Context, opts RunOptions) (string, error) {
    portStr := fmt.Sprintf("%d", opts.WorkshopPort)
    hostPort := nat.Port("8080/tcp")

    resp, err := c.dc.ContainerCreate(ctx,
        &container.Config{
            Image: opts.Image,
            Env: []string{
                fmt.Sprintf("WORKSHOP_MANAGEMENT_URL=%s", opts.ManagementURL),
            },
        },
        &container.HostConfig{
            PortBindings: nat.PortMap{
                hostPort: []nat.PortBinding{
                    {HostIP: "0.0.0.0", HostPort: portStr},
                },
            },
            AutoRemove: false,
        },
        nil,
        nil,
        opts.Name,
    )
    if err != nil {
        return "", fmt.Errorf("creating container: %w", err)
    }

    if err := c.dc.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
        return "", fmt.Errorf("starting container: %w", err)
    }

    return resp.ID, nil
}

// StopContainer stops and removes a container.
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
    timeout := 10
    if err := c.dc.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
        return fmt.Errorf("stopping container: %w", err)
    }
    if err := c.dc.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
        return fmt.Errorf("removing container: %w", err)
    }
    return nil
}

// PullImage pulls an image from a registry. Blocks until complete.
func (c *Client) PullImage(ctx context.Context, imageRef string) error {
    reader, err := c.dc.ImagePull(ctx, imageRef, image.PullOptions{})
    if err != nil {
        return fmt.Errorf("pulling image: %w", err)
    }
    defer reader.Close()
    // Drain the reader to wait for completion
    buf := make([]byte, 4096)
    for {
        _, err := reader.Read(buf)
        if err != nil {
            break
        }
    }
    return nil
}

// ReadFileFromImage runs a temporary container and reads a file from it.
func (c *Client) ReadFileFromImage(ctx context.Context, imageRef, path string) ([]byte, error) {
    // Create temporary container without starting it
    resp, err := c.dc.ContainerCreate(ctx,
        &container.Config{
            Image: imageRef,
            Cmd:   []string{"cat", path},
        },
        &container.HostConfig{AutoRemove: true},
        nil, nil, "",
    )
    if err != nil {
        return nil, fmt.Errorf("creating temp container: %w", err)
    }

    // Start, wait, collect output
    if err := c.dc.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
        return nil, fmt.Errorf("starting temp container: %w", err)
    }

    statusCh, errCh := c.dc.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
    select {
    case err := <-errCh:
        if err != nil {
            return nil, fmt.Errorf("waiting for container: %w", err)
        }
    case <-statusCh:
    }

    logs, err := c.dc.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true})
    if err != nil {
        return nil, fmt.Errorf("reading container logs: %w", err)
    }
    defer logs.Close()

    var out []byte
    buf := make([]byte, 4096)
    for {
        n, err := logs.Read(buf)
        if n > 0 {
            // Docker log format has 8-byte header per line; strip it
            // Actually use io.ReadAll with header stripping
            out = append(out, buf[:n]...)
        }
        if err != nil {
            break
        }
    }
    return out, nil
}

// GenerateName creates a container name with a random suffix.
func GenerateName(prefix string) string {
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    suffix := make([]byte, 6)
    for i := range suffix {
        suffix[i] = chars[rng.Intn(len(chars))]
    }
    return fmt.Sprintf("%s-%s", prefix, string(suffix))
}

// ContainerPort returns the host port mapped to containerPort for a running container.
func (c *Client) ContainerPort(ctx context.Context, containerID string, containerPort int) (int, error) {
    info, err := c.dc.ContainerInspect(ctx, containerID)
    if err != nil {
        return 0, err
    }
    portStr := fmt.Sprintf("%d/tcp", containerPort)
    bindings, ok := info.NetworkSettings.Ports[nat.Port(portStr)]
    if !ok || len(bindings) == 0 {
        return 0, fmt.Errorf("port %d not mapped", containerPort)
    }
    var port int
    fmt.Sscanf(bindings[0].HostPort, "%d", &port)
    return port, nil
}
```

---

## `cli/management/server.go`

The management HTTP server — runs on the host, survives container replacements.

```go
package management

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "sync"

    "github.com/asocpro/workshop-builder/cli/docker"
)

// Server is the host-side management HTTP server.
type Server struct {
    mu              sync.Mutex
    port            int
    dc              *docker.Client
    workshopImage   string  // base image name (e.g. "localhost/hello-linux")
    currentID       string  // currently running container ID
    workshopPort    int     // port the workshop container listens on
}

// NewServer creates (but does not start) the management server.
func NewServer(port int, dc *docker.Client, imageRef string) (*Server, error) {
    // Parse the base image from the step image ref
    // e.g. "localhost/hello-linux:step-1-intro" → "localhost/hello-linux"
    baseImage := imageRef
    if idx := lastColon(imageRef); idx != -1 {
        baseImage = imageRef[:idx]
    }

    return &Server{
        port:          port,
        dc:            dc,
        workshopImage: baseImage,
    }, nil
}

func lastColon(s string) int {
    for i := len(s) - 1; i >= 0; i-- {
        if s[i] == ':' {
            return i
        }
    }
    return -1
}

// SetCurrentContainer records the running container ID and its workshop port.
func (s *Server) SetCurrentContainer(containerID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.currentID = containerID
}

func (s *Server) SetWorkshopPort(port int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.workshopPort = port
}

// CurrentContainer returns the current container ID.
func (s *Server) CurrentContainer() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.currentID
}

// Start starts the management HTTP server in a goroutine.
func (s *Server) Start() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", s.handleIndex)
    mux.HandleFunc("/step/", s.handleGoToStep)
    mux.HandleFunc("/status", s.handleStatus)

    srv := &http.Server{
        Addr:    fmt.Sprintf(":%d", s.port),
        Handler: mux,
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("management server error: %v", err)
        }
    }()
}
```

---

## `cli/management/handlers.go`

```go
package management

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"

    "github.com/asocpro/workshop-builder/cli/docker"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    // Simple HTML management UI
    // TODO: read workshop.json to get step list
    fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Workshop Management</title></head>
<body>
<h1>Workshop Management</h1>
<p>Workshop Image: %s</p>
<p>Current Container: %s</p>
<hr>
<p>POST to /step/{step-id} to switch to a step.</p>
<hr>
<p><a href="/status">Status (JSON)</a></p>
</body>
</html>`, s.workshopImage, s.currentID)
}

func (s *Server) handleGoToStep(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "POST required", http.StatusMethodNotAllowed)
        return
    }

    // Extract step ID from path: /step/{id}
    stepID := strings.TrimPrefix(r.URL.Path, "/step/")
    if stepID == "" {
        http.Error(w, "step ID required", http.StatusBadRequest)
        return
    }

    newImage := fmt.Sprintf("%s:%s", s.workshopImage, stepID)
    ctx := context.Background()

    // Get current container info before stopping
    s.mu.Lock()
    oldID := s.currentID
    workshopPort := s.workshopPort
    s.mu.Unlock()

    log.Printf("Transitioning to step %s (image: %s)", stepID, newImage)

    // Stop current container
    if oldID != "" {
        log.Printf("Stopping container %s", oldID)
        if err := s.dc.StopContainer(ctx, oldID); err != nil {
            log.Printf("warning: stopping old container: %v", err)
        }
    }

    // Start new container on same port
    // Need management server URL — use the management server's own URL
    mgmtURL := fmt.Sprintf("http://host.docker.internal:%d", s.port)
    // Note: on Linux, host.docker.internal may not resolve — use host IP instead
    // Simple approach: use the local network bridge IP (172.17.0.1 typically)
    // Better: pass management URL as a field set at construction time

    newID, err := s.dc.RunContainer(ctx, docker.RunOptions{
        Image:         newImage,
        Name:          docker.GenerateName("workshop-workspace"),
        WorkshopPort:  workshopPort,
        ManagementURL: mgmtURL,
    })
    if err != nil {
        http.Error(w, fmt.Sprintf("starting new container: %v", err), http.StatusInternalServerError)
        return
    }

    s.mu.Lock()
    s.currentID = newID
    s.mu.Unlock()

    log.Printf("New container running: %s", newID)

    writeJSON(w, http.StatusOK, map[string]string{
        "status":      "ok",
        "containerID": newID,
        "step":        stepID,
        "image":       newImage,
    })
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
    s.mu.Lock()
    status := map[string]any{
        "workshopImage":   s.workshopImage,
        "currentContainer": s.currentID,
        "workshopPort":   s.workshopPort,
    }
    s.mu.Unlock()
    writeJSON(w, http.StatusOK, status)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}
```

---

## Dagger: Add `BuildCLI` Function

Add to `dagger/main.go`:

```go
// BuildCLI cross-compiles the CLI binary for the host platform.
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
    return dag.Container().
        From("golang:1.24-alpine").
        WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
        WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
        WithDirectory("/src", src).
        WithWorkdir("/src").
        WithEnvVariable("CGO_ENABLED", "0").
        WithEnvVariable("GOOS", targetOS).
        WithEnvVariable("GOARCH", targetArch).
        WithExec([]string{
            "go", "build",
            "-ldflags", "-s -w",
            "-o", "/out/workshop",
            "./cli/",
        }).
        File("/out/workshop")
}
```

## Makefile update

```makefile
build-cli:
	dagger call build-cli --src . -o workshop
	chmod +x workshop
```

---

## Key Decisions

### Port allocation

Use `net.Listen("tcp", ":0")` to find free ports — done in `freePort()`. Never hardcode ports.

### Container naming

`workshop-workspace-<6-char-random>` to avoid conflicts with leftover containers.

### Management URL and host networking

When the management server runs on the host and the workshop container needs to reach it, the URL depends on the platform:
- **macOS/Windows Docker Desktop**: `host.docker.internal` resolves to the host
- **Linux with bridge networking**: Use `172.17.0.1` (the default Docker bridge gateway) or pass `--add-host host.docker.internal:host-gateway` to the container

Simpler MVP approach: store the management port and compute the URL based on `localhost` in the RunOptions — the management server passes its URL as an env var to the container. On Linux, pass `--network host` to the workshop container so `localhost` from inside the container reaches the host.

Alternatively, detect host IP:
```go
// hostIP returns the host IP reachable from containers (Linux-specific fallback)
func hostIP() string {
    ifaces, _ := net.Interfaces()
    for _, iface := range ifaces {
        if iface.Name == "docker0" {
            addrs, _ := iface.Addrs()
            if len(addrs) > 0 {
                // Extract IP from CIDR notation
                ip, _, _ := net.ParseCIDR(addrs[0].String())
                return ip.String()
            }
        }
    }
    return "172.17.0.1" // fallback
}
```

### `workshop run <step-image>` vs `workshop run <workshop-name>`

The plan says `workshop run localhost/hello-linux:step-1-intro` — takes a specific step image. The CLI reads `workshop.json` from that image to get the step list and workshop identity. This is the correct approach per docs: "The CLI reads `workshop.json` from the first step's image."

The management server shows all steps from `workshop.json`, allowing navigation to any step.

### `ReadFileFromImage` for workshop.json

Before starting the container, the CLI reads `workshop.json` from the image to populate the management UI step list. Use `ReadFileFromImage` for this.

Note: Docker's container logs have an 8-byte multiplexed stream header. Use `stdcopy.StdCopy` from `github.com/docker/docker/pkg/stdcopy` to properly demux:

```go
import "github.com/docker/docker/pkg/stdcopy"

var stdout, stderr bytes.Buffer
stdcopy.StdCopy(&stdout, &stderr, logs)
return stdout.Bytes(), nil
```
