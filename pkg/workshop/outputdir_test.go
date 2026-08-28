package workshop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompileToDir(t *testing.T) {
	out := t.TempDir()

	// Pre-existing runtime content must survive recompilation.
	runtimeDir := filepath.Join(out, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(runtimeDir, "command-log.jsonl")
	if err := os.WriteFile(logPath, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := CompileToDir("testdata/valid-with-setup", out)
	if err != nil {
		t.Fatalf("CompileToDir: %v", err)
	}
	if loaded.Manifest.Workshop.Name != "test-with-setup" {
		t.Errorf("loaded name = %q, want test-with-setup", loaded.Manifest.Workshop.Name)
	}

	for _, path := range []string{
		"workshop.json",
		"steps/step-one/meta.json",
		"steps/step-one/content.md",
		"steps/step-one/setup.json",
		"steps/step-one/stage/hello.txt",
		"steps/step-two/meta.json",
		"steps/step-two/setup.json",
	} {
		if _, err := os.Stat(filepath.Join(out, path)); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}

	// step-one setup.json carries files, commands, env
	var s1 SetupJSON
	data, err := os.ReadFile(filepath.Join(out, "steps/step-one/setup.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &s1); err != nil {
		t.Fatal(err)
	}
	if len(s1.Files) != 1 || s1.Files[0].Source != "hello.txt" || s1.Files[0].Target != "/workspace/hello.txt" {
		t.Errorf("step-one setup files = %+v", s1.Files)
	}
	if len(s1.Commands) != 1 || s1.Commands[0] != "echo one >> /tmp/setup-log" {
		t.Errorf("step-one setup commands = %+v", s1.Commands)
	}
	if s1.Env["STEP_ONE"] != "yes" {
		t.Errorf("step-one setup env = %+v", s1.Env)
	}

	// step-two setup.json is present but empty (non-nil slices/map)
	var s2 SetupJSON
	data, err = os.ReadFile(filepath.Join(out, "steps/step-two/setup.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatal(err)
	}
	if len(s2.Files) != 0 || len(s2.Commands) != 0 || len(s2.Env) != 0 {
		t.Errorf("step-two setup should be empty, got %+v", s2)
	}

	// staged file content matches source
	staged, err := os.ReadFile(filepath.Join(out, "steps/step-one/stage/hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(staged) != "hello from stage\n" {
		t.Errorf("staged content = %q", staged)
	}

	// runtime/ untouched
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("runtime file should survive recompile: %v", err)
	}

	// Recompile is idempotent and removes stale steps
	staleDir := filepath.Join(out, "steps", "step-removed")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileToDir("testdata/valid-with-setup", out); err != nil {
		t.Fatalf("recompile: %v", err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Errorf("stale step dir should be removed on recompile")
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("runtime file should survive recompile: %v", err)
	}
}

func TestCompileToDirConventionFiles(t *testing.T) {
	out := t.TempDir()
	if _, err := CompileToDir("testdata/valid-with-goss", out); err != nil {
		t.Fatalf("CompileToDir: %v", err)
	}
	for _, path := range []string{
		"steps/step-one/goss.yaml",
		"steps/step-one/hints.md",
		"steps/step-one/solve.md",
	} {
		if _, err := os.Stat(filepath.Join(out, path)); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "steps/step-two/goss.yaml")); !os.IsNotExist(err) {
		t.Errorf("step-two should have no goss.yaml")
	}
}
