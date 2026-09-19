package parser

import (
	"fmt"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseDeclaration() ast.Declaration {
	switch {
	case p.match(token.Export):
		return p.parseCABIExport(p.previous())
	case p.match(token.Public, token.Private):
		start := p.previous()
		visibility := ast.Private
		if start.Kind == token.Public {
			visibility = ast.Public
		}
		functionToken, ok := p.expect(token.Function, "expected 'function' after method visibility")
		if !ok {
			p.synchronizeDeclaration()
			return nil
		}
		function := p.parseFunction(functionToken)
		return p.externalMethodFromFunction(start, visibility, function, true)
	case p.match(token.Function):
		start := p.previous()
		function := p.parseFunction(start)
		return p.externalMethodFromFunction(start, ast.Private, function, false)
	case p.match(token.Final, token.Abstract):
		start := p.previous()
		final, abstract := start.Kind == token.Final, start.Kind == token.Abstract
		for p.match(token.Final, token.Abstract) {
			if p.previous().Kind == token.Final {
				if final {
					p.report(p.previous(), "duplicate final modifier")
				}
				final = true
			} else {
				if abstract {
					p.report(p.previous(), "duplicate abstract modifier")
				}
				abstract = true
			}
		}
		classToken, ok := p.expect(token.Class, "expected 'class' after class modifier")
		if !ok {
			p.synchronizeDeclaration()
			return nil
		}
		decl := p.parseClass(classToken)
		if decl != nil {
			decl.Final, decl.Abstract = final, abstract
			decl.Span = start.Span.Merge(decl.Span)
		}
		return decl
	case p.match(token.Class):
		if decl := p.parseClass(p.previous()); decl != nil {
			return decl
		}
		return nil
	case p.match(token.Struct):
		if decl := p.parseStruct(p.previous()); decl != nil {
			return decl
		}
		return nil
	case p.at(token.Identifier) && p.peek().Lexeme == "type":
		return p.parseTypeDeclaration(p.advance(), false)
	case p.at(token.Identifier) && p.peek().Lexeme == "alias":
		return p.parseTypeDeclaration(p.advance(), true)
	case p.at(token.Identifier) && p.peek().Lexeme == "enum":
		return p.parseEnum(p.advance())
	case p.at(token.Identifier) && p.peek().Lexeme == "constraint":
		return p.parseConstraint(p.advance())
	case p.match(token.Interface):
		if decl := p.parseInterface(p.previous()); decl != nil {
			return decl
		}
		return nil
	case p.match(token.Const):
		if p.at(token.LeftBracket) {
			p.report(p.peek(), "multiple binding declarations are only allowed inside functions")
			p.synchronizeDeclaration()
			return nil
		}
		if decl := p.parseVariable(p.previous(), true); decl != nil {
			return decl
		}
		return nil
	case p.match(token.Let):
		if p.at(token.LeftBracket) {
			p.report(p.peek(), "multiple binding declarations are only allowed inside functions")
			p.synchronizeDeclaration()
			return nil
		}
		if decl := p.parseVariable(p.previous(), false); decl != nil {
			return decl
		}
		return nil
	default:
		p.report(p.peek(), "expected a top-level declaration")
		p.synchronizeDeclaration()
		return nil
	}
}

func (p *Parser) parseConstraint(start token.Token) *ast.InterfaceDecl {
	name, ok := p.expect(token.Identifier, "expected constraint name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	parameters, valid := p.parseTypeParameters("constraint")
	if !valid {
		p.synchronizeDeclaration()
		return nil
	}
	if _, ok = p.expect(token.Assign, "expected '=' after constraint name"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	declaration := &ast.InterfaceDecl{Name: name.Lexeme, NameSpan: name.Span, Constraint: true, TypeParameters: parameters}
	var separator token.Kind
	for {
		startTerm := p.peek()
		underlying := p.match(token.Tilde)
		term, valid := p.parseTypeInternal(false)
		if !valid {
			p.synchronizeDeclaration()
			return nil
		}
		termSpan := term.Span
		if underlying {
			termSpan = startTerm.Span.Merge(term.Span)
		}
		declaration.Terms = append(declaration.Terms, ast.TypeSetTerm{Type: term, Underlying: underlying, Span: termSpan})
		if !p.match(token.Pipe, token.Ampersand) {
			break
		}
		operator := p.previous()
		if separator != "" && separator != operator.Kind {
			p.report(operator, "cannot mix '|' and '&' in a constraint declaration; use named intermediate constraints")
			p.synchronizeDeclaration()
			return nil
		}
		separator = operator.Kind
		declaration.Intersection = separator == token.Ampersand
		if p.at(token.Semicolon) || p.at(token.EOF) {
			p.report(p.peek(), "expected constraint term after '"+operator.Lexeme+"'")
			p.synchronizeDeclaration()
			return nil
		}
	}
	end, valid := p.expectTerminator("expected ';' after constraint declaration")
	if !valid {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	declaration.Span = start.Span.Merge(end.Span)
	return declaration
}

func (p *Parser) parseEnum(start token.Token) *ast.EnumDecl {
	name, ok := p.expect(token.Identifier, "expected enum name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	underlying := ast.TypeRef{Name: "int", NameSpan: name.Span, Span: name.Span}
	if p.match(token.Colon) {
		var valid bool
		underlying, valid = p.parseType()
		if !valid {
			p.synchronizeDeclaration()
			return nil
		}
	}
	if _, ok = p.expect(token.LeftBrace, "expected '{' after enum name"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	declaration := &ast.EnumDecl{Name: name.Lexeme, NameSpan: name.Span, Underlying: underlying}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		member, valid := p.expect(token.Identifier, "expected enum member name")
		if !valid {
			p.synchronizeTo(token.RightBrace)
			break
		}
		item := ast.EnumMember{Name: member.Lexeme, NameSpan: member.Span, Span: member.Span}
		if p.match(token.Assign) {
			item.Value = p.parseExpression()
			if item.Value == nil {
				p.synchronizeTo(token.RightBrace)
				break
			}
			item.Span = member.Span.Merge(item.Value.GetSpan())
		}
		declaration.Members = append(declaration.Members, item)
		if !p.match(token.Comma) {
			if !p.at(token.RightBrace) {
				p.report(p.peek(), "expected ',' or '}' after enum member")
				p.synchronizeTo(token.RightBrace)
			}
			break
		}
	}
	end, valid := p.expect(token.RightBrace, "expected '}' after enum body")
	if !valid {
		end = p.previous()
	}
	if p.match(token.Semicolon) {
		end = p.previous()
	}
	declaration.Span = start.Span.Merge(end.Span)
	return declaration
}

func (p *Parser) parseTypeDeclaration(start token.Token, alias bool) *ast.TypeDecl {
	name, ok := p.expect(token.Identifier, "expected type name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	owner := "defined type"
	if alias {
		owner = "alias"
	}
	typeParameters, typeParametersValid := p.parseTypeParameters(owner)
	if _, ok = p.expect(token.Assign, "expected '=' after type name"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	if alias {
		if p.at(token.Identifier) && p.peek().Lexeme == "distinct" {
			p.advance()
			p.report(p.previous(), "alias declarations are transparent and cannot use 'distinct'")
		}
	} else {
		if !p.at(token.Identifier) || p.peek().Lexeme != "distinct" {
			p.report(p.peek(), "defined type requires 'distinct' after '='; use 'alias' for a transparent alias")
			p.synchronizeDeclaration()
			return nil
		}
		p.advance()
	}
	underlying, valid := p.parseType()
	if !valid {
		p.synchronizeDeclaration()
		return nil
	}
	end, valid := p.expectTerminator("expected ';' after type declaration")
	if !valid {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	if !typeParametersValid {
		return nil
	}
	return &ast.TypeDecl{Name: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters, Underlying: underlying, Alias: alias, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseCABIExport(start token.Token) ast.Declaration {
	boundary, ok := p.expect(token.Identifier, "expected 'c' after 'export'")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	if boundary.Lexeme != "c" {
		p.report(boundary, "expected 'c' after 'export'")
		p.synchronizeDeclaration()
		return nil
	}
	if _, ok = p.expect(token.LeftParen, "expected '(' after 'export c'"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	var symbols []string
	var symbolSpans []source.Span
	if p.at(token.RightParen) {
		p.report(p.peek(), "C ABI export symbol list cannot be empty")
	}
	for !p.at(token.RightParen) && !p.at(token.EOF) {
		symbolToken, valid := p.expect(token.String, "expected C ABI symbol string")
		if !valid {
			p.synchronizeTo(token.RightParen)
			break
		}
		symbol, err := strconv.Unquote(symbolToken.Lexeme)
		if err != nil {
			p.report(symbolToken, "invalid C ABI symbol string")
			symbol = ""
		}
		symbols = append(symbols, symbol)
		symbolSpans = append(symbolSpans, symbolToken.Span)
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.RightParen) {
			break
		}
	}
	if _, ok = p.expect(token.RightParen, "expected ')' after C ABI symbol"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	if p.match(token.Function) {
		functionToken := p.previous()
		if len(symbols) != 1 {
			p.report(functionToken, fmt.Sprintf("inline C ABI export expects exactly one symbol, got %d", len(symbols)))
		}
		function := p.parseFunction(functionToken)
		if function == nil {
			return nil
		}
		if len(function.Parameters) > 0 && function.Parameters[0].Name == "this" {
			p.report(functionToken, "C ABI export cannot declare a receiver parameter")
			return nil
		}
		function.CABIExport = true
		if len(symbols) != 0 {
			function.CABISymbol = symbols[0]
			function.CABISymbolSpan = symbolSpans[0]
		}
		function.CABIExportSpan = start.Span.Merge(functionToken.Span)
		function.Span = start.Span.Merge(function.Span)
		return function
	}
	if _, ok = p.expect(token.LeftBrace, "expected 'function' or '{' after C ABI export symbols"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	var names []string
	var nameSpans []source.Span
	if p.at(token.RightBrace) {
		p.report(p.peek(), "C ABI export name list cannot be empty")
	}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		name, valid := p.expect(token.Identifier, "expected top-level function name in C ABI export list")
		if !valid {
			p.synchronizeTo(token.RightBrace)
			break
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
	end, valid := p.expect(token.RightBrace, "expected '}' after C ABI export names")
	if !valid {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	semicolon, valid := p.expectTerminator("expected ';' after C ABI export list")
	if !valid {
		p.synchronizeDeclaration()
		semicolon = end
	}
	return &ast.CABIExportDecl{Symbols: symbols, SymbolSpans: symbolSpans, Names: names, NameSpans: nameSpans, Span: start.Span.Merge(semicolon.Span)}
}
