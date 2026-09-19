package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseFor(start token.Token) ast.Statement {
	if _, ok := p.expect(token.LeftParen, "expected '(' after 'for'"); !ok {
		return nil
	}
	if p.at(token.Const) || p.at(token.Let) {
		checkpoint := p.checkpoint()
		if ranged, recognized := p.tryParseForRange(start); recognized {
			return ranged
		}
		p.restore(checkpoint)
	}
	var initializer ast.Statement
	if p.match(token.Semicolon) {
		// Empty initializer.
	} else if p.match(token.Const) {
		if p.at(token.LeftBracket) {
			initializer = p.parseMultiVariable(p.previous(), true)
		} else if variable := p.parseVariable(p.previous(), true); variable != nil {
			initializer = variable
		}
	} else if p.match(token.Let) {
		if p.at(token.LeftBracket) {
			initializer = p.parseMultiVariable(p.previous(), false)
		} else if variable := p.parseVariable(p.previous(), false); variable != nil {
			initializer = variable
		}
	} else {
		initializer = p.parseSimpleStatement(true)
	}
	var condition ast.Expression
	if initializer != nil && p.previous().Kind != token.Semicolon {
		p.report(p.peek(), "expected explicit ';' after for initializer")
		return nil
	}
	if !p.at(token.Semicolon) {
		condition = p.parseExpression()
	}
	if _, ok := p.expect(token.Semicolon, "expected ';' after for condition"); !ok {
		return nil
	}
	var post ast.Statement
	if !p.at(token.RightParen) {
		post = p.parseSimpleStatement(false)
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after for clauses"); !ok {
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	return &ast.ForStmt{Initializer: initializer, Condition: condition, Post: post, Body: body, Span: start.Span.Merge(body.Span)}
}

func (p *Parser) tryParseForRange(start token.Token) (ast.Statement, bool) {
	constant := p.match(token.Const)
	if !constant && !p.match(token.Let) {
		return nil, false
	}
	bindings := make([]ast.RangeBinding, 0, 2)
	bracketed := p.match(token.LeftBracket)
	for {
		name, ok := p.expect(token.Identifier, "expected range binding name")
		if !ok {
			return nil, false
		}
		binding := ast.RangeBinding{Name: name.Lexeme, NameSpan: name.Span}
		if p.match(token.Colon) {
			binding.Type, ok = p.parseType()
			if !ok {
				return nil, false
			}
		}
		bindings = append(bindings, binding)
		if !bracketed || !p.match(token.Comma) {
			break
		}
		if p.at(token.RightBracket) {
			break
		}
	}
	if bracketed {
		if _, ok := p.expect(token.RightBracket, "expected ']' after range bindings"); !ok {
			return nil, false
		}
	}
	if !p.match(token.Of) {
		return nil, false
	}
	if bracketed && len(bindings) != 2 {
		p.report(p.previous(), "range binding list requires exactly two bindings")
	}
	source := p.parseExpression()
	if source == nil {
		p.synchronizeStatement()
		return nil, true
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after range expression"); !ok {
		return nil, true
	}
	body := p.parseBlock()
	if body == nil {
		return nil, true
	}
	return &ast.ForRangeStmt{
		Constant: constant, Bindings: bindings, Source: source,
		Body: body, Span: start.Span.Merge(body.Span),
	}, true
}
