package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseFunction(start token.Token) *ast.FunctionDecl {
	name, ok := p.expect(token.Identifier, "expected function name")
	if !ok {
		p.synchronizeDeclaration()
		return nil
	}
	return p.parseFunctionAfterName(start, name)
}

func (p *Parser) parseFunctionAfterName(start, name token.Token) *ast.FunctionDecl {
	return p.parseFunctionTail(start, name, false)
}

func (p *Parser) parseFunctionTail(start, name token.Token, abstract bool) *ast.FunctionDecl {
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
	var body *ast.BlockStmt
	if abstract && !p.at(token.LeftBrace) {
		end, valid := p.expectTerminator("expected ';' after abstract method signature")
		if !valid {
			return nil
		}
		// Keep a nonnil empty body for AST visitors; Abstract distinguishes it
		// from a concrete method with an empty implementation.
		body = &ast.BlockStmt{Span: end.Span}
	} else {
		if abstract {
			p.report(p.peek(), "abstract methods cannot have a body")
		}
		body = p.parseBlock()
	}
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

func (p *Parser) parseParameters(end token.Kind) ([]ast.Parameter, bool) {
	return p.parseParametersWithInference(end, false)
}

func (p *Parser) parseParametersWithInference(end token.Kind, infer bool) ([]ast.Parameter, bool) {
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
		typeRef := ast.TypeRef{}
		if !infer || p.at(token.Colon) {
			if _, ok = p.expect(token.Colon, "expected ':' after parameter name"); !ok {
				return nil, false
			}
			typeRef, ok = p.parseType()
			if !ok {
				return nil, false
			}
		}
		if variadic && typeRef.IsSpecified() && !typeRef.IsSlice() {
			p.report(name, "rest parameter type must be a slice")
			return nil, false
		}
		span := name.Span
		if typeRef.IsSpecified() {
			span = span.Merge(typeRef.Span)
		}
		parameters = append(parameters, ast.Parameter{Name: name.Lexeme, Type: typeRef, Variadic: variadic, Span: span})
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
