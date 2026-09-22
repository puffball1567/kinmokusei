package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

// Keep the application grammar deliberately bounded: a name (optionally
// qualified) followed by an optional factory argument list. In particular an
// object literal or class body on the next line must not become part of it.
func (p *Parser) parseDecorators() []*ast.Decorator {
	var decorators []*ast.Decorator
	for p.match(token.At) {
		start := p.previous()
		name, ok := p.expect(token.Identifier, "expected decorator name after '@'")
		if !ok {
			break
		}
		var expression ast.Expression = &ast.IdentifierExpr{Name: name.Lexeme, Span: name.Span}
		for p.match(token.Dot) {
			member, valid := p.expect(token.Identifier, "expected decorator member name after '.'")
			if !valid {
				return decorators
			}
			expression = &ast.MemberExpr{Object: expression, Name: member.Lexeme, NameSpan: member.Span, Span: expression.GetSpan().Merge(member.Span)}
		}
		if p.match(token.LeftParen) {
			arguments, expanded, end, valid := p.parseArguments()
			if !valid {
				return decorators
			}
			expression = &ast.CallExpr{Callee: expression, Arguments: arguments, Expanded: expanded, Span: expression.GetSpan().Merge(end.Span)}
		}
		decorator := &ast.Decorator{Expression: expression, Span: start.Span.Merge(expression.GetSpan())}
		decorators = append(decorators, decorator)
		p.decorators = append(p.decorators, decorator)
	}
	return decorators
}

func (p *Parser) attachDeclarationDecorators(declaration ast.Declaration, decorators []*ast.Decorator) {
	if len(decorators) == 0 {
		return
	}
	if class, ok := declaration.(*ast.ClassDecl); ok && class != nil {
		class.Decorators = append(decorators, class.Decorators...)
		return
	}
	p.report(token.Token{Span: decorators[0].Span}, "decorators on top-level declarations require a class")
}
