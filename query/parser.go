package query

import (
	"fmt"
	"strings"
)

// Parser parses a query string into an AST.
type Parser interface {
	Parse(input string) (Expr, error)
}

// DefaultParser implements the recursive descent parser.
type DefaultParser struct{}

// NewParser creates a new DefaultParser.
func NewParser() *DefaultParser {
	return &DefaultParser{}
}

// Parse tokenizes and parses the input string into an AST.
func (p *DefaultParser) Parse(input string) (Expr, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty expression")
	}

	tokens, err := Lex(input)
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}

	parser := &parser{tokens: tokens, pos: 0}
	expr, err := parser.parseExpr()
	if err != nil {
		return nil, err
	}

	if parser.current().Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token %q at position %d",
			parser.current().Value, parser.current().Pos)
	}

	return expr, nil
}

// parser is the internal recursive descent parser state.
type parser struct {
	tokens []Token
	pos    int
}

func (p *parser) current() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF}
}

func (p *parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(typ TokenType) (Token, error) {
	tok := p.current()
	if tok.Type != typ {
		return tok, fmt.Errorf("expected %s, got %s %q at position %d",
			typ, tok.Type, tok.Value, tok.Pos)
	}
	p.advance()
	return tok, nil
}

// parseExpr: expr = term ((AND | OR) term)*
// Precedence: NOT > AND > OR, so we parse OR at the top level.
func (p *parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

// parseOr: or_expr = and_expr (OR and_expr)*
func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenOR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: TokenOR, Left: left, Right: right}
	}

	return left, nil
}

// parseAnd: and_expr = unary (AND unary)*
func (p *parser) parseAnd() (Expr, error) {
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

// parseUnary: unary = NOT unary | primary
func (p *parser) parseUnary() (Expr, error) {
	if p.current().Type == TokenNOT {
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: TokenNOT, Expr: expr}, nil
	}

	return p.parsePrimary()
}

// parsePrimary: primary = grouped | func_call | predicate
func (p *parser) parsePrimary() (Expr, error) {
	// Grouped expression
	if p.current().Type == TokenLParen {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, fmt.Errorf("unmatched parenthesis: %w", err)
		}
		return expr, nil
	}

	// Must be an identifier (field name or function name)
	if p.current().Type != TokenIdent {
		return nil, fmt.Errorf("expected field name or function, got %s %q at position %d",
			p.current().Type, p.current().Value, p.current().Pos)
	}

	ident := p.advance()

	// Check if it's a function call: ident(
	if p.current().Type == TokenLParen {
		return p.parseFuncCall(ident.Value)
	}

	// It's a field name. Could have a dot-qualified part remaining in the ident
	// (since our lexer treats dots as ident parts).
	field := ident.Value

	// Parse operator
	op := p.current()
	switch op.Type {
	case TokenEQ, TokenNEQ, TokenLT, TokenLTE, TokenGT, TokenGTE, TokenTilde, TokenIN:
		p.advance()
	default:
		return nil, fmt.Errorf("expected operator, got %s %q at position %d",
			op.Type, op.Value, op.Pos)
	}

	// Parse value
	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &Predicate{
		Field: field,
		Op:    op.Type,
		Value: val,
	}, nil
}

// parseFuncCall parses a function call: name(args...)
func (p *parser) parseFuncCall(name string) (Expr, error) {
	p.advance() // skip (

	var args []Value
	if p.current().Type != TokenRParen {
		for {
			val, err := p.parseFuncArg()
			if err != nil {
				return nil, err
			}
			args = append(args, val)
			if p.current().Type != TokenComma {
				break
			}
			p.advance() // skip comma
		}
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, fmt.Errorf("expected ) for function %s: %w", name, err)
	}

	return &FuncCall{Name: name, Args: args}, nil
}

// parseFuncArg parses a function argument — either a value or an unquoted identifier (field name).
func (p *parser) parseFuncArg() (Value, error) {
	tok := p.current()

	switch tok.Type {
	case TokenString:
		p.advance()
		return &StringValue{Val: tok.Value}, nil
	case TokenNumber:
		p.advance()
		return &NumberValue{Val: tok.Value}, nil
	case TokenIdent:
		p.advance()
		return &IdentValue{Name: tok.Value}, nil
	default:
		return nil, fmt.Errorf("expected function argument, got %s %q at position %d",
			tok.Type, tok.Value, tok.Pos)
	}
}

// parseValue parses a value expression on the right side of an operator.
func (p *parser) parseValue() (Value, error) {
	tok := p.current()

	switch tok.Type {
	case TokenString:
		p.advance()
		return &StringValue{Val: tok.Value}, nil

	case TokenNumber:
		p.advance()
		return &NumberValue{Val: tok.Value}, nil

	case TokenLBrack:
		return p.parseList()

	case TokenIdent:
		// Could be a function call in value position: func(args...)
		p.advance()
		if p.current().Type == TokenLParen {
			return p.parseValueFuncCall(tok.Value)
		}
		return &IdentValue{Name: tok.Value}, nil

	default:
		return nil, fmt.Errorf("expected value, got %s %q at position %d",
			tok.Type, tok.Value, tok.Pos)
	}
}

// parseValueFuncCall parses a function call in value position.
func (p *parser) parseValueFuncCall(name string) (Value, error) {
	p.advance() // skip (

	var args []Value
	if p.current().Type != TokenRParen {
		for {
			val, err := p.parseFuncArg()
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
		return nil, fmt.Errorf("expected ) for function %s: %w", name, err)
	}

	return &FuncValue{Name: name, Args: args}, nil
}

// parseList parses a list literal: [val1, val2, ...]
func (p *parser) parseList() (Value, error) {
	p.advance() // skip [

	var values []Value
	for p.current().Type != TokenRBrack {
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		values = append(values, val)
		if p.current().Type == TokenComma {
			p.advance()
		}
	}

	if _, err := p.expect(TokenRBrack); err != nil {
		return nil, fmt.Errorf("expected ]: %w", err)
	}

	return &ListValue{Values: values}, nil
}
