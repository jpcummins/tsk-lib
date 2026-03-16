package model

// Config represents a parsed .config.toml or team.toml configuration (Section 10).
type Config struct {
	// Path to the config file relative to the repository root.
	Path string

	// Repository version (only valid at root .config.toml).
	Version string

	// Default field values applied to tasks in scope.
	Defaults Defaults

	// Inheritance opt-in by field.
	Inherit Inherit

	// Custom status -> base category mapping.
	StatusMap StatusMap

	// Iteration-specific status mapping.
	IterationStatusMap StatusMap
}

// Defaults holds default field values for tasks (Section 10).
type Defaults struct {
	Assignee string
	Status   string
	Estimate string
}

// Inherit controls which fields are inherited from parent configs (Section 10).
type Inherit struct {
	Assignee bool
	Status   bool
	Estimate bool
}

// StatusMap maps custom status names to their base category and sort order.
type StatusMap map[string]StatusEntry

// StatusEntry defines a custom status value's category and display order.
type StatusEntry struct {
	Category StatusCategory
	Order    int
}

// Resolve looks up a custom status and returns its base category.
// Returns StatusCategoryTodo if the status is not found in the map.
func (sm StatusMap) Resolve(status string) StatusCategory {
	if sm == nil {
		return StatusCategoryTodo
	}
	if entry, ok := sm[status]; ok {
		return entry.Category
	}
	// Fall back to treating the status as a base category directly.
	switch StatusCategory(status) {
	case StatusCategoryTodo, StatusCategoryInProgress, StatusCategoryDone:
		return StatusCategory(status)
	}
	return StatusCategoryTodo
}
