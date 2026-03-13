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
	running bool
}

// NewTTYDManager creates a manager for ttyd on the given port.
func NewTTYDManager(port int) *TTYDManager {
	return &TTYDManager{port: port}
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
	cmd := exec.Command(
		"ttyd",
		"--port", fmt.Sprintf("%d", m.port),
		"--interface", "127.0.0.1", // bind to localhost only; backend proxies externally
		"--base-path", "/ttyd",
		"--writable", // allow input from browser
		"--",
		"/bin/bash",
		"--login",
	)
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
