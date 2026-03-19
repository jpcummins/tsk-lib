// Package sql compiles query ASTs into parameterized SQLite queries.
package sql

// fieldMapping maps DSL field names to SQL column expressions.
var fieldMapping = map[string]string{
	// Task fields
	"task.status":     "t.status",
	"task.assignee":   "t.assignee",
	"task.due":        "t.due",
	"task.created_at": "t.created_at",
	"task.updated_at": "t.updated_at",
	"task.estimate":   "t.estimate",
	"task.path":       "t.path",
	"task.summary":    "t.summary",
	"task.type":       "t.type",
	"task.weight":     "t.weight",

	// Iteration fields
	"iteration.id":    "i.id",
	"iteration.team":  "i.team",
	"iteration.start": "i.start_time",
	"iteration.end":   "i.end_time",

	// SLA fields
	"sla.id":        "sr.rule_id",
	"sla.status":    "sr.status",
	"sla.target":    "sr.target",
	"sla.elapsed":   "sr.elapsed",
	"sla.remaining": "sr.remaining",
}

// relationFields are fields that require subquery-based matching.
var relationFields = map[string]string{
	"task.labels":       "task_labels",
	"task.dependencies": "task_dependencies",
}
