package scan

// EntryKind classifies a file found during scanning.
type EntryKind int

const (
	EntryTask          EntryKind = iota // tasks/**/*.md
	EntryRootConfig                     // config.toml (root)
	EntryProjectConfig                  // tasks/**/config.toml
	EntryTeamConfig                     // teams/*/team.toml
	EntryIteration                      // teams/*/iterations/*.md
	EntrySLA                            // sla.toml (root only)
)

// String returns a human-readable name for the entry kind.
func (k EntryKind) String() string {
	switch k {
	case EntryTask:
		return "task"
	case EntryRootConfig:
		return "root_config"
	case EntryProjectConfig:
		return "project_config"
	case EntryTeamConfig:
		return "team_config"
	case EntryIteration:
		return "iteration"
	case EntrySLA:
		return "sla"
	default:
		return "unknown"
	}
}

// Entry represents a classified file found during scanning.
type Entry struct {
	// Kind is the classification of this file.
	Kind EntryKind

	// Path is the path relative to the repository root.
	Path string

	// Content is the raw file content.
	Content []byte
}
