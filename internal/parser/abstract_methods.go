package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseAbstractMethod(start token.Token) *ast.FunctionDecl {
	name, ok := p.expect(token.Identifier, "expected abstract method name")
	if !ok {
		return nil
	}
	return p.parseFunctionTail(start, name, true)
}
