package process

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// recorderScript wraps the shell in asciinema. It runs per ttyd client
// connection (ttyd re-execs its `--` command for each client), so the
// timestamp is generated at connect time and each session gets its own file.
// If asciinema is not installed (possible in standalone mode), it degrades to
// running the shell directly — the terminal still works, only recording is
// skipped.
const recorderScript = `if command -v asciinema >/dev/null 2>&1; then
  ts=$(date -u +%Y%m%dT%H%M%SZ)
  exec asciinema rec --stdin "$WORKSHOP_RUNTIME_DIR/session-$ts.cast" -c "$WORKSHOP_SHELL_CMD"
else
  exec $WORKSHOP_SHELL_CMD
fi`

// TTYDManager spawns and supervises ttyd, recording each session via asciinema.
type TTYDManager struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	port       int
	runtimeDir string
	shellCmd   string
	baseEnv    []string
	running    bool
}

// NewTTYDManager creates a manager for ttyd on the given port with the default
// shell (bash login shell — container modes, where instrumentation is sourced
// globally via /etc/bash.bashrc). runtimeDir is where session-<ts>.cast files
// are written.
func NewTTYDManager(port int, runtimeDir string) *TTYDManager {
	return NewTTYDManagerWithShell(port, runtimeDir, nil, nil)
}

// NewTTYDManagerWithShell creates a manager with an explicit shell argv and
// extra environment variables. Standalone mode uses this to scope
// instrumentation to workshop sessions only:
//
//	bash --rcfile <root>/workshop-rcfile -i    (with WORKSHOP_ROOT set)
func NewTTYDManagerWithShell(port int, runtimeDir string, shell []string, extraEnv map[string]string) *TTYDManager {
	if len(shell) == 0 {
		shell = []string{"/bin/bash", "--login"}
	}
	env := os.Environ()
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	return &TTYDManager{
		port:       port,
		runtimeDir: runtimeDir,
		shellCmd:   strings.Join(shell, " "),
		baseEnv:    env,
	}
}

// args builds the ttyd argv. ttyd runs `sh -c <recorderScript>` per client;
// the recorder wraps the real shell in asciinema.
func (m *TTYDManager) args() []string {
	return []string{
		"--port", fmt.Sprintf("%d", m.port),
		"--interface", "127.0.0.1", // bind to localhost only; backend proxies externally
		"--base-path", "/ttyd",
		"--writable", // allow input from browser
		"--",
		"sh", "-c", recorderScript,
	}
}

// env is the child environment: the base environment plus the recorder inputs.
func (m *TTYDManager) env() []string {
	return append(m.baseEnv,
		"WORKSHOP_RUNTIME_DIR="+m.runtimeDir,
		"WORKSHOP_SHELL_CMD="+m.shellCmd,
	)
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
	cmd := exec.Command("ttyd", m.args()...)
	cmd.Env = m.env()
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
