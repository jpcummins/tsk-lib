package query

import "fmt"

// Parser turns a DSL query string into a validated AST.
type Parser interface {
	Parse(input string) (Expr, error)
}

// DefaultParser is the standard recursive-descent parser.
type DefaultParser struct{}

// NewParser creates a new DefaultParser.
func NewParser() *DefaultParser {
	return &DefaultParser{}
}

// Parse tokenizes the input and produces an AST.
func (p *DefaultParser) Parse(input string) (Expr, error) {
	tokens, err := Lex(input)
	if err != nil {
		return nil, err
	}

	parser := &parser{tokens: tokens, pos: 0}
	expr, err := parser.parseExpr()
	if err != nil {
		return nil, err
	}

	// Ensure we consumed all tokens.
	if parser.current().Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token %q at position %d",
			parser.current().Literal, parser.current().Pos)
	}

	return expr, nil
}

// parser is the internal recursive descent parser state.
type parser struct {
	tokens []Token
	pos    int
}

func (p *parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() Token {
	t := p.current()
	p.pos++
	return t
}

func (p *parser) expect(typ TokenType) (Token, error) {
	t := p.current()
	if t.Type != typ {
		return t, fmt.Errorf("expected %s, got %s (%q) at position %d",
			typ, t.Type, t.Literal, t.Pos)
	}
	p.pos++
	return t, nil
}

// parseExpr handles: term ( ("AND" | "OR") term )*
// Precedence: NOT > AND > OR, so we split into parseOrExpr and parseAndExpr.
func (p *parser) parseExpr() (Expr, error) {
	return p.parseOrExpr()
}

// parseOrExpr handles: andExpr ( "OR" andExpr )*
func (p *parser) parseOrExpr() (Expr, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenOR {
		p.advance()
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: TokenOR, Left: left, Right: right}
	}

	return left, nil
}

// parseAndExpr handles: unaryExpr ( "AND" unaryExpr )*
func (p *parser) parseAndExpr() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenAND {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: TokenAND, Left: left, Right: right}
	}

	return left, nil
}

// parseUnary handles: "NOT"? primary
func (p *parser) parseUnary() (Expr, error) {
	if p.current().Type == TokenNOT {
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: TokenNOT, Operand: operand}, nil
	}

	return p.parsePrimary()
}

// parsePrimary handles: predicate | funcCall | "(" expr ")"
func (p *parser) parsePrimary() (Expr, error) {
	tok := p.current()

	// Grouped expression.
	if tok.Type == TokenLParen {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil
	}

	// Identifier — could be a field (predicate) or function call.
	if tok.Type == TokenIdent {
		// Look ahead: is this a function call?
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TokenLParen {
			return p.parseFuncCall()
		}

		// Otherwise it's a predicate: field op value
		return p.parsePredicate()
	}

	return nil, fmt.Errorf("unexpected token %s (%q) at position %d, expected predicate or '('",
		tok.Type, name(tok), tok.Pos)
}

// parsePredicate handles: field op value
func (p *parser) parsePredicate() (Expr, error) {
	fieldTok := p.advance()
	field := fieldTok.Literal

	opTok := p.current()
	if !isOperator(opTok.Type) {
		return nil, fmt.Errorf("expected operator after %q, got %s (%q) at position %d",
			field, opTok.Type, name(opTok), opTok.Pos)
	}
	p.advance()

	// IN expects a list or value.
	if opTok.Type == TokenIN {
		val, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("parsing IN value for %q: %w", field, err)
		}
		return &Predicate{Field: field, Op: opTok.Type, Value: val}, nil
	}

	val, err := p.parseValue()
	if err != nil {
		return nil, fmt.Errorf("parsing value for %q: %w", field, err)
	}

	return &Predicate{Field: field, Op: opTok.Type, Value: val}, nil
}

// parseFuncCall handles: ident "(" [args] ")"
func (p *parser) parseFuncCall() (Expr, error) {
	nameTok := p.advance() // function name
	p.advance()            // skip "("

	var args []Value

	if p.current().Type != TokenRParen {
		for {
			val, err := p.parseValue()
			if err != nil {
				return nil, fmt.Errorf("parsing argument for %s(): %w", nameTok.Literal, err)
			}
			args = append(args, val)

			if p.current().Type != TokenComma {
				break
			}
			p.advance() // skip comma
		}
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, fmt.Errorf("expected ')' after %s() arguments: %w", nameTok.Literal, err)
	}

	return &FuncCall{Name: nameTok.Literal, Args: args}, nil
}

// parseValue parses a literal value or list.
func (p *parser) parseValue() (Value, error) {
	tok := p.current()

	switch tok.Type {
	case TokenString:
		p.advance()
		return StringValue{Val: tok.Literal}, nil

	case TokenNumber:
		p.advance()
		return NumberValue{Val: tok.Literal}, nil

	case TokenDate:
		p.advance()
		return DateValue{Val: tok.Literal}, nil

	case TokenDuration:
		p.advance()
		return DurationValue{Val: tok.Literal}, nil

	case TokenIdent:
		// Could be a bare identifier used as a value (e.g., "done", "in_progress")
		// or a function call in value position (e.g., team("backend"), me(), date("today")).
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TokenLParen {
			fc, err := p.parseFuncCallValue()
			if err != nil {
				return nil, err
			}
			return fc, nil
		}
		p.advance()
		return IdentValue{Val: tok.Literal}, nil

	case TokenLBrack:
		return p.parseList()

	default:
		return nil, fmt.Errorf("expected value, got %s (%q) at position %d",
			tok.Type, name(tok), tok.Pos)
	}
}

// parseFuncCallValue parses a function call in value position.
func (p *parser) parseFuncCallValue() (Value, error) {
	nameTok := p.advance() // function name
	p.advance()            // skip "("

	var args []Value

	if p.current().Type != TokenRParen {
		for {
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			args = append(args, val)
			if p.current().Type != TokenComma {
				break
			}
			p.advance()
		}
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return FuncValue{Call: &FuncCall{Name: nameTok.Literal, Args: args}}, nil
}

// parseList handles: "[" value ("," value)* "]"
func (p *parser) parseList() (Value, error) {
	p.advance() // skip "["
	var items []Value

	if p.current().Type != TokenRBrack {
		for {
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			items = append(items, val)
			if p.current().Type != TokenComma {
				break
			}
			p.advance()
		}
	}

	if _, err := p.expect(TokenRBrack); err != nil {
		return nil, err
	}

	return ListValue{Items: items}, nil
}

func isOperator(t TokenType) bool {
	switch t {
	case TokenEQ, TokenNEQ, TokenLT, TokenLTE, TokenGT, TokenGTE, TokenTilde, TokenIN:
		return true
	}
	return false
}

func name(t Token) string {
	if t.Literal != "" {
		return t.Literal
	}
	return t.Type.String()
}
