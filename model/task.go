package model

import "time"

// StatusCategory represents the base status categories.
type StatusCategory string

const (
	StatusIcebox     StatusCategory = "icebox"
	StatusTodo       StatusCategory = "todo"
	StatusInProgress StatusCategory = "in_progress"
	StatusDone       StatusCategory = "done"
)

// FieldChange records a change to a task header field (change_log entry).
type FieldChange struct {
	Field string    `yaml:"field" json:"field"`
	From  string    `yaml:"from" json:"from"`
	To    string    `yaml:"to" json:"to"`
	At    time.Time `yaml:"at" json:"at"`
}

// Task is the atomic unit of work — a Markdown file with YAML front matter.
// All fields are optional per the spec.
type Task struct {
	// Identity
	Path     CanonicalPath // canonical path relative to tasks/
	Parent   CanonicalPath // parent path (empty if top-level)
	IsReadme bool          // true if sourced from a README.md

	// Redirect stubs
	RedirectTo CanonicalPath // non-empty if this is a redirect stub
	IsStub     bool

	// Fields (all optional)
	CreatedAt    *time.Time      // created_at
	Due          *time.Time      // due
	Assignee     string          // person identifier, email, or "team:<name>"
	Dependencies []CanonicalPath // canonical paths
	Summary      string
	Estimate     *Duration
	Status       string         // custom status value
	Category     StatusCategory // resolved from status map
	UpdatedAt    *time.Time
	ChangeLog    []FieldChange
	Labels       []string
	Type         string // identifier
	Weight       *float64

	// Content
	Body string // markdown body after front matter
}

// HasStatus returns true if the task has a status set.
func (t *Task) HasStatus() bool {
	return t.Status != ""
}

// HasField returns true if the named field is present on the task.
func (t *Task) HasField(field string) bool {
	switch field {
	case "created_at":
		return t.CreatedAt != nil
	case "due":
		return t.Due != nil
	case "assignee":
		return t.Assignee != ""
	case "dependencies":
		return len(t.Dependencies) > 0
	case "summary":
		return t.Summary != ""
	case "estimate":
		return t.Estimate != nil
	case "status":
		return t.Status != ""
	case "updated_at":
		return t.UpdatedAt != nil
	case "change_log":
		return len(t.ChangeLog) > 0
	case "labels":
		return len(t.Labels) > 0
	case "type":
		return t.Type != ""
	case "weight":
		return t.Weight != nil
	case "path":
		return !t.Path.IsEmpty()
	default:
		return false
	}
}
