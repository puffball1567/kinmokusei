package parser

import (
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

// Terminators are recognized only where a complete statement/declaration may
// end. Do not insert tokens in the lexer: newlines inside expressions, calls,
// types, and generic parameter lists must retain their ordinary meaning.
func (p *Parser) lineBreakBeforeNext() bool {
	return p.current > 0 && p.peek().Span.Start.Line > p.previous().Span.End.Line
}

func (p *Parser) implicitTerminator() bool {
	return p.at(token.RightBrace) || p.at(token.EOF) || p.lineBreakBeforeNext()
}

func (p *Parser) expectTerminator(message string) (token.Token, bool) {
	if p.match(token.Semicolon) {
		return p.previous(), true
	}
	if p.implicitTerminator() {
		previous := p.previous()
		return token.Token{Kind: token.Semicolon, Span: source.Span{
			Path: previous.Span.Path, Start: previous.Span.End, End: previous.Span.End,
		}}, true
	}
	return p.expect(token.Semicolon, message)
}
