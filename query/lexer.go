package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer tokenizes a DSL query string.
type Lexer struct {
	input  string
	pos    int
	tokens []Token
}

// Lex tokenizes the input string and returns the token stream.
func Lex(input string) ([]Token, error) {
	l := &Lexer{input: input}
	if err := l.lex(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

func (l *Lexer) lex() error {
	for l.pos < len(l.input) {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			break
		}

		ch := l.input[l.pos]

		switch {
		case ch == '(':
			l.emit(TokenLParen, "(")
		case ch == ')':
			l.emit(TokenRParen, ")")
		case ch == '[':
			l.emit(TokenLBrack, "[")
		case ch == ']':
			l.emit(TokenRBrack, "]")
		case ch == ',':
			l.emit(TokenComma, ",")
		case ch == '~':
			l.emit(TokenTilde, "~")
		case ch == '=':
			l.emit(TokenEQ, "=")
		case ch == '!' && l.peek() == '=':
			l.pos++
			l.emit(TokenNEQ, "!=")
		case ch == '<' && l.peek() == '=':
			l.pos++
			l.emit(TokenLTE, "<=")
		case ch == '<':
			l.emit(TokenLT, "<")
		case ch == '>' && l.peek() == '=':
			l.pos++
			l.emit(TokenGTE, ">=")
		case ch == '>':
			l.emit(TokenGT, ">")
		case ch == '"' || ch == '\'':
			if err := l.lexString(ch); err != nil {
				return err
			}
			continue // lexString handles pos advancement
		case isDigit(ch) || (ch == '-' && l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1])):
			l.lexNumberOrDateOrDuration()
			continue
		case isIdentStart(ch):
			l.lexIdentOrKeyword()
			continue
		default:
			return fmt.Errorf("unexpected character %q at position %d", ch, l.pos)
		}

		l.pos++
	}

	l.tokens = append(l.tokens, Token{Type: TokenEOF, Pos: l.pos})
	return nil
}

func (l *Lexer) emit(typ TokenType, lit string) {
	l.tokens = append(l.tokens, Token{Type: typ, Literal: lit, Pos: l.pos})
}

func (l *Lexer) peek() byte {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *Lexer) lexString(quote byte) error {
	start := l.pos
	l.pos++ // skip opening quote
	var sb strings.Builder

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == quote {
			l.tokens = append(l.tokens, Token{
				Type:    TokenString,
				Literal: sb.String(),
				Pos:     start,
			})
			l.pos++ // skip closing quote
			return nil
		}
		if ch == '\\' && l.pos+1 < len(l.input) {
			l.pos++
			sb.WriteByte(l.input[l.pos])
		} else {
			sb.WriteByte(ch)
		}
		l.pos++
	}

	return fmt.Errorf("unterminated string starting at position %d", start)
}

func (l *Lexer) lexNumberOrDateOrDuration() {
	start := l.pos

	// Consume the entire token (digits, dots, dashes, colons, T, Z, +, letters for duration).
	for l.pos < len(l.input) && isNumberOrDateChar(l.input[l.pos]) {
		l.pos++
	}

	lit := l.input[start:l.pos]

	// Classify: date (contains T and Z/+/-), duration (ends with m/h/d/w), or number.
	typ := classifyNumeric(lit)
	l.tokens = append(l.tokens, Token{Type: typ, Literal: lit, Pos: start})
}

func (l *Lexer) lexIdentOrKeyword() {
	start := l.pos

	for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
		l.pos++
	}

	lit := l.input[start:l.pos]

	// Check keywords.
	switch strings.ToUpper(lit) {
	case "AND":
		l.tokens = append(l.tokens, Token{Type: TokenAND, Literal: lit, Pos: start})
	case "OR":
		l.tokens = append(l.tokens, Token{Type: TokenOR, Literal: lit, Pos: start})
	case "NOT":
		l.tokens = append(l.tokens, Token{Type: TokenNOT, Literal: lit, Pos: start})
	case "IN":
		l.tokens = append(l.tokens, Token{Type: TokenIN, Literal: lit, Pos: start})
	default:
		l.tokens = append(l.tokens, Token{Type: TokenIdent, Literal: lit, Pos: start})
	}
}

// classifyNumeric determines if a numeric-like literal is a number, date, or duration.
func classifyNumeric(lit string) TokenType {
	// Duration: ends with m, h, d, or w (and the rest is numeric).
	if len(lit) > 1 {
		last := lit[len(lit)-1]
		if last == 'm' || last == 'h' || last == 'd' || last == 'w' {
			return TokenDuration
		}
	}

	// Date: contains 'T' (RFC3339 format).
	if strings.ContainsRune(lit, 'T') {
		return TokenDate
	}

	return TokenNumber
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch) || ch == '.' || ch == '_'
}

func isNumberOrDateChar(ch byte) bool {
	return isDigit(ch) || ch == '.' || ch == '-' || ch == ':' ||
		ch == 'T' || ch == 'Z' || ch == '+' ||
		ch == 'm' || ch == 'h' || ch == 'd' || ch == 'w'
}
