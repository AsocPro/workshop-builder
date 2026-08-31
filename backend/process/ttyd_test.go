package process

import (
	"strings"
	"testing"
)

func TestArgsWrapShellInRecorder(t *testing.T) {
	m := NewTTYDManager(7681, "/workshop/runtime")
	args := m.args()

	// ttyd args must end with: -- sh -c <recorderScript>
	if len(args) < 3 {
		t.Fatalf("args too short: %v", args)
	}
	if args[len(args)-3] != "sh" || args[len(args)-2] != "-c" {
		t.Fatalf("expected shell wrapper `sh -c`, got tail %v", args[len(args)-3:])
	}
	script := args[len(args)-1]
	if !strings.Contains(script, "asciinema rec") {
		t.Errorf("recorder script missing asciinema rec: %q", script)
	}
	if !strings.Contains(script, "session-$ts.cast") {
		t.Errorf("recorder script missing per-session filename: %q", script)
	}
	if !strings.Contains(script, `exec $WORKSHOP_SHELL_CMD`) {
		t.Errorf("recorder script missing degradation fallback: %q", script)
	}
}

func TestEnvCarriesRuntimeDirAndShellCmd(t *testing.T) {
	m := NewTTYDManager(7681, "/workshop/runtime")
	env := m.env()
	want := map[string]string{
		"WORKSHOP_RUNTIME_DIR": "/workshop/runtime",
		"WORKSHOP_SHELL_CMD":   "/bin/bash --login",
	}
	for k, v := range want {
		if !hasEnv(env, k+"="+v) {
			t.Errorf("env missing %s=%s\ngot: %v", k, v, env)
		}
	}
}

func TestWithShellSetsShellCmd(t *testing.T) {
	m := NewTTYDManagerWithShell(7681, "/root/.workshop/x/runtime",
		[]string{"/bin/bash", "--rcfile", "/root/.workshop/x/workshop-rcfile", "-i"},
		map[string]string{"WORKSHOP_ROOT": "/root/.workshop/x"})
	if !hasEnv(m.env(), "WORKSHOP_SHELL_CMD=/bin/bash --rcfile /root/.workshop/x/workshop-rcfile -i") {
		t.Errorf("shell cmd not set from explicit shell; env: %v", m.env())
	}
	if !hasEnv(m.env(), "WORKSHOP_ROOT=/root/.workshop/x") {
		t.Errorf("extra env not preserved; env: %v", m.env())
	}
}

func hasEnv(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}
