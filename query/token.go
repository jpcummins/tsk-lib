// Package query implements the DSL parser for the tsk query language (Section 12.1).
// It produces an AST from a query string, which can be consumed by the sql/ package
// for compilation into SQL, or by any other execution backend.
package query

// TokenType identifies the type of a lexer token.
type TokenType int

const (
	// Literals and identifiers.
	TokenIdent    TokenType = iota // field names, unquoted values like "done"
	TokenString                    // "quoted string"
	TokenNumber                    // 42, 3.14
	TokenDuration                  // 2h, 1.5d, 30m, 1w
	TokenDate                      // 2026-04-01T17:00:00Z

	// Keywords.
	TokenAND // AND
	TokenOR  // OR
	TokenNOT // NOT
	TokenIN  // IN

	// Operators.
	TokenEQ    // =
	TokenNEQ   // !=
	TokenLT    // <
	TokenLTE   // <=
	TokenGT    // >
	TokenGTE   // >=
	TokenTilde // ~ (contains)

	// Delimiters.
	TokenLParen // (
	TokenRParen // )
	TokenLBrack // [
	TokenRBrack // ]
	TokenComma  // ,

	// Special.
	TokenEOF
)

// Token is a single lexer token with its type, literal value, and position.
type Token struct {
	Type    TokenType
	Literal string
	Pos     int // Byte offset in the input string.
}

// String returns a human-readable name for the token type.
func (t TokenType) String() string {
	switch t {
	case TokenIdent:
		return "IDENT"
	case TokenString:
		return "STRING"
	case TokenNumber:
		return "NUMBER"
	case TokenDuration:
		return "DURATION"
	case TokenDate:
		return "DATE"
	case TokenAND:
		return "AND"
	case TokenOR:
		return "OR"
	case TokenNOT:
		return "NOT"
	case TokenIN:
		return "IN"
	case TokenEQ:
		return "="
	case TokenNEQ:
		return "!="
	case TokenLT:
		return "<"
	case TokenLTE:
		return "<="
	case TokenGT:
		return ">"
	case TokenGTE:
		return ">="
	case TokenTilde:
		return "~"
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenLBrack:
		return "["
	case TokenRBrack:
		return "]"
	case TokenComma:
		return ","
	case TokenEOF:
		return "EOF"
	default:
		return "UNKNOWN"
	}
}
