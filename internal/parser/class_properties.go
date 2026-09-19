package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseClassAccessor(start token.Token) *ast.MethodDecl {
	name, ok := p.expect(token.Identifier, "expected property name")
	if !ok {
		return nil
	}
	if _, ok = p.expect(token.LeftParen, "expected '(' after property name"); !ok {
		return nil
	}
	parameters, ok := p.parseParameters(token.RightParen)
	if !ok {
		p.synchronizeTo(token.RightParen)
	}
	if _, valid := p.expect(token.RightParen, "expected ')' after accessor parameters"); !valid {
		return nil
	}
	result := ast.TypeRef{Name: "void", Span: name.Span}
	if p.match(token.Colon) {
		var valid bool
		result, valid = p.parseType()
		if !valid {
			return nil
		}
	} else if start.Lexeme == "get" {
		p.report(p.peek(), "getter requires an explicit return type")
	}
	body := p.parseBlock()
	if body == nil || !ok {
		return nil
	}
	return &ast.MethodDecl{Accessor: start.Lexeme, Name: name.Lexeme, NameSpan: name.Span,
		Parameters: parameters, ReturnType: result, Body: body, Span: start.Span.Merge(body.Span)}
}
