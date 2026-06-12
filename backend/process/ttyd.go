package process

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// TTYDManager spawns and supervises ttyd.
type TTYDManager struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	port    int
	shell   []string
	env     []string
	running bool
}

// NewTTYDManager creates a manager for ttyd on the given port with the
// default shell (bash login shell — container modes, where instrumentation
// is sourced globally via /etc/bash.bashrc).
func NewTTYDManager(port int) *TTYDManager {
	return NewTTYDManagerWithShell(port, nil, nil)
}

// NewTTYDManagerWithShell creates a manager with an explicit shell argv and
// extra environment variables for the spawned shell. Standalone mode uses
// this to scope instrumentation to workshop sessions only:
//
//	bash --rcfile <root>/workshop-rcfile -i    (with WORKSHOP_ROOT set)
func NewTTYDManagerWithShell(port int, shell []string, extraEnv map[string]string) *TTYDManager {
	if len(shell) == 0 {
		shell = []string{"/bin/bash", "--login"}
	}
	env := os.Environ()
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	return &TTYDManager{port: port, shell: shell, env: env}
}

// Start spawns ttyd and supervises it (restarts on exit).
func (m *TTYDManager) Start() {
	go m.supervise()
}

func (m *TTYDManager) supervise() {
	for {
		if err := m.spawn(); err != nil {
			log.Printf("ttyd exited: %v — restarting in 2s", err)
		}
		time.Sleep(2 * time.Second)
	}
}

func (m *TTYDManager) spawn() error {
	m.mu.Lock()
	args := []string{
		"--port", fmt.Sprintf("%d", m.port),
		"--interface", "127.0.0.1", // bind to localhost only; backend proxies externally
		"--base-path", "/ttyd",
		"--writable", // allow input from browser
		"--",
	}
	args = append(args, m.shell...)
	cmd := exec.Command("ttyd", args...)
	cmd.Env = m.env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	m.cmd = cmd
	m.running = true
	m.mu.Unlock()

	err := cmd.Run()

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	return err
}
