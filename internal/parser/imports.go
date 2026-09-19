package parser

import (
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseImport(start token.Token) (ast.ImportDecl, bool) {
	if p.match(token.Go) {
		return p.parseGoImport(start)
	}
	if _, ok := p.expect(token.LeftBrace, "expected '{' after 'import'"); !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	var names []string
	var nameSpans []source.Span
	if p.at(token.RightBrace) {
		p.report(p.peek(), "import list cannot be empty")
	}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		name, ok := p.expect(token.Identifier, "expected imported name")
		if !ok {
			p.synchronizeDeclaration()
			return ast.ImportDecl{}, false
		}
		names = append(names, name.Lexeme)
		nameSpans = append(nameSpans, name.Span)
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.RightBrace) {
			break
		}
	}
	if _, ok := p.expect(token.RightBrace, "expected '}' after imported names"); !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	if _, ok := p.expect(token.From, "expected 'from' after imported names"); !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	pathToken, ok := p.expect(token.String, "expected module path string")
	if !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	path, err := strconv.Unquote(pathToken.Lexeme)
	if err != nil {
		p.report(pathToken, "invalid module path string")
		path = ""
	}
	end, ok := p.expectTerminator("expected ';' after import")
	if !ok {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	return ast.ImportDecl{Names: names, NameSpans: nameSpans, Path: path, PathSpan: pathToken.Span, Span: start.Span.Merge(end.Span)}, true
}

func (p *Parser) parseGoImport(start token.Token) (ast.ImportDecl, bool) {
	if p.at(token.LeftBrace) {
		imported, ok := p.parseImport(start)
		imported.Go = true
		return imported, ok
	}
	alias, ok := p.expect(token.Identifier, "expected Go package alias after 'import go'")
	if !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	if _, ok = p.expect(token.From, "expected 'from' after Go package alias"); !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	pathToken, ok := p.expect(token.String, "expected Go import path string")
	if !ok {
		p.synchronizeDeclaration()
		return ast.ImportDecl{}, false
	}
	path, err := strconv.Unquote(pathToken.Lexeme)
	if err != nil {
		p.report(pathToken, "invalid Go import path string")
		path = ""
	}
	end, ok := p.expectTerminator("expected ';' after Go import")
	if !ok {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	return ast.ImportDecl{Go: true, Alias: alias.Lexeme, AliasSpan: alias.Span, Path: path, PathSpan: pathToken.Span, Span: start.Span.Merge(end.Span)}, true
}
