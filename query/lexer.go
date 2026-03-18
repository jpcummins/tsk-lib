package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer tokenizes a query string.
type Lexer struct {
	input  string
	pos    int
	tokens []Token
}

// Lex tokenizes the input string into a slice of tokens.
func Lex(input string) ([]Token, error) {
	l := &Lexer{input: input}
	if err := l.lex(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

func (l *Lexer) lex() error {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]

		// Skip whitespace
		if unicode.IsSpace(rune(ch)) {
			l.pos++
			continue
		}

		switch ch {
		case '(':
			l.emit(TokenLParen, "(")
		case ')':
			l.emit(TokenRParen, ")")
		case '[':
			l.emit(TokenLBrack, "[")
		case ']':
			l.emit(TokenRBrack, "]")
		case ',':
			l.emit(TokenComma, ",")
		case '=':
			l.emit(TokenEQ, "=")
		case '~':
			l.emit(TokenTilde, "~")
		case '!':
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
				l.tokens = append(l.tokens, Token{Type: TokenNEQ, Value: "!=", Pos: l.pos})
				l.pos += 2
				continue
			}
			return fmt.Errorf("unexpected character '!' at position %d", l.pos)
		case '<':
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
				l.tokens = append(l.tokens, Token{Type: TokenLTE, Value: "<=", Pos: l.pos})
				l.pos += 2
				continue
			}
			l.emit(TokenLT, "<")
		case '>':
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
				l.tokens = append(l.tokens, Token{Type: TokenGTE, Value: ">=", Pos: l.pos})
				l.pos += 2
				continue
			}
			l.emit(TokenGT, ">")
		case '"':
			s, err := l.lexString()
			if err != nil {
				return err
			}
			l.tokens = append(l.tokens, Token{Type: TokenString, Value: s, Pos: l.pos})
			continue // lexString advances pos
		default:
			if isIdentStart(ch) {
				l.lexIdentOrKeyword()
				continue
			}
			if isDigit(ch) || ch == '-' {
				l.lexNumber()
				continue
			}
			return fmt.Errorf("unexpected character '%c' at position %d", ch, l.pos)
		}
	}

	l.tokens = append(l.tokens, Token{Type: TokenEOF, Pos: l.pos})
	return nil
}

func (l *Lexer) emit(typ TokenType, value string) {
	l.tokens = append(l.tokens, Token{Type: typ, Value: value, Pos: l.pos})
	l.pos++
}

func (l *Lexer) lexString() (string, error) {
	start := l.pos
	l.pos++ // skip opening quote

	var b strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			l.pos++
			next := l.input[l.pos]
			switch next {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
			}
			l.pos++
			continue
		}
		if ch == '"' {
			l.pos++ // skip closing quote
			return b.String(), nil
		}
		b.WriteByte(ch)
		l.pos++
	}

	return "", fmt.Errorf("unterminated string starting at position %d", start)
}

func (l *Lexer) lexIdentOrKeyword() {
	start := l.pos
	for l.pos < len(l.input) && isIdentPart(l.input[l.pos]) {
		l.pos++
	}

	value := l.input[start:l.pos]

	// Check for keywords (case-sensitive)
	switch value {
	case "AND":
		l.tokens = append(l.tokens, Token{Type: TokenAND, Value: value, Pos: start})
	case "OR":
		l.tokens = append(l.tokens, Token{Type: TokenOR, Value: value, Pos: start})
	case "NOT":
		l.tokens = append(l.tokens, Token{Type: TokenNOT, Value: value, Pos: start})
	case "IN":
		l.tokens = append(l.tokens, Token{Type: TokenIN, Value: value, Pos: start})
	default:
		l.tokens = append(l.tokens, Token{Type: TokenIdent, Value: value, Pos: start})
	}
}

func (l *Lexer) lexNumber() {
	start := l.pos
	if l.input[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		l.pos++
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	l.tokens = append(l.tokens, Token{Type: TokenNumber, Value: l.input[start:l.pos], Pos: start})
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch) || ch == '_' || ch == '.'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
