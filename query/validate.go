package query

import (
	"strings"
	"time"

	"github.com/jpcummins/tsk-lib/model"
)

// FieldType classifies what operations are valid on a field.
type FieldType int

const (
	FieldString   FieldType = iota
	FieldEnum               // only =, !=, IN
	FieldDatetime           // supports ordering
	FieldDuration           // supports ordering
	FieldList               // accessed via has()
	FieldNumber             // supports ordering
)

// fieldInfo describes a known query field.
type fieldInfo struct {
	Namespace string
	Name      string
	Type      FieldType
}

// knownFields maps qualified field names to their type info.
var knownFields = map[string]fieldInfo{
	// Task fields
	"task.status":          {Namespace: "task", Name: "status", Type: FieldEnum},
	"task.status.category": {Namespace: "task", Name: "status.category", Type: FieldEnum},
	"task.assignee":        {Namespace: "task", Name: "assignee", Type: FieldString},
	"task.due":             {Namespace: "task", Name: "due", Type: FieldDatetime},
	"task.created_at":      {Namespace: "task", Name: "created_at", Type: FieldDatetime},
	"task.updated_at":      {Namespace: "task", Name: "updated_at", Type: FieldDatetime},
	"task.estimate":        {Namespace: "task", Name: "estimate", Type: FieldDuration},
	"task.path":            {Namespace: "task", Name: "path", Type: FieldString},
	"task.summary":         {Namespace: "task", Name: "summary", Type: FieldString},
	"task.type":            {Namespace: "task", Name: "type", Type: FieldString},
	"task.dependencies":    {Namespace: "task", Name: "dependencies", Type: FieldList},
	"task.labels":          {Namespace: "task", Name: "labels", Type: FieldList},
	"task.weight":          {Namespace: "task", Name: "weight", Type: FieldNumber},

	// Iteration fields
	"iteration.id":    {Namespace: "iteration", Name: "id", Type: FieldString},
	"iteration.team":  {Namespace: "iteration", Name: "team", Type: FieldString},
	"iteration.start": {Namespace: "iteration", Name: "start", Type: FieldDatetime},
	"iteration.end":   {Namespace: "iteration", Name: "end", Type: FieldDatetime},

	// SLA fields
	"sla.id":        {Namespace: "sla", Name: "id", Type: FieldString},
	"sla.status":    {Namespace: "sla", Name: "status", Type: FieldEnum},
	"sla.target":    {Namespace: "sla", Name: "target", Type: FieldDuration},
	"sla.elapsed":   {Namespace: "sla", Name: "elapsed", Type: FieldDuration},
	"sla.remaining": {Namespace: "sla", Name: "remaining", Type: FieldDuration},
}

// knownFunctions maps function names to their expected arity (-1 = variadic).
var knownFunctions = map[string]int{
	"exists":  1,
	"missing": 1,
	"has":     2,
	"date":    1,
	"team":    1,
	"me":      0,
	"my_team": 0,
}

// Validator validates a parsed AST.
type Validator interface {
	Validate(expr Expr, ctx *ValidationContext) model.Diagnostics
}

// ValidationContext provides context for validation.
type ValidationContext struct {
	// IsReportingContext indicates whether SLA fields are allowed.
	IsReportingContext bool
}

// DefaultValidator implements semantic validation.
type DefaultValidator struct{}

// NewValidator creates a new DefaultValidator.
func NewValidator() *DefaultValidator {
	return &DefaultValidator{}
}

// Validate performs semantic validation on the AST.
// Returns diagnostics (errors and warnings).
func (v *DefaultValidator) Validate(expr Expr, ctx *ValidationContext) model.Diagnostics {
	if ctx == nil {
		ctx = &ValidationContext{}
	}
	var diags model.Diagnostics
	v.validateExpr(expr, ctx, &diags)

	// Check cross-namespace OR rules
	v.validateCrossNamespace(expr, &diags)

	return diags
}

func (v *DefaultValidator) validateExpr(expr Expr, ctx *ValidationContext, diags *model.Diagnostics) {
	switch e := expr.(type) {
	case *BinaryExpr:
		v.validateExpr(e.Left, ctx, diags)
		v.validateExpr(e.Right, ctx, diags)

	case *UnaryExpr:
		v.validateExpr(e.Expr, ctx, diags)

	case *Predicate:
		v.validatePredicate(e, ctx, diags)

	case *FuncCall:
		v.validateFuncCall(e, ctx, diags)
	}
}

func (v *DefaultValidator) validatePredicate(pred *Predicate, ctx *ValidationContext, diags *model.Diagnostics) {
	// Resolve namespace
	qualifiedField := pred.Field
	if !strings.Contains(qualifiedField, ".") {
		// Unqualified defaults to task.*
		qualifiedField = "task." + qualifiedField
	}

	fi, known := knownFields[qualifiedField]
	if !known {
		// Check if it's a partial match (e.g., "status.category" -> "task.status.category")
		if !strings.HasPrefix(qualifiedField, "task.") {
			fi2, known2 := knownFields["task."+qualifiedField]
			if known2 {
				fi = fi2
				known = true
				qualifiedField = "task." + qualifiedField
			}
		}
	}

	if !known {
		*diags = append(*diags, model.NewWarningf(model.CodeQueryUnknownField,
			"unknown field: %s", pred.Field))
		return
	}

	// Store resolved namespace info
	pred.Namespace = fi.Namespace
	pred.FieldName = fi.Name

	// Check SLA field context
	if fi.Namespace == "sla" && !ctx.IsReportingContext {
		*diags = append(*diags, model.NewErrorf(model.CodeQueryUnqualifiedSLA,
			"SLA fields can only be used in reporting context: %s", pred.Field))
		return
	}

	// Validate operator-type compatibility
	v.validateOperator(pred, fi, diags)

	// Validate value
	v.validateValue(pred.Value, pred.Op, diags)
}

func (v *DefaultValidator) validateOperator(pred *Predicate, fi fieldInfo, diags *model.Diagnostics) {
	switch fi.Type {
	case FieldEnum:
		// Enums only support =, !=, IN
		switch pred.Op {
		case TokenEQ, TokenNEQ, TokenIN:
			// ok
		default:
			*diags = append(*diags, model.NewErrorf(model.CodeQueryInvalidOperator,
				"ordering operator %s is not valid for enum field %s", pred.Op, pred.Field))
		}

	case FieldList:
		// Lists shouldn't use comparison operators directly
		// (should be accessed via has())
		switch pred.Op {
		case TokenEQ, TokenNEQ, TokenTilde:
			// Allow these for flexibility
		default:
			*diags = append(*diags, model.NewErrorf(model.CodeQueryInvalidOperator,
				"operator %s is not valid for list field %s", pred.Op, pred.Field))
		}

	case FieldDatetime, FieldDuration, FieldNumber:
		// All operators are valid for comparable types

	case FieldString:
		// All operators are valid for strings
	}
}

func (v *DefaultValidator) validateValue(val Value, op TokenType, diags *model.Diagnostics) {
	switch value := val.(type) {
	case *StringValue:
		// Relative dates outside date() function are invalid
		if looksLikeRelativeDate(value.Val) {
			*diags = append(*diags, model.NewErrorf(model.CodeQueryInvalidValue,
				"relative dates must use date() function: %q", value.Val))
			return
		}

	case *FuncValue:
		v.validateValueFunc(value, diags)

	case *ListValue:
		for _, item := range value.Values {
			v.validateValue(item, TokenEQ, diags)
		}
	}
}

func (v *DefaultValidator) validateValueFunc(fn *FuncValue, diags *model.Diagnostics) {
	arity, known := knownFunctions[fn.Name]
	if !known {
		*diags = append(*diags, model.NewErrorf(model.CodeQueryUnknownFunction,
			"unknown function: %s", fn.Name))
		return
	}

	if len(fn.Args) != arity {
		*diags = append(*diags, model.NewErrorf(model.CodeQueryInvalidSyntax,
			"function %s expects %d arguments, got %d", fn.Name, arity, len(fn.Args)))
		return
	}

	// Validate date() arguments
	if fn.Name == "date" && len(fn.Args) == 1 {
		if sv, ok := fn.Args[0].(*StringValue); ok {
			if !isValidDateSpec(sv.Val) {
				*diags = append(*diags, model.NewErrorf(model.CodeQueryInvalidValue,
					"invalid date value: %q", sv.Val))
			}
		}
	}
}

func (v *DefaultValidator) validateFuncCall(fc *FuncCall, ctx *ValidationContext, diags *model.Diagnostics) {
	arity, known := knownFunctions[fc.Name]
	if !known {
		*diags = append(*diags, model.NewErrorf(model.CodeQueryUnknownFunction,
			"unknown function: %s", fc.Name))
		return
	}

	if len(fc.Args) != arity {
		*diags = append(*diags, model.NewErrorf(model.CodeQueryInvalidSyntax,
			"function %s expects %d arguments, got %d", fc.Name, arity, len(fc.Args)))
	}

	// For exists/missing, the arg should be a field name
	if fc.Name == "exists" || fc.Name == "missing" {
		if len(fc.Args) == 1 {
			if ident, ok := fc.Args[0].(*IdentValue); ok {
				// Validate that it's a known field
				qualifiedField := ident.Name
				if !strings.Contains(qualifiedField, ".") {
					qualifiedField = "task." + qualifiedField
				}
				if _, known := knownFields[qualifiedField]; !known {
					*diags = append(*diags, model.NewWarningf(model.CodeQueryUnknownField,
						"unknown field: %s", ident.Name))
				}
			}
		}
	}

	// For has(), first arg should be a list field, second a value
	if fc.Name == "has" && len(fc.Args) == 2 {
		if ident, ok := fc.Args[0].(*IdentValue); ok {
			qualifiedField := ident.Name
			if !strings.Contains(qualifiedField, ".") {
				qualifiedField = "task." + qualifiedField
			}
			fi, known := knownFields[qualifiedField]
			if known && fi.Type != FieldList {
				*diags = append(*diags, model.NewWarningf(model.CodeQueryUnknownField,
					"field %s is not a list field", ident.Name))
			}
		}
	}
}

// validateCrossNamespace checks that OR is not used across namespaces.
func (v *DefaultValidator) validateCrossNamespace(expr Expr, diags *model.Diagnostics) {
	namespaces := collectNamespaces(expr)
	if len(namespaces) <= 1 {
		return
	}

	// Check for OR across different namespaces
	if hasORCrossNamespace(expr) {
		*diags = append(*diags, model.NewError(model.CodeQueryORCrossNamespace,
			"OR across different namespaces (task, iteration, sla) is not allowed"))
	}
}

// collectNamespaces returns all namespaces referenced in the expression.
func collectNamespaces(expr Expr) map[string]bool {
	ns := make(map[string]bool)
	collectNamespacesHelper(expr, ns)
	return ns
}

func collectNamespacesHelper(expr Expr, ns map[string]bool) {
	switch e := expr.(type) {
	case *BinaryExpr:
		collectNamespacesHelper(e.Left, ns)
		collectNamespacesHelper(e.Right, ns)
	case *UnaryExpr:
		collectNamespacesHelper(e.Expr, ns)
	case *Predicate:
		if e.Namespace != "" {
			ns[e.Namespace] = true
		} else {
			// Resolve from field name
			field := e.Field
			if strings.HasPrefix(field, "iteration.") {
				ns["iteration"] = true
			} else if strings.HasPrefix(field, "sla.") {
				ns["sla"] = true
			} else {
				ns["task"] = true
			}
		}
	case *FuncCall:
		// Functions operate on task fields by default
		if len(e.Args) > 0 {
			if ident, ok := e.Args[0].(*IdentValue); ok {
				if strings.HasPrefix(ident.Name, "iteration.") {
					ns["iteration"] = true
				} else if strings.HasPrefix(ident.Name, "sla.") {
					ns["sla"] = true
				} else {
					ns["task"] = true
				}
			} else {
				ns["task"] = true
			}
		} else {
			ns["task"] = true
		}
	}
}

// hasORCrossNamespace checks if any OR node has children in different namespaces.
func hasORCrossNamespace(expr Expr) bool {
	switch e := expr.(type) {
	case *BinaryExpr:
		if e.Op == TokenOR {
			leftNS := collectNamespaces(e.Left)
			rightNS := collectNamespaces(e.Right)

			// Check if different namespaces exist across the OR
			for ns := range leftNS {
				for rns := range rightNS {
					if ns != rns {
						return true
					}
				}
			}
		}
		// Also recurse into children
		return hasORCrossNamespace(e.Left) || hasORCrossNamespace(e.Right)
	case *UnaryExpr:
		return hasORCrossNamespace(e.Expr)
	}
	return false
}

// --- Helper functions ---

func looksLikeRelativeDate(s string) bool {
	if len(s) < 2 {
		return false
	}
	// Patterns like "-7d", "+3d", "today", "yesterday", "tomorrow"
	switch s {
	case "today", "yesterday", "tomorrow":
		return true
	}
	last := s[len(s)-1]
	return (last == 'd' || last == 'w' || last == 'h' || last == 'm') &&
		(s[0] == '-' || s[0] == '+' || (s[0] >= '0' && s[0] <= '9'))
}

func isValidDatetime(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func isValidDuration(s string) bool {
	_, err := model.ParseDuration(s)
	return err == nil
}

func isValidDateSpec(s string) bool {
	// Valid date specs: RFC3339, or relative strings like "-7d", "today", etc.
	if isValidDatetime(s) {
		return true
	}
	if looksLikeRelativeDate(s) {
		return true
	}
	return false
}
