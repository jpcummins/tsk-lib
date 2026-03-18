package query

// TokenType represents the type of a lexer token.
type TokenType int

const (
	TokenEOF TokenType = iota

	// Literals
	TokenIdent  // field names, function names
	TokenString // "quoted string"
	TokenNumber // 42, 3.14

	// Keywords
	TokenAND
	TokenOR
	TokenNOT
	TokenIN

	// Operators
	TokenEQ    // =
	TokenNEQ   // !=
	TokenLT    // <
	TokenLTE   // <=
	TokenGT    // >
	TokenGTE   // >=
	TokenTilde // ~

	// Delimiters
	TokenLParen // (
	TokenRParen // )
	TokenLBrack // [
	TokenRBrack // ]
	TokenComma  // ,
	TokenDot    // .
)

// String returns a human-readable name for the token type.
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenIdent:
		return "IDENT"
	case TokenString:
		return "STRING"
	case TokenNumber:
		return "NUMBER"
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
	case TokenDot:
		return "."
	default:
		return "UNKNOWN"
	}
}

// Token represents a single lexer token.
type Token struct {
	Type  TokenType
	Value string
	Pos   int
}
