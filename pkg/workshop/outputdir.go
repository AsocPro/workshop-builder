package workshop

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SetupJSON is the per-step runtime setup spec written to setup.json.
// Files are staged under the step's stage/ directory at compile time and
// copied to their targets when the step is activated (devcontainer and
// standalone modes).
type SetupJSON struct {
	Files    []SetupFile       `json:"files"`
	Commands []string          `json:"commands"`
	Env      map[string]string `json:"env"`
}

// SetupFile maps a staged file (relative to stage/) to an absolute target path.
type SetupFile struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode,omitempty"`
}

// CompileToDir parses, validates, and compiles the workshop at srcDir,
// writing the full flat-file layout to outDir:
//
//	<outDir>/workshop.json
//	<outDir>/steps/<id>/meta.json
//	<outDir>/steps/<id>/content.md
//	<outDir>/steps/<id>/{goss.yaml,hints.md,explain.md,solve.md,llm.json,llm-docs/}  (if present)
//	<outDir>/steps/<id>/setup.json
//	<outDir>/steps/<id>/stage/<files...>
//
// Existing workshop.json and steps/ in outDir are replaced; everything else
// (notably runtime/) is left untouched so recompiling on restart preserves
// command logs, recordings, and state events.
func CompileToDir(srcDir, outDir string) (*LoadedWorkshop, error) {
	loaded, err := Parse(srcDir)
	if err != nil {
		return nil, err
	}
	if err := Validate(loaded); err != nil {
		return nil, err
	}
	compiled, err := Compile(loaded)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(outDir, "steps")); err != nil {
		return nil, fmt.Errorf("cleaning steps dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "workshop.json"), compiled.WorkshopJSON, 0644); err != nil {
		return nil, fmt.Errorf("writing workshop.json: %w", err)
	}

	for i, cs := range compiled.Steps {
		step := loaded.Steps[i]
		if err := writeStepDir(outDir, &step, &cs); err != nil {
			return nil, fmt.Errorf("writing step %s: %w", step.ID, err)
		}
	}

	return loaded, nil
}

func writeStepDir(outDir string, step *LoadedStep, cs *CompiledStep) error {
	stepOut := filepath.Join(outDir, "steps", step.ID)
	if err := os.MkdirAll(stepOut, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(stepOut, "meta.json"), cs.MetaJSON, 0644); err != nil {
		return err
	}
	if cs.LLMJSON != nil {
		if err := os.WriteFile(filepath.Join(stepOut, "llm.json"), cs.LLMJSON, 0644); err != nil {
			return err
		}
	}

	if err := copyFile(filepath.Join(step.Dir, "content.md"), filepath.Join(stepOut, "content.md")); err != nil {
		return err
	}
	for _, name := range []string{"goss.yaml", "hints.md", "explain.md", "solve.md"} {
		src := filepath.Join(step.Dir, name)
		if fileExists(src) {
			if err := copyFile(src, filepath.Join(stepOut, name)); err != nil {
				return err
			}
		}
	}
	if step.HasLLMDocs {
		if err := copyDir(filepath.Join(step.Dir, "llm-docs"), filepath.Join(stepOut, "llm-docs")); err != nil {
			return err
		}
	}

	// setup.json is written even when empty — runtime logic stays unconditional.
	setup := SetupJSON{
		Files:    []SetupFile{},
		Commands: step.Spec.Commands,
		Env:      step.Spec.Env,
	}
	if setup.Commands == nil {
		setup.Commands = []string{}
	}
	if setup.Env == nil {
		setup.Env = map[string]string{}
	}
	for _, fm := range step.Spec.Files {
		setup.Files = append(setup.Files, SetupFile{
			Source: fm.Source,
			Target: fm.Target,
			Mode:   fm.Mode,
		})
		src := filepath.Join(step.Dir, "files", fm.Source)
		if err := copyFile(src, filepath.Join(stepOut, "stage", fm.Source)); err != nil {
			return fmt.Errorf("staging %s: %w", fm.Source, err)
		}
	}
	setupBytes, err := json.MarshalIndent(setup, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stepOut, "setup.json"), setupBytes, 0644)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}
