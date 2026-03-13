package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WorkshopJSON mirrors /workshop/workshop.json
type WorkshopJSON struct {
	Name           string          `json:"name"`
	Image          string          `json:"image"`
	Navigation     string          `json:"navigation"`
	Infrastructure *InfraJSON      `json:"infrastructure,omitempty"`
	Steps          []StepRef       `json:"steps"`
}

type InfraJSON struct {
	Cluster         *ClusterJSON         `json:"cluster,omitempty"`
	ExtraContainers []ExtraContainerJSON `json:"extraContainers,omitempty"`
}

type ClusterJSON struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
}

type ExtraContainerJSON struct {
	Name  string            `json:"name"`
	Image string            `json:"image"`
	Ports []PortJSON        `json:"ports,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
}

type PortJSON struct {
	Port        int    `json:"port"`
	Description string `json:"description,omitempty"`
}

type StepRef struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Group    string   `json:"group,omitempty"`
	Requires []string `json:"requires,omitempty"`
	Position int      `json:"position"`
}

// MetaJSON mirrors /workshop/steps/<id>/meta.json
type MetaJSON struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Group      string   `json:"group,omitempty"`
	Requires   []string `json:"requires,omitempty"`
	Position   int      `json:"position"`
	HasGoss    bool     `json:"hasGoss"`
	HasLlm     bool     `json:"hasLlm"`
	HasHints   bool     `json:"hasHints"`
	HasExplain bool     `json:"hasExplain"`
	HasSolve   bool     `json:"hasSolve"`
}

// Metadata is the in-memory representation of all workshop flat files.
type Metadata struct {
	WorkshopRoot string
	Workshop     WorkshopJSON
	Steps        []MetaJSON           // ordered by position
	StepsByID    map[string]*MetaJSON // fast lookup
}

// LoadMetadata reads workshop.json and all steps/*/meta.json from workshopRoot.
func LoadMetadata(workshopRoot string) (*Metadata, error) {
	// Read workshop.json
	wjPath := filepath.Join(workshopRoot, "workshop.json")
	data, err := os.ReadFile(wjPath)
	if err != nil {
		return nil, fmt.Errorf("reading workshop.json: %w", err)
	}
	var wj WorkshopJSON
	if err := json.Unmarshal(data, &wj); err != nil {
		return nil, fmt.Errorf("parsing workshop.json: %w", err)
	}

	m := &Metadata{
		WorkshopRoot: workshopRoot,
		Workshop:     wj,
		StepsByID:    make(map[string]*MetaJSON),
	}

	// Read each step's meta.json in order
	for _, ref := range wj.Steps {
		metaPath := filepath.Join(workshopRoot, "steps", ref.ID, "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			return nil, fmt.Errorf("reading steps/%s/meta.json: %w", ref.ID, err)
		}
		var meta MetaJSON
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("parsing steps/%s/meta.json: %w", ref.ID, err)
		}
		m.Steps = append(m.Steps, meta)
		m.StepsByID[ref.ID] = &m.Steps[len(m.Steps)-1]
	}

	return m, nil
}

// StepContentPath returns the path to content.md for a step.
func (m *Metadata) StepContentPath(stepID string) string {
	return filepath.Join(m.WorkshopRoot, "steps", stepID, "content.md")
}

// StepGossPath returns the path to goss.yaml for a step.
func (m *Metadata) StepGossPath(stepID string) string {
	return filepath.Join(m.WorkshopRoot, "steps", stepID, "goss.yaml")
}

// StepHelpPath returns the path to a static help file for a step.
// mode is one of: hints, explain, solve
func (m *Metadata) StepHelpPath(stepID, mode string) string {
	return filepath.Join(m.WorkshopRoot, "steps", stepID, mode+".md")
}
