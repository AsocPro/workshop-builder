package workshop

import (
	"encoding/json"
	"fmt"
)

// Compile converts a validated LoadedWorkshop into compiled JSON bytes.
// No file I/O — returns byte slices only. The Dagger pipeline injects them.
func Compile(w *LoadedWorkshop) (*CompiledWorkshop, error) {
	nav := w.Manifest.Workshop.Navigation
	if nav == "" {
		nav = "linear"
	}

	// Build workshop.json
	wj := WorkshopJSON{
		Name:       w.Manifest.Workshop.Name,
		Image:      w.Manifest.Workshop.Image,
		Navigation: nav,
	}

	if w.Manifest.Infrastructure != nil {
		wj.Infrastructure = compileInfra(w.Manifest.Infrastructure)
	}

	for i, step := range w.Steps {
		ref := StepRef{
			ID:       step.ID,
			Title:    step.Spec.Title,
			Group:    step.Spec.Group,
			Requires: step.Spec.Requires,
			Position: i,
		}
		wj.Steps = append(wj.Steps, ref)
	}

	wjBytes, err := json.MarshalIndent(wj, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling workshop.json: %w", err)
	}

	// Build per-step artifacts
	compiled := &CompiledWorkshop{
		WorkshopJSON: wjBytes,
	}

	for i, step := range w.Steps {
		cs, err := compileStep(i, &step)
		if err != nil {
			return nil, fmt.Errorf("compiling step %s: %w", step.ID, err)
		}
		compiled.Steps = append(compiled.Steps, *cs)
	}

	return compiled, nil
}

func compileStep(position int, step *LoadedStep) (*CompiledStep, error) {
	hasLlm := step.Spec.LLM != nil
	meta := MetaJSON{
		ID:         step.ID,
		Title:      step.Spec.Title,
		Group:      step.Spec.Group,
		Requires:   step.Spec.Requires,
		Position:   position,
		HasGoss:    step.HasGoss,
		HasLlm:     hasLlm,
		HasHints:   step.HasHints,
		HasExplain: step.HasExplain,
		HasSolve:   step.HasSolve,
	}

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling meta.json: %w", err)
	}

	cs := &CompiledStep{
		ID:       step.ID,
		MetaJSON: metaBytes,
	}

	if step.Spec.LLM != nil {
		llm := LLMJSON{
			Context: step.Spec.LLM.Context,
			HasDocs: step.HasLLMDocs,
		}
		llmBytes, err := json.MarshalIndent(llm, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling llm.json: %w", err)
		}
		cs.LLMJSON = llmBytes
	}

	return cs, nil
}

func compileInfra(infra *InfraYAML) *InfraJSON {
	ij := &InfraJSON{}
	if infra.Cluster != nil {
		ij.Cluster = &ClusterJSON{
			Enabled:  infra.Cluster.Enabled,
			Provider: infra.Cluster.Provider,
		}
	}
	for _, ec := range infra.ExtraContainers {
		ecj := ExtraContainerJSON{
			Name:  ec.Name,
			Image: ec.Image,
			Env:   ec.Env,
		}
		for _, p := range ec.Ports {
			ecj.Ports = append(ecj.Ports, PortJSON{
				Port:        p.Port,
				Description: p.Description,
			})
		}
		ij.ExtraContainers = append(ij.ExtraContainers, ecj)
	}
	return ij
}
