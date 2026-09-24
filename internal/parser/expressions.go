package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

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
			name := p.peek()
			if !token.IsIdentifierName(name.Kind) {
				p.expect(token.Identifier, "expected member name after '.'")
				return expr
			}
			p.advance()
			expr = &ast.MemberExpr{Object: expr, Name: name.Lexeme, NameSpan: name.Span, Span: expr.GetSpan().Merge(name.Span)}
		case p.at(token.LeftBracket) && isExplicitTypeArgumentCallee(expr):
			checkpoint := p.checkpoint()
			typeArguments, ok := p.tryParseCallTypeArguments()
			if ok {
				args, expanded, end, valid := p.parseArguments()
				if !valid {
					return expr
				}
				expr = &ast.CallExpr{Callee: expr, TypeArguments: typeArguments, Arguments: args, Expanded: expanded, Span: expr.GetSpan().Merge(end.Span)}
				continue
			}
			p.restore(checkpoint)
			p.advance()
			expr = p.parseSubscript(expr)
		case p.at(token.Less) && isExplicitTypeArgumentCallee(expr):
			checkpoint := p.checkpoint()
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
					p.restore(checkpoint)
					return expr
				}
				if _, unqualified := expr.(*ast.IdentifierExpr); unqualified && p.disallowUnqualifiedComposite {
					p.restore(checkpoint)
					return expr
				}
				typeRef, valid := qualifiedTypeExpression(expr)
				if !valid {
					p.restore(checkpoint)
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
			p.restore(checkpoint)
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

func isExplicitTypeArgumentCallee(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.IdentifierExpr, *ast.MemberExpr:
		return true
	default:
		return false
	}
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
	case token.Imaginary:
		p.advance()
		return &ast.LiteralExpr{Kind: ast.ImaginaryLiteral, Text: tok.Lexeme, Span: tok.Span}
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
