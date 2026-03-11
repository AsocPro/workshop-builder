package workshop

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Parse reads workshop.yaml and all referenced step directories.
// workshopDir is the absolute path to the workshop root (contains workshop.yaml).
func Parse(workshopDir string) (*LoadedWorkshop, error) {
	// 1. Read and parse workshop.yaml
	manifestPath := filepath.Join(workshopDir, "workshop.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading workshop.yaml: %w", err)
	}
	var manifest WorkshopYAML
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing workshop.yaml: %w", err)
	}

	// 2. Parse each step directory in manifest order
	loaded := &LoadedWorkshop{
		WorkshopDir: workshopDir,
		Manifest:    manifest,
	}
	for _, stepID := range manifest.Steps {
		step, err := parseStep(workshopDir, stepID)
		if err != nil {
			return nil, err
		}
		loaded.Steps = append(loaded.Steps, *step)
	}

	return loaded, nil
}

func parseStep(workshopDir, stepID string) (*LoadedStep, error) {
	stepDir := filepath.Join(workshopDir, "steps", stepID)

	// Read step.yaml
	specPath := filepath.Join(stepDir, "step.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading steps/%s/step.yaml: %w", stepID, err)
	}
	var spec StepYAML
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing steps/%s/step.yaml: %w", stepID, err)
	}

	step := &LoadedStep{
		ID:   stepID,
		Dir:  stepDir,
		Spec: spec,
	}

	// Detect convention files
	step.HasGoss = fileExists(filepath.Join(stepDir, "goss.yaml"))
	step.HasHints = fileExists(filepath.Join(stepDir, "hints.md"))
	step.HasExplain = fileExists(filepath.Join(stepDir, "explain.md"))
	step.HasSolve = fileExists(filepath.Join(stepDir, "solve.md"))
	step.HasLLMDocs = dirExistsAndNonEmpty(filepath.Join(stepDir, "llm-docs"))

	return step, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExistsAndNonEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}
