package parser

import (
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

// Recursive descent and the resulting tree need independent limits: loops in
// the parser can build deep left-associated expressions and postfix types.
const (
	maxParserNesting = 512
	maxSyntaxDepth   = 256
)

func (p *Parser) enterNesting() bool {
	if p.nestingExceeded {
		return false
	}
	if p.nesting >= maxParserNesting {
		p.exceedNesting(p.peek().Span)
		return false
	}
	p.nesting++
	return true
}

func (p *Parser) leaveNesting() { p.nesting-- }

func (p *Parser) exceedNesting(span source.Span) {
	if p.nestingExceeded {
		return
	}
	p.diagnostics = append(p.diagnostics, diagnostic.Diagnostic{Message: "syntax nesting is too deep", Span: span})
	p.nestingToken = token.Token{Kind: token.EOF, Span: span}
	p.nestingExceeded = true
}
