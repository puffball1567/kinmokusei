package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
	"strconv"
)

func (p *Parser) parseOptionalChain(object ast.Expression) ast.Expression {
	operator := p.advance()
	receiver := &ast.OptionalReceiverExpr{Marker: "\x00optional" + strconv.Itoa(operator.Span.Start.Offset)}
	var value ast.Expression
	switch {
	case p.match(token.LeftParen):
		args, expanded, end, ok := p.parseArguments()
		if !ok {
			return object
		}
		value = &ast.CallExpr{Callee: receiver, Arguments: args, Expanded: expanded, Span: object.GetSpan().Merge(end.Span)}
	case p.match(token.LeftBracket):
		value = p.parseSubscript(receiver)
	default:
		name := p.peek()
		if !token.IsIdentifierName(name.Kind) {
			p.report(name, "expected member, call or index after '?.'")
			return object
		}
		p.advance()
		value = &ast.MemberExpr{Object: receiver, Name: name.Lexeme, NameSpan: name.Span, Span: object.GetSpan().Merge(name.Span)}
	}
	value = p.parseCallSuffix(value, true)
	return &ast.OptionalChainExpr{Object: object, Value: value, Receiver: receiver, Span: object.GetSpan().Merge(value.GetSpan())}
}
