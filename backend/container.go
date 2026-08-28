package backend

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/asocpro/workshop-builder/backend/process"
	"github.com/asocpro/workshop-builder/backend/store"
)

// RunContainer is the container-mode entrypoint (Docker local mode, cluster
// mode, devcontainer mode) — configured entirely by environment variables.
func RunContainer() {
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
