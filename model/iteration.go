package model

import "time"

// Iteration represents a team iteration/sprint (Section 9).
type Iteration struct {
	// Identity — derived from the file path under teams/<team>/iterations/.
	CanonicalPath CanonicalPath

	// Required fields
	Start  time.Time
	End    time.Time
	Status string
	Tasks  []CanonicalPath // Ordered list of task canonical paths.

	// Resolved
	StatusCategory StatusCategory // Resolved from iteration status map.
	Team           string         // Derived from directory path or explicit field.

	// Optional fields
	Name     string
	Capacity *Duration

	// Content
	Body string
}
