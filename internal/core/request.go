package core

// Request is the single input shape for every capability (ADR-0003).
type Request struct {
	Dir      string   // project root
	Packages []string // package specs for add/remove
	Group    string   // dependency group, e.g. "dev"
	Script   string   // script name or path for run
	Args     []string // passthrough arguments
	DryRun   bool
	Yes      bool
}

// Result is the single output shape for every capability.
type Result struct {
	Summary  string   `json:"summary"`
	Data     any      `json:"data,omitempty"`
	Commands []string `json:"commands,omitempty"`
}
