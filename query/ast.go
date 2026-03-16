package query

// Expr is the interface for all AST expression nodes.
type Expr interface {
	exprNode() // Marker method to restrict the interface.
}

// BinaryExpr represents a boolean combination: left AND|OR right.
type BinaryExpr struct {
	Op    TokenType // TokenAND or TokenOR
	Left  Expr
	Right Expr
}

func (BinaryExpr) exprNode() {}

// UnaryExpr represents a NOT expression.
type UnaryExpr struct {
	Op      TokenType // TokenNOT
	Operand Expr
}

func (UnaryExpr) exprNode() {}

// Predicate represents a comparison: field op value.
// Example: task.status = "done", summary ~ "security"
type Predicate struct {
	Field string    // e.g., "task.status", "status.category", "summary"
	Op    TokenType // TokenEQ, TokenNEQ, TokenLT, etc.
	Value Value
}

func (Predicate) exprNode() {}

// FuncCall represents a function invocation.
// Example: has(labels, "capitalizable"), exists(estimate), team("backend"), me()
type FuncCall struct {
	Name string
	Args []Value
}

func (FuncCall) exprNode() {}

// Value is the interface for literal values in the AST.
type Value interface {
	valueNode()
}

// StringValue is a quoted or unquoted string literal.
type StringValue struct {
	Val string
}

func (StringValue) valueNode() {}

// NumberValue is a numeric literal.
type NumberValue struct {
	Val string // Raw string to preserve precision.
}

func (NumberValue) valueNode() {}

// DateValue is an RFC3339 date literal.
type DateValue struct {
	Val string // Raw RFC3339 string.
}

func (DateValue) valueNode() {}

// DurationValue is a duration literal (e.g., "2h", "1.5d").
type DurationValue struct {
	Val string
}

func (DurationValue) valueNode() {}

// ListValue is a list of values: ["a", "b", "c"].
type ListValue struct {
	Items []Value
}

func (ListValue) valueNode() {}

// IdentValue is an unquoted identifier used as a value (e.g., "done", "in_progress").
type IdentValue struct {
	Val string
}

func (IdentValue) valueNode() {}

// FuncValue wraps a function call used in value position (e.g., team("backend"), me()).
type FuncValue struct {
	Call *FuncCall
}

func (FuncValue) valueNode() {}
