package model

import "time"

// SLAStatus represents the computed status of an SLA measurement.
type SLAStatus string

const (
	SLAOk       SLAStatus = "ok"
	SLAAtRisk   SLAStatus = "at_risk"
	SLABreached SLAStatus = "breached"
)

// SLARule defines an SLA rule from sla.toml.
type SLARule struct {
	// ID is the unique identifier within the SLA file.
	ID string

	// Name is the human-readable label.
	Name string

	// Query is the DSL expression that selects applicable tasks.
	Query string

	// Target is the allowed time window.
	Target Duration

	// WarnAt is the optional threshold for at_risk status.
	// Zero value means not set.
	WarnAt *Duration

	// Start is the start event type ("due" or "status:<value>").
	Start string

	// Stop is the stop event type ("status:<value>" or "due").
	Stop string

	// Severity is the reporting label.
	Severity string
}

// SLAResult is a computed measurement for one task against one rule.
type SLAResult struct {
	// RuleID is the SLA rule identifier.
	RuleID string

	// TaskPath is the canonical path of the task.
	TaskPath CanonicalPath

	// Status is the computed SLA status.
	Status SLAStatus

	// StartTime is when the SLA started (may be zero if not started).
	StartTime *time.Time

	// StopTime is when the SLA stopped (may be zero if not stopped).
	StopTime *time.Time

	// Target is the allowed duration.
	Target Duration

	// Elapsed is the time elapsed since start.
	Elapsed Duration

	// Remaining is target - elapsed (may be negative if breached).
	Remaining Duration
}

// EvaluateSLAStatus computes the SLA status given elapsed time, target, and optional warn_at.
func EvaluateSLAStatus(elapsed, target Duration, warnAt *Duration) SLAStatus {
	if elapsed > target {
		return SLABreached
	}
	if warnAt != nil && elapsed > *warnAt {
		return SLAAtRisk
	}
	return SLAOk
}
