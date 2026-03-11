package workshop

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validStepID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Validate checks a LoadedWorkshop for structural correctness.
// Returns nil on success, or a ValidationError with all errors found.
func Validate(w *LoadedWorkshop) error {
	var errs []string

	// Manifest validation
	if w.Manifest.Version != "v1" {
		errs = append(errs, `version: must be "v1"`)
	}
	if w.Manifest.Workshop.Name == "" {
		errs = append(errs, "workshop.name: required")
	}
	if w.Manifest.Workshop.Image == "" {
		errs = append(errs, "workshop.image: required")
	}
	nav := w.Manifest.Workshop.Navigation
	if nav == "" {
		nav = "linear" // default
	}
	if nav != "linear" && nav != "free" && nav != "guided" {
		errs = append(errs, `workshop.navigation: must be one of: linear, free, guided`)
	}

	// Base validation
	hasImage := w.Manifest.Base.Image != ""
	hasCF := w.Manifest.Base.ContainerFile != ""
	if !hasImage && !hasCF {
		errs = append(errs, "base: exactly one of image or containerFile is required")
	}
	if hasImage && hasCF {
		errs = append(errs, "base: image and containerFile are mutually exclusive")
	}
	if hasCF {
		cfPath := filepath.Join(w.WorkshopDir, w.Manifest.Base.ContainerFile)
		if _, err := os.Stat(cfPath); err != nil {
			errs = append(errs, fmt.Sprintf("base.containerFile: file not found: %s", w.Manifest.Base.ContainerFile))
		}
	}

	// Steps list
	if len(w.Manifest.Steps) == 0 {
		errs = append(errs, "steps: at least one step is required")
	}

	// Step ID uniqueness
	seen := map[string]int{}
	for i, id := range w.Manifest.Steps {
		if !validStepID.MatchString(id) {
			errs = append(errs, fmt.Sprintf("steps[%d]: must be lowercase alphanumeric and hyphens only", i))
		}
		if prev, ok := seen[id]; ok {
			errs = append(errs, fmt.Sprintf(`steps[%d]: "%s" is already used at position %d`, i, id, prev))
		}
		seen[id] = i

		// Directory existence
		stepDir := filepath.Join(w.WorkshopDir, "steps", id)
		if _, err := os.Stat(stepDir); err != nil {
			errs = append(errs, fmt.Sprintf("steps[%d]: directory not found: steps/%s/", i, id))
		}
	}

	// Infrastructure validation
	if w.Manifest.Infrastructure != nil {
		infra := w.Manifest.Infrastructure
		if infra.Cluster != nil && infra.Cluster.Enabled {
			if infra.Cluster.Provider == "" {
				errs = append(errs, "infrastructure.cluster.provider: required when enabled is true")
			} else if infra.Cluster.Provider != "k3d" && infra.Cluster.Provider != "vcluster" {
				errs = append(errs, `infrastructure.cluster.provider: must be one of: k3d, vcluster`)
			}
		}
		containerNames := map[string]bool{}
		for i, ec := range infra.ExtraContainers {
			if ec.Name == "" {
				errs = append(errs, fmt.Sprintf("infrastructure.extraContainers[%d]: name is required", i))
			}
			if ec.Image == "" {
				errs = append(errs, fmt.Sprintf("infrastructure.extraContainers[%d]: image is required", i))
			}
			if containerNames[ec.Name] {
				errs = append(errs, fmt.Sprintf(`infrastructure.extraContainers[%d]: name "%s" is already used`, i, ec.Name))
			}
			containerNames[ec.Name] = true
		}
	}

	// Per-step validation
	allStepIDs := map[string]bool{}
	for _, id := range w.Manifest.Steps {
		allStepIDs[id] = true
	}

	for _, step := range w.Steps {
		stepDir := filepath.Join(w.WorkshopDir, "steps", step.ID)

		if !fileExists(filepath.Join(stepDir, "step.yaml")) {
			errs = append(errs, fmt.Sprintf("steps/%s/: step.yaml not found", step.ID))
		}
		if !fileExists(filepath.Join(stepDir, "content.md")) {
			errs = append(errs, fmt.Sprintf("steps/%s/: content.md not found", step.ID))
		}
		if step.Spec.Title == "" {
			errs = append(errs, fmt.Sprintf("steps/%s/step.yaml: title is required", step.ID))
		}

		// Navigation consistency
		if nav == "linear" {
			if step.Spec.Group != "" {
				errs = append(errs, fmt.Sprintf("steps/%s/step.yaml: group not allowed in linear navigation mode", step.ID))
			}
			if len(step.Spec.Requires) > 0 {
				errs = append(errs, fmt.Sprintf("steps/%s/step.yaml: requires not allowed in linear navigation mode", step.ID))
			}
		}

		// Requires references exist
		for _, req := range step.Spec.Requires {
			if !allStepIDs[req] {
				errs = append(errs, fmt.Sprintf(`steps/%s/step.yaml: requires: unknown step "%s"`, step.ID, req))
			}
		}

		// File source existence
		for _, fm := range step.Spec.Files {
			srcPath := filepath.Join(stepDir, "files", fm.Source)
			if !fileExists(srcPath) {
				errs = append(errs, fmt.Sprintf("steps/%s/step.yaml: files: source not found: files/%s", step.ID, fm.Source))
			}
			if !strings.HasPrefix(fm.Target, "/") {
				errs = append(errs, fmt.Sprintf("steps/%s/step.yaml: files: target must be an absolute path", step.ID))
			}
		}

		// llm-docs not empty
		llmDocsDir := filepath.Join(stepDir, "llm-docs")
		if _, err := os.Stat(llmDocsDir); err == nil {
			entries, _ := os.ReadDir(llmDocsDir)
			if len(entries) == 0 {
				errs = append(errs, fmt.Sprintf("steps/%s/llm-docs/: directory is empty", step.ID))
			}
		}
	}

	// TODO: cycle detection for requires graph (defer to post-MVP)

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ValidationError collects all validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Errors, "\n")
}
