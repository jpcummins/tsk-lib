// Package query eval provides in-memory query evaluation against a model.Repository.
// This is used for conformance testing and small repositories.
package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpcummins/tsk-lib/model"
)

// EvalContext provides runtime context for query evaluation.
type EvalContext struct {
	// Repository is the parsed repository.
	Repository *model.Repository

	// Now is the current time (for date() relative calculations).
	Now time.Time

	// CurrentUser is the current user identifier (for me()).
	CurrentUser string

	// SLAResults provides pre-computed SLA results for sla.* queries.
	SLAResults []*model.SLAResult

	// IsReportingContext enables SLA field access.
	IsReportingContext bool
}

// Evaluate runs a parsed and validated query against the repository,
// returning matching task canonical paths.
func Evaluate(expr Expr, ctx *EvalContext) []string {
	var matches []string

	for _, task := range ctx.Repository.Tasks {
		if task.IsStub {
			continue
		}
		if evalExpr(expr, task, ctx) {
			matches = append(matches, string(task.Path))
		}
	}

	return matches
}

func evalExpr(expr Expr, task *model.Task, ctx *EvalContext) bool {
	switch e := expr.(type) {
	case *BinaryExpr:
		left := evalExpr(e.Left, task, ctx)
		right := evalExpr(e.Right, task, ctx)
		if e.Op == TokenAND {
			return left && right
		}
		return left || right // OR

	case *UnaryExpr:
		// NOT on iteration/sla predicates negates existence
		inner := evalExpr(e.Expr, task, ctx)
		return !inner

	case *Predicate:
		return evalPredicate(e, task, ctx)

	case *FuncCall:
		return evalFuncCall(e, task, ctx)
	}

	return false
}

func evalPredicate(pred *Predicate, task *model.Task, ctx *EvalContext) bool {
	field := resolveFieldName(pred)

	// Handle iteration.* fields
	if strings.HasPrefix(field, "iteration.") {
		return evalIterationPredicate(field, pred, task, ctx)
	}

	// Handle sla.* fields
	if strings.HasPrefix(field, "sla.") {
		return evalSLAPredicate(field, pred, task, ctx)
	}

	// Task fields
	fieldVal := getTaskFieldValue(field, task)
	return compareValue(fieldVal, pred.Op, pred.Value, ctx)
}

// resolveFieldName returns the fully qualified field name for a predicate.
func resolveFieldName(pred *Predicate) string {
	// If the validator already resolved it, use that
	if pred.Namespace != "" {
		return pred.Namespace + "." + pred.FieldName
	}

	field := pred.Field
	// Already qualified with known namespace
	if strings.HasPrefix(field, "task.") ||
		strings.HasPrefix(field, "iteration.") ||
		strings.HasPrefix(field, "sla.") {
		return field
	}

	// Unqualified defaults to task.*
	return "task." + field
}

func evalIterationPredicate(field string, pred *Predicate, task *model.Task, ctx *EvalContext) bool {
	// Find iterations that contain this task and match the predicate
	for _, iter := range ctx.Repository.Iterations {
		taskInIter := false
		for _, tp := range iter.Tasks {
			if tp == task.Path {
				taskInIter = true
				break
			}
		}
		if !taskInIter {
			continue
		}

		val := getIterationFieldValue(field, iter)
		if compareValue(val, pred.Op, pred.Value, ctx) {
			return true
		}
	}
	return false
}

func evalSLAPredicate(field string, pred *Predicate, task *model.Task, ctx *EvalContext) bool {
	for _, result := range ctx.SLAResults {
		if result.TaskPath != task.Path {
			continue
		}
		val := getSLAFieldValue(field, result)
		if compareValue(val, pred.Op, pred.Value, ctx) {
			return true
		}
	}
	return false
}

func evalFuncCall(fc *FuncCall, task *model.Task, ctx *EvalContext) bool {
	switch fc.Name {
	case "exists":
		if len(fc.Args) == 1 {
			if ident, ok := fc.Args[0].(*IdentValue); ok {
				fieldName := ident.Name
				if !strings.Contains(fieldName, ".") {
					fieldName = "task." + fieldName
				}
				return taskFieldExists(fieldName, task)
			}
		}

	case "missing":
		if len(fc.Args) == 1 {
			if ident, ok := fc.Args[0].(*IdentValue); ok {
				fieldName := ident.Name
				if !strings.Contains(fieldName, ".") {
					fieldName = "task." + fieldName
				}
				return !taskFieldExists(fieldName, task)
			}
		}

	case "has":
		if len(fc.Args) == 2 {
			fieldArg, ok1 := fc.Args[0].(*IdentValue)
			valueArg, ok2 := fc.Args[1].(*StringValue)
			if ok1 && ok2 {
				return taskListContains(fieldArg.Name, valueArg.Val, task)
			}
		}
	}

	return false
}

// --- Field value extraction ---

type fieldValue struct {
	isSet    bool
	str      string
	time     *time.Time
	duration *model.Duration
	number   *float64
}

func getTaskFieldValue(field string, task *model.Task) fieldValue {
	switch field {
	case "task.status":
		return fieldValue{isSet: task.Status != "", str: task.Status}
	case "task.assignee":
		return fieldValue{isSet: task.Assignee != "", str: task.Assignee}
	case "task.summary":
		return fieldValue{isSet: task.Summary != "", str: task.Summary}
	case "task.path":
		return fieldValue{isSet: !task.Path.IsEmpty(), str: string(task.Path)}
	case "task.type":
		return fieldValue{isSet: task.Type != "", str: task.Type}
	case "task.due":
		if task.Due != nil {
			return fieldValue{isSet: true, time: task.Due}
		}
		return fieldValue{}
	case "task.created_at":
		if task.CreatedAt != nil {
			return fieldValue{isSet: true, time: task.CreatedAt}
		}
		return fieldValue{}
	case "task.updated_at":
		if task.UpdatedAt != nil {
			return fieldValue{isSet: true, time: task.UpdatedAt}
		}
		return fieldValue{}
	case "task.estimate":
		if task.Estimate != nil {
			return fieldValue{isSet: true, duration: task.Estimate}
		}
		return fieldValue{}
	case "task.weight":
		if task.Weight != nil {
			return fieldValue{isSet: true, number: task.Weight}
		}
		return fieldValue{}
	}
	return fieldValue{}
}

func getIterationFieldValue(field string, iter *model.Iteration) fieldValue {
	switch field {
	case "iteration.id":
		return fieldValue{isSet: true, str: iter.ID}
	case "iteration.team":
		return fieldValue{isSet: true, str: iter.Team}
	case "iteration.start":
		return fieldValue{isSet: true, time: &iter.Start}
	case "iteration.end":
		return fieldValue{isSet: true, time: &iter.End}
	}
	return fieldValue{}
}

func getSLAFieldValue(field string, result *model.SLAResult) fieldValue {
	switch field {
	case "sla.id":
		return fieldValue{isSet: true, str: result.RuleID}
	case "sla.status":
		return fieldValue{isSet: true, str: string(result.Status)}
	case "sla.target":
		return fieldValue{isSet: true, duration: &result.Target}
	case "sla.elapsed":
		return fieldValue{isSet: true, duration: &result.Elapsed}
	case "sla.remaining":
		return fieldValue{isSet: true, duration: &result.Remaining}
	}
	return fieldValue{}
}

func taskFieldExists(field string, task *model.Task) bool {
	switch field {
	case "task.created_at":
		return task.CreatedAt != nil
	case "task.due":
		return task.Due != nil
	case "task.updated_at":
		return task.UpdatedAt != nil
	case "task.estimate":
		return task.Estimate != nil
	case "task.status":
		return task.Status != ""
	case "task.assignee":
		return task.Assignee != ""
	case "task.summary":
		return task.Summary != ""
	case "task.type":
		return task.Type != ""
	case "task.labels":
		return len(task.Labels) > 0
	case "task.dependencies":
		return len(task.Dependencies) > 0
	case "task.weight":
		return task.Weight != nil
	case "task.change_log":
		return len(task.ChangeLog) > 0
	}
	return false
}

func taskListContains(field string, value string, task *model.Task) bool {
	if !strings.Contains(field, ".") {
		field = "task." + field
	}
	switch field {
	case "task.labels":
		for _, l := range task.Labels {
			if strings.EqualFold(l, value) {
				return true
			}
		}
	case "task.dependencies":
		for _, d := range task.Dependencies {
			if string(d) == value {
				return true
			}
		}
	}
	return false
}

// --- Value comparison ---

func compareValue(fv fieldValue, op TokenType, val Value, ctx *EvalContext) bool {
	if !fv.isSet {
		return false // missing field → predicate is false
	}

	// Resolve the comparison value
	switch v := val.(type) {
	case *StringValue:
		return compareString(fv, op, v.Val, ctx)

	case *FuncValue:
		return compareFuncValue(fv, op, v, ctx)

	case *ListValue:
		if op == TokenIN {
			for _, item := range v.Values {
				if compareValue(fv, TokenEQ, item, ctx) {
					return true
				}
			}
			return false
		}

	case *NumberValue:
		// Try numeric comparison
		return compareString(fv, op, v.Val, ctx)

	case *IdentValue:
		// Unquoted identifiers in value position are treated as string literals.
		return compareString(fv, op, v.Name, ctx)
	}

	return false
}

func compareString(fv fieldValue, op TokenType, val string, ctx *EvalContext) bool {
	// Time-based comparison
	if fv.time != nil {
		valTime, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return false
		}
		return compareTime(*fv.time, op, valTime)
	}

	// Duration-based comparison
	if fv.duration != nil {
		valDur, err := model.ParseDuration(val)
		if err != nil {
			return false
		}
		return compareDuration(*fv.duration, op, valDur)
	}

	// String comparison
	a := fv.str
	b := val

	switch op {
	case TokenEQ:
		return a == b
	case TokenNEQ:
		return a != b
	case TokenTilde:
		return strings.Contains(strings.ToLower(a), strings.ToLower(b))
	case TokenLT:
		return a < b
	case TokenLTE:
		return a <= b
	case TokenGT:
		return a > b
	case TokenGTE:
		return a >= b
	}

	return false
}

func compareFuncValue(fv fieldValue, op TokenType, fn *FuncValue, ctx *EvalContext) bool {
	switch fn.Name {
	case "date":
		if fv.time != nil && len(fn.Args) == 1 {
			if sv, ok := fn.Args[0].(*StringValue); ok {
				resolved := ResolveDate(sv.Val, ctx.Now)
				if resolved != nil {
					return compareTime(*fv.time, op, *resolved)
				}
			}
		}

	case "team":
		if len(fn.Args) == 1 {
			if sv, ok := fn.Args[0].(*StringValue); ok {
				return matchTeam(fv.str, sv.Val, ctx)
			}
		}

	case "me":
		// Try direct match first.
		if compareString(fv, op, ctx.CurrentUser, ctx) {
			return true
		}
		// Resolve aliases through team membership (identifier ↔ email).
		if ctx.Repository != nil {
			for _, team := range ctx.Repository.Teams {
				for _, member := range team.Members {
					if member.Identifier == ctx.CurrentUser || member.Email == ctx.CurrentUser {
						if compareString(fv, op, member.Identifier, ctx) {
							return true
						}
						if member.Email != "" && compareString(fv, op, member.Email, ctx) {
							return true
						}
					}
				}
			}
		}
		return false

	case "my_team":
		// Resolve all teams the current user belongs to
		for _, team := range ctx.Repository.Teams {
			for _, member := range team.Members {
				if member.Identifier == ctx.CurrentUser || member.Email == ctx.CurrentUser {
					if fv.str == "team:"+team.Name {
						return true
					}
				}
			}
		}
	}

	return false
}

func matchTeam(assignee string, teamName string, ctx *EvalContext) bool {
	// Match "team:<name>" prefix
	if assignee == "team:"+teamName {
		return true
	}

	// Match any member of the team
	if ctx.Repository != nil {
		if team, ok := ctx.Repository.Teams[teamName]; ok {
			for _, member := range team.Members {
				if assignee == member.Identifier || assignee == member.Email || assignee == member.Value {
					return true
				}
			}
		}
	}

	return false
}

// ResolveDate resolves a date specification to a time.Time.
// Supports RFC3339, relative strings like "-7d", and named dates like "today".
func ResolveDate(spec string, now time.Time) *time.Time {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, spec); err == nil {
		return &t
	}

	// Use the local date but in UTC to match RFC3339 task timestamps.
	// We take the local date (year/month/day) and create a UTC midnight from it,
	// so that "today" means "today in the user's timezone" expressed as UTC.
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Named relative dates
	switch spec {
	case "today":
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return &t
	case "yesterday":
		t := now.AddDate(0, 0, -1)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return &t
	case "tomorrow":
		t := now.AddDate(0, 0, 1)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return &t
	}

	// Parse relative duration: "-7d", "+3d", "-2w", etc.
	// For date() contexts, durations use calendar time, not work time.
	if len(spec) >= 2 {
		s := strings.TrimPrefix(spec, "+")
		unit := s[len(s)-1:]
		numStr := s[:len(s)-1]
		val := 0.0
		if _, err := fmt.Sscanf(numStr, "%f", &val); err == nil {
			switch unit {
			case "d":
				days := int(val)
				base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
				t := base.AddDate(0, 0, days)
				return &t
			case "w":
				days := int(val * 7)
				base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
				t := base.AddDate(0, 0, days)
				return &t
			case "h":
				t := now.Add(time.Duration(val) * time.Hour)
				return &t
			case "m":
				months := int(val)
				base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
				t := base.AddDate(0, months, 0)
				return &t
			}
		}
	}

	return nil
}

func compareTime(a time.Time, op TokenType, b time.Time) bool {
	switch op {
	case TokenEQ:
		return a.Equal(b)
	case TokenNEQ:
		return !a.Equal(b)
	case TokenLT:
		return a.Before(b)
	case TokenLTE:
		return !a.After(b)
	case TokenGT:
		return a.After(b)
	case TokenGTE:
		return !a.Before(b)
	}
	return false
}

func compareDuration(a model.Duration, op TokenType, b model.Duration) bool {
	switch op {
	case TokenEQ:
		return a == b
	case TokenNEQ:
		return a != b
	case TokenLT:
		return a < b
	case TokenLTE:
		return a <= b
	case TokenGT:
		return a > b
	case TokenGTE:
		return a >= b
	}
	return false
}
