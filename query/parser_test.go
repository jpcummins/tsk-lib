package query

import (
	"testing"
)

// TestParse_SpecExamples tests all query examples from Section 12.1.7 of the spec.
func TestParse_SpecExamples(t *testing.T) {
	examples := []struct {
		name  string
		input string
	}{
		{
			name:  "security tasks",
			input: `summary ~ "security" AND status.category != done`,
		},
		{
			name:  "assignee IN list with due",
			input: `assignee IN ["alex@example.com", "jp"] AND due <= 2026-03-22T23:59:59Z`,
		},
		{
			name:  "team assignee",
			input: `assignee = "team:backend" AND status.category != done`,
		},
		{
			name:  "team function",
			input: `assignee = team("backend") AND status.category != done`,
		},
		{
			name:  "dependency",
			input: `dependency = "launch/plan"`,
		},
		{
			name:  "missing estimate",
			input: `status.category = in_progress AND missing(estimate)`,
		},
		{
			name:  "iteration status",
			input: `iteration.status = in_progress AND iteration.start <= date("today")`,
		},
		{
			name:  "SLA breach",
			input: `sla.id = "security-30d" AND sla.status = "breached"`,
		},
		{
			name:  "has labels",
			input: `has(labels, "capitalizable") AND status.category = done`,
		},
	}

	p := NewParser()
	for _, tt := range examples {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if expr == nil {
				t.Fatal("Parse returned nil expression")
			}
		})
	}
}

// TestParse_ComplexExpressions tests precedence and grouping.
func TestParse_ComplexExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "NOT with AND",
			input: `has(labels, "capitalizable") AND NOT has(labels, "not-capitalizable")`,
		},
		{
			name:  "grouped OR",
			input: `(status = "todo" OR status = "up_next") AND assignee = "jp"`,
		},
		{
			name:  "triple AND",
			input: `iteration.team = my_team() AND iteration.start <= date("today") AND iteration.end >= date("today") AND status.category != done`,
		},
		{
			name:  "nested NOT",
			input: `NOT (status.category = done OR status.category = todo)`,
		},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if expr == nil {
				t.Fatal("Parse returned nil expression")
			}
		})
	}
}

// TestParse_ASTStructure verifies the AST shape for a simple query.
func TestParse_ASTStructure(t *testing.T) {
	p := NewParser()
	expr, err := p.Parse(`status = "done"`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	pred, ok := expr.(*Predicate)
	if !ok {
		t.Fatalf("expected *Predicate, got %T", expr)
	}
	if pred.Field != "status" {
		t.Errorf("field = %q, want %q", pred.Field, "status")
	}
	if pred.Op != TokenEQ {
		t.Errorf("op = %v, want TokenEQ", pred.Op)
	}
	sv, ok := pred.Value.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", pred.Value)
	}
	if sv.Val != "done" {
		t.Errorf("value = %q, want %q", sv.Val, "done")
	}
}

func TestParse_FuncCallAST(t *testing.T) {
	p := NewParser()
	expr, err := p.Parse(`has(labels, "capitalizable")`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	fc, ok := expr.(*FuncCall)
	if !ok {
		t.Fatalf("expected *FuncCall, got %T", expr)
	}
	if fc.Name != "has" {
		t.Errorf("name = %q, want %q", fc.Name, "has")
	}
	if len(fc.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(fc.Args))
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"unclosed paren", "(status = done"},
		{"missing operator", "status done"},
		{"unterminated string", `status = "done`},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Parse(tt.input)
			if err == nil {
				t.Errorf("Parse(%q) expected error", tt.input)
			}
		})
	}
}

func TestValidate_ValidQueries(t *testing.T) {
	p := NewParser()
	v := NewValidator()

	queries := []string{
		`status.category = done`,
		`task.assignee = "jp"`,
		`assignee IN ["alex@example.com", "jp"]`,
		`has(labels, "capitalizable")`,
		`missing(estimate)`,
		`exists(due)`,
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			expr, err := p.Parse(q)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if err := v.Validate(expr); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

func TestValidate_InvalidQueries(t *testing.T) {
	p := NewParser()
	v := NewValidator()

	tests := []struct {
		name  string
		query string
	}{
		{"unknown field", `bogus = "foo"`},
		{"ordering on enum", `status < "foo"`},
		{"unknown function", `bogus_func()`},
		{"wrong arity", `has("x")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := p.Parse(tt.query)
			if err != nil {
				t.Skipf("Parse error (expected): %v", err)
			}
			if err := v.Validate(expr); err == nil {
				t.Errorf("Validate(%q) expected error", tt.query)
			}
		})
	}
}

func TestLex_Tokens(t *testing.T) {
	tokens, err := Lex(`status.category != done AND due <= 2026-03-22T23:59:59Z`)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	// Expected: IDENT("status.category") NEQ IDENT("done") AND IDENT("due") LTE DATE EOF
	expected := []TokenType{
		TokenIdent, TokenNEQ, TokenIdent, TokenAND,
		TokenIdent, TokenLTE, TokenDate, TokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
		return
	}

	for i, tok := range tokens {
		if tok.Type != expected[i] {
			t.Errorf("token[%d] type = %s, want %s (literal=%q)",
				i, tok.Type, expected[i], tok.Literal)
		}
	}
}
