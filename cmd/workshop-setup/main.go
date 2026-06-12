// workshop-setup applies step setup (staged files + commands) for all steps
// up to and including a target step. Used by the devcontainer feature's
// postCreateCommand for skip-ahead, sharing backend/setup with the backend's
// activate handler.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/asocpro/workshop-builder/backend/setup"
)

func main() {
	workshopDir := flag.String("workshop-dir", "/workshop", "compiled workshop directory")
	throughStep := flag.String("step", "", "apply setup through this step ID (inclusive)")
	flag.Parse()

	if *throughStep == "" {
		fmt.Println("No step specified — starting at step 1, no setup applied.")
		return
	}

	stepIDs, err := readStepOrder(*workshopDir)
	if err != nil {
		log.Fatalf("reading workshop.json: %v", err)
	}

	found := false
	for _, id := range stepIDs {
		s, err := setup.Load(*workshopDir, id)
		if err != nil {
			log.Fatalf("loading setup for %s: %v", id, err)
		}
		fmt.Printf("Applying setup for %s...\n", id)
		if err := setup.Apply(*workshopDir, id, s); err != nil {
			log.Fatalf("applying setup for %s: %v", id, err)
		}
		if id == *throughStep {
			found = true
			break
		}
	}
	if !found {
		log.Fatalf("step %q not found in workshop", *throughStep)
	}
	fmt.Printf("Setup applied through %s.\n", *throughStep)
}

func readStepOrder(workshopDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(workshopDir, "workshop.json"))
	if err != nil {
		return nil, err
	}
	var wj struct {
		Steps []struct {
			ID string `json:"id"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &wj); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(wj.Steps))
	for _, s := range wj.Steps {
		ids = append(ids, s.ID)
	}
	return ids, nil
}
