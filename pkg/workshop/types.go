package workshop

// ─── YAML Input Types (author-facing) ───────────────────────────────────────

// WorkshopYAML is the top-level workshop.yaml structure.
type WorkshopYAML struct {
	Version        string      `yaml:"version"`
	Workshop       WorkshopMeta `yaml:"workshop"`
	Base           Base         `yaml:"base"`
	Infrastructure *InfraYAML  `yaml:"infrastructure,omitempty"`
	Steps          []string     `yaml:"steps"`
}

type WorkshopMeta struct {
	Name       string `yaml:"name"`
	Image      string `yaml:"image"`
	Navigation string `yaml:"navigation,omitempty"` // linear|free|guided, default: linear
}

type Base struct {
	Image         string `yaml:"image,omitempty"`
	ContainerFile string `yaml:"containerFile,omitempty"`
}

type InfraYAML struct {
	Cluster         *ClusterYAML         `yaml:"cluster,omitempty"`
	ExtraContainers []ExtraContainerYAML `yaml:"extraContainers,omitempty"`
}

type ClusterYAML struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider,omitempty"`
}

type ExtraContainerYAML struct {
	Name  string            `yaml:"name"`
	Image string            `yaml:"image"`
	Ports []PortYAML        `yaml:"ports,omitempty"`
	Env   map[string]string `yaml:"env,omitempty"`
}

type PortYAML struct {
	Port        int    `yaml:"port"`
	Description string `yaml:"description,omitempty"`
}

// StepYAML is the per-step step.yaml structure.
type StepYAML struct {
	Title    string            `yaml:"title"`
	Group    string            `yaml:"group,omitempty"`
	Requires []string          `yaml:"requires,omitempty"`
	Files    []FileMapping     `yaml:"files,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Commands []string          `yaml:"commands,omitempty"`
	LLM      *LLMConfig        `yaml:"llm,omitempty"`
}

type FileMapping struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Mode   string `yaml:"mode,omitempty"`
}

type LLMConfig struct {
	Context string `yaml:"context,omitempty"`
}

// ─── Loaded Types (parsed, pre-validation) ──────────────────────────────────

// LoadedWorkshop holds the fully parsed workshop definition.
type LoadedWorkshop struct {
	WorkshopDir string       // absolute path to workshop directory
	Manifest    WorkshopYAML // parsed workshop.yaml
	Steps       []LoadedStep // parsed steps in manifest order
}

// LoadedStep holds the parsed state of one step directory.
type LoadedStep struct {
	ID   string   // directory name (= step ID)
	Dir  string   // absolute path to step directory
	Spec StepYAML // parsed step.yaml

	// Convention file presence flags (set during parsing)
	HasGoss    bool
	HasHints   bool
	HasExplain bool
	HasSolve   bool
	HasLLMDocs bool
}

// ─── Compiled Output Types (JSON-serialized into /workshop/) ────────────────

// CompiledWorkshop is the output of Compile().
type CompiledWorkshop struct {
	WorkshopJSON []byte // → /workshop/workshop.json
	Steps        []CompiledStep
}

// CompiledStep holds the compiled artifacts for one step.
type CompiledStep struct {
	ID       string
	MetaJSON []byte // → /workshop/steps/<id>/meta.json
	// LLMJSON is nil if no llm config
	LLMJSON []byte // → /workshop/steps/<id>/llm.json (nil if no LLM config)
}

// ─── JSON wire types (for encoding/json) ────────────────────────────────────
// These are the actual JSON structures written to disk.

type WorkshopJSON struct {
	Name           string     `json:"name"`
	Image          string     `json:"image"`
	Navigation     string     `json:"navigation"`
	Infrastructure *InfraJSON `json:"infrastructure,omitempty"`
	Steps          []StepRef  `json:"steps"`
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

type LLMJSON struct {
	Context string `json:"context,omitempty"`
	HasDocs bool   `json:"hasDocs"`
}
