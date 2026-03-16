package query

import "fmt"

// Validator checks a query AST for semantic correctness:
// valid field names, operator/type compatibility, function arities.
type Validator interface {
	Validate(expr Expr) error
}

// DefaultValidator is the standard query validator.
type DefaultValidator struct{}

// NewValidator creates a new DefaultValidator.
func NewValidator() *DefaultValidator {
	return &DefaultValidator{}
}

// FieldType describes the type of a queryable field.
type FieldType int

const (
	FieldString   FieldType = iota
	FieldEnum               // status, status.category — only =, !=, IN
	FieldDatetime           // dates — supports ordering
	FieldDuration           // durations — supports ordering
	FieldList               // labels, dependency — use has()
)

// validFields maps known field names to their types (Section 12.1.4.1).
var validFields = map[string]FieldType{
	// Task fields.
	"task.status":          FieldEnum,
	"task.status.category": FieldEnum,
	"task.assignee":        FieldString,
	"task.due":             FieldDatetime,
	"task.date":            FieldDatetime,
	"task.updated_at":      FieldDatetime,
	"task.estimate":        FieldDuration,
	"task.path":            FieldString,
	"task.summary":         FieldString,
	"task.dependency":      FieldList,
	"task.labels":          FieldList,

	// Unqualified task fields (default to task.*).
	"status":          FieldEnum,
	"status.category": FieldEnum,
	"assignee":        FieldString,
	"due":             FieldDatetime,
	"date":            FieldDatetime,
	"updated_at":      FieldDatetime,
	"estimate":        FieldDuration,
	"path":            FieldString,
	"summary":         FieldString,
	"dependency":      FieldList,
	"labels":          FieldList,

	// Iteration fields.
	"iteration.team":            FieldString,
	"iteration.status":          FieldEnum,
	"iteration.status.category": FieldEnum,
	"iteration.start":           FieldDatetime,
	"iteration.end":             FieldDatetime,
	"iteration.path":            FieldString,

	// SLA fields (reporting only).
	"sla.id":        FieldString,
	"sla.status":    FieldEnum,
	"sla.target":    FieldDuration,
	"sla.elapsed":   FieldDuration,
	"sla.remaining": FieldDuration,
}

// validFunctions maps function names to their expected arity (-1 for variadic).
var validFunctions = map[string]int{
	"exists":  1,
	"missing": 1,
	"has":     2,
	"date":    1,
	"team":    1,
	"me":      0,
	"my_team": 0,
}

// Validate checks the AST for semantic correctness.
func (v *DefaultValidator) Validate(expr Expr) error {
	return validateExpr(expr)
}

func validateExpr(expr Expr) error {
	switch e := expr.(type) {
	case *BinaryExpr:
		if err := validateExpr(e.Left); err != nil {
			return err
		}
		return validateExpr(e.Right)

	case *UnaryExpr:
		return validateExpr(e.Operand)

	case *Predicate:
		return validatePredicate(e)

	case *FuncCall:
		return validateFuncCall(e)

	default:
		return fmt.Errorf("unknown expression type %T", expr)
	}
}

func validatePredicate(p *Predicate) error {
	fieldType, ok := validFields[p.Field]
	if !ok {
		return fmt.Errorf("unknown field %q", p.Field)
	}

	// Check operator compatibility (Section 12.1.3.1).
	switch fieldType {
	case FieldEnum:
		if p.Op != TokenEQ && p.Op != TokenNEQ && p.Op != TokenIN {
			return fmt.Errorf("field %q (enum) only supports =, !=, IN; got %s", p.Field, p.Op)
		}
	case FieldList:
		// List fields support = for individual element match.
		if p.Op != TokenEQ && p.Op != TokenNEQ && p.Op != TokenTilde {
			return fmt.Errorf("field %q (list) only supports =, !=, ~; got %s; use has() for membership", p.Field, p.Op)
		}
	}

	return nil
}

func validateFuncCall(fc *FuncCall) error {
	expectedArity, ok := validFunctions[fc.Name]
	if !ok {
		return fmt.Errorf("unknown function %q", fc.Name)
	}

	if expectedArity >= 0 && len(fc.Args) != expectedArity {
		return fmt.Errorf("function %s() expects %d arguments, got %d",
			fc.Name, expectedArity, len(fc.Args))
	}

	return nil
}
