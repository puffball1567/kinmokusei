package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) atCABIExport() bool {
	return p.current+1 < len(p.tokens) && p.tokens[p.current+1].Kind == token.Identifier && p.tokens[p.current+1].Lexeme == "c"
}

func (p *Parser) parseSourceExport(start token.Token) (ast.ExportDecl, ast.Declaration) {
	exported := ast.ExportDecl{Span: start.Span}
	if p.match(token.LeftBrace) {
		for !p.at(token.RightBrace) && !p.at(token.EOF) {
			name, ok := p.expect(token.Identifier, "expected exported name")
			if !ok {
				p.synchronizeDeclaration()
				return exported, nil
			}
			exported.Names = append(exported.Names, ast.ExportName{Name: name.Lexeme, NameSpan: name.Span})
			if !p.match(token.Comma) {
				break
			}
		}
		if _, ok := p.expect(token.RightBrace, "expected '}' after exported names"); !ok {
			p.synchronizeDeclaration()
			return exported, nil
		}
		end, ok := p.expectTerminator("expected ';' after export list")
		if !ok {
			p.synchronizeDeclaration()
			end = p.previous()
		}
		exported.Span = start.Span.Merge(end.Span)
		return exported, nil
	}
	// Do not recursively consume another export directive as a declaration.
	if p.at(token.Export) {
		p.report(p.peek(), "expected declaration or '{' after 'export'")
		return exported, nil
	}
	declaration := p.parseDeclaration()
	if declaration == nil {
		return exported, nil
	}
	name, span := ast.DeclarationBinding(declaration)
	if name == "" {
		p.report(start, "export requires a local top-level binding")
		return exported, nil
	}
	exported.Inline = true
	exported.Names = []ast.ExportName{{Name: name, NameSpan: span}}
	exported.Span = start.Span.Merge(declaration.GetSpan())
	return exported, declaration
}
