package engine

import (
	"strings"
	"time"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/query"
)

// evaluateSLA computes SLA results for all tasks in the repository against all
// SLA rules. It returns one SLAResult per (task, rule) pair where the task
// matches the rule's query and the SLA start event has occurred.
//
// now is the reference time used to compute elapsed duration for active SLAs
// (those whose stop event has not yet occurred).
func evaluateSLA(repo *model.Repository, now time.Time) []*model.SLAResult {
	if len(repo.SLARules) == 0 {
		return nil
	}

	qp := query.NewParser()
	var results []*model.SLAResult

	for _, rule := range repo.SLARules {
		// Parse the rule's applicability query. If it fails, skip the rule —
		// rule queries are validated at parse time so this should not happen in
		// practice.
		expr, err := qp.Parse(rule.Query)
		if err != nil {
			continue
		}

		// Evaluate the query against the repository to find applicable tasks.
		// SLA rule queries never reference sla.* fields (spec §6.0.1), so we
		// use an EvalContext without SLAResults.
		evalCtx := &query.EvalContext{
			Repository: repo,
			Now:        now,
		}
		matchPaths := query.Evaluate(expr, evalCtx)
		matchSet := make(map[model.CanonicalPath]bool, len(matchPaths))
		for _, p := range matchPaths {
			matchSet[model.CanonicalPath(p)] = true
		}

		for _, task := range repo.Tasks {
			if task.IsStub {
				continue
			}
			if !matchSet[task.Path] {
				continue
			}

			result := computeTaskSLA(task, rule, now)
			if result != nil {
				results = append(results, result)
			}
		}
	}

	return results
}

// computeTaskSLA computes the SLA result for a single task against a single
// rule. Returns nil if the SLA start event has not occurred for this task.
func computeTaskSLA(task *model.Task, rule *model.SLARule, now time.Time) *model.SLAResult {
	startTime, stopTime := findSLAEvents(task, rule)
	if startTime == nil {
		// Start event has not occurred; SLA is not active for this task.
		return nil
	}

	// Compute elapsed time. When the stop event has occurred after the start
	// event, the SLA is complete and elapsed is measured up to the stop time.
	// Otherwise it is still active and elapsed is measured up to now.
	var referenceTime time.Time
	if stopTime != nil && stopTime.After(*startTime) {
		referenceTime = *stopTime
	} else {
		referenceTime = now
	}

	wallElapsed := referenceTime.Sub(*startTime)

	// Convert wall-clock elapsed time to work-time Duration (minutes).
	// The spec uses 8h/day work time for SLA durations.
	elapsedMinutes := wallElapsed.Hours() / float64(model.HoursPerDay) * float64(model.MinutesPerDay)
	elapsed := model.Duration(elapsedMinutes)
	remaining := rule.Target - elapsed

	status := model.EvaluateSLAStatus(elapsed, rule.Target, rule.WarnAt)

	return &model.SLAResult{
		RuleID:    rule.ID,
		TaskPath:  task.Path,
		Status:    status,
		StartTime: startTime,
		StopTime:  stopTime,
		Target:    rule.Target,
		Elapsed:   elapsed,
		Remaining: remaining,
	}
}

// findSLAEvents scans a task's ChangeLog (and falls back to UpdatedAt) to
// locate the most recent start and stop event times defined by the rule.
func findSLAEvents(task *model.Task, rule *model.SLARule) (startTime *time.Time, stopTime *time.Time) {
	startStatus := strings.TrimPrefix(rule.Start, "status:")
	stopStatus := strings.TrimPrefix(rule.Stop, "status:")

	// Walk the ChangeLog to find the most recent transitions into the start
	// and stop statuses. Later entries overwrite earlier ones so that the most
	// recent cycle is used (spec §6: most recent start/stop pair).
	for i := range task.ChangeLog {
		cl := &task.ChangeLog[i]
		if cl.Field != "status" {
			continue
		}
		if cl.To == startStatus {
			t := cl.At
			startTime = &t
		}
		if cl.To == stopStatus {
			t := cl.At
			stopTime = &t
		}
	}

	// If no ChangeLog entry found for the start status, fall back to UpdatedAt
	// when the task's current status matches the start status (spec §6.0.1).
	if startTime == nil && task.Status == startStatus && task.UpdatedAt != nil {
		startTime = task.UpdatedAt
	}

	return startTime, stopTime
}

// containsSLAFields reports whether expr contains any sla.* predicate.
// This is used by Engine.Query to auto-detect the need for a reporting context.
func containsSLAFields(expr query.Expr) bool {
	switch e := expr.(type) {
	case *query.BinaryExpr:
		return containsSLAFields(e.Left) || containsSLAFields(e.Right)
	case *query.UnaryExpr:
		return containsSLAFields(e.Expr)
	case *query.Predicate:
		// After parsing, Namespace is set by the validator. Before validation
		// we check the raw Field string.
		if e.Namespace == "sla" {
			return true
		}
		return strings.HasPrefix(e.Field, "sla.")
	}
	return false
}
