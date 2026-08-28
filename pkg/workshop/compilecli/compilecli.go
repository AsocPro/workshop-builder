// Package compilecli is the CLI adapter over pkg/workshop: it compiles a
// workshop source directory and either writes the flat-file layout to a
// directory or emits compiled metadata as JSON. Used by the `workshop compile`
// subcommand and, via that, by the Dagger pipeline.
package compilecli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/asocpro/workshop-builder/pkg/workshop"
)

// Output is the JSON structure emitted to the writer.
type Output struct {
	WorkshopJSON string       `json:"workshopJson"`
	BaseImage    string       `json:"baseImage"`
	Steps        []StepOutput `json:"steps"`
}

// FileMapping mirrors workshop.FileMapping for JSON output.
type FileMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode,omitempty"`
}

// StepOutput holds compiled step data plus the build-time file/command/env specs.
type StepOutput struct {
	ID         string            `json:"id"`
	MetaJSON   string            `json:"metaJson"`
	LLMJson    string            `json:"llmJson,omitempty"`
	HasGoss    bool              `json:"hasGoss"`
	HasHints   bool              `json:"hasHints"`
	HasExplain bool              `json:"hasExplain"`
	HasSolve   bool              `json:"hasSolve"`
	HasLLMDocs bool              `json:"hasLlmDocs"`
	Files      []FileMapping     `json:"files,omitempty"`
	Commands   []string          `json:"commands,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

// Run compiles the workshop at workshopPath. With outputDir set it writes the
// flat-file layout there and produces no JSON. Otherwise it encodes the Output
// JSON to w.
func Run(workshopPath, outputDir string, w io.Writer) error {
	if workshopPath == "" {
		return fmt.Errorf("workshop path is required")
	}
	abs, err := filepath.Abs(workshopPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	if outputDir != "" {
		if _, err := workshop.CompileToDir(abs, outputDir); err != nil {
			return fmt.Errorf("compiling workshop: %w", err)
		}
		return nil
	}

	loaded, err := workshop.Parse(abs)
	if err != nil {
		return fmt.Errorf("parsing workshop: %w", err)
	}
	if err := workshop.Validate(loaded); err != nil {
		return fmt.Errorf("validating workshop: %w", err)
	}
	compiled, err := workshop.Compile(loaded)
	if err != nil {
		return fmt.Errorf("compiling workshop: %w", err)
	}

	baseImage := loaded.Manifest.Base.Image
	if baseImage == "" {
		baseImage = "workshop-base:ubuntu"
	}
	out := Output{
		WorkshopJSON: string(compiled.WorkshopJSON),
		BaseImage:    baseImage,
	}
	for i, cs := range compiled.Steps {
		s := loaded.Steps[i]
		so := StepOutput{
			ID:         cs.ID,
			MetaJSON:   string(cs.MetaJSON),
			HasGoss:    s.HasGoss,
			HasHints:   s.HasHints,
			HasExplain: s.HasExplain,
			HasSolve:   s.HasSolve,
			HasLLMDocs: s.HasLLMDocs,
		}
		if cs.LLMJSON != nil {
			so.LLMJson = string(cs.LLMJSON)
		}
		for _, fm := range s.Spec.Files {
			so.Files = append(so.Files, FileMapping{Source: fm.Source, Target: fm.Target, Mode: fm.Mode})
		}
		if len(s.Spec.Commands) > 0 {
			so.Commands = s.Spec.Commands
		}
		if len(s.Spec.Env) > 0 {
			so.Env = s.Spec.Env
		}
		out.Steps = append(out.Steps, so)
	}

	if err := json.NewEncoder(w).Encode(out); err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}
	return nil
}
