package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/asocpro/workshop-builder/backend/process"
	"github.com/asocpro/workshop-builder/backend/servecmd"
	"github.com/asocpro/workshop-builder/backend/store"
)

func main() {
	// `workshop-backend service install|uninstall` — systemd self-install.
	if len(os.Args) > 1 && os.Args[1] == "service" {
		if err := servecmd.RunService(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	var opts standaloneOptions
	flag.StringVar(&opts.ServeDir, "serve", "", "compile and serve a workshop source directory (standalone mode)")
	flag.StringVar(&opts.Listen, "listen", "127.0.0.1:8080", "listen address (standalone mode)")
	flag.StringVar(&opts.AuthUser, "auth-user", "", "basic auth username (standalone mode)")
	flag.StringVar(&opts.AuthPasswordFile, "auth-password-file", "", "file containing the basic auth password (standalone mode; alternative: WORKSHOP_AUTH_PASSWORD env)")
	flag.Parse()

	if opts.ServeDir != "" {
		if err := runStandalone(opts); err != nil {
			log.Fatal(err)
		}
		return
	}
	if opts.AuthUser != "" || opts.AuthPasswordFile != "" {
		log.Fatal("--auth-user/--auth-password-file require --serve (standalone mode)")
	}
	runContainer()
}

// runContainer is the container-mode entrypoint (Docker local mode, cluster
// mode, devcontainer mode) — configured entirely by environment variables.
func runContainer() {
	workshopRoot := os.Getenv("WORKSHOP_ROOT")
	if workshopRoot == "" {
		workshopRoot = "/workshop"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	managementURL := os.Getenv("WORKSHOP_MANAGEMENT_URL")
	mode := os.Getenv("WORKSHOP_MODE")
	if mode == "" {
		mode = "container"
	}

	// Load metadata from flat files
	meta, err := store.LoadMetadata(workshopRoot)
	if err != nil {
		log.Fatalf("loading workshop metadata: %v", err)
	}

	// Initialize in-memory state
	st := store.NewState(meta)

	// Ensure runtime dir exists for state events
	runtimeDir := filepath.Join(workshopRoot, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		log.Printf("warning: could not create runtime dir: %v", err)
	}

	// Start command log watcher
	commandLogPath := filepath.Join(workshopRoot, "runtime", "command-log.jsonl")
	cmdLog := store.NewCommandLog(commandLogPath)
	cmdLog.Start()

	// Spawn ttyd (terminal)
	ttydMgr := process.NewTTYDManager(7681)
	ttydMgr.Start()

	// Create and start HTTP server
	srv := NewServer(meta, st, managementURL, cmdLog, mode)

	addr := ":" + port
	fmt.Printf("Workshop backend listening on %s\n", addr)
	fmt.Printf("Workshop: %s (%s navigation, %s mode)\n", meta.Workshop.Name, meta.Workshop.Navigation, mode)
	if managementURL != "" {
		fmt.Printf("Management URL: %s\n", managementURL)
	}

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
