package model

import "time"

// StatusCategory represents the base status categories (Section 4).
type StatusCategory string

const (
	StatusCategoryTodo       StatusCategory = "todo"
	StatusCategoryInProgress StatusCategory = "in_progress"
	StatusCategoryDone       StatusCategory = "done"
)

// StatusLogEntry records a status transition with a timestamp (Section 5).
type StatusLogEntry struct {
	Status string
	At     time.Time
}

// Task is the atomic unit of the tsk system (Section 5).
// All fields here represent the resolved state after inheritance,
// redirect resolution, and label merging.
type Task struct {
	// Identity
	CanonicalPath CanonicalPath
	ParentPath    CanonicalPath
	IsReadme      bool // True if this task came from a README.md.

	// Redirect stubs (Section 2.4)
	IsStub     bool
	RedirectTo CanonicalPath // Only set if IsStub is true.

	// Required fields
	Date time.Time

	// Optional fields
	Due            *time.Time
	Assignee       string
	Dependencies   []CanonicalPath
	Summary        string
	Estimate       *Duration
	Status         string
	StatusCategory StatusCategory // Resolved from status map.
	UpdatedAt      *time.Time
	StatusLog      []StatusLogEntry
	Labels         []string // Effective labels (after union inheritance).
	Weight         *int

	// Content
	Body string // Markdown body after front matter.
}
