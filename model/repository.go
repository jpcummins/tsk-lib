package model

// Repository is the aggregate root for a fully resolved tsk repository.
type Repository struct {
	// Version is the repository spec version from root config.toml.
	Version string

	// Tasks maps canonical paths to resolved tasks.
	Tasks map[CanonicalPath]*Task

	// Iterations contains all parsed iterations.
	Iterations []*Iteration

	// Teams maps team names to team definitions.
	Teams map[string]*Team

	// SLARules contains all parsed SLA rules.
	SLARules []*SLARule

	// SLAResults contains computed SLA results (populated after evaluation).
	SLAResults []*SLAResult

	// Stubs maps stub paths to their redirect targets.
	Stubs map[CanonicalPath]CanonicalPath

	// Diagnostics collects all warnings and errors from parsing/resolution.
	Diagnostics Diagnostics
}

// NewRepository creates an empty Repository.
func NewRepository() *Repository {
	return &Repository{
		Tasks: make(map[CanonicalPath]*Task),
		Teams: make(map[string]*Team),
		Stubs: make(map[CanonicalPath]CanonicalPath),
	}
}

// OrderedTasks returns tasks ordered by weight (lower first), then by path (lexicographic).
func (r *Repository) OrderedTasks() []*Task {
	tasks := make([]*Task, 0, len(r.Tasks))
	for _, t := range r.Tasks {
		if !t.IsStub {
			tasks = append(tasks, t)
		}
	}
	SortTasks(tasks)
	return tasks
}
