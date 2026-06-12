package servecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateUnit(t *testing.T) {
	cfg := ServeConfig{
		ServeDir:         "/opt/onboarding-workshop",
		Listen:           "0.0.0.0:8080",
		AuthUser:         "admin",
		AuthPasswordFile: "/etc/workshop/pass",
	}
	unit := GenerateUnit("/usr/local/bin/workshop-backend", cfg, "platform-admin")

	for _, want := range []string{
		"Description=Workshop runbook (onboarding-workshop)",
		"ExecStart=/usr/local/bin/workshop-backend --serve /opt/onboarding-workshop --listen 0.0.0.0:8080 --auth-user admin --auth-password-file /etc/workshop/pass",
		"Restart=on-failure",
		"User=platform-admin",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit missing %q:\n%s", want, unit)
		}
	}
}

func TestGenerateUnit_UserUnit(t *testing.T) {
	cfg := ServeConfig{ServeDir: "/opt/w", Listen: "127.0.0.1:8080"}
	unit := GenerateUnit("/usr/local/bin/workshop-backend", cfg, "")

	if strings.Contains(unit, "User=") {
		t.Errorf("user unit must not contain User=:\n%s", unit)
	}
	if !strings.Contains(unit, "WantedBy=default.target") {
		t.Errorf("user unit should want default.target:\n%s", unit)
	}
	if strings.Contains(unit, "--auth-user") {
		t.Errorf("unit should omit auth flags when unset:\n%s", unit)
	}
}

func TestGenerateUnit_QuotesPaths(t *testing.T) {
	cfg := ServeConfig{ServeDir: "/opt/my workshop", Listen: "127.0.0.1:8080"}
	unit := GenerateUnit("/usr/local/bin/workshop-backend", cfg, "op")
	if !strings.Contains(unit, `--serve "/opt/my workshop"`) {
		t.Errorf("path with spaces should be quoted:\n%s", unit)
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": true,
		"localhost:8080": true,
		"[::1]:8080":     true,
		"0.0.0.0:8080":   false,
		":8080":          false,
		"10.0.0.5:8080":  false,
		"garbage":        false,
	}
	for addr, want := range cases {
		if got := IsLoopback(addr); got != want {
			t.Errorf("IsLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestReadPasswordFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pass")
	if err := os.WriteFile(path, []byte("s3cret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pass, err := ReadPasswordFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pass != "s3cret" {
		t.Errorf("pass = %q, want s3cret (trailing newline trimmed)", pass)
	}

	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPasswordFile(empty); err == nil {
		t.Error("empty password file should error")
	}
}

func TestRunInstall_RefusesNonLoopbackWithoutAuth(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "workshop.yaml"), []byte("version: v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runInstall([]string{"--serve", dir, "--listen", "0.0.0.0:8080", "--print"})
	if err == nil || !strings.Contains(err.Error(), "requires --auth-user") {
		t.Errorf("expected refusal, got %v", err)
	}
}

func TestRunInstall_RequiresWorkshopYaml(t *testing.T) {
	err := runInstall([]string{"--serve", t.TempDir(), "--print"})
	if err == nil || !strings.Contains(err.Error(), "workshop.yaml") {
		t.Errorf("expected workshop.yaml validation error, got %v", err)
	}
}
