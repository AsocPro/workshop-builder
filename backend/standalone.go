package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/asocpro/workshop-builder/backend/instrumentation"
	"github.com/asocpro/workshop-builder/backend/process"
	"github.com/asocpro/workshop-builder/backend/servecmd"
	"github.com/asocpro/workshop-builder/backend/store"
	"github.com/asocpro/workshop-builder/pkg/workshop"
)

type standaloneOptions struct {
	ServeDir         string
	Listen           string
	AuthUser         string
	AuthPasswordFile string
}

// runStandalone serves a workshop straight from a source checkout: compile to
// WORKSHOP_ROOT, scope shell instrumentation to workshop terminal sessions,
// and serve with optional basic auth. Single user per server; the server's
// state is the workshop's state.
func runStandalone(opts standaloneOptions) error {
	srcDir, err := filepath.Abs(opts.ServeDir)
	if err != nil {
		return fmt.Errorf("resolving workshop path: %w", err)
	}

	// Parse up-front for the workshop name (root resolution) and the
	// infrastructure guard — fail before touching the filesystem.
	loaded, err := workshop.Parse(srcDir)
	if err != nil {
		return err
	}
	if err := guardInfrastructure(loaded); err != nil {
		return err
	}

	root := os.Getenv("WORKSHOP_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory (set WORKSHOP_ROOT explicitly): %w", err)
		}
		root = filepath.Join(home, ".workshop", loaded.Manifest.Workshop.Name)
	}

	// Recompile on every start — `git pull && restart` is the content
	// iteration loop. CompileToDir leaves runtime/ untouched.
	if _, err := workshop.CompileToDir(srcDir, root); err != nil {
		return err
	}
	fmt.Printf("Compiled %s → %s\n", srcDir, root)

	runtimeDir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("creating runtime dir: %w", err)
	}

	rcfile, err := instrumentation.WriteRcfile(root)
	if err != nil {
		return fmt.Errorf("writing instrumentation rcfile: %w", err)
	}

	authPass, err := resolveAuthPassword(opts)
	if err != nil {
		return err
	}
	if !servecmd.IsLoopback(opts.Listen) && opts.AuthUser == "" {
		log.Printf("WARNING: listening on non-loopback address %s without authentication — anyone who can reach this port gets a shell on this server. Use --auth-user/--auth-password-file, or keep the default loopback listen address and connect via SSH port-forward.", opts.Listen)
	}

	meta, err := store.LoadMetadata(root)
	if err != nil {
		return fmt.Errorf("loading compiled metadata: %w", err)
	}
	st := store.NewState(meta)

	cmdLog := store.NewCommandLog(filepath.Join(runtimeDir, "command-log.jsonl"))
	cmdLog.Start()

	// Instrumentation is scoped: only this shell sources the logging hook.
	ttydMgr := process.NewTTYDManagerWithShell(7681,
		[]string{"/bin/bash", "--rcfile", rcfile, "-i"},
		map[string]string{"WORKSHOP_ROOT": root},
	)
	ttydMgr.Start()

	var handler http.Handler = NewServer(meta, st, "", cmdLog, "standalone")
	if opts.AuthUser != "" {
		handler = BasicAuth(opts.AuthUser, authPass, handler)
	}

	fmt.Printf("Workshop backend listening on %s (standalone mode)\n", opts.Listen)
	fmt.Printf("Workshop: %s (%s navigation)\n", meta.Workshop.Name, meta.Workshop.Navigation)
	return http.ListenAndServe(opts.Listen, handler)
}

// guardInfrastructure rejects workshops whose runtime requirements standalone
// mode cannot provide. The server's environment is assumed to already exist.
func guardInfrastructure(loaded *workshop.LoadedWorkshop) error {
	infra := loaded.Manifest.Infrastructure
	if infra == nil {
		return nil
	}
	if infra.Cluster != nil && infra.Cluster.Enabled {
		provider := infra.Cluster.Provider
		if provider == "" {
			provider = "kubernetes"
		}
		return fmt.Errorf("standalone mode does not provision infrastructure: this workshop requires a %s cluster; provision it on this server manually or run the workshop in container mode (workshop run)", provider)
	}
	if len(infra.ExtraContainers) > 0 {
		return fmt.Errorf("standalone mode does not provision infrastructure: this workshop requires extra containers (%s); provide them on this server manually or run the workshop in container mode (workshop run)", infra.ExtraContainers[0].Name)
	}
	return nil
}

// resolveAuthPassword loads the basic auth password from the password file or
// WORKSHOP_AUTH_PASSWORD. Passwords never come from argv — `ps` and shell
// history are shared surfaces on exactly the kind of server this runs on.
func resolveAuthPassword(opts standaloneOptions) (string, error) {
	if opts.AuthUser == "" && opts.AuthPasswordFile == "" {
		return "", nil
	}
	if opts.AuthUser == "" {
		return "", fmt.Errorf("--auth-password-file requires --auth-user")
	}
	if opts.AuthPasswordFile != "" {
		pass, err := servecmd.ReadPasswordFile(opts.AuthPasswordFile)
		if err != nil {
			return "", err
		}
		return pass, nil
	}
	if pass := os.Getenv("WORKSHOP_AUTH_PASSWORD"); pass != "" {
		return pass, nil
	}
	return "", fmt.Errorf("--auth-user requires --auth-password-file or WORKSHOP_AUTH_PASSWORD")
}
