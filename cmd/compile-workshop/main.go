// compile-workshop outputs compiled workshop metadata as JSON to stdout.
// Used by the Dagger pipeline via: go run ./cmd/compile-workshop/ --workshop <path>
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/asocpro/workshop-builder/pkg/workshop"
)

// Output is the JSON structure emitted to stdout.
type Output struct {
	WorkshopJSON string       `json:"workshopJson"`
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

func main() {
	workshopPath := flag.String("workshop", "", "path to workshop directory (relative to repo root or absolute)")
	flag.Parse()

	if *workshopPath == "" {
		log.Fatal("--workshop is required")
	}

	abs, err := filepath.Abs(*workshopPath)
	if err != nil {
		log.Fatalf("resolving path: %v", err)
	}

	loaded, err := workshop.Parse(abs)
	if err != nil {
		log.Fatalf("parsing workshop: %v", err)
	}
	if err := workshop.Validate(loaded); err != nil {
		log.Fatalf("validating workshop: %v", err)
	}

	compiled, err := workshop.Compile(loaded)
	if err != nil {
		log.Fatalf("compiling workshop: %v", err)
	}

	out := Output{
		WorkshopJSON: string(compiled.WorkshopJSON),
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
			so.Files = append(so.Files, FileMapping{
				Source: fm.Source,
				Target: fm.Target,
				Mode:   fm.Mode,
			})
		}
		if len(s.Spec.Commands) > 0 {
			so.Commands = s.Spec.Commands
		}
		if len(s.Spec.Env) > 0 {
			so.Env = s.Spec.Env
		}
		out.Steps = append(out.Steps, so)
	}

	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		log.Fatalf("encoding output: %v", err)
	}
}
