package parser

import (
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func (p *Parser) parseType() (ast.TypeRef, bool) {
	return p.parseTypeInternal(true)
}

func (p *Parser) parseTypeInternal(allowNullable bool) (ast.TypeRef, bool) {
	if p.match(token.Interface) {
		start := p.previous()
		if _, ok := p.expect(token.LeftBrace, "expected '{' after anonymous interface"); !ok {
			return ast.TypeRef{}, false
		}
		var methods []ast.ObjectTypeField
		for !p.at(token.RightBrace) && !p.at(token.EOF) {
			name, ok := p.expect(token.Identifier, "expected anonymous interface method name")
			if !ok {
				p.synchronizeStatement()
				continue
			}
			if _, ok = p.expect(token.LeftParen, "expected '(' after anonymous interface method name"); !ok {
				p.synchronizeStatement()
				continue
			}
			parameters, valid := p.parseParameters(token.RightParen)
			if !valid {
				p.synchronizeTo(token.RightParen)
			}
			if _, ok = p.expect(token.RightParen, "expected ')' after anonymous interface parameters"); !ok {
				p.synchronizeStatement()
				continue
			}
			if _, ok = p.expect(token.Colon, "expected ':' before anonymous interface result"); !ok {
				p.synchronizeStatement()
				continue
			}
			result, ok := p.parseType()
			if !ok {
				p.synchronizeStatement()
				continue
			}
			if _, ok = p.expectTerminator("expected ';' after anonymous interface method"); !ok {
				p.synchronizeStatement()
			}
			parameterTypes := make([]ast.TypeRef, len(parameters))
			for index, parameter := range parameters {
				parameterTypes[index] = parameter.Type
			}
			methodType := ast.TypeRef{Parameters: parameterTypes, Return: &result, Variadic: len(parameters) != 0 && parameters[len(parameters)-1].Variadic, Span: name.Span.Merge(result.Span)}
			methods = append(methods, ast.ObjectTypeField{Name: name.Lexeme, JSONName: name.Lexeme, Type: methodType, Span: name.Span.Merge(result.Span)})
		}
		end, ok := p.expect(token.RightBrace, "expected '}' after anonymous interface")
		if !ok {
			return ast.TypeRef{}, false
		}
		return p.parseTypeSuffix(ast.TypeRef{Go: true, GoInterface: true, ObjectFields: methods, Span: start.Span.Merge(end.Span)}, allowNullable)
	}
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
		length, err := strconv.ParseInt(lengthToken.Lexeme, 0, 64)
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
		if _, ok = p.expectFatArrow("expected '=>' in function type"); !ok {
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
