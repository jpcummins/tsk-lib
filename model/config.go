package model

// Config represents a parsed tsk.toml file.
type Config struct {
	// Path is the directory this config applies to (empty for root).
	Path CanonicalPath

	// Version is the repository version (only valid at root).
	Version string
}
