// Package setup applies per-step setup specs (setup.json) — staged file
// copies and shell commands. It is the single implementation behind both the
// backend's activate handler (devcontainer/standalone modes) and RunThrough,
// used by the devcontainer postCreateCommand skip-ahead helper.
package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// FileMapping maps a staged file (relative to the step's stage/ directory)
// to an absolute target path.
type FileMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode,omitempty"`
}

// StepSetup mirrors /workshop/steps/<id>/setup.json.
type StepSetup struct {
	Files    []FileMapping     `json:"files"`
	Commands []string          `json:"commands"`
	Env      map[string]string `json:"env"`
}

// InPlaceMode reports whether the given WORKSHOP_MODE uses in-place step
// transitions (setup applied inside the running environment) instead of
// external container swaps. Handlers branch on this capability, never on the
// raw mode string.
func InPlaceMode(mode string) bool {
	return mode == "devcontainer" || mode == "standalone"
}

// Load reads setup.json for a step under workshopRoot.
func Load(workshopRoot, stepID string) (*StepSetup, error) {
	path := filepath.Join(workshopRoot, "steps", stepID, "setup.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading setup.json for %s: %w", stepID, err)
	}
	var s StepSetup
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing setup.json for %s: %w", stepID, err)
	}
	return &s, nil
}

// Apply executes a step's setup: copies staged files to their targets, then
// runs commands via `sh -c` with the step's env appended.
func Apply(workshopRoot, stepID string, s *StepSetup) error {
	stageDir := filepath.Join(workshopRoot, "steps", stepID, "stage")

	for _, f := range s.Files {
		src := filepath.Join(stageDir, f.Source)
		mode := os.FileMode(0644)
		if f.Mode != "" {
			parsed, err := strconv.ParseUint(f.Mode, 8, 32)
			if err != nil {
				return fmt.Errorf("step %s: invalid mode %q for %s: %w", stepID, f.Mode, f.Source, err)
			}
			mode = os.FileMode(parsed)
		}
		if err := copyFile(src, f.Target, mode); err != nil {
			return fmt.Errorf("step %s: copying %s → %s: %w", stepID, f.Source, f.Target, err)
		}
	}

	env := os.Environ()
	for k, v := range s.Env {
		env = append(env, k+"="+v)
	}
	for _, cmdStr := range s.Commands {
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("step %s: command %q: %w", stepID, cmdStr, err)
		}
	}
	return nil
}

// RunThrough applies each step's setup, in workshop order, up to and including
// throughStep. Empty throughStep is a no-op (start at step 1). It backs the
// devcontainer skip-ahead helper. Errors are returned, not fatal.
func RunThrough(workshopDir, throughStep string) error {
	if throughStep == "" {
		fmt.Println("No step specified — starting at step 1, no setup applied.")
		return nil
	}
	stepIDs, err := readStepOrder(workshopDir)
	if err != nil {
		return fmt.Errorf("reading workshop.json: %w", err)
	}
	found := false
	for _, id := range stepIDs {
		s, err := Load(workshopDir, id)
		if err != nil {
			return fmt.Errorf("loading setup for %s: %w", id, err)
		}
		fmt.Printf("Applying setup for %s...\n", id)
		if err := Apply(workshopDir, id, s); err != nil {
			return fmt.Errorf("applying setup for %s: %w", id, err)
		}
		if id == throughStep {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("step %q not found in workshop", throughStep)
	}
	fmt.Printf("Setup applied through %s.\n", throughStep)
	return nil
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

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	// O_CREATE only applies mode to new files; enforce on overwrite too.
	if err := out.Chmod(mode); err != nil {
		return err
	}
	return out.Close()
}
