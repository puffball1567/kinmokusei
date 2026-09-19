package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) looksLikeArrow() bool {
	depth := 0
	for i := p.current; i < len(p.tokens); i++ {
		switch p.tokens[i].Kind {
		case token.LeftParen:
			depth++
		case token.RightParen:
			depth--
			if depth == 0 {
				if i+1 >= len(p.tokens) {
					return false
				}
				return p.tokens[i+1].Kind == token.FatArrow || p.tokens[i+1].Kind == token.Colon
			}
		}
	}
	return false
}

func (p *Parser) parseArrow() ast.Expression {
	start, _ := p.expect(token.LeftParen, "expected '('")
	parameters, ok := p.parseParametersWithInference(token.RightParen, true)
	if !ok {
		p.synchronizeStatement()
		return nil
	}
	if _, ok = p.expect(token.RightParen, "expected ')' after arrow parameters"); !ok {
		return nil
	}
	var returnType *ast.TypeRef
	if p.match(token.Colon) {
		parsed, valid := p.parseType()
		if !valid {
			return nil
		}
		returnType = &parsed
	}
	if _, ok = p.expectFatArrow("expected '=>' after arrow signature"); !ok {
		return nil
	}
	arrow := &ast.ArrowExpr{Parameters: parameters, ReturnType: returnType}
	if p.at(token.LeftBrace) {
		arrow.BlockBody = p.parseBlock()
		if arrow.BlockBody == nil {
			return nil
		}
		arrow.Span = start.Span.Merge(arrow.BlockBody.Span)
		return arrow
	}
	arrow.ExpressionBody = p.parseExpression()
	if arrow.ExpressionBody == nil {
		return nil
	}
	arrow.Span = start.Span.Merge(arrow.ExpressionBody.GetSpan())
	return arrow
}
