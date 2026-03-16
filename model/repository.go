package model

// Repository is the fully resolved in-memory representation of a tsk workspace.
// Produced by the parse/ package after scanning, parsing, and resolution.
type Repository struct {
	// Root directory that was scanned.
	Root string

	// All resolved tasks (stubs excluded, redirects resolved).
	Tasks []*Task

	// All iterations across all teams.
	Iterations []*Iteration

	// All teams with their members and config.
	Teams []*Team

	// Resolved config chain (ordered from root to most specific).
	Configs []*Config

	// SLA rules from sla.toml.
	SLARules []*SLARule

	// Stubs for reference (not included in task list, but useful for diagnostics).
	Stubs []*Task

	// Warnings accumulated during resolution.
	Warnings []string
}
