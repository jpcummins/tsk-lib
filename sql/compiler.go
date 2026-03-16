package sql

import (
	"fmt"
	"strings"

	"github.com/jp/tsk-lib/query"
)

// Compiler translates a query AST into a SQL SELECT statement.
type Compiler interface {
	Compile(expr query.Expr, ctx CompileContext) (sql string, params []any, err error)
}

// DefaultCompiler is the standard SQL compiler targeting the tsk SQLite schema.
type DefaultCompiler struct{}

// NewCompiler creates a new DefaultCompiler.
func NewCompiler() *DefaultCompiler {
	return &DefaultCompiler{}
}

// Compile takes a validated query AST and produces a SQL SELECT statement
// that returns task rows matching the query.
func (c *DefaultCompiler) Compile(expr query.Expr, ctx CompileContext) (string, []any, error) {
	state := &compileState{ctx: ctx}

	whereClause, params, err := state.compileExpr(expr)
	if err != nil {
		return "", nil, err
	}

	// Build the full SELECT statement.
	var sb strings.Builder
	sb.WriteString("SELECT t.canonical_path, t.parent_path, t.date, t.due, t.assignee, ")
	sb.WriteString("t.summary, t.estimate_mins, t.status, t.status_category, ")
	sb.WriteString("t.updated_at, t.weight, t.body, t.is_readme\n")
	sb.WriteString("FROM tasks t\n")

	// Add JOINs if needed.
	if state.needsIterJoin {
		sb.WriteString("JOIN iteration_tasks it ON it.task_path = t.canonical_path\n")
		sb.WriteString("JOIN iterations iter ON iter.canonical_path = it.iteration_path\n")
	}
	if state.needsSLAJoin {
		sb.WriteString("JOIN sla_results sla ON sla.task_path = t.canonical_path\n")
	}

	sb.WriteString("WHERE ")
	sb.WriteString(whereClause)

	// Deduplicate if joins could produce multiple rows per task.
	if state.needsIterJoin || state.needsSLAJoin {
		sb.WriteString("\nGROUP BY t.canonical_path")
	}

	return sb.String(), params, nil
}

// compileState tracks JOINs needed during compilation.
type compileState struct {
	ctx           CompileContext
	needsIterJoin bool
	needsSLAJoin  bool
}

// compileExpr recursively compiles an expression into a SQL WHERE fragment.
func (s *compileState) compileExpr(expr query.Expr) (string, []any, error) {
	switch e := expr.(type) {
	case *query.BinaryExpr:
		return s.compileBinary(e)
	case *query.UnaryExpr:
		return s.compileUnary(e)
	case *query.Predicate:
		return s.compilePredicate(e)
	case *query.FuncCall:
		return s.compileFunc(e)
	default:
		return "", nil, fmt.Errorf("unknown expression type %T", expr)
	}
}

func (s *compileState) compileBinary(e *query.BinaryExpr) (string, []any, error) {
	left, lParams, err := s.compileExpr(e.Left)
	if err != nil {
		return "", nil, err
	}
	right, rParams, err := s.compileExpr(e.Right)
	if err != nil {
		return "", nil, err
	}

	op := "AND"
	if e.Op == query.TokenOR {
		op = "OR"
	}

	sql := fmt.Sprintf("(%s %s %s)", left, op, right)
	return sql, append(lParams, rParams...), nil
}

func (s *compileState) compileUnary(e *query.UnaryExpr) (string, []any, error) {
	operand, params, err := s.compileExpr(e.Operand)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("NOT (%s)", operand), params, nil
}

func (s *compileState) compilePredicate(p *query.Predicate) (string, []any, error) {
	info, ok := lookupField(p.Field)
	if !ok {
		return "", nil, fmt.Errorf("unknown field %q", p.Field)
	}

	// Track required JOINs.
	if info.needsIterJoin {
		s.needsIterJoin = true
	}
	if info.needsSLAJoin {
		s.needsSLAJoin = true
	}

	// Handle relation fields (dependency, labels) with subqueries.
	if info.isRelation {
		return s.compileRelationPredicate(p, info)
	}

	// Handle IN operator specially.
	if p.Op == query.TokenIN {
		return s.compileIN(info.column, p.Value)
	}

	// If the value is a function that expands to a self-contained expression
	// (e.g., team("backend") -> "t.assignee IN (?, ?, ?)"), use it directly.
	if fv, ok := p.Value.(query.FuncValue); ok {
		valSQL, valParams, err := expandFunc(fv.Call, s.ctx)
		if err != nil {
			return "", nil, err
		}
		// Functions like team(), me(), my_team() expand to expressions
		// that already include the column reference.
		switch fv.Call.Name {
		case "team", "my_team":
			// These expand to "t.assignee IN (...)" — use directly.
			return valSQL, valParams, nil
		case "me":
			// me() expands to just "?" — use as a normal value.
			return fmt.Sprintf("%s = %s", info.column, valSQL), valParams, nil
		case "date":
			// date() expands to "?" — use as a normal value.
			sqlOp, err := mapOperator(p.Op)
			if err != nil {
				return "", nil, err
			}
			return fmt.Sprintf("%s %s %s", info.column, sqlOp, valSQL), valParams, nil
		default:
			return fmt.Sprintf("%s = %s", info.column, valSQL), valParams, nil
		}
	}

	// Resolve the RHS value.
	valSQL, valParams, err := s.compileValue(p.Value, info)
	if err != nil {
		return "", nil, err
	}

	// Map DSL operator to SQL operator.
	sqlOp, err := mapOperator(p.Op)
	if err != nil {
		return "", nil, err
	}

	// Tilde (~) is case-insensitive contains.
	if p.Op == query.TokenTilde {
		return fmt.Sprintf("%s LIKE '%%' || ? || '%%' COLLATE NOCASE", info.column), valParams, nil
	}

	sql := fmt.Sprintf("%s %s %s", info.column, sqlOp, valSQL)
	return sql, valParams, nil
}

func (s *compileState) compileRelationPredicate(p *query.Predicate, info fieldInfo) (string, []any, error) {
	valSQL, valParams, err := s.compileValue(p.Value, info)
	if err != nil {
		return "", nil, err
	}

	sqlOp, err := mapOperator(p.Op)
	if err != nil {
		return "", nil, err
	}

	if p.Op == query.TokenTilde {
		sql := fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE task_path = t.canonical_path AND %s LIKE '%%' || ? || '%%' COLLATE NOCASE)",
			info.relation, info.relColumn)
		return sql, valParams, nil
	}

	_ = valSQL
	sql := fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE task_path = t.canonical_path AND %s %s ?)",
		info.relation, info.relColumn, sqlOp)
	return sql, valParams, nil
}

func (s *compileState) compileIN(column string, val query.Value) (string, []any, error) {
	list, ok := val.(query.ListValue)
	if !ok {
		// Single value IN.
		valStr, err := extractStringValue(val)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s IN (?)", column), []any{valStr}, nil
	}

	placeholders := make([]string, len(list.Items))
	params := make([]any, len(list.Items))
	for i, item := range list.Items {
		placeholders[i] = "?"
		v, err := extractStringValue(item)
		if err != nil {
			return "", nil, err
		}
		params[i] = v
	}

	sql := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	return sql, params, nil
}

func (s *compileState) compileFunc(fc *query.FuncCall) (string, []any, error) {
	return expandFunc(fc, s.ctx)
}

func (s *compileState) compileValue(val query.Value, info fieldInfo) (string, []any, error) {
	switch v := val.(type) {
	case query.StringValue:
		return "?", []any{v.Val}, nil

	case query.IdentValue:
		return "?", []any{v.Val}, nil

	case query.NumberValue:
		return "?", []any{v.Val}, nil

	case query.DateValue:
		return "?", []any{v.Val}, nil

	case query.DurationValue:
		if info.isDuration {
			mins, err := durationToMinutes(v.Val)
			if err != nil {
				return "", nil, err
			}
			return "?", []any{mins}, nil
		}
		return "?", []any{v.Val}, nil

	case query.FuncValue:
		return expandFunc(v.Call, s.ctx)

	case query.ListValue:
		// Lists are handled by compileIN.
		return "?", nil, nil

	default:
		return "", nil, fmt.Errorf("unsupported value type %T", val)
	}
}

func mapOperator(op query.TokenType) (string, error) {
	switch op {
	case query.TokenEQ:
		return "=", nil
	case query.TokenNEQ:
		return "!=", nil
	case query.TokenLT:
		return "<", nil
	case query.TokenLTE:
		return "<=", nil
	case query.TokenGT:
		return ">", nil
	case query.TokenGTE:
		return ">=", nil
	case query.TokenTilde:
		return "LIKE", nil
	case query.TokenIN:
		return "IN", nil
	default:
		return "", fmt.Errorf("unsupported operator %s", op)
	}
}

// extractStringValue gets the string representation from a Value.
func extractStringValue(val query.Value) (string, error) {
	switch v := val.(type) {
	case query.StringValue:
		return v.Val, nil
	case query.IdentValue:
		return v.Val, nil
	case query.NumberValue:
		return v.Val, nil
	case query.DateValue:
		return v.Val, nil
	case query.DurationValue:
		return v.Val, nil
	default:
		return "", fmt.Errorf("cannot extract string from %T", val)
	}
}
