package query

// Expr is the interface for all AST expression nodes.
type Expr interface {
	exprNode()
}

// BinaryExpr represents a binary expression (AND, OR).
type BinaryExpr struct {
	Op    TokenType // TokenAND or TokenOR
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode() {}

// UnaryExpr represents a unary expression (NOT).
type UnaryExpr struct {
	Op   TokenType // TokenNOT
	Expr Expr
}

func (*UnaryExpr) exprNode() {}

// Predicate represents a field comparison: field op value.
type Predicate struct {
	Field     string // qualified or unqualified field name
	Namespace string // "task", "iteration", "sla" (resolved during validation)
	FieldName string // unqualified field name (resolved during validation)
	Op        TokenType
	Value     Value
}

func (*Predicate) exprNode() {}

// FuncCall represents a predicate function call: func(args...).
type FuncCall struct {
	Name string
	Args []Value
}

func (*FuncCall) exprNode() {}

// Value is the interface for all AST value nodes.
type Value interface {
	valueNode()
}

// StringValue represents a quoted string literal.
type StringValue struct {
	Val string
}

func (*StringValue) valueNode() {}

// NumberValue represents a numeric literal.
type NumberValue struct {
	Val string // kept as string for precision
}

func (*NumberValue) valueNode() {}

// ListValue represents a list literal: [val1, val2, ...].
type ListValue struct {
	Values []Value
}

func (*ListValue) valueNode() {}

// IdentValue represents an unquoted identifier used as a value (e.g., field ref in function args).
type IdentValue struct {
	Name string
}

func (*IdentValue) valueNode() {}

// FuncValue represents a function call used in value position.
type FuncValue struct {
	Name string
	Args []Value
}

func (*FuncValue) valueNode() {}
