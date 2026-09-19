package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/token"
)

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
		p.synchronizeStatement()
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
