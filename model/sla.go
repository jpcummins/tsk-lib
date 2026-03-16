package model

// SLARule represents a rule from sla.toml (Section 12).
type SLARule struct {
	ID       string
	Name     string
	Query    string   // DSL expression string.
	Target   Duration // Allowed time window.
	Start    string   // Start event: "date", "due", or "status:<value>".
	Stop     string   // Stop event: "status:<value>" or "due".
	Severity string   // Reporting label (e.g., "low", "medium", "high", "critical").
}

// SLAStatus represents the evaluated status of an SLA for a task.
type SLAStatus string

const (
	SLAStatusOK       SLAStatus = "ok"
	SLAStatusAtRisk   SLAStatus = "at_risk"
	SLAStatusBreached SLAStatus = "breached"
)

// SLAResult is the evaluated SLA state for a specific task and rule.
type SLAResult struct {
	RuleID    string
	TaskPath  CanonicalPath
	Status    SLAStatus
	Target    Duration
	Elapsed   Duration
	Remaining Duration
}
