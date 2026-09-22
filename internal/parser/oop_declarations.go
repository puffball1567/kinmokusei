package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseInterface(start token.Token) *ast.InterfaceDecl {
	name, ok := p.expect(token.Identifier, "expected interface name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	typeParameters, typeParametersValid := p.parseTypeParameters("interface")
	declaration := &ast.InterfaceDecl{Name: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters}
	if p.match(token.Extends) {
		for {
			base, valid := p.parseType()
			if !valid {
				return nil
			}
			declaration.Bases = append(declaration.Bases, base)
			if !p.match(token.Comma) {
				break
			}
		}
	}
	if _, ok = p.expect(token.LeftBrace, "expected '{' after interface name"); !ok {
		return nil
	}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		if p.at(token.Identifier) && (p.peek().Lexeme == "get" || p.peek().Lexeme == "set") && p.atNext(token.Identifier) {
			if accessor := p.parseAccessor(p.advance(), true); accessor != nil {
				declaration.Methods = append(declaration.Methods, ast.InterfaceMethod{Accessor: accessor.Accessor,
					Name: accessor.Name, NameSpan: accessor.NameSpan, Parameters: accessor.Parameters, ReturnType: accessor.ReturnType, Span: accessor.Span})
			}
			continue
		}
		methodStart, valid := p.expect(token.Function, "expected interface method")
		if !valid {
			// Statement recovery can stop immediately at a statement keyword or
			// after a semicolon. Neither is a valid interface member: consume
			// the offending token so malformed bodies cannot loop forever.
			p.advance()
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
		returnType, valid := p.parseCallableReturnType()
		if !valid {
			p.synchronizeStatement()
			continue
		}
		end, valid := p.expectTerminator("expected ';' after interface method")
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
	class := &ast.ClassDecl{Name: name.Lexeme, SourceName: name.Lexeme, NameSpan: name.Span, TypeParameters: typeParameters}
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
		decorators := p.parseDecorators()
		visibility := ast.Private
		if p.match(token.Public) {
			visibility = ast.Public
		} else if p.match(token.Protected) {
			visibility = ast.Protected
		} else {
			p.match(token.Private)
		}
		static, virtual, override, final, abstract := false, false, false, false, false
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
			case p.match(token.Abstract):
				if abstract {
					p.report(p.previous(), "duplicate abstract modifier")
				}
				abstract = true
			default:
				goto modifiersComplete
			}
		}
	modifiersComplete:
		switch {
		case p.at(token.Identifier) && (p.peek().Lexeme == "get" || p.peek().Lexeme == "set") && p.atNext(token.Identifier):
			accessor := p.parseAccessor(p.advance(), abstract)
			if accessor != nil {
				accessor.Decorators = decorators
				accessor.Visibility = visibility
				accessor.Static, accessor.Virtual, accessor.Override, accessor.Final, accessor.Abstract = static, virtual, override, final, abstract
				class.Methods = append(class.Methods, accessor)
			}
		case p.match(token.Constructor):
			if static || virtual || override || final || abstract {
				p.report(p.previous(), "constructor cannot have static, virtual, override, final, or abstract modifiers")
			}
			constructor := p.parseConstructor(p.previous())
			if constructor != nil {
				constructor.Decorators = decorators
			}
			if class.Constructor != nil {
				p.report(p.previous(), "class can only declare one constructor")
			} else {
				class.Constructor = constructor
			}
		case p.match(token.Function):
			var function *ast.FunctionDecl
			if abstract {
				function = p.parseAbstractMethod(p.previous())
			} else {
				function = p.parseFunction(p.previous())
			}
			if function != nil {
				class.Methods = append(class.Methods, &ast.MethodDecl{
					Decorators: decorators,
					Name:       function.Name, NameSpan: function.NameSpan, TypeParameters: function.TypeParameters, Parameters: function.Parameters, ReturnType: function.ReturnType,
					Body: function.Body, Visibility: visibility, Static: static, Virtual: virtual, Override: override, Final: final, Abstract: abstract, Span: function.Span,
				})
			}
		case p.at(token.Identifier) || p.at(token.Const):
			constant := p.match(token.Const)
			if constant && !static {
				p.report(p.previous(), "class constants require the static modifier")
			}
			if virtual || override || final || abstract {
				p.report(p.peek(), "fields cannot have virtual, override, final, or abstract modifiers")
			}
			fieldName, valid := p.expect(token.Identifier, "expected field name")
			if !valid {
				p.synchronizeStatement()
				continue
			}
			if _, ok = p.expect(token.Colon, "expected ':' after field name"); !ok {
				p.synchronizeStatement()
				continue
			}
			fieldType, valid := p.parseType()
			if !valid {
				p.synchronizeStatement()
				continue
			}
			var initializer ast.Expression
			if p.match(token.Assign) {
				initializer = p.parseExpression()
			}
			end, valid := p.expectTerminator("expected ';' after field declaration")
			if !valid {
				p.synchronizeStatement()
				end = p.previous()
			}
			class.Fields = append(class.Fields, ast.FieldDecl{Decorators: decorators, Static: static, Constant: constant, Name: fieldName.Lexeme, NameSpan: fieldName.Span, Type: fieldType, Initializer: initializer, Visibility: visibility, Span: fieldName.Span.Merge(end.Span)})
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
		end, valid := p.expectTerminator("expected ';' after struct field declaration")
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
		decorators := p.parseDecorators()
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
		parameters = append(parameters, ast.Parameter{Decorators: decorators, Name: name.Lexeme, Type: typeRef, Variadic: variadic, Visibility: visibility, IsField: isField, Span: name.Span.Merge(typeRef.Span)})
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
