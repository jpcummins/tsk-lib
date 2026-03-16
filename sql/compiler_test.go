package sql

import (
	"strings"
	"testing"
	"time"

	"github.com/jpcummins/tsk-lib/query"
)

// mockContext implements CompileContext for testing.
type mockContext struct {
	user    string
	teams   []string
	members map[string][]string
	dateNow time.Time
}

func (m *mockContext) CurrentUser() string        { return m.user }
func (m *mockContext) CurrentUserTeams() []string { return m.teams }
func (m *mockContext) TeamMembers(team string) ([]string, error) {
	return m.members[team], nil
}
func (m *mockContext) ResolveDate(token string) (time.Time, error) {
	switch token {
	case "today":
		return m.dateNow, nil
	case "yesterday":
		return m.dateNow.AddDate(0, 0, -1), nil
	default:
		return time.Parse(time.RFC3339, token)
	}
}

func defaultCtx() *mockContext {
	return &mockContext{
		user:  "alice@example.com",
		teams: []string{"backend"},
		members: map[string][]string{
			"backend":  {"alice@example.com", "bob@example.com"},
			"frontend": {"carol@example.com"},
		},
		dateNow: time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
	}
}

func TestCompile_SimplePredicates(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSQL    string // substring that should appear
		wantParams int    // expected param count
	}{
		{
			name:       "equality",
			input:      `status = "done"`,
			wantSQL:    "t.status = ?",
			wantParams: 1,
		},
		{
			name:       "inequality",
			input:      `status.category != done`,
			wantSQL:    "t.status_category != ?",
			wantParams: 1,
		},
		{
			name:       "contains",
			input:      `summary ~ "security"`,
			wantSQL:    "LIKE '%' || ? || '%'",
			wantParams: 1,
		},
		{
			name:       "less than date",
			input:      `due <= 2026-03-22T23:59:59Z`,
			wantSQL:    "t.due <= ?",
			wantParams: 1,
		},
	}

	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			sql, params, err := c.Compile(expr, ctx)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}

			if !strings.Contains(sql, tt.wantSQL) {
				t.Errorf("SQL missing %q\ngot: %s", tt.wantSQL, sql)
			}
			if len(params) != tt.wantParams {
				t.Errorf("expected %d params, got %d: %v", tt.wantParams, len(params), params)
			}
		})
	}
}

func TestCompile_Functions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSQL string
	}{
		{
			name:    "has labels",
			input:   `has(labels, "capitalizable")`,
			wantSQL: "EXISTS (SELECT 1 FROM task_labels",
		},
		{
			name:    "missing estimate",
			input:   `missing(estimate)`,
			wantSQL: "IS NULL",
		},
		{
			name:    "exists due",
			input:   `exists(due)`,
			wantSQL: "IS NOT NULL",
		},
	}

	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			sql, _, err := c.Compile(expr, ctx)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}

			if !strings.Contains(sql, tt.wantSQL) {
				t.Errorf("SQL missing %q\ngot: %s", tt.wantSQL, sql)
			}
		})
	}
}

func TestCompile_IterationJoin(t *testing.T) {
	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	expr, err := p.Parse(`iteration.status = in_progress AND iteration.start <= date("today")`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sql, _, err := c.Compile(expr, ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !strings.Contains(sql, "JOIN iteration_tasks") {
		t.Errorf("expected iteration JOIN\ngot: %s", sql)
	}
	if !strings.Contains(sql, "JOIN iterations iter") {
		t.Errorf("expected iterations JOIN\ngot: %s", sql)
	}
	if !strings.Contains(sql, "GROUP BY") {
		t.Errorf("expected GROUP BY for deduplication\ngot: %s", sql)
	}
}

func TestCompile_TeamFunction(t *testing.T) {
	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	expr, err := p.Parse(`assignee = team("backend") AND status.category != done`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sql, params, err := c.Compile(expr, ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !strings.Contains(sql, "t.assignee IN") {
		t.Errorf("expected team expansion\ngot: %s", sql)
	}

	// Should have team:backend + alice + bob = 3 params for team, plus 1 for status.category
	if len(params) < 3 {
		t.Errorf("expected at least 3 params for team expansion, got %d: %v", len(params), params)
	}
}

func TestCompile_INOperator(t *testing.T) {
	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	expr, err := p.Parse(`assignee IN ["alex@example.com", "jp"]`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sql, params, err := c.Compile(expr, ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !strings.Contains(sql, "t.assignee IN (?, ?)") {
		t.Errorf("expected IN clause\ngot: %s", sql)
	}
	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d: %v", len(params), params)
	}
}

func TestCompile_AND_OR_NOT(t *testing.T) {
	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	expr, err := p.Parse(`has(labels, "capitalizable") AND NOT has(labels, "not-capitalizable")`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sql, params, err := c.Compile(expr, ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !strings.Contains(sql, "AND") {
		t.Errorf("expected AND\ngot: %s", sql)
	}
	if !strings.Contains(sql, "NOT") {
		t.Errorf("expected NOT\ngot: %s", sql)
	}
	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d: %v", len(params), params)
	}
}

// TestCompile_SpecExamples compiles all spec examples to ensure they produce valid SQL.
func TestCompile_SpecExamples(t *testing.T) {
	examples := []string{
		`summary ~ "security" AND status.category != done`,
		`assignee IN ["alex@example.com", "jp"] AND due <= 2026-03-22T23:59:59Z`,
		`assignee = "team:backend" AND status.category != done`,
		`assignee = team("backend") AND status.category != done`,
		`dependency = "launch/plan"`,
		`status.category = in_progress AND missing(estimate)`,
		`iteration.status = in_progress AND iteration.start <= date("today")`,
		// TODO: SLA queries disabled — see fields.go and query/validate.go
		// `sla.id = "security-30d" AND sla.status = "breached"`,
		`has(labels, "capitalizable") AND status.category = done`,
		`has(labels, "capitalizable") AND NOT has(labels, "not-capitalizable")`,
	}

	p := query.NewParser()
	c := NewCompiler()
	ctx := defaultCtx()

	for _, input := range examples {
		t.Run(input, func(t *testing.T) {
			expr, err := p.Parse(input)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			sql, params, err := c.Compile(expr, ctx)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}

			// Basic sanity checks.
			if !strings.Contains(sql, "SELECT") {
				t.Errorf("SQL missing SELECT\ngot: %s", sql)
			}
			if !strings.Contains(sql, "FROM tasks t") {
				t.Errorf("SQL missing FROM tasks\ngot: %s", sql)
			}
			if !strings.Contains(sql, "WHERE") {
				t.Errorf("SQL missing WHERE\ngot: %s", sql)
			}

			t.Logf("SQL: %s", sql)
			t.Logf("Params: %v", params)
		})
	}
}

// TODO: SLA status tests disabled — see fields.go and query/validate.go.
// Re-enable once SLA query architecture is designed.
//
// func TestCompile_SLAStatus(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		input   string
// 		wantSQL string
// 	}{
// 		{
// 			name:    "sla status breached",
// 			input:   `sla.status = "breached"`,
// 			wantSQL: "sla.status = ?",
// 		},
// 	}
// 	...
// }
