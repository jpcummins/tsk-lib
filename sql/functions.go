package sql

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/query"
)

// CompileContext provides runtime values needed during SQL generation.
// Implementations supply user identity, team membership, and date resolution.
type CompileContext interface {
	CurrentUser() string
	CurrentUserTeams() []string
	TeamMembers(team string) ([]string, error)
	ResolveDate(token string) (time.Time, error)
}

// expandFunc compiles a FuncCall into a SQL fragment with bind params.
func expandFunc(fc *query.FuncCall, ctx CompileContext) (string, []any, error) {
	switch fc.Name {
	case "exists":
		return expandExists(fc)
	case "missing":
		return expandMissing(fc)
	case "has":
		return expandHas(fc)
	case "date":
		return expandDate(fc, ctx)
	case "team":
		return expandTeam(fc, ctx)
	case "me":
		return expandMe(ctx)
	case "my_team":
		return expandMyTeam(ctx)
	default:
		return "", nil, fmt.Errorf("unknown function %q", fc.Name)
	}
}

// expandExists: exists(field) -> field IS NOT NULL
func expandExists(fc *query.FuncCall) (string, []any, error) {
	fieldName, err := identArg(fc, 0)
	if err != nil {
		return "", nil, err
	}
	info, ok := lookupField(fieldName)
	if !ok {
		return "", nil, fmt.Errorf("exists(): unknown field %q", fieldName)
	}

	if info.isRelation {
		return fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE task_path = t.canonical_path)", info.relation), nil, nil
	}

	return fmt.Sprintf("%s IS NOT NULL AND %s != ''", info.column, info.column), nil, nil
}

// expandMissing: missing(field) -> field IS NULL OR field = ”
func expandMissing(fc *query.FuncCall) (string, []any, error) {
	fieldName, err := identArg(fc, 0)
	if err != nil {
		return "", nil, err
	}
	info, ok := lookupField(fieldName)
	if !ok {
		return "", nil, fmt.Errorf("missing(): unknown field %q", fieldName)
	}

	if info.isRelation {
		return fmt.Sprintf("NOT EXISTS (SELECT 1 FROM %s WHERE task_path = t.canonical_path)", info.relation), nil, nil
	}

	if info.isDuration {
		return fmt.Sprintf("%s IS NULL", info.column), nil, nil
	}

	return fmt.Sprintf("(%s IS NULL OR %s = '')", info.column, info.column), nil, nil
}

// expandHas: has(field, value) -> EXISTS subquery against relation table
func expandHas(fc *query.FuncCall) (string, []any, error) {
	fieldName, err := identArg(fc, 0)
	if err != nil {
		return "", nil, err
	}
	valStr, err := stringArg(fc, 1)
	if err != nil {
		return "", nil, err
	}

	info, ok := lookupField(fieldName)
	if !ok {
		return "", nil, fmt.Errorf("has(): unknown field %q", fieldName)
	}

	if !info.isRelation {
		return "", nil, fmt.Errorf("has(): field %q is not a list field", fieldName)
	}

	sql := fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE task_path = t.canonical_path AND %s = ? COLLATE NOCASE)",
		info.relation, info.relColumn)
	return sql, []any{valStr}, nil
}

// expandDate: date("today") -> resolved RFC3339 timestamp
func expandDate(fc *query.FuncCall, ctx CompileContext) (string, []any, error) {
	token, err := stringArg(fc, 0)
	if err != nil {
		return "", nil, err
	}

	resolved, err := ctx.ResolveDate(token)
	if err != nil {
		return "", nil, fmt.Errorf("date(%q): %w", token, err)
	}

	return "?", []any{resolved.Format(time.RFC3339)}, nil
}

// expandTeam: team("backend") -> assignee = "team:backend" OR assignee IN (members...)
func expandTeam(fc *query.FuncCall, ctx CompileContext) (string, []any, error) {
	teamName, err := stringArg(fc, 0)
	if err != nil {
		return "", nil, err
	}

	members, err := ctx.TeamMembers(teamName)
	if err != nil {
		return "", nil, fmt.Errorf("team(%q): %w", teamName, err)
	}

	params := []any{"team:" + teamName}
	placeholders := []string{"?"}
	for _, m := range members {
		placeholders = append(placeholders, "?")
		params = append(params, m)
	}

	sql := fmt.Sprintf("t.assignee IN (%s)", strings.Join(placeholders, ", "))
	return sql, params, nil
}

// expandMe: me() -> current user identifier
func expandMe(ctx CompileContext) (string, []any, error) {
	user := ctx.CurrentUser()
	if user == "" {
		return "", nil, fmt.Errorf("me(): current user not configured")
	}
	return "?", []any{user}, nil
}

// expandMyTeam: my_team() -> expand to team: prefix and all member emails
func expandMyTeam(ctx CompileContext) (string, []any, error) {
	teams := ctx.CurrentUserTeams()
	if len(teams) == 0 {
		return "", nil, fmt.Errorf("my_team(): current user is not a member of any team")
	}

	var params []any
	var placeholders []string

	for _, teamName := range teams {
		placeholders = append(placeholders, "?")
		params = append(params, "team:"+teamName)

		members, err := ctx.TeamMembers(teamName)
		if err != nil {
			return "", nil, fmt.Errorf("my_team(): resolving team %q: %w", teamName, err)
		}
		for _, m := range members {
			placeholders = append(placeholders, "?")
			params = append(params, m)
		}
	}

	sql := fmt.Sprintf("t.assignee IN (%s)", strings.Join(placeholders, ", "))
	return sql, params, nil
}

// identArg extracts an identifier argument from a function call.
func identArg(fc *query.FuncCall, idx int) (string, error) {
	if idx >= len(fc.Args) {
		return "", fmt.Errorf("%s(): missing argument %d", fc.Name, idx)
	}
	switch v := fc.Args[idx].(type) {
	case query.IdentValue:
		return v.Val, nil
	case query.StringValue:
		return v.Val, nil
	default:
		return "", fmt.Errorf("%s(): argument %d must be an identifier, got %T", fc.Name, idx, fc.Args[idx])
	}
}

// stringArg extracts a string argument from a function call.
func stringArg(fc *query.FuncCall, idx int) (string, error) {
	if idx >= len(fc.Args) {
		return "", fmt.Errorf("%s(): missing argument %d", fc.Name, idx)
	}
	switch v := fc.Args[idx].(type) {
	case query.StringValue:
		return v.Val, nil
	case query.IdentValue:
		return v.Val, nil
	default:
		return "", fmt.Errorf("%s(): argument %d must be a string, got %T", fc.Name, idx, fc.Args[idx])
	}
}

// durationToMinutes converts a duration value string to minutes.
func durationToMinutes(val string) (int, error) {
	d, err := model.ParseDuration(val)
	if err != nil {
		return 0, err
	}
	return d.Minutes, nil
}
