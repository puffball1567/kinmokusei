package parser

import (
	"fmt"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

type Parser struct {
	tokens                       []token.Token
	current                      int
	diagnostics                  []diagnostic.Diagnostic
	disallowUnqualifiedComposite bool
	disallowCompositeBeforeBlock bool
}

func Parse(tokens []token.Token) (*ast.Program, []diagnostic.Diagnostic) {
	p := &Parser{tokens: tokens}
	program := &ast.Program{}
	for !p.at(token.EOF) {
		start := p.current
		if p.match(token.Import) {
			if imported, ok := p.parseImport(p.previous()); ok {
				program.Imports = append(program.Imports, imported)
			}
			continue
		}
		decl := p.parseDeclaration()
		if decl != nil {
			program.Declarations = append(program.Declarations, decl)
		}
		if p.current == start {
			p.advance()
		}
	}
	return program, p.diagnostics
}

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
	end, ok := p.expect(token.Semicolon, "expected ';' after import")
	if !ok {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	return ast.ImportDecl{Names: names, NameSpans: nameSpans, Path: path, PathSpan: pathToken.Span, Span: start.Span.Merge(end.Span)}, true
}

func (p *Parser) parseGoImport(start token.Token) (ast.ImportDecl, bool) {
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
	end, ok := p.expect(token.Semicolon, "expected ';' after Go import")
	if !ok {
		p.synchronizeDeclaration()
		end = p.previous()
	}
	return ast.ImportDecl{Go: true, Alias: alias.Lexeme, AliasSpan: alias.Span, Path: path, PathSpan: pathToken.Span, Span: start.Span.Merge(end.Span)}, true
}

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
	case p.match(token.Final):
		start := p.previous()
		classToken, ok := p.expect(token.Class, "expected 'class' after 'final'")
		if !ok {
			p.synchronizeDeclaration()
			return nil
		}
		decl := p.parseClass(classToken)
		if decl != nil {
			decl.Final = true
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
	end, valid := p.expect(token.Semicolon, "expected ';' after type declaration")
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
	semicolon, valid := p.expect(token.Semicolon, "expected ';' after C ABI export list")
	if !valid {
		p.synchronizeDeclaration()
		semicolon = end
	}
	return &ast.CABIExportDecl{Symbols: symbols, SymbolSpans: symbolSpans, Names: names, NameSpans: nameSpans, Span: start.Span.Merge(semicolon.Span)}
}

func (p *Parser) parseInterface(start token.Token) *ast.InterfaceDecl {
	name, ok := p.expect(token.Identifier, "expected interface name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	typeParameters, typeParametersValid := p.parseTypeParameters("interface")
	if _, ok = p.expect(token.LeftBrace, "expected '{' after interface name"); !ok {
		return nil
	}
	declaration := &ast.InterfaceDecl{Name: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		methodStart, valid := p.expect(token.Function, "expected interface method")
		if !valid {
			p.synchronizeStatement()
			continue
		}
		methodName, valid := p.expect(token.Identifier, "expected method name")
		if !valid {
			p.synchronizeStatement()
			continue
		}
		if _, valid = p.expect(token.LeftParen, "expected '(' after method name"); !valid {
			p.synchronizeStatement()
			continue
		}
		parameters, valid := p.parseParameters(token.RightParen)
		if !valid {
			p.synchronizeTo(token.RightParen)
		}
		if _, valid = p.expect(token.RightParen, "expected ')' after parameters"); !valid {
			p.synchronizeStatement()
			continue
		}
		if _, valid = p.expect(token.Colon, "expected ':' before return type"); !valid {
			p.synchronizeStatement()
			continue
		}
		returnType, valid := p.parseType()
		if !valid {
			p.synchronizeStatement()
			continue
		}
		end, valid := p.expect(token.Semicolon, "expected ';' after interface method")
		if !valid {
			p.synchronizeStatement()
			end = p.previous()
		}
		declaration.Methods = append(declaration.Methods, ast.InterfaceMethod{
			Name: methodName.Lexeme, NameSpan: methodName.Span, Parameters: parameters, ReturnType: returnType, Span: methodStart.Span.Merge(end.Span),
		})
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after interface body")
	if !ok {
		end = p.previous()
	}
	declaration.Span = start.Span.Merge(end.Span)
	if !typeParametersValid {
		return nil
	}
	return declaration
}

func (p *Parser) parseClass(start token.Token) *ast.ClassDecl {
	name, ok := p.expect(token.Identifier, "expected class name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	typeParameters, typeParametersValid := p.parseTypeParameters("class")
	class := &ast.ClassDecl{Name: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters}
	if p.match(token.Extends) {
		base, valid := p.parseType()
		if !valid {
			return nil
		}
		class.Base = &base
	}
	if p.match(token.Implements) {
		for {
			implemented, valid := p.parseType()
			if !valid {
				return nil
			}
			class.Implements = append(class.Implements, implemented)
			if !p.match(token.Comma) {
				break
			}
		}
	}
	if _, ok = p.expect(token.LeftBrace, "expected '{' after class name"); !ok {
		return nil
	}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		visibility := ast.Private
		if p.match(token.Public) {
			visibility = ast.Public
		} else if p.match(token.Protected) {
			visibility = ast.Protected
		} else {
			p.match(token.Private)
		}
		static, virtual, override, final := false, false, false, false
		for {
			switch {
			case p.match(token.Static):
				if static {
					p.report(p.previous(), "duplicate static modifier")
				}
				static = true
			case p.match(token.Virtual):
				if virtual {
					p.report(p.previous(), "duplicate virtual modifier")
				}
				virtual = true
			case p.match(token.Override):
				if override {
					p.report(p.previous(), "duplicate override modifier")
				}
				override = true
			case p.match(token.Final):
				if final {
					p.report(p.previous(), "duplicate final modifier")
				}
				final = true
			default:
				goto modifiersComplete
			}
		}
	modifiersComplete:
		switch {
		case p.match(token.Constructor):
			if static || virtual || override || final {
				p.report(p.previous(), "constructor cannot have static, virtual, override, or final modifiers")
			}
			constructor := p.parseConstructor(p.previous())
			if class.Constructor != nil {
				p.report(p.previous(), "class can only declare one constructor")
			} else {
				class.Constructor = constructor
			}
		case p.match(token.Function):
			function := p.parseFunction(p.previous())
			if function != nil {
				if len(function.TypeParameters) != 0 {
					p.report(token.Token{Span: function.NameSpan}, "generic class methods are not supported; declare a top-level generic function")
					continue
				}
				class.Methods = append(class.Methods, &ast.MethodDecl{
					Name: function.Name, NameSpan: function.NameSpan, TypeParameters: function.TypeParameters, Parameters: function.Parameters, ReturnType: function.ReturnType,
					Body: function.Body, Visibility: visibility, Static: static, Virtual: virtual, Override: override, Final: final, Span: function.Span,
				})
			}
		case p.at(token.Identifier):
			if static || virtual || override || final {
				p.report(p.peek(), "fields cannot have static, virtual, override, or final modifiers")
			}
			fieldName := p.advance()
			if _, ok = p.expect(token.Colon, "expected ':' after field name"); !ok {
				p.synchronizeStatement()
				continue
			}
			fieldType, valid := p.parseType()
			if !valid {
				p.synchronizeStatement()
				continue
			}
			end, valid := p.expect(token.Semicolon, "expected ';' after field declaration")
			if !valid {
				p.synchronizeStatement()
				end = p.previous()
			}
			class.Fields = append(class.Fields, ast.FieldDecl{Name: fieldName.Lexeme, NameSpan: fieldName.Span, Type: fieldType, Visibility: visibility, Span: fieldName.Span.Merge(end.Span)})
		default:
			p.report(p.peek(), "expected a field, constructor, or method")
			p.advance()
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after class body")
	if !ok {
		end = p.previous()
	}
	class.Span = start.Span.Merge(end.Span)
	if !typeParametersValid {
		return nil
	}
	return class
}

func (p *Parser) parseStruct(start token.Token) *ast.StructDecl {
	name, ok := p.expect(token.Identifier, "expected struct name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	typeParameters, typeParametersValid := p.parseTypeParameters("struct")
	if _, ok = p.expect(token.LeftBrace, "expected '{' after struct name"); !ok {
		return nil
	}
	declaration := &ast.StructDecl{Name: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		visibility := ast.Private
		if p.match(token.Public) {
			visibility = ast.Public
		} else {
			p.match(token.Private)
		}
		pointerReceiver := false
		if p.at(token.Identifier) && p.peek().Lexeme == "pointer" && p.atNext(token.Function) {
			p.advance()
			pointerReceiver = true
		}
		if p.match(token.Function) {
			function := p.parseFunction(p.previous())
			if function != nil {
				if len(function.TypeParameters) != 0 {
					p.report(token.Token{Span: function.NameSpan}, "generic struct methods are not supported; declare a top-level generic function")
					continue
				}
				declaration.Methods = append(declaration.Methods, &ast.MethodDecl{
					Name: function.Name, NameSpan: function.NameSpan, TypeParameters: function.TypeParameters, Parameters: function.Parameters, ReturnType: function.ReturnType,
					Body: function.Body, Visibility: visibility, PointerReceiver: pointerReceiver, Span: function.Span,
				})
			}
			continue
		}
		if pointerReceiver {
			p.report(p.peek(), "pointer must modify a struct method")
		}
		fieldName, valid := p.expect(token.Identifier, "expected a struct field")
		if !valid {
			p.advance()
			p.synchronizeStatement()
			continue
		}
		if _, valid = p.expect(token.Colon, "expected ':' after struct field name"); !valid {
			p.synchronizeStatement()
			continue
		}
		fieldType, valid := p.parseType()
		if !valid {
			p.synchronizeStatement()
			continue
		}
		end, valid := p.expect(token.Semicolon, "expected ';' after struct field declaration")
		if !valid {
			p.synchronizeStatement()
			end = p.previous()
		}
		declaration.Fields = append(declaration.Fields, ast.FieldDecl{
			Name: fieldName.Lexeme, NameSpan: fieldName.Span, Type: fieldType,
			Visibility: visibility, Span: fieldName.Span.Merge(end.Span),
		})
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after struct body")
	if !ok {
		end = p.previous()
	}
	declaration.Span = start.Span.Merge(end.Span)
	if !typeParametersValid {
		return nil
	}
	return declaration
}

func (p *Parser) parseConstructor(start token.Token) *ast.ConstructorDecl {
	if _, ok := p.expect(token.LeftParen, "expected '(' after constructor"); !ok {
		return nil
	}
	parameters, ok := p.parseConstructorParameters()
	if !ok {
		p.synchronizeTo(token.RightParen)
	}
	if _, valid := p.expect(token.RightParen, "expected ')' after constructor parameters"); !valid {
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	return &ast.ConstructorDecl{Parameters: parameters, Body: body, Span: start.Span.Merge(body.Span)}
}

func (p *Parser) parseConstructorParameters() ([]ast.Parameter, bool) {
	var parameters []ast.Parameter
	if p.at(token.RightParen) {
		return parameters, true
	}
	for {
		visibility := ast.Private
		isField := false
		if p.match(token.Public) {
			visibility, isField = ast.Public, true
		} else if p.match(token.Protected) {
			visibility, isField = ast.Protected, true
		} else if p.match(token.Private) {
			isField = true
		}
		variadic := p.match(token.Ellipsis)
		name, ok := p.expect(token.Identifier, "expected constructor parameter name")
		if !ok {
			return nil, false
		}
		if _, ok = p.expect(token.Colon, "expected ':' after constructor parameter name"); !ok {
			return nil, false
		}
		typeRef, ok := p.parseType()
		if !ok {
			return nil, false
		}
		if variadic && !typeRef.IsSlice() {
			p.report(name, "rest parameter type must be a slice")
			return nil, false
		}
		parameters = append(parameters, ast.Parameter{Name: name.Lexeme, Type: typeRef, Variadic: variadic, Visibility: visibility, IsField: isField, Span: name.Span.Merge(typeRef.Span)})
		if !p.match(token.Comma) {
			break
		}
		if variadic {
			p.report(p.previous(), "rest parameter must be the final parameter")
			return nil, false
		}
		if p.at(token.RightParen) {
			break
		}
	}
	return parameters, true
}

func (p *Parser) parseFunction(start token.Token) *ast.FunctionDecl {
	name, ok := p.expect(token.Identifier, "expected function name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	return p.parseFunctionAfterName(start, name)
}

func (p *Parser) parseFunctionAfterName(start, name token.Token) *ast.FunctionDecl {
	typeParameters, typeParametersValid := p.parseTypeParameters("function")
	if _, ok := p.expect(token.LeftParen, "expected '(' after function name"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	var parameters []ast.Parameter
	parametersValid := true
	if !p.at(token.RightParen) {
		for {
			variadic := p.match(token.Ellipsis)
			paramName := p.peek()
			if !p.match(token.Identifier, token.This) {
				p.report(paramName, "expected parameter name")
				parametersValid = false
				p.synchronizeTo(token.RightParen)
				break
			}
			if paramName.Kind == token.This && len(parameters) != 0 {
				p.report(paramName, "receiver parameter 'this' must be the first parameter")
			}
			if variadic && paramName.Kind == token.This {
				p.report(paramName, "receiver parameter 'this' cannot be a rest parameter")
				parametersValid = false
			}
			if _, valid := p.expect(token.Colon, "expected ':' after parameter name"); !valid {
				parametersValid = false
				p.synchronizeTo(token.RightParen)
				break
			}
			paramType, valid := p.parseType()
			if !valid {
				parametersValid = false
				p.synchronizeTo(token.RightParen)
				break
			}
			if variadic && !paramType.IsSlice() {
				p.report(paramName, "rest parameter type must be a slice")
				parametersValid = false
			}
			parameters = append(parameters, ast.Parameter{
				Name:     paramName.Lexeme,
				Type:     paramType,
				Variadic: variadic,
				Span:     paramName.Span.Merge(paramType.Span),
			})
			if !p.match(token.Comma) {
				break
			}
			if variadic {
				p.report(p.previous(), "rest parameter must be the final parameter")
				parametersValid = false
				p.synchronizeTo(token.RightParen)
				break
			}
			if p.at(token.RightParen) {
				break
			}
		}
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after parameters"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	if _, ok := p.expect(token.Colon, "expected ':' before return type"); !ok {
		p.synchronizeDeclaration()
		return nil
	}
	returnType, ok := p.parseType()
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		p.synchronizeDeclaration()
		return nil
	}
	if !typeParametersValid || !parametersValid {
		return nil
	}
	return &ast.FunctionDecl{
		Name: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters, Parameters: parameters, ReturnType: returnType, Body: body,
		Span: start.Span.Merge(body.Span),
	}
}

func (p *Parser) parseTypeParameters(owner string) ([]ast.TypeParameter, bool) {
	if !p.match(token.Less) {
		return nil, true
	}
	var parameters []ast.TypeParameter
	valid := true
	if p.at(token.Greater) {
		p.report(p.peek(), "generic "+owner+" type parameter list cannot be empty")
		valid = false
	}
	for !p.at(token.Greater) && !p.at(token.EOF) {
		name, ok := p.expect(token.Identifier, "expected generic "+owner+" type parameter name")
		if !ok {
			valid = false
			p.synchronizeTo(token.Greater)
			break
		}
		parameter := ast.TypeParameter{Name: name.Lexeme, NameSpan: name.Span, Span: name.Span}
		if p.match(token.Extends) {
			constraint, constraintValid := p.parseType()
			if !constraintValid {
				valid = false
				p.synchronizeTo(token.Greater)
				break
			}
			parameter.Constraint = &constraint
			parameter.Span = name.Span.Merge(constraint.Span)
		}
		parameters = append(parameters, parameter)
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.Greater) {
			p.report(p.peek(), "expected generic "+owner+" type parameter name after ','")
			valid = false
			break
		}
	}
	if _, ok := p.expectTypeGreater("expected '>' after generic " + owner + " type parameters"); !ok {
		return parameters, false
	}
	return parameters, valid
}

func (p *Parser) externalMethodFromFunction(start token.Token, visibility ast.Visibility, function *ast.FunctionDecl, required bool) ast.Declaration {
	if function == nil {
		return nil
	}
	if len(function.Parameters) == 0 || function.Parameters[0].Name != "this" {
		if required {
			p.report(start, "external method requires 'this' as its first parameter")
			return nil
		}
		return function
	}
	receiver := function.Parameters[0]
	receiverNameSpan := receiver.Span
	receiverNameSpan.End.Offset = receiverNameSpan.Start.Offset + len("this")
	receiverNameSpan.End.Column = receiverNameSpan.Start.Column + len("this")
	return &ast.MethodDecl{
		Name: function.Name, NameSpan: function.NameSpan, TypeParameters: function.TypeParameters, Parameters: function.Parameters[1:], ReturnType: function.ReturnType,
		Body: function.Body, Visibility: visibility, External: true, ReceiverName: "this",
		ReceiverNameSpan: receiverNameSpan, ReceiverType: receiver.Type, Span: start.Span.Merge(function.Span),
	}
}

func (p *Parser) parseType() (ast.TypeRef, bool) {
	return p.parseTypeInternal(true)
}

func (p *Parser) parseTypeInternal(allowNullable bool) (ast.TypeRef, bool) {
	if p.match(token.LeftBrace) {
		start := p.previous()
		var fields []ast.ObjectTypeField
		if !p.at(token.RightBrace) {
			for {
				name, ok := p.expect(token.Identifier, "expected object type field name")
				if !ok {
					return ast.TypeRef{}, false
				}
				if _, ok = p.expect(token.Colon, "expected ':' after object type field name"); !ok {
					return ast.TypeRef{}, false
				}
				fieldType, ok := p.parseType()
				if !ok {
					return ast.TypeRef{}, false
				}
				fields = append(fields, ast.ObjectTypeField{Name: name.Lexeme, JSONName: name.Lexeme, Type: fieldType, Span: name.Span.Merge(fieldType.Span)})
				if !p.match(token.Comma) {
					break
				}
				if p.at(token.RightBrace) {
					break
				}
			}
		}
		end, ok := p.expect(token.RightBrace, "expected '}' after object type")
		if !ok {
			return ast.TypeRef{}, false
		}
		ref := ast.TypeRef{Object: true, ObjectFields: fields, Span: start.Span.Merge(end.Span)}
		return p.parseTypeSuffix(ref, allowNullable)
	}
	if p.match(token.LeftBracket) {
		start := p.previous()
		lengthToken, ok := p.expect(token.Integer, "expected fixed array length")
		if !ok {
			return ast.TypeRef{}, false
		}
		length, err := strconv.ParseInt(lengthToken.Lexeme, 10, 64)
		if err != nil {
			p.report(lengthToken, "fixed array length is out of range")
			return ast.TypeRef{}, false
		}
		if _, ok = p.expect(token.RightBracket, "expected ']' after fixed array length"); !ok {
			return ast.TypeRef{}, false
		}
		element, ok := p.parseTypeInternal(false)
		if !ok {
			return ast.TypeRef{}, false
		}
		ref := ast.TypeRef{Element: &element, FixedLength: &length, Span: start.Span.Merge(element.Span)}
		return p.parseTypeSuffix(ref, allowNullable)
	}
	if p.match(token.Star) {
		start := p.previous()
		pointee, ok := p.parseTypeInternal(false)
		if !ok {
			return ast.TypeRef{}, false
		}
		ref := ast.TypeRef{Pointee: &pointee, Span: start.Span.Merge(pointee.Span)}
		return p.parseTypeSuffix(ref, allowNullable)
	}
	if p.match(token.LeftParen) {
		start := p.previous()
		parameters, ok := p.parseParameters(token.RightParen)
		if !ok {
			return ast.TypeRef{}, false
		}
		endParameters, ok := p.expect(token.RightParen, "expected ')' after function type parameters")
		if !ok {
			return ast.TypeRef{}, false
		}
		if _, ok = p.expect(token.FatArrow, "expected '=>' in function type"); !ok {
			return ast.TypeRef{}, false
		}
		result, ok := p.parseType()
		if !ok {
			return ast.TypeRef{}, false
		}
		parameterTypes := make([]ast.TypeRef, len(parameters))
		for i, parameter := range parameters {
			parameterTypes[i] = parameter.Type
		}
		_ = endParameters
		ref := ast.TypeRef{Parameters: parameterTypes, Return: &result, Variadic: len(parameters) != 0 && parameters[len(parameters)-1].Variadic, Span: start.Span.Merge(result.Span)}
		return p.parseTypeSuffix(ref, allowNullable)
	}
	tok, ok := p.expect(token.Identifier, "expected type name")
	if !ok {
		return ast.TypeRef{}, false
	}
	ref := ast.TypeRef{Name: tok.Lexeme, NameSpan: tok.Span, Span: tok.Span}
	if p.match(token.Dot) {
		name, valid := p.expect(token.Identifier, "expected Go type name after '.'")
		if !valid {
			return ast.TypeRef{}, false
		}
		ref.Qualifier = tok.Lexeme
		ref.QualifierSpan = tok.Span
		ref.Name = name.Lexeme
		ref.NameSpan = name.Span
		ref.Go = true
		ref.Span = tok.Span.Merge(name.Span)
	}
	if p.match(token.Less) {
		for {
			argument, valid := p.parseType()
			if !valid {
				return ast.TypeRef{}, false
			}
			ref.GenericArguments = append(ref.GenericArguments, argument)
			if !p.match(token.Comma) {
				break
			}
		}
		end, valid := p.expectTypeGreater("expected '>' after generic type arguments")
		if !valid {
			return ast.TypeRef{}, false
		}
		ref.Span = ref.Span.Merge(end.Span)
	}
	return p.parseTypeSuffix(ref, allowNullable)
}

func (p *Parser) parseTypeSuffix(ref ast.TypeRef, allowNullable bool) (ast.TypeRef, bool) {
	for p.match(token.LeftBracket) {
		end, ok := p.expect(token.RightBracket, "expected ']' in array type")
		if !ok {
			return ast.TypeRef{}, false
		}
		element := ref
		ref = ast.TypeRef{Element: &element, Span: element.Span.Merge(end.Span)}
	}
	if allowNullable && p.match(token.Pipe) {
		nullToken, ok := p.expect(token.Null, "expected 'null' after '|' in nullable type")
		if !ok {
			return ast.TypeRef{}, false
		}
		ref.Nullable = true
		ref.Span = ref.Span.Merge(nullToken.Span)
	}
	return ref, true
}

func (p *Parser) parseParameters(end token.Kind) ([]ast.Parameter, bool) {
	var parameters []ast.Parameter
	if p.at(end) {
		return parameters, true
	}
	for {
		variadic := p.match(token.Ellipsis)
		name, ok := p.expect(token.Identifier, "expected parameter name")
		if !ok {
			return nil, false
		}
		if _, ok = p.expect(token.Colon, "expected ':' after parameter name"); !ok {
			return nil, false
		}
		typeRef, ok := p.parseType()
		if !ok {
			return nil, false
		}
		if variadic && !typeRef.IsSlice() {
			p.report(name, "rest parameter type must be a slice")
			return nil, false
		}
		parameters = append(parameters, ast.Parameter{Name: name.Lexeme, Type: typeRef, Variadic: variadic, Span: name.Span.Merge(typeRef.Span)})
		if !p.match(token.Comma) {
			break
		}
		if variadic {
			p.report(p.previous(), "rest parameter must be the final parameter")
			return nil, false
		}
		if p.at(end) {
			break
		}
	}
	return parameters, true
}

func (p *Parser) parseVariable(start token.Token, constant bool) *ast.VariableDecl {
	name, ok := p.expect(token.Identifier, "expected variable name")
	if !ok {
		p.synchronizeStatement()
		return nil
	}
	typeRef := ast.TypeRef{}
	if p.match(token.Colon) {
		typeRef, ok = p.parseType()
		if !ok {
			p.synchronizeStatement()
			return nil
		}
	}
	if _, ok = p.expect(token.Assign, "expected '=' before initializer"); !ok {
		p.synchronizeStatement()
		return nil
	}
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after variable declaration")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.VariableDecl{
		Constant: constant, Name: name.Lexeme, NameSpan: name.Span, Type: typeRef, Value: value,
		Span: start.Span.Merge(end.Span),
	}
}

func (p *Parser) parseMultiVariable(start token.Token, constant bool) *ast.MultiVariableDecl {
	if _, ok := p.expect(token.LeftBracket, "expected '[' before bindings"); !ok {
		return nil
	}
	var bindings []ast.Binding
	for !p.at(token.RightBracket) && !p.at(token.EOF) {
		name, ok := p.expect(token.Identifier, "expected binding name")
		if !ok {
			p.synchronizeStatement()
			return nil
		}
		bindings = append(bindings, ast.Binding{Name: name.Lexeme, Span: name.Span})
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.RightBracket) {
			break
		}
	}
	if _, ok := p.expect(token.RightBracket, "expected ']' after bindings"); !ok {
		p.synchronizeStatement()
		return nil
	}
	if len(bindings) == 0 {
		p.report(p.previous(), "multiple binding declaration requires at least one binding")
	}
	if _, ok := p.expect(token.Assign, "expected '=' before initializer"); !ok {
		p.synchronizeStatement()
		return nil
	}
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after variable declaration")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.MultiVariableDecl{Constant: constant, Bindings: bindings, Value: value, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseBlock() *ast.BlockStmt {
	start, ok := p.expect(token.LeftBrace, "expected '{'")
	if !ok {
		return nil
	}
	block := &ast.BlockStmt{}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		before := p.current
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		if before == p.current {
			p.advance()
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after block")
	if !ok {
		end = p.previous()
	}
	block.Span = start.Span.Merge(end.Span)
	return block
}

func (p *Parser) parseStatement() ast.Statement {
	switch {
	case p.match(token.Const):
		if p.at(token.LeftBracket) {
			return p.parseMultiVariable(p.previous(), true)
		}
		if stmt := p.parseVariable(p.previous(), true); stmt != nil {
			return stmt
		}
		return nil
	case p.match(token.Let):
		if p.at(token.LeftBracket) {
			return p.parseMultiVariable(p.previous(), false)
		}
		if stmt := p.parseVariable(p.previous(), false); stmt != nil {
			return stmt
		}
		return nil
	case p.match(token.Return):
		return p.parseReturn(p.previous())
	case p.match(token.Throw):
		return p.parseThrow(p.previous())
	case p.match(token.Try):
		return p.parseTry(p.previous())
	case p.match(token.If):
		return p.parseIf(p.previous())
	case p.match(token.While):
		return p.parseWhile(p.previous())
	case p.match(token.For):
		return p.parseFor(p.previous())
	case p.match(token.Select):
		return p.parseSelect(p.previous())
	case p.match(token.Switch):
		return p.parseSwitch(p.previous())
	case p.match(token.Break):
		return p.parseBranch(p.previous(), ast.BreakBranch)
	case p.match(token.Continue):
		return p.parseBranch(p.previous(), ast.ContinueBranch)
	case p.match(token.Goto):
		return p.parseBranch(p.previous(), ast.GotoBranch)
	case p.match(token.Fallthrough):
		return p.parseBranch(p.previous(), ast.FallthroughBranch)
	case p.match(token.Defer):
		return p.parseCallControl(p.previous(), ast.DeferCall)
	case p.match(token.Go):
		return p.parseCallControl(p.previous(), ast.GoCall)
	case p.match(token.Detach):
		return p.parseDetach(p.previous())
	case p.at(token.LeftBrace):
		if stmt := p.parseBlock(); stmt != nil {
			return stmt
		}
		return nil
	case p.at(token.Identifier) && p.atNext(token.Colon):
		return p.parseLabeledStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseThrow(start token.Token) ast.Statement {
	if p.match(token.Semicolon) {
		return &ast.ThrowStmt{Bare: true, Span: start.Span.Merge(p.previous().Span)}
	}
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after thrown error")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.ThrowStmt{Value: value, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseTry(start token.Token) ast.Statement {
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	statement := &ast.TryStmt{Body: body, Span: start.Span.Merge(body.Span)}
	for p.match(token.Catch) {
		clause := &ast.CatchClause{}
		if _, ok := p.expect(token.LeftParen, "expected '(' after 'catch'"); !ok {
			p.synchronizeStatement()
			return statement
		}
		name, ok := p.expect(token.Identifier, "expected catch binding name")
		if !ok {
			p.synchronizeStatement()
			return statement
		}
		clause.Name = name.Lexeme
		clause.NameSpan = name.Span
		if _, ok = p.expect(token.Colon, "expected ':' after catch binding name"); !ok {
			p.synchronizeStatement()
			return statement
		}
		catchType, valid := p.parseType()
		if !valid {
			p.synchronizeStatement()
			return statement
		}
		clause.Type = catchType
		if _, ok = p.expect(token.RightParen, "expected ')' after catch binding"); !ok {
			p.synchronizeStatement()
			return statement
		}
		clause.Body = p.parseBlock()
		if clause.Body == nil {
			return statement
		}
		statement.Catches = append(statement.Catches, clause)
		statement.Span = start.Span.Merge(clause.Body.Span)
	}
	if p.match(token.Finally) {
		statement.FinallyBody = p.parseBlock()
		if statement.FinallyBody == nil {
			return statement
		}
		statement.Span = start.Span.Merge(statement.FinallyBody.Span)
	}
	if len(statement.Catches) == 0 && statement.FinallyBody == nil {
		p.report(p.peek(), "try requires catch, finally, or both")
	}
	return statement
}

func (p *Parser) parseReturn(start token.Token) ast.Statement {
	if p.match(token.Semicolon) {
		return &ast.ReturnStmt{Span: start.Span.Merge(p.previous().Span)}
	}
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after return value")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.ReturnStmt{Value: value, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseIf(start token.Token) ast.Statement {
	if _, ok := p.expect(token.LeftParen, "expected '(' after 'if'"); !ok {
		p.synchronizeStatement()
		return nil
	}
	condition := p.parseExpression()
	if condition == nil {
		p.synchronizeStatement()
		return nil
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after condition"); !ok {
		p.synchronizeStatement()
		return nil
	}
	then := p.parseBlock()
	if then == nil {
		return nil
	}
	var elseBranch ast.Statement
	end := then.Span
	if p.match(token.Else) {
		if p.match(token.If) {
			elseBranch = p.parseIf(p.previous())
		} else {
			elseBranch = p.parseBlock()
		}
		if elseBranch != nil {
			end = elseBranch.GetSpan()
		}
	}
	return &ast.IfStmt{Condition: condition, Then: then, Else: elseBranch, Span: start.Span.Merge(end)}
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	return p.parseSimpleStatement(true)
}

func (p *Parser) parseSimpleStatement(requireSemicolon bool) ast.Statement {
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	if p.match(token.LeftArrow) {
		sent := p.parseExpression()
		if sent == nil {
			p.synchronizeStatement()
			return nil
		}
		end := sent.GetSpan()
		if requireSemicolon {
			tok, valid := p.expect(token.Semicolon, "expected ';' after channel send")
			if !valid {
				p.synchronizeStatement()
			} else {
				end = tok.Span
			}
		}
		return &ast.ChannelSendStmt{Channel: value, Value: sent, Span: value.GetSpan().Merge(end)}
	}
	if p.match(token.Assign, token.PlusAssign, token.MinusAssign, token.StarAssign, token.SlashAssign, token.PercentAssign, token.AndAssign, token.OrAssign, token.XorAssign, token.AndNotAssign, token.ShlAssign, token.ShrAssign) {
		operator := p.previous()
		if array, ok := value.(*ast.ArrayLiteralExpr); ok {
			if operator.Kind != token.Assign {
				p.report(operator, "compound assignment requires a single assignable target")
				p.synchronizeStatement()
				return nil
			}
			bindings := make([]ast.Binding, len(array.Elements))
			for i, element := range array.Elements {
				identifier, valid := element.(*ast.IdentifierExpr)
				if !valid {
					p.report(p.previous(), "multiple assignment targets must be names")
					p.synchronizeStatement()
					return nil
				}
				bindings[i] = ast.Binding{Name: identifier.Name, Span: identifier.Span}
			}
			if len(bindings) == 0 {
				p.report(p.previous(), "multiple assignment requires at least one target")
			}
			right := p.parseExpression()
			if right == nil {
				return nil
			}
			end := right.GetSpan()
			if requireSemicolon {
				tok, valid := p.expect(token.Semicolon, "expected ';' after assignment")
				if !valid {
					p.synchronizeStatement()
				} else {
					end = tok.Span
				}
			}
			return &ast.MultiAssignmentStmt{Bindings: bindings, Value: right, Span: value.GetSpan().Merge(end)}
		}
		if !isAssignmentTarget(value) {
			p.report(operator, "invalid assignment target")
			p.synchronizeStatement()
			return nil
		}
		right := p.parseExpression()
		if right == nil {
			return nil
		}
		end := right.GetSpan()
		if requireSemicolon {
			tok, valid := p.expect(token.Semicolon, "expected ';' after assignment")
			if !valid {
				p.synchronizeStatement()
			} else {
				end = tok.Span
			}
		}
		return &ast.AssignmentStmt{Target: value, Operator: operator.Lexeme, Value: right, Span: value.GetSpan().Merge(end)}
	}
	if p.match(token.Increment, token.Decrement) {
		operator := p.previous()
		if !isAssignmentTarget(value) {
			p.report(operator, "invalid increment or decrement target")
			p.synchronizeStatement()
			return nil
		}
		end := operator.Span
		if requireSemicolon {
			tok, valid := p.expect(token.Semicolon, "expected ';' after increment or decrement")
			if !valid {
				p.synchronizeStatement()
			} else {
				end = tok.Span
			}
		}
		return &ast.IncDecStmt{Target: value, Operator: operator.Lexeme, Span: value.GetSpan().Merge(end)}
	}
	end := value.GetSpan()
	if requireSemicolon {
		tok, ok := p.expect(token.Semicolon, "expected ';' after expression")
		if !ok {
			p.synchronizeStatement()
		} else {
			end = tok.Span
		}
	}
	return &ast.ExpressionStmt{Value: value, Span: value.GetSpan().Merge(end)}
}

func isAssignmentTarget(value ast.Expression) bool {
	switch value := value.(type) {
	case *ast.IdentifierExpr, *ast.MemberExpr, *ast.IndexExpr:
		return true
	case *ast.UnaryExpr:
		return value.Operator == "*"
	default:
		return false
	}
}

func (p *Parser) parseWhile(start token.Token) ast.Statement {
	if _, ok := p.expect(token.LeftParen, "expected '(' after 'while'"); !ok {
		return nil
	}
	condition := p.parseExpression()
	if condition == nil {
		return nil
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after condition"); !ok {
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	return &ast.WhileStmt{Condition: condition, Body: body, Span: start.Span.Merge(body.Span)}
}

func (p *Parser) parseFor(start token.Token) ast.Statement {
	if _, ok := p.expect(token.LeftParen, "expected '(' after 'for'"); !ok {
		return nil
	}
	if p.at(token.Const) || p.at(token.Let) {
		cursor := p.current
		diagnosticCount := len(p.diagnostics)
		if ranged, recognized := p.tryParseForRange(start); recognized {
			return ranged
		}
		p.current = cursor
		p.diagnostics = p.diagnostics[:diagnosticCount]
	}
	var initializer ast.Statement
	if p.match(token.Semicolon) {
		// Empty initializer.
	} else if p.match(token.Const) {
		if p.at(token.LeftBracket) {
			initializer = p.parseMultiVariable(p.previous(), true)
		} else if variable := p.parseVariable(p.previous(), true); variable != nil {
			initializer = variable
		}
	} else if p.match(token.Let) {
		if p.at(token.LeftBracket) {
			initializer = p.parseMultiVariable(p.previous(), false)
		} else if variable := p.parseVariable(p.previous(), false); variable != nil {
			initializer = variable
		}
	} else {
		initializer = p.parseSimpleStatement(true)
	}
	var condition ast.Expression
	if !p.at(token.Semicolon) {
		condition = p.parseExpression()
	}
	if _, ok := p.expect(token.Semicolon, "expected ';' after for condition"); !ok {
		return nil
	}
	var post ast.Statement
	if !p.at(token.RightParen) {
		post = p.parseSimpleStatement(false)
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after for clauses"); !ok {
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	return &ast.ForStmt{Initializer: initializer, Condition: condition, Post: post, Body: body, Span: start.Span.Merge(body.Span)}
}

func (p *Parser) tryParseForRange(start token.Token) (ast.Statement, bool) {
	constant := p.match(token.Const)
	if !constant && !p.match(token.Let) {
		return nil, false
	}
	bindings := make([]ast.RangeBinding, 0, 2)
	bracketed := p.match(token.LeftBracket)
	for {
		name, ok := p.expect(token.Identifier, "expected range binding name")
		if !ok {
			return nil, false
		}
		binding := ast.RangeBinding{Name: name.Lexeme, NameSpan: name.Span}
		if p.match(token.Colon) {
			binding.Type, ok = p.parseType()
			if !ok {
				return nil, false
			}
		}
		bindings = append(bindings, binding)
		if !bracketed || !p.match(token.Comma) {
			break
		}
		if p.at(token.RightBracket) {
			break
		}
	}
	if bracketed {
		if _, ok := p.expect(token.RightBracket, "expected ']' after range bindings"); !ok {
			return nil, false
		}
	}
	if !p.match(token.Of) {
		return nil, false
	}
	if bracketed && len(bindings) != 2 {
		p.report(p.previous(), "range binding list requires exactly two bindings")
	}
	source := p.parseExpression()
	if source == nil {
		return nil, true
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after range expression"); !ok {
		return nil, true
	}
	body := p.parseBlock()
	if body == nil {
		return nil, true
	}
	return &ast.ForRangeStmt{
		Constant: constant, Bindings: bindings, Source: source,
		Body: body, Span: start.Span.Merge(body.Span),
	}, true
}

func (p *Parser) parseSelect(start token.Token) ast.Statement {
	if _, ok := p.expect(token.LeftBrace, "expected '{' after 'select'"); !ok {
		return nil
	}
	stmt := &ast.SelectStmt{}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		before := p.current
		if p.match(token.Case) {
			if clause := p.parseSelectCase(p.previous()); clause != nil {
				stmt.Cases = append(stmt.Cases, *clause)
			}
		} else if p.match(token.Default) {
			clauseStart := p.previous()
			body := p.parseBlock()
			if body != nil {
				stmt.Cases = append(stmt.Cases, ast.SelectCase{Kind: ast.SelectDefault, Body: body, Span: clauseStart.Span.Merge(body.Span)})
			}
		} else {
			p.report(p.peek(), "expected 'case' or 'default' in select")
			p.advance()
		}
		if before == p.current {
			p.advance()
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after select")
	if !ok {
		end = p.previous()
	}
	stmt.Span = start.Span.Merge(end.Span)
	return stmt
}

func (p *Parser) parseSelectCase(start token.Token) *ast.SelectCase {
	if p.match(token.Const) || p.match(token.Let) {
		constant := p.previous().Kind == token.Const
		bindings, ok := p.parseSelectBindings()
		if !ok {
			return nil
		}
		if _, ok = p.expect(token.Assign, "expected '=' before select receive"); !ok {
			return nil
		}
		if _, ok = p.expect(token.LeftArrow, "select binding requires a channel receive"); !ok {
			return nil
		}
		channel := p.parseExpressionBeforeBlock()
		if channel == nil {
			return nil
		}
		body := p.parseBlock()
		if body == nil {
			return nil
		}
		return &ast.SelectCase{Kind: ast.SelectReceive, Constant: constant, Declare: true, Bindings: bindings, Channel: channel, Body: body, Span: start.Span.Merge(body.Span)}
	}

	communication := p.parseExpressionBeforeBlock()
	if communication == nil {
		return nil
	}
	clause := &ast.SelectCase{Span: start.Span}
	if p.match(token.LeftArrow) {
		value := p.parseExpressionBeforeBlock()
		if value == nil {
			return nil
		}
		clause.Kind, clause.Channel, clause.Value = ast.SelectSend, communication, value
	} else if p.match(token.Assign) {
		targets, ok := p.selectAssignmentTargets(communication)
		if !ok {
			return nil
		}
		if _, ok = p.expect(token.LeftArrow, "select assignment requires a channel receive"); !ok {
			return nil
		}
		channel := p.parseExpressionBeforeBlock()
		if channel == nil {
			return nil
		}
		clause.Kind, clause.Targets, clause.Channel = ast.SelectReceive, targets, channel
	} else if receive, ok := communication.(*ast.UnaryExpr); ok && receive.Operator == "<-" {
		clause.Kind, clause.Channel = ast.SelectReceive, receive.Operand
	} else {
		p.report(p.peek(), "select case requires a channel send or receive")
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	clause.Body = body
	clause.Span = start.Span.Merge(body.Span)
	return clause
}

type parsedSwitchClause struct {
	typeCase  *ast.TypeSwitchCase
	valueCase *ast.ValueSwitchCase
}

func (p *Parser) parseSwitch(start token.Token) ast.Statement {
	if _, ok := p.expect(token.LeftParen, "expected '(' after 'switch'"); !ok {
		return nil
	}
	value := p.parseExpression()
	if value == nil {
		return nil
	}
	if _, ok := p.expect(token.RightParen, "expected ')' after switch value"); !ok {
		return nil
	}
	if _, ok := p.expect(token.LeftBrace, "expected '{' after switch value"); !ok {
		return nil
	}
	var clauses []parsedSwitchClause
	hasTypeCase := false
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		before := p.current
		if p.match(token.Case) {
			caseStart := p.previous()
			if p.at(token.Const) || p.at(token.Let) {
				if clause := p.parseTypeSwitchBindingCase(caseStart); clause != nil {
					clauses = append(clauses, parsedSwitchClause{typeCase: clause})
					hasTypeCase = true
				}
			} else if clause := p.parseValueSwitchCase(caseStart); clause != nil {
				clauses = append(clauses, parsedSwitchClause{valueCase: clause})
			}
		} else if p.match(token.Default) {
			clauseStart := p.previous()
			body := p.parseBlock()
			if body != nil {
				clauses = append(clauses, parsedSwitchClause{valueCase: &ast.ValueSwitchCase{Default: true, Body: body, Span: clauseStart.Span.Merge(body.Span)}})
			}
		} else {
			p.report(p.peek(), "expected 'case' or 'default' in switch")
			p.advance()
		}
		if before == p.current {
			p.advance()
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after switch")
	if !ok {
		end = p.previous()
	}
	span := start.Span.Merge(end.Span)
	if !hasTypeCase {
		stmt := &ast.ValueSwitchStmt{Value: value, Span: span}
		for _, clause := range clauses {
			if clause.valueCase != nil {
				stmt.Cases = append(stmt.Cases, *clause.valueCase)
			}
		}
		return stmt
	}
	stmt := &ast.TypeSwitchStmt{Value: value, Span: span}
	for _, clause := range clauses {
		if clause.typeCase != nil {
			stmt.Cases = append(stmt.Cases, *clause.typeCase)
			continue
		}
		valueClause := clause.valueCase
		if valueClause == nil {
			continue
		}
		if valueClause.Default {
			stmt.Cases = append(stmt.Cases, ast.TypeSwitchCase{Default: true, Body: valueClause.Body, Span: valueClause.Span})
			continue
		}
		if len(valueClause.Values) == 1 {
			if literal, ok := valueClause.Values[0].(*ast.LiteralExpr); ok && (literal.Kind == ast.NilLiteral || literal.Kind == ast.NullLiteral) {
				stmt.Cases = append(stmt.Cases, ast.TypeSwitchCase{Nil: true, Body: valueClause.Body, Span: valueClause.Span})
				continue
			}
		}
		p.diagnostics = append(p.diagnostics, diagnostic.Diagnostic{Message: "type switch cannot mix type cases with value cases", Span: valueClause.Span})
	}
	return stmt
}

func (p *Parser) parseTypeSwitchBindingCase(start token.Token) *ast.TypeSwitchCase {
	constant := p.match(token.Const)
	if !constant && !p.match(token.Let) {
		p.report(p.peek(), "type switch case requires 'const' or 'let'")
		return nil
	}
	name, ok := p.expect(token.Identifier, "expected type switch binding name")
	if !ok {
		return nil
	}
	if _, ok = p.expect(token.As, "expected 'as' before type switch case type"); !ok {
		return nil
	}
	caseType, ok := p.parseType()
	if !ok {
		return nil
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	return &ast.TypeSwitchCase{Constant: constant, Name: name.Lexeme, NameSpan: name.Span, Type: caseType, Body: body, Span: start.Span.Merge(body.Span)}
}

func (p *Parser) parseValueSwitchCase(start token.Token) *ast.ValueSwitchCase {
	clause := &ast.ValueSwitchCase{}
	for {
		value := p.parseExpressionBeforeBlock()
		if value == nil {
			return nil
		}
		clause.Values = append(clause.Values, value)
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.LeftBrace) {
			p.report(p.peek(), "expected switch case value after ','")
			return nil
		}
	}
	body := p.parseBlock()
	if body == nil {
		return nil
	}
	clause.Body = body
	clause.Span = start.Span.Merge(body.Span)
	return clause
}

func (p *Parser) parseSelectBindings() ([]ast.Binding, bool) {
	if !p.match(token.LeftBracket) {
		name, ok := p.expect(token.Identifier, "expected select receive binding")
		if !ok {
			return nil, false
		}
		return []ast.Binding{{Name: name.Lexeme, Span: name.Span}}, true
	}
	var bindings []ast.Binding
	for !p.at(token.RightBracket) && !p.at(token.EOF) {
		name, ok := p.expect(token.Identifier, "expected select receive binding")
		if !ok {
			return nil, false
		}
		bindings = append(bindings, ast.Binding{Name: name.Lexeme, Span: name.Span})
		if !p.match(token.Comma) {
			break
		}
	}
	if _, ok := p.expect(token.RightBracket, "expected ']' after select receive bindings"); !ok {
		return nil, false
	}
	if len(bindings) < 2 {
		p.report(p.previous(), "checked select receive requires two bindings")
	}
	return bindings, true
}

func (p *Parser) selectAssignmentTargets(value ast.Expression) ([]ast.Expression, bool) {
	if array, ok := value.(*ast.ArrayLiteralExpr); ok {
		if len(array.Elements) < 2 {
			p.report(p.peek(), "checked select receive requires two targets")
		}
		for _, target := range array.Elements {
			identifier, valid := target.(*ast.IdentifierExpr)
			if !valid {
				p.report(p.peek(), "checked select receive targets must be names")
				return nil, false
			}
			_ = identifier
		}
		return array.Elements, true
	}
	switch target := value.(type) {
	case *ast.IdentifierExpr, *ast.MemberExpr, *ast.IndexExpr:
		return []ast.Expression{value}, true
	case *ast.UnaryExpr:
		if target.Operator == "*" {
			return []ast.Expression{value}, true
		}
	}
	p.report(p.peek(), "invalid select receive assignment target")
	return nil, false
}

func (p *Parser) parseBranch(start token.Token, kind ast.BranchKind) ast.Statement {
	var label token.Token
	if kind == ast.GotoBranch {
		var ok bool
		label, ok = p.expect(token.Identifier, "expected label name after 'goto'")
		if !ok {
			p.synchronizeStatement()
			return nil
		}
	} else if kind != ast.FallthroughBranch && p.at(token.Identifier) {
		label = p.advance()
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after branch statement")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.BranchStmt{Kind: kind, Label: label.Lexeme, LabelSpan: label.Span, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseLabeledStatement() ast.Statement {
	label := p.advance()
	p.advance() // ':'
	statement := p.parseStatement()
	if statement == nil {
		p.report(label, "expected statement after label")
		return nil
	}
	return &ast.LabeledStmt{Label: label.Lexeme, LabelSpan: label.Span, Statement: statement, Span: label.Span.Merge(statement.GetSpan())}
}

func (p *Parser) parseCallControl(start token.Token, kind ast.CallControlKind) ast.Statement {
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	if _, ok := value.(*ast.CallExpr); !ok {
		keyword := "defer"
		if kind == ast.GoCall {
			keyword = "go"
		}
		p.report(p.previous(), keyword+" requires a function or method call")
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after call")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.CallControlStmt{Kind: kind, Value: value, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseDetach(start token.Token) ast.Statement {
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expect(token.Semicolon, "expected ';' after detached task")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.DetachStmt{Value: value, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseExpression() ast.Expression { return p.parseBinary(1) }

func (p *Parser) parseExpressionBeforeBlock() ast.Expression {
	previous := p.disallowUnqualifiedComposite
	previousAll := p.disallowCompositeBeforeBlock
	p.disallowUnqualifiedComposite = true
	p.disallowCompositeBeforeBlock = true
	defer func() {
		p.disallowUnqualifiedComposite = previous
		p.disallowCompositeBeforeBlock = previousAll
	}()
	return p.parseExpression()
}

func (p *Parser) parseBinary(minPrecedence int) ast.Expression {
	left := p.parseUnary()
	if left == nil {
		return nil
	}
	for {
		op := p.peek()
		precedence := binaryPrecedence(op.Kind)
		if precedence < minPrecedence {
			break
		}
		p.advance()
		right := p.parseBinary(precedence + 1)
		if right == nil {
			p.report(p.peek(), "expected expression after operator")
			return left
		}
		left = &ast.BinaryExpr{Left: left, Operator: op.Lexeme, Right: right, Span: left.GetSpan().Merge(right.GetSpan())}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expression {
	return p.parseTypeAssertion()
}

func (p *Parser) parseTypeAssertion() ast.Expression {
	expr := p.parsePrefix()
	if expr == nil {
		return nil
	}
	for {
		if p.match(token.As) {
			checked := false
			switch {
			case p.match(token.Bang):
			case p.match(token.Question):
				checked = true
			default:
				p.report(p.peek(), "expected '!' or '?' after 'as'")
				return expr
			}
			assertedType, ok := p.parseType()
			if !ok {
				return expr
			}
			expr = &ast.GoTypeAssertionExpr{Value: expr, Type: assertedType, Checked: checked, Span: expr.GetSpan().Merge(assertedType.Span)}
			continue
		}
		if p.match(token.Question) {
			question := p.previous()
			expr = &ast.PropagateExpr{Value: expr, Span: expr.GetSpan().Merge(question.Span)}
			continue
		}
		break
	}
	return expr
}

func (p *Parser) parsePrefix() ast.Expression {
	if p.match(token.Go) {
		start := p.previous()
		value := p.parsePrefix()
		call, ok := value.(*ast.CallExpr)
		if !ok {
			p.report(start, "go expression requires a function or method call")
			return nil
		}
		return &ast.TaskStartExpr{Call: call, Span: start.Span.Merge(call.Span)}
	}
	if p.match(token.Await) {
		start := p.previous()
		value := p.parsePrefix()
		if value == nil {
			p.report(p.peek(), "expected task expression after 'await'")
			return nil
		}
		return &ast.AwaitExpr{Value: value, Span: start.Span.Merge(value.GetSpan())}
	}
	if p.match(token.Bang, token.Minus, token.Plus, token.Caret, token.Star, token.Ampersand, token.LeftArrow) {
		op := p.previous()
		operand := p.parsePrefix()
		if operand == nil {
			return nil
		}
		return &ast.UnaryExpr{Operator: op.Lexeme, Operand: operand, Span: op.Span.Merge(operand.GetSpan())}
	}
	return p.parseCall()
}

func (p *Parser) parseCall() ast.Expression {
	expr := p.parsePrimary()
	if expr == nil {
		return nil
	}
	for {
		switch {
		case p.match(token.LeftParen):
			args, expanded, end, ok := p.parseArguments()
			if !ok {
				return expr
			}
			expr = &ast.CallExpr{Callee: expr, Arguments: args, Expanded: expanded, Span: expr.GetSpan().Merge(end.Span)}
		case p.match(token.Dot):
			name, ok := p.expect(token.Identifier, "expected member name after '.'")
			if !ok {
				return expr
			}
			expr = &ast.MemberExpr{Object: expr, Name: name.Lexeme, NameSpan: name.Span, Span: expr.GetSpan().Merge(name.Span)}
		case p.at(token.LeftBracket) && isExplicitTypeArgumentCallee(expr):
			start := p.current
			diagnosticCount := len(p.diagnostics)
			typeArguments, ok := p.tryParseCallTypeArguments()
			if ok {
				args, expanded, end, valid := p.parseArguments()
				if !valid {
					return expr
				}
				expr = &ast.CallExpr{Callee: expr, TypeArguments: typeArguments, Arguments: args, Expanded: expanded, Span: expr.GetSpan().Merge(end.Span)}
				continue
			}
			p.current = start
			p.diagnostics = p.diagnostics[:diagnosticCount]
			p.advance()
			expr = p.parseSubscript(expr)
		case p.at(token.Less) && isExplicitTypeArgumentCallee(expr):
			start := p.current
			diagnosticCount := len(p.diagnostics)
			typeArguments, ok := p.tryParseAngleTypeArguments()
			if ok && p.match(token.LeftParen) {
				args, expanded, end, valid := p.parseArguments()
				if !valid {
					return expr
				}
				expr = &ast.CallExpr{Callee: expr, TypeArguments: typeArguments, Arguments: args, Expanded: expanded, Span: expr.GetSpan().Merge(end.Span)}
				continue
			}
			if ok && p.at(token.LeftBrace) {
				if p.disallowCompositeBeforeBlock {
					p.current = start
					p.diagnostics = p.diagnostics[:diagnosticCount]
					return expr
				}
				if _, unqualified := expr.(*ast.IdentifierExpr); unqualified && p.disallowUnqualifiedComposite {
					p.current = start
					p.diagnostics = p.diagnostics[:diagnosticCount]
					return expr
				}
				typeRef, valid := qualifiedTypeExpression(expr)
				if !valid {
					return expr
				}
				typeRef.GenericArguments = typeArguments
				typeRef.Span = typeRef.Span.Merge(p.previous().Span)
				expr = p.parseGoCompositeLiteral(typeRef)
				if expr == nil {
					return nil
				}
				continue
			}
			p.current = start
			p.diagnostics = p.diagnostics[:diagnosticCount]
			return expr
		case p.match(token.LeftBracket):
			expr = p.parseSubscript(expr)
		case p.at(token.LeftBrace):
			if p.disallowCompositeBeforeBlock {
				return expr
			}
			if _, unqualified := expr.(*ast.IdentifierExpr); unqualified && p.disallowUnqualifiedComposite {
				return expr
			}
			typeRef, ok := qualifiedTypeExpression(expr)
			if !ok {
				return expr
			}
			expr = p.parseGoCompositeLiteral(typeRef)
			if expr == nil {
				return nil
			}
		default:
			return expr
		}
	}
}

func (p *Parser) parseSubscript(object ast.Expression) ast.Expression {
	var low ast.Expression
	if !p.at(token.Colon) {
		low = p.parseExpression()
		if low == nil {
			return object
		}
	}
	if !p.match(token.Colon) {
		end, ok := p.expect(token.RightBracket, "expected ']' after index")
		if !ok || low == nil {
			return object
		}
		return &ast.IndexExpr{Object: object, Index: low, Span: object.GetSpan().Merge(end.Span)}
	}

	var high ast.Expression
	if !p.at(token.Colon) && !p.at(token.RightBracket) {
		high = p.parseExpression()
		if high == nil {
			return object
		}
	}
	full := p.match(token.Colon)
	var max ast.Expression
	if full {
		if high == nil {
			p.report(p.previous(), "3-index slice requires a high bound")
		}
		if p.at(token.RightBracket) {
			p.report(p.peek(), "3-index slice requires a max bound")
		} else {
			max = p.parseExpression()
			if max == nil {
				return object
			}
		}
	}
	end, ok := p.expect(token.RightBracket, "expected ']' after slice expression")
	if !ok {
		return object
	}
	return &ast.SliceExpr{Object: object, Low: low, High: high, Max: max, Full: full, Span: object.GetSpan().Merge(end.Span)}
}

func isQualifiedMemberExpression(expr ast.Expression) bool {
	member, ok := expr.(*ast.MemberExpr)
	if !ok {
		return false
	}
	_, ok = member.Object.(*ast.IdentifierExpr)
	return ok
}

func isExplicitTypeArgumentCallee(expr ast.Expression) bool {
	if isQualifiedMemberExpression(expr) {
		return true
	}
	_, ok := expr.(*ast.IdentifierExpr)
	return ok
}

// tryParseCallTypeArguments recognizes package.Function[T, U](...) and the
// explicitly typed builtin calls. The caller rolls parser state and diagnostics
// back when the bracket is an ordinary index expression.
func (p *Parser) tryParseCallTypeArguments() ([]ast.TypeRef, bool) {
	if !p.match(token.LeftBracket) {
		return nil, false
	}
	var arguments []ast.TypeRef
	for {
		argument, ok := p.parseType()
		if !ok {
			return nil, false
		}
		arguments = append(arguments, argument)
		if !p.match(token.Comma) {
			break
		}
	}
	if _, ok := p.expect(token.RightBracket, "expected ']' after explicit Go type arguments"); !ok {
		return nil, false
	}
	if !p.match(token.LeftParen) {
		return nil, false
	}
	return arguments, true
}

// tryParseAngleTypeArguments recognizes the type-argument portion of the
// TypeScript-shaped function<T, U>(...) spelling and qualified Go composite
// types. The caller decides whether '(' or '{' is valid in that context.
func (p *Parser) tryParseAngleTypeArguments() ([]ast.TypeRef, bool) {
	if !p.match(token.Less) {
		return nil, false
	}
	var arguments []ast.TypeRef
	for {
		argument, ok := p.parseType()
		if !ok {
			return nil, false
		}
		arguments = append(arguments, argument)
		if !p.match(token.Comma) {
			break
		}
	}
	if _, ok := p.expectTypeGreater("expected '>' after explicit type arguments"); !ok {
		return nil, false
	}
	return arguments, true
}

func (p *Parser) parseArguments() ([]ast.Expression, bool, token.Token, bool) {
	var args []ast.Expression
	expanded := false
	if !p.at(token.RightParen) {
		for {
			arg := p.parseExpression()
			if arg == nil {
				return nil, false, p.peek(), false
			}
			args = append(args, arg)
			if p.match(token.Ellipsis) {
				if expanded {
					p.report(p.previous(), "a call can contain only one spread argument")
				}
				expanded = true
				if !p.at(token.RightParen) && !p.at(token.Comma) {
					p.report(p.peek(), "expected ',' or ')' after spread argument")
				}
			}
			if !p.match(token.Comma) {
				break
			}
			if p.at(token.RightParen) {
				break
			}
			if expanded {
				p.report(p.peek(), "spread argument must be the final argument")
			}
		}
	}
	end, ok := p.expect(token.RightParen, "expected ')' after arguments")
	return args, expanded, end, ok
}

func (p *Parser) parsePrimary() ast.Expression {
	tok := p.peek()
	switch tok.Kind {
	case token.Identifier, token.This, token.Super:
		p.advance()
		return &ast.IdentifierExpr{Name: tok.Lexeme, Span: tok.Span}
	case token.Integer:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.IntegerLiteral, Text: tok.Lexeme, Span: tok.Span}
	case token.Float:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.FloatLiteral, Text: tok.Lexeme, Span: tok.Span}
	case token.String:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.StringLiteral, Text: tok.Lexeme, Span: tok.Span}
	case token.True, token.False:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.BooleanLiteral, Text: tok.Lexeme, Span: tok.Span}
	case token.Nil:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.NilLiteral, Text: tok.Lexeme, Span: tok.Span}
	case token.Null:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.NullLiteral, Text: tok.Lexeme, Span: tok.Span}
	case token.LeftParen:
		if p.looksLikeArrow() {
			return p.parseArrow()
		}
		start := p.advance()
		previous := p.disallowUnqualifiedComposite
		previousAll := p.disallowCompositeBeforeBlock
		p.disallowUnqualifiedComposite = false
		p.disallowCompositeBeforeBlock = false
		expr := p.parseExpression()
		p.disallowUnqualifiedComposite = previous
		p.disallowCompositeBeforeBlock = previousAll
		end, ok := p.expect(token.RightParen, "expected ')' after expression")
		if !ok || expr == nil {
			return expr
		}
		// Parentheses do not need a dedicated AST node, but preserve their full span
		// when the expression is later used as the callee of a call.
		_ = start.Span.Merge(end.Span)
		return expr
	case token.LeftBracket:
		return p.parseArrayLiteral()
	case token.LeftBrace:
		return p.parseObjectLiteral()
	case token.New:
		return p.parseNew()
	default:
		p.report(tok, "expected expression")
		return nil
	}
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	start := p.advance()
	var elements []ast.Expression
	if !p.at(token.RightBracket) {
		for {
			element := p.parseExpression()
			if element == nil {
				return nil
			}
			elements = append(elements, element)
			if !p.match(token.Comma) {
				break
			}
			if p.at(token.RightBracket) {
				break
			}
		}
	}
	end, ok := p.expect(token.RightBracket, "expected ']' after array literal")
	if !ok {
		return nil
	}
	return &ast.ArrayLiteralExpr{Elements: elements, Span: start.Span.Merge(end.Span)}
}

func (p *Parser) parseObjectLiteral() ast.Expression {
	start := p.advance()
	var fields []ast.ObjectField
	if !p.at(token.RightBrace) {
		for {
			name, ok := p.expect(token.Identifier, "expected object field name")
			if !ok {
				return nil
			}
			if _, ok = p.expect(token.Colon, "expected ':' after object field name"); !ok {
				return nil
			}
			value := p.parseExpression()
			if value == nil {
				return nil
			}
			fields = append(fields, ast.ObjectField{Name: name.Lexeme, NameSpan: name.Span, Value: value, Span: name.Span.Merge(value.GetSpan())})
			if !p.match(token.Comma) {
				break
			}
			if p.at(token.RightBrace) {
				break
			}
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after object literal")
	if !ok {
		return nil
	}
	return &ast.ObjectLiteralExpr{Fields: fields, Span: start.Span.Merge(end.Span)}
}

func qualifiedTypeExpression(expression ast.Expression) (ast.TypeRef, bool) {
	if identifier, ok := expression.(*ast.IdentifierExpr); ok {
		return ast.TypeRef{Name: identifier.Name, NameSpan: identifier.Span, Span: identifier.Span}, true
	}
	member, ok := expression.(*ast.MemberExpr)
	if !ok {
		return ast.TypeRef{}, false
	}
	qualifier, ok := member.Object.(*ast.IdentifierExpr)
	if !ok {
		return ast.TypeRef{}, false
	}
	return ast.TypeRef{
		Name: member.Name, NameSpan: member.NameSpan, Qualifier: qualifier.Name, QualifierSpan: qualifier.Span,
		Go: true, Span: member.Span,
	}, true
}

func (p *Parser) parseGoCompositeLiteral(typeRef ast.TypeRef) ast.Expression {
	start, ok := p.expect(token.LeftBrace, "expected '{' after Go struct type")
	if !ok {
		return nil
	}
	var fields []ast.ObjectField
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		name, valid := p.expect(token.Identifier, "expected Go struct field name")
		if !valid {
			p.synchronizeStatement()
			return nil
		}
		if _, valid = p.expect(token.Colon, "expected ':' after Go struct field name"); !valid {
			p.synchronizeStatement()
			return nil
		}
		value := p.parseExpression()
		if value == nil {
			return nil
		}
		fields = append(fields, ast.ObjectField{Name: name.Lexeme, NameSpan: name.Span, Value: value, Span: name.Span.Merge(value.GetSpan())})
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.RightBrace) {
			break
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after Go struct literal")
	if !ok {
		return nil
	}
	return &ast.GoCompositeLiteralExpr{Type: typeRef, Fields: fields, Span: typeRef.Span.Merge(start.Span).Merge(end.Span)}
}

func (p *Parser) parseNew() ast.Expression {
	start := p.advance()
	name, ok := p.expect(token.Identifier, "expected class name after 'new'")
	if !ok {
		return nil
	}
	var typeArguments []ast.TypeRef
	if p.at(token.Less) {
		var valid bool
		typeArguments, valid = p.tryParseAngleTypeArguments()
		if !valid {
			return nil
		}
	}
	if _, ok = p.expect(token.LeftParen, "expected '(' after class name"); !ok {
		return nil
	}
	arguments, expanded, end, ok := p.parseArguments()
	if !ok {
		return nil
	}
	return &ast.NewExpr{ClassName: name.Lexeme, ClassNameSpan: name.Span, TypeArguments: typeArguments, Arguments: arguments, Expanded: expanded, Span: start.Span.Merge(end.Span)}
}

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
	parameters, ok := p.parseParameters(token.RightParen)
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
	if _, ok = p.expect(token.FatArrow, "expected '=>' after arrow signature"); !ok {
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

func binaryPrecedence(kind token.Kind) int {
	switch kind {
	case token.Or:
		return 1
	case token.And:
		return 2
	case token.Equal, token.StrictEqual, token.NotEqual, token.StrictUnequal:
		return 3
	case token.Less, token.LessEqual, token.Greater, token.GreaterEqual:
		return 4
	case token.Plus, token.Minus, token.Pipe, token.Caret:
		return 5
	case token.Star, token.Slash, token.Percent, token.ShiftLeft, token.ShiftRight, token.Ampersand, token.AndNot:
		return 6
	default:
		return 0
	}
}

func (p *Parser) expect(kind token.Kind, message string) (token.Token, bool) {
	if p.at(kind) {
		return p.advance(), true
	}
	p.report(p.peek(), message)
	return p.peek(), false
}

// expectTypeGreater resolves the only lexical ambiguity between nested generic
// type arguments and the shift-right operator. In a type context, a >> token
// closes two adjacent generic argument lists one > at a time.
func (p *Parser) expectTypeGreater(message string) (token.Token, bool) {
	if p.at(token.Greater) {
		return p.advance(), true
	}
	if p.at(token.ShiftRight) {
		combined := p.peek()
		middle := combined.Span.Start
		middle.Offset++
		middle.Column++
		first := token.Token{
			Kind:   token.Greater,
			Lexeme: ">",
			Span:   source.Span{Path: combined.Span.Path, Start: combined.Span.Start, End: middle},
		}
		p.tokens[p.current] = token.Token{
			Kind:   token.Greater,
			Lexeme: ">",
			Span:   source.Span{Path: combined.Span.Path, Start: middle, End: combined.Span.End},
		}
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
	if p.current == 0 {
		return p.peek()
	}
	return p.tokens[p.current-1]
}

func (p *Parser) advance() token.Token {
	tok := p.peek()
	if p.current < len(p.tokens) {
		p.current++
	}
	return tok
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
		case token.Import, token.Function, token.Public, token.Private, token.Protected, token.Final, token.Class, token.Struct, token.Interface, token.Const, token.Let:
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
