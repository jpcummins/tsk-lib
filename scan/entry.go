// Package scan walks a tsk repository root and classifies files into typed entries.
// It performs only I/O and classification — no content parsing.
package scan

// EntryKind classifies a file's role in the tsk repository.
type EntryKind int

const (
	// EntryTask is a Markdown file under tasks/ (task or stub).
	EntryTask EntryKind = iota
	// EntryRootConfig is the root .config.toml.
	EntryRootConfig
	// EntryProjectConfig is a .config.toml under tasks/.
	EntryProjectConfig
	// EntryTeamConfig is a team.toml under teams/<team>/.
	EntryTeamConfig
	// EntryIteration is a Markdown file under teams/<team>/iterations/.
	EntryIteration
	// EntrySLA is the root sla.toml.
	EntrySLA
)

// Entry is a raw file discovered during scanning.
// It carries the file's bytes and classification but no parsed content.
type Entry struct {
	// Path is the file path relative to the repository root (forward slashes).
	Path string

	// Kind classifies the file's role.
	Kind EntryKind

	// Content is the raw file bytes.
	Content []byte
}
