package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/asocpro/workshop-builder/backend/process"
	"github.com/asocpro/workshop-builder/backend/store"
)

func main() {
	workshopRoot := os.Getenv("WORKSHOP_ROOT")
	if workshopRoot == "" {
		workshopRoot = "/workshop"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	managementURL := os.Getenv("WORKSHOP_MANAGEMENT_URL")

	// Load metadata from flat files
	meta, err := store.LoadMetadata(workshopRoot)
	if err != nil {
		log.Fatalf("loading workshop metadata: %v", err)
	}

	// Initialize in-memory state
	st := store.NewState(meta)

	// Spawn ttyd (terminal)
	ttydMgr := process.NewTTYDManager(7681)
	ttydMgr.Start()

	// Create and start HTTP server
	srv := NewServer(meta, st, managementURL)

	addr := ":" + port
	fmt.Printf("Workshop backend listening on %s\n", addr)
	fmt.Printf("Workshop: %s (%s navigation)\n", meta.Workshop.Name, meta.Workshop.Navigation)
	if managementURL != "" {
		fmt.Printf("Management URL: %s\n", managementURL)
	}

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
