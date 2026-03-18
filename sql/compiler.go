package sql

import (
	"fmt"
	"strings"

	"github.com/jpcummins/tsk-lib/query"
)

// Compiler compiles a validated query AST into a parameterized SQL query.
type Compiler interface {
	Compile(expr query.Expr, ctx CompileContext) (string, []any, error)
}

// CompileContext provides runtime context for SQL generation.
type CompileContext interface {
	CurrentUser() string
	CurrentUserAliases() []string
	CurrentUserTeams() []string
	TeamMembers(teamName string) []string
	ResolveDate(spec string) string
}

// DefaultCompiler implements the SQL compiler.
type DefaultCompiler struct{}

// NewCompiler creates a new DefaultCompiler.
func NewCompiler() *DefaultCompiler {
	return &DefaultCompiler{}
}

// compileState tracks JOIN requirements during compilation.
type compileState struct {
	params        []any
	needsIterJoin bool
	needsSLAJoin  bool
}

// Compile generates a parameterized SQL SELECT from a query AST.
func (c *DefaultCompiler) Compile(expr query.Expr, ctx CompileContext) (string, []any, error) {
	state := &compileState{}

	where, err := compileExpr(expr, state, ctx)
	if err != nil {
		return "", nil, err
	}

	var sb strings.Builder
	sb.WriteString("SELECT DISTINCT t.path FROM tasks t")

	if state.needsIterJoin {
		sb.WriteString(" JOIN iteration_tasks it ON t.path = it.task_path")
		sb.WriteString(" JOIN iterations i ON it.iteration_id = i.id")
	}

	if state.needsSLAJoin {
		sb.WriteString(" JOIN sla_results sr ON t.path = sr.task_path")
	}

	sb.WriteString(" WHERE t.is_stub = 0")
	if where != "" {
		sb.WriteString(" AND (")
		sb.WriteString(where)
		sb.WriteString(")")
	}

	return sb.String(), state.params, nil
}

func compileExpr(expr query.Expr, state *compileState, ctx CompileContext) (string, error) {
	switch e := expr.(type) {
	case *query.BinaryExpr:
		left, err := compileExpr(e.Left, state, ctx)
		if err != nil {
			return "", err
		}
		right, err := compileExpr(e.Right, state, ctx)
		if err != nil {
			return "", err
		}
		op := "AND"
		if e.Op == query.TokenOR {
			op = "OR"
		}
		return fmt.Sprintf("(%s %s %s)", left, op, right), nil

	case *query.UnaryExpr:
		inner, err := compileExpr(e.Expr, state, ctx)
		if err != nil {
			return "", err
		}
		// NOT on iteration/sla negates existence
		return fmt.Sprintf("NOT (%s)", inner), nil

	case *query.Predicate:
		return compilePredicate(e, state, ctx)

	case *query.FuncCall:
		return compileFuncCall(e, state, ctx)
	}

	return "", fmt.Errorf("unknown expression type: %T", expr)
}

func compilePredicate(pred *query.Predicate, state *compileState, ctx CompileContext) (string, error) {
	field := resolveField(pred)

	// Check for iteration/sla fields
	if strings.HasPrefix(field, "iteration.") {
		state.needsIterJoin = true
	}
	if strings.HasPrefix(field, "sla.") {
		state.needsSLAJoin = true
	}

	// Get SQL column
	col, ok := fieldMapping[field]
	if !ok {
		return "0", nil // unknown field -> always false
	}

	// Compile operator and value
	sqlOp := compileSQLOp(pred.Op)

	switch v := pred.Value.(type) {
	case *query.StringValue:
		if pred.Op == query.TokenTilde {
			state.params = append(state.params, "%"+v.Val+"%")
			return fmt.Sprintf("%s LIKE ? COLLATE NOCASE", col), nil
		}
		state.params = append(state.params, v.Val)
		return fmt.Sprintf("%s %s ?", col, sqlOp), nil

	case *query.NumberValue:
		state.params = append(state.params, v.Val)
		return fmt.Sprintf("%s %s ?", col, sqlOp), nil

	case *query.ListValue:
		placeholders := make([]string, len(v.Values))
		for i, item := range v.Values {
			if sv, ok := item.(*query.StringValue); ok {
				state.params = append(state.params, sv.Val)
			}
			placeholders[i] = "?"
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")), nil

	case *query.IdentValue:
		// Unquoted identifiers in value position are treated as string literals.
		state.params = append(state.params, v.Name)
		return fmt.Sprintf("%s %s ?", col, sqlOp), nil

	case *query.FuncValue:
		return compileFuncValuePredicate(col, sqlOp, v, state, ctx)
	}

	return "0", nil
}

func compileFuncCall(fc *query.FuncCall, state *compileState, ctx CompileContext) (string, error) {
	switch fc.Name {
	case "exists":
		if len(fc.Args) == 1 {
			if ident, ok := fc.Args[0].(*query.IdentValue); ok {
				field := "task." + ident.Name
				if col, ok := fieldMapping[field]; ok {
					return fmt.Sprintf("%s IS NOT NULL AND %s != ''", col, col), nil
				}
			}
		}

	case "missing":
		if len(fc.Args) == 1 {
			if ident, ok := fc.Args[0].(*query.IdentValue); ok {
				field := "task." + ident.Name
				if col, ok := fieldMapping[field]; ok {
					return fmt.Sprintf("(%s IS NULL OR %s = '')", col, col), nil
				}
			}
		}

	case "has":
		if len(fc.Args) == 2 {
			if ident, ok := fc.Args[0].(*query.IdentValue); ok {
				if sv, ok := fc.Args[1].(*query.StringValue); ok {
					field := ident.Name
					if !strings.Contains(field, ".") {
						field = "task." + field
					}
					if table, ok := relationFields[field]; ok {
						state.params = append(state.params, sv.Val)
						return fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE task_path = t.path AND value = ?)", table), nil
					}
				}
			}
		}
	}

	return "0", nil
}

func compileFuncValuePredicate(col, sqlOp string, fn *query.FuncValue, state *compileState, ctx CompileContext) (string, error) {
	switch fn.Name {
	case "date":
		if len(fn.Args) == 1 {
			if sv, ok := fn.Args[0].(*query.StringValue); ok {
				resolved := ctx.ResolveDate(sv.Val)
				state.params = append(state.params, resolved)
				return fmt.Sprintf("%s %s ?", col, sqlOp), nil
			}
		}

	case "team":
		if len(fn.Args) == 1 {
			if sv, ok := fn.Args[0].(*query.StringValue); ok {
				members := ctx.TeamMembers(sv.Val)
				values := append([]string{"team:" + sv.Val}, members...)
				placeholders := make([]string, len(values))
				for i, v := range values {
					state.params = append(state.params, v)
					placeholders[i] = "?"
				}
				return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")), nil
			}
		}

	case "me":
		aliases := ctx.CurrentUserAliases()
		if len(aliases) <= 1 {
			// No aliases resolved; fall back to direct comparison.
			state.params = append(state.params, ctx.CurrentUser())
			return fmt.Sprintf("%s %s ?", col, sqlOp), nil
		}
		// Match against all known aliases (identifier, email, etc.).
		placeholders := make([]string, len(aliases))
		for i, a := range aliases {
			state.params = append(state.params, a)
			placeholders[i] = "?"
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")), nil

	case "my_team":
		teams := ctx.CurrentUserTeams()
		if len(teams) == 0 {
			return "0", nil
		}
		placeholders := make([]string, len(teams))
		for i, t := range teams {
			state.params = append(state.params, "team:"+t)
			placeholders[i] = "?"
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")), nil
	}

	return "0", nil
}

func resolveField(pred *query.Predicate) string {
	if pred.Namespace != "" {
		return pred.Namespace + "." + pred.FieldName
	}
	field := pred.Field
	if !strings.Contains(field, ".") ||
		(!strings.HasPrefix(field, "task.") &&
			!strings.HasPrefix(field, "iteration.") &&
			!strings.HasPrefix(field, "sla.")) {
		return "task." + field
	}
	return field
}

func compileSQLOp(op query.TokenType) string {
	switch op {
	case query.TokenEQ:
		return "="
	case query.TokenNEQ:
		return "!="
	case query.TokenLT:
		return "<"
	case query.TokenLTE:
		return "<="
	case query.TokenGT:
		return ">"
	case query.TokenGTE:
		return ">="
	case query.TokenTilde:
		return "LIKE"
	default:
		return "="
	}
}
