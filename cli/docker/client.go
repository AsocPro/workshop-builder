// Package docker provides a minimal Docker API client using only stdlib.
// It speaks the Docker REST API over a Unix socket, making it compatible
// with both Docker and Podman.
package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Client talks to the Docker daemon over its Unix socket.
type Client struct {
	hc  *http.Client
	base string      // base URL, e.g. "http://docker/v1.43"
	svc *exec.Cmd    // non-nil if we started podman system service ourselves
}

// NewClient creates a Docker client. Resolution order:
//  1. DOCKER_HOST env var (unix:///path)
//  2. /var/run/docker.sock  (Docker)
//  3. $XDG_RUNTIME_DIR/podman/podman.sock  (Podman rootless, socket already running)
//  4. Auto-start `podman system service` and use its socket
func NewClient() (*Client, error) {
	socketPath, svc, err := resolveSocket()
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{
		hc:   &http.Client{Transport: tr},
		base: "http://docker/v1.43",
		svc:  svc,
	}, nil
}

// resolveSocket finds or starts a Docker-compatible API socket.
// Returns (socketPath, subprocess-or-nil, error).
func resolveSocket() (string, *exec.Cmd, error) {
	// 1. Explicit DOCKER_HOST — honour it and fail clearly if it's wrong
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		path := strings.TrimPrefix(h, "unix://")
		if !socketAlive(path) {
			return "", nil, fmt.Errorf("DOCKER_HOST socket not responding: %s", path)
		}
		return path, nil, nil
	}

	// 2. Docker socket (dockerd always keeps this running)
	if socketAlive("/var/run/docker.sock") {
		return "/var/run/docker.sock", nil, nil
	}

	// 3. Podman rootless socket (already running via systemd socket activation)
	if p := podmanSocketPath(); socketAlive(p) {
		return p, nil, nil
	}

	// 4. Auto-start podman system service — socket file may exist but be stale,
	//    so remove it first to let podman bind a fresh one.
	sockPath := podmanSocketPath()
	os.Remove(sockPath) //nolint:errcheck
	svc, err := startPodmanService(sockPath)
	if err != nil {
		return "", nil, err
	}
	return sockPath, svc, nil
}

// socketAlive returns true if the path exists and accepts a TCP connection.
// A stale socket file (process gone) will pass os.Stat but fail Dial.
func socketAlive(path string) bool {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// podmanSocketPath returns the canonical Podman rootless socket path,
// respecting XDG_RUNTIME_DIR if set.
func podmanSocketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "podman", "podman.sock")
	}
	return fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid())
}

// startPodmanService launches `podman system service` and waits for its socket
// to become available. The returned *exec.Cmd must be killed on exit (Close does this).
func startPodmanService(socketPath string) (*exec.Cmd, error) {
	log.Printf("Podman socket not found — starting podman system service at %s", socketPath)

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating podman socket dir: %w", err)
	}

	cmd := exec.Command("podman", "system", "service", "--time=0", "unix://"+socketPath)
	// Run in its own process group so Ctrl-C in the terminal doesn't send SIGINT
	// to podman — we need the socket alive until after we've stopped the container.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting podman system service: %w (is podman installed?)", err)
	}

	// Poll until socket appears (up to 5 s)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			log.Printf("Podman socket ready")
			return cmd, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	cmd.Process.Kill() //nolint:errcheck
	return nil, fmt.Errorf("timed out waiting for podman socket at %s", socketPath)
}

// Close shuts down any subprocess we started (e.g. podman system service).
func (c *Client) Close() error {
	if c.svc != nil && c.svc.Process != nil {
		c.svc.Process.Kill()  //nolint:errcheck
		c.svc.Wait()          //nolint:errcheck
	}
	return nil
}

// do sends an API request and returns the response. Caller must close the body.
func (c *Client) do(ctx context.Context, method, path string, reqBody any) (*http.Response, error) {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("docker API %s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}

// ─── Container create/start/stop/remove ───────────────────────────────────────

type createRequest struct {
	Image      string            `json:"Image"`
	Cmd        []string          `json:"Cmd,omitempty"`
	Env        []string          `json:"Env,omitempty"`
	HostConfig createHostConfig  `json:"HostConfig"`
}

type createHostConfig struct {
	PortBindings map[string][]portBinding `json:"PortBindings,omitempty"`
	ExtraHosts   []string                 `json:"ExtraHosts,omitempty"`
	AutoRemove   bool                     `json:"AutoRemove,omitempty"`
}

type portBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type createResponse struct {
	ID string `json:"Id"`
}

// RunOptions configures a workshop container.
type RunOptions struct {
	Image         string
	Name          string
	WorkshopPort  int
	ManagementURL string
}

// RunContainer starts a workshop container and returns its ID.
// It adds host.docker.internal:host-gateway to ExtraHosts so the container
// can reach the management server running on the host (required on Linux).
func (c *Client) RunContainer(ctx context.Context, opts RunOptions) (string, error) {
	portKey := "8080/tcp"
	body := createRequest{
		Image: opts.Image,
		Env: []string{
			"WORKSHOP_MANAGEMENT_URL=" + opts.ManagementURL,
		},
		HostConfig: createHostConfig{
			PortBindings: map[string][]portBinding{
				portKey: {{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", opts.WorkshopPort)}},
			},
			ExtraHosts: []string{"host.docker.internal:host-gateway"},
		},
	}

	path := "/containers/create"
	if opts.Name != "" {
		path += "?name=" + opts.Name
	}
	resp, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}
	var cr createResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("parsing create response: %w", err)
	}
	resp.Body.Close()

	startResp, err := c.do(ctx, http.MethodPost, "/containers/"+cr.ID+"/start", nil)
	if err != nil {
		return "", fmt.Errorf("starting container: %w", err)
	}
	startResp.Body.Close()
	return cr.ID, nil
}

// StopContainer stops and removes a container.
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10
	stopResp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/containers/%s/stop?t=%d", containerID, timeout), nil)
	if err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}
	stopResp.Body.Close()

	rmResp, err := c.do(ctx, http.MethodDelete, "/containers/"+containerID+"?force=true", nil)
	if err != nil {
		return fmt.Errorf("removing container: %w", err)
	}
	rmResp.Body.Close()
	return nil
}

// StepInfo is a minimal step descriptor parsed from workshop.json.
type StepInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ReadWorkshopSteps reads /workshop/workshop.json from an image and returns the step list.
func (c *Client) ReadWorkshopSteps(ctx context.Context, imageRef string) ([]StepInfo, error) {
	data, err := c.readFileFromImage(ctx, imageRef, "/workshop/workshop.json")
	if err != nil {
		return nil, err
	}
	var wj struct {
		Steps []StepInfo `json:"steps"`
	}
	if err := json.Unmarshal(data, &wj); err != nil {
		return nil, fmt.Errorf("parsing workshop.json: %w", err)
	}
	return wj.Steps, nil
}

// readFileFromImage runs a one-shot container to cat a file and returns its contents.
func (c *Client) readFileFromImage(ctx context.Context, imageRef, path string) ([]byte, error) {
	body := createRequest{
		Image:      imageRef,
		Cmd:        []string{"cat", path},
		HostConfig: createHostConfig{AutoRemove: false},
	}
	resp, err := c.do(ctx, http.MethodPost, "/containers/create", body)
	if err != nil {
		return nil, fmt.Errorf("creating temp container: %w", err)
	}
	var cr createResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("parsing create response: %w", err)
	}
	resp.Body.Close()

	// Clean up on exit
	defer func() {
		rmResp, _ := c.do(ctx, http.MethodDelete, "/containers/"+cr.ID+"?force=true", nil)
		if rmResp != nil {
			rmResp.Body.Close()
		}
	}()

	startResp, err := c.do(ctx, http.MethodPost, "/containers/"+cr.ID+"/start", nil)
	if err != nil {
		return nil, fmt.Errorf("starting temp container: %w", err)
	}
	startResp.Body.Close()

	// Wait for container to exit
	waitResp, err := c.do(ctx, http.MethodPost, "/containers/"+cr.ID+"/wait?condition=not-running", nil)
	if err != nil {
		return nil, fmt.Errorf("waiting for container: %w", err)
	}
	io.Copy(io.Discard, waitResp.Body) //nolint:errcheck
	waitResp.Body.Close()

	// Get stdout logs
	logsResp, err := c.do(ctx, http.MethodGet, "/containers/"+cr.ID+"/logs?stdout=true", nil)
	if err != nil {
		return nil, fmt.Errorf("reading container logs: %w", err)
	}
	defer logsResp.Body.Close()

	// Docker's log stream is multiplexed: each frame has an 8-byte header.
	// Byte 0: stream type (1=stdout). Bytes 4-7: frame size (big-endian uint32).
	return demuxStdout(logsResp.Body)
}

// demuxStdout reads Docker's multiplexed log stream and returns stdout bytes.
func demuxStdout(r io.Reader) ([]byte, error) {
	var out bytes.Buffer
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("reading log header: %w", err)
		}
		streamType := header[0]
		frameSize := binary.BigEndian.Uint32(header[4:8])
		frame := make([]byte, frameSize)
		if _, err := io.ReadFull(r, frame); err != nil {
			return nil, fmt.Errorf("reading log frame: %w", err)
		}
		if streamType == 1 { // stdout
			out.Write(frame)
		}
	}
	return out.Bytes(), nil
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
