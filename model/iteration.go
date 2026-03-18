package model

import "time"

// Iteration represents a time-boxed work period (sprint).
type Iteration struct {
	// ID is derived from path: <team>/<filename> (lowercase, no extension).
	ID string

	// Team is derived from the directory path.
	Team string

	// Start is the iteration start time.
	Start time.Time

	// End is the iteration end time.
	End time.Time

	// Tasks is an ordered list of canonical task paths.
	Tasks []CanonicalPath

	// Body is the markdown content after front matter.
	Body string
}

// DeriveStatus returns the iteration status based on the current time.
// - "todo" if now < start
// - "in_progress" if start <= now <= end
// - "done" if now > end
func (it *Iteration) DeriveStatus(now time.Time) StatusCategory {
	if now.Before(it.Start) {
		return StatusTodo
	}
	if now.After(it.End) {
		return StatusDone
	}
	return StatusInProgress
}
