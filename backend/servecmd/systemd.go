// Package servecmd implements the `workshop-backend service` subcommand —
// generating and installing a systemd unit that freezes a standalone-mode
// invocation — plus small helpers shared with the standalone serve path.
package servecmd

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ServeConfig is the frozen `--serve` invocation baked into the unit.
type ServeConfig struct {
	ServeDir         string
	Listen           string
	AuthUser         string
	AuthPasswordFile string
}

const systemUnitPath = "/etc/systemd/system/workshop.service"

// RunService dispatches `service install [flags]` and `service uninstall`.
func RunService(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workshop-backend service install|uninstall [flags]")
	}
	switch args[0] {
	case "install":
		return runInstall(args[1:])
	case "uninstall":
		return runUninstall(args[1:])
	default:
		return fmt.Errorf("unknown service subcommand %q (want install or uninstall)", args[0])
	}
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("service install", flag.ContinueOnError)
	var cfg ServeConfig
	var userUnit, printOnly bool
	fs.StringVar(&cfg.ServeDir, "serve", "", "workshop source directory to serve")
	fs.StringVar(&cfg.Listen, "listen", "127.0.0.1:8080", "listen address")
	fs.StringVar(&cfg.AuthUser, "auth-user", "", "basic auth username")
	fs.StringVar(&cfg.AuthPasswordFile, "auth-password-file", "", "file containing the basic auth password")
	fs.BoolVar(&userUnit, "user", false, "install a user unit (~/.config/systemd/user/) instead of a system unit")
	fs.BoolVar(&printOnly, "print", false, "print the generated unit to stdout instead of installing it")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate at install time, not at 3am on a Restart=on-failure loop.
	if cfg.ServeDir == "" {
		return fmt.Errorf("service install: --serve is required")
	}
	serveDir, err := filepath.Abs(cfg.ServeDir)
	if err != nil {
		return err
	}
	cfg.ServeDir = serveDir
	if _, err := os.Stat(filepath.Join(serveDir, "workshop.yaml")); err != nil {
		return fmt.Errorf("service install: %s does not contain workshop.yaml", serveDir)
	}
	if cfg.AuthPasswordFile != "" {
		passFile, err := filepath.Abs(cfg.AuthPasswordFile)
		if err != nil {
			return err
		}
		cfg.AuthPasswordFile = passFile
		if _, err := ReadPasswordFile(passFile); err != nil {
			return fmt.Errorf("service install: %w", err)
		}
	}
	// A persistent unattended service does not get the ad-hoc path's leeway:
	// network exposure without auth is refused outright.
	if !IsLoopback(cfg.Listen) && (cfg.AuthUser == "" || cfg.AuthPasswordFile == "") {
		return fmt.Errorf("service install: non-loopback --listen %s requires --auth-user and --auth-password-file", cfg.Listen)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving own path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving own path: %w", err)
	}

	unit := GenerateUnit(exe, cfg, serviceUser(userUnit))

	if printOnly {
		fmt.Print(unit)
		return nil
	}

	unitPath, err := unitFilePath(userUnit)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return err
	}

	fmt.Printf("Wrote %s\n\nNext steps:\n", unitPath)
	if userUnit {
		fmt.Println("  systemctl --user daemon-reload && systemctl --user enable --now workshop")
	} else {
		fmt.Println("  systemctl daemon-reload && systemctl enable --now workshop")
	}
	return nil
}

func runUninstall(args []string) error {
	fs := flag.NewFlagSet("service uninstall", flag.ContinueOnError)
	var userUnit bool
	fs.BoolVar(&userUnit, "user", false, "remove the user unit instead of the system unit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	unitPath, err := unitFilePath(userUnit)
	if err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service uninstall: %s does not exist", unitPath)
		}
		return err
	}
	fmt.Printf("Removed %s — run systemctl daemon-reload to finish.\n", unitPath)
	return nil
}

// GenerateUnit renders the systemd unit. unitUser is empty for user units
// (systemd --user runs as the owning user; User= is not allowed there).
func GenerateUnit(exePath string, cfg ServeConfig, unitUser string) string {
	exec := []string{
		unitQuote(exePath),
		"--serve", unitQuote(cfg.ServeDir),
		"--listen", unitQuote(cfg.Listen),
	}
	if cfg.AuthUser != "" {
		exec = append(exec, "--auth-user", unitQuote(cfg.AuthUser))
	}
	if cfg.AuthPasswordFile != "" {
		exec = append(exec, "--auth-password-file", unitQuote(cfg.AuthPasswordFile))
	}

	var b strings.Builder
	b.WriteString("[Unit]\n")
	fmt.Fprintf(&b, "Description=Workshop runbook (%s)\n", filepath.Base(cfg.ServeDir))
	b.WriteString("After=network.target\n\n")
	b.WriteString("[Service]\n")
	fmt.Fprintf(&b, "ExecStart=%s\n", strings.Join(exec, " "))
	b.WriteString("Restart=on-failure\n")
	if unitUser != "" {
		fmt.Fprintf(&b, "User=%s\n", unitUser)
	}
	b.WriteString("\n[Install]\n")
	if unitUser != "" {
		b.WriteString("WantedBy=multi-user.target\n")
	} else {
		b.WriteString("WantedBy=default.target\n")
	}
	return b.String()
}

// serviceUser returns the User= value for a system unit: the invoking user —
// under sudo, the user who ran sudo, never silently root. The service must
// run as a real operator account; the terminal shell, kubeconfig, and RBAC
// all depend on it. User units return "" (systemd --user owns the identity).
func serviceUser(userUnit bool) string {
	if userUnit {
		return ""
	}
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

func unitFilePath(userUnit bool) (string, error) {
	if !userUnit {
		return systemUnitPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "workshop.service"), nil
}

// unitQuote quotes an ExecStart argument when needed (systemd word splitting
// honors double quotes).
func unitQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"'") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

// IsLoopback reports whether a listen address binds only to loopback.
// An empty host (":8080") binds all interfaces.
func IsLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil || host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ReadPasswordFile reads a basic auth password from a file, trimming the
// trailing newline.
func ReadPasswordFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading password file: %w", err)
	}
	pass := strings.TrimRight(string(data), "\r\n")
	if pass == "" {
		return "", fmt.Errorf("password file %s is empty", path)
	}
	return pass, nil
}
