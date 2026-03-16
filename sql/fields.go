// Package sql compiles query AST nodes into SQL statements targeting
// the tsk SQLite schema. It maps DSL fields to table columns and
// expands built-in functions into SQL expressions.
package sql

// fieldMapping maps DSL field names to SQL column expressions.
// Fields prefixed with "task." or unqualified map to the tasks table.
// Fields prefixed with "iteration." require a JOIN.
// Fields prefixed with "sla." require a JOIN to a materialized SLA results table.
var fieldMapping = map[string]fieldInfo{
	// Task fields (qualified).
	"task.status":          {column: "t.status"},
	"task.status.category": {column: "t.status_category"},
	"task.assignee":        {column: "t.assignee"},
	"task.due":             {column: "t.due"},
	"task.date":            {column: "t.date"},
	"task.updated_at":      {column: "t.updated_at"},
	"task.estimate":        {column: "t.estimate_mins", isDuration: true},
	"task.path":            {column: "t.canonical_path"},
	"task.summary":         {column: "t.summary"},
	"task.dependency":      {column: "", isRelation: true, relation: "task_dependencies", relColumn: "dependency_path"},
	"task.labels":          {column: "", isRelation: true, relation: "task_labels", relColumn: "label"},

	// Task fields (unqualified — default to task.*).
	"status":          {column: "t.status"},
	"status.category": {column: "t.status_category"},
	"assignee":        {column: "t.assignee"},
	"due":             {column: "t.due"},
	"date":            {column: "t.date"},
	"updated_at":      {column: "t.updated_at"},
	"estimate":        {column: "t.estimate_mins", isDuration: true},
	"path":            {column: "t.canonical_path"},
	"summary":         {column: "t.summary"},
	"dependency":      {column: "", isRelation: true, relation: "task_dependencies", relColumn: "dependency_path"},
	"labels":          {column: "", isRelation: true, relation: "task_labels", relColumn: "label"},

	// Iteration fields (require JOIN).
	"iteration.team":            {column: "iter.team", needsIterJoin: true},
	"iteration.status":          {column: "iter.status", needsIterJoin: true},
	"iteration.status.category": {column: "iter.status_category", needsIterJoin: true},
	"iteration.start":           {column: "iter.start", needsIterJoin: true},
	"iteration.end":             {column: "iter.end", needsIterJoin: true},
	"iteration.path":            {column: "iter.canonical_path", needsIterJoin: true},

	// TODO: SLA fields need significant design work.
	//
	// The spec defines sla.* fields for reporting, but their implementation
	// requires careful thought:
	// - Should SLA status be computed at index time (stale) or query time (slow)?
	// - How to efficiently join tasks with SLA rules, evaluate rule queries,
	//   calculate elapsed time, and derive status at query time?
	// - Should we use a materialized view, background refresh, or real-time
	//   computation?
	//
	// For now, SLA fields are disabled. Uncomment once implemented.
	//
	// "sla.id":        {column: "sla.rule_id", needsSLAJoin: true},
	// "sla.status":    {column: "sla.status", needsSLAJoin: true},
	// "sla.target":    {column: "sla.target_mins", needsSLAJoin: true, isDuration: true},
	// "sla.elapsed":   {column: "sla.elapsed_mins", needsSLAJoin: true, isDuration: true},
	// "sla.remaining": {column: "sla.remaining_mins", needsSLAJoin: true, isDuration: true},
}

// fieldInfo describes how a DSL field maps to SQL.
type fieldInfo struct {
	column        string // SQL column expression (e.g., "t.status").
	isDuration    bool   // If true, values are in minutes and need conversion.
	isRelation    bool   // If true, requires a subquery against a relation table.
	relation      string // Relation table name (for isRelation fields).
	relColumn     string // Column in the relation table to match against.
	needsIterJoin bool   // If true, requires JOIN to iterations via iteration_tasks.
	needsSLAJoin  bool   // If true, requires JOIN to sla_results.
}

// lookupField returns the field info for a DSL field name.
func lookupField(field string) (fieldInfo, bool) {
	info, ok := fieldMapping[field]
	return info, ok
}
