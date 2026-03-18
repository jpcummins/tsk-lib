package model

// StatusEntry maps a custom status name to a base category and sort order.
type StatusEntry struct {
	Category StatusCategory `toml:"category" json:"category"`
	Order    int            `toml:"order" json:"order"`
}

// StatusMap maps custom status names to their base categories and sort order.
type StatusMap map[string]StatusEntry

// Resolve returns the StatusCategory for a custom status name.
// Returns empty string if the status is not in the map.
func (m StatusMap) Resolve(status string) StatusCategory {
	if entry, ok := m[status]; ok {
		return entry.Category
	}
	return ""
}

// Merge returns a new StatusMap that combines parent and child maps.
// Child entries override parent entries for the same key.
func (m StatusMap) Merge(child StatusMap) StatusMap {
	result := make(StatusMap, len(m)+len(child))
	for k, v := range m {
		result[k] = v
	}
	for k, v := range child {
		result[k] = v
	}
	return result
}

// Config represents a parsed config.toml file.
type Config struct {
	// Path is the directory this config applies to (empty for root).
	Path CanonicalPath

	// Version is the repository version (only valid at root).
	Version string

	// StatusMap maps custom status names to base categories.
	StatusMap StatusMap
}
