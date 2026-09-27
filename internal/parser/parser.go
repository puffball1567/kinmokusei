package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

type Parser struct {
	decorators                   []*ast.Decorator
	tokens                       []token.Token
	current                      int
	previousToken                token.Token
	tokenEdits                   []parserTokenEdit
	diagnostics                  []diagnostic.Diagnostic
	disallowUnqualifiedComposite bool
	disallowCompositeBeforeBlock bool
}

func Parse(tokens []token.Token) (*ast.Program, []diagnostic.Diagnostic) {
	// Type-context token splitting must not modify the caller's lexer output.
	p := &Parser{tokens: append([]token.Token(nil), tokens...)}
	program := &ast.Program{}
	for !p.at(token.EOF) {
		start := p.current
		if p.match(token.Import) {
			if imported, ok := p.parseImport(p.previous()); ok {
				program.Imports = append(program.Imports, imported)
			}
			continue
		}
		var decl ast.Declaration
		decorators := p.parseDecorators()
		if p.at(token.Export) && !p.atCABIExport() {
			var exported ast.ExportDecl
			exported, decl = p.parseSourceExport(p.advance())
			program.Exports = append(program.Exports, exported)
		} else {
			decl = p.parseDeclaration()
		}
		if decl != nil {
			p.attachDeclarationDecorators(decl, decorators)
			program.Declarations = append(program.Declarations, decl)
		}
		if p.current == start {
			p.advance()
		}
	}
	program.Decorators = p.decorators
	return program, p.diagnostics
}

// A generic closer can split the lexer token >= in Box<T>=> into > and =.
// Rejoin that adjacent remainder with > only where an arrow is required.
func (p *Parser) expectFatArrow(message string) (token.Token, bool) {
	if p.at(token.Assign) && p.current+1 < len(p.tokens) {
		next := p.tokens[p.current+1]
		if next.Kind == token.Greater && p.peek().Span.End.Offset == next.Span.Start.Offset {
			first := p.advance()
			last := p.advance()
			arrow := token.Token{Kind: token.FatArrow, Lexeme: "=>", Span: source.Span{
				Path: first.Span.Path, Start: first.Span.Start, End: last.Span.End,
			}}
			p.previousToken = arrow
			return arrow, true
		}
	}
	return p.expect(token.FatArrow, message)
}

func (p *Parser) expect(kind token.Kind, message string) (token.Token, bool) {
	if p.at(kind) {
		return p.advance(), true
	}
	p.report(p.peek(), message)
	return p.peek(), false
}

// expectTypeGreater consumes one closer only in a type context. The remaining
// suffix is still a token: >> becomes >, >= becomes =, and >>= becomes >=.
// Speculative callers restore these edits when the input is an expression.
func (p *Parser) expectTypeGreater(message string) (token.Token, bool) {
	if p.at(token.Greater) {
		return p.advance(), true
	}
	var remainder token.Kind
	switch p.peek().Kind {
	case token.ShiftRight:
		remainder = token.Greater
	case token.GreaterEqual:
		remainder = token.Assign
	case token.ShrAssign:
		remainder = token.GreaterEqual
	}
	if remainder != "" {
		combined := p.peek()
		middle := combined.Span.Start
		middle.Offset++
		middle.Column++
		first := token.Token{
			Kind:   token.Greater,
			Lexeme: ">",
			Span:   source.Span{Path: combined.Span.Path, Start: combined.Span.Start, End: middle},
		}
		p.tokenEdits = append(p.tokenEdits, parserTokenEdit{index: p.current, original: combined})
		p.tokens[p.current] = token.Token{
			Kind:   remainder,
			Lexeme: combined.Lexeme[1:],
			Span:   source.Span{Path: combined.Span.Path, Start: middle, End: combined.Span.End},
		}
		p.previousToken = first
		return first, true
	}
	p.report(p.peek(), message)
	return p.peek(), false
}

func (p *Parser) match(kinds ...token.Kind) bool {
	for _, kind := range kinds {
		if p.at(kind) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) at(kind token.Kind) bool { return p.peek().Kind == kind }

func (p *Parser) atNext(kind token.Kind) bool {
	return p.current+1 < len(p.tokens) && p.tokens[p.current+1].Kind == kind
}

func (p *Parser) peek() token.Token {
	if len(p.tokens) == 0 {
		return token.Token{Kind: token.EOF}
	}
	if p.current >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.current]
}

func (p *Parser) previous() token.Token {
	if p.previousToken.Kind == "" {
		return p.peek()
	}
	return p.previousToken
}

func (p *Parser) advance() token.Token {
	tok := p.peek()
	if p.current < len(p.tokens) {
		p.current++
	}
	p.previousToken = tok
	return tok
}

type parserTokenEdit struct {
	index    int
	original token.Token
}

type parserCheckpoint struct {
	current, diagnostics, edits int
	previous                    token.Token
	decorators                  int
}

func (p *Parser) checkpoint() parserCheckpoint {
	return parserCheckpoint{p.current, len(p.diagnostics), len(p.tokenEdits), p.previousToken, len(p.decorators)}
}

func (p *Parser) restore(checkpoint parserCheckpoint) {
	for i := len(p.tokenEdits) - 1; i >= checkpoint.edits; i-- {
		edit := p.tokenEdits[i]
		p.tokens[edit.index] = edit.original
	}
	p.tokenEdits = p.tokenEdits[:checkpoint.edits]
	p.current = checkpoint.current
	p.diagnostics = p.diagnostics[:checkpoint.diagnostics]
	p.previousToken = checkpoint.previous
	p.decorators = p.decorators[:checkpoint.decorators]
}

func (p *Parser) report(tok token.Token, message string) {
	if tok.Kind == token.EOF {
		message += " before end of file"
	}
	p.diagnostics = append(p.diagnostics, diagnostic.Diagnostic{Message: message, Span: tok.Span})
}

func (p *Parser) synchronizeStatement() {
	for !p.at(token.EOF) && !p.at(token.RightBrace) {
		if p.previous().Kind == token.Semicolon {
			return
		}
		switch p.peek().Kind {
		case token.Const, token.Let, token.Return, token.If, token.While, token.For, token.Break, token.Continue, token.Goto, token.Fallthrough, token.Defer, token.Go, token.Detach:
			return
		}
		p.advance()
	}
}

func (p *Parser) synchronizeDeclaration() {
	for !p.at(token.EOF) {
		if p.current > 0 && p.previous().Kind == token.Semicolon {
			return
		}
		switch p.peek().Kind {
		case token.Import, token.Function, token.Public, token.Private, token.Protected, token.Final, token.Abstract, token.Class, token.Struct, token.Interface, token.Const, token.Let:
			return
		}
		p.advance()
	}
}

func (p *Parser) synchronizeTo(kind token.Kind) {
	for !p.at(kind) && !p.at(token.EOF) {
		p.advance()
	}
}
