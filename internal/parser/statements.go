package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

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
	end, ok := p.expectTerminator("expected ';' after variable declaration")
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
	end, ok := p.expectTerminator("expected ';' after variable declaration")
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
	if p.implicitTerminator() && !p.at(token.Semicolon) {
		return &ast.ThrowStmt{Bare: true, Span: start.Span}
	}
	if p.match(token.Semicolon) {
		return &ast.ThrowStmt{Bare: true, Span: start.Span.Merge(p.previous().Span)}
	}
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expectTerminator("expected ';' after thrown error")
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
	if p.implicitTerminator() && !p.at(token.Semicolon) {
		return &ast.ReturnStmt{Span: start.Span}
	}
	if p.match(token.Semicolon) {
		return &ast.ReturnStmt{Span: start.Span.Merge(p.previous().Span)}
	}
	value := p.parseExpression()
	if value == nil {
		p.synchronizeStatement()
		return nil
	}
	end, ok := p.expectTerminator("expected ';' after return value")
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
			// Do not box a nil *BlockStmt into a non-nil Statement interface
			// when an incomplete else branch fails to parse.
			if block := p.parseBlock(); block != nil {
				elseBranch = block
			}
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
			tok, valid := p.expectTerminator("expected ';' after channel send")
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
				tok, valid := p.expectTerminator("expected ';' after assignment")
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
			tok, valid := p.expectTerminator("expected ';' after assignment")
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
			tok, valid := p.expectTerminator("expected ';' after increment or decrement")
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
		tok, ok := p.expectTerminator("expected ';' after expression")
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

func (p *Parser) parseBranch(start token.Token, kind ast.BranchKind) ast.Statement {
	var label token.Token
	if kind == ast.GotoBranch {
		var ok bool
		label, ok = p.expect(token.Identifier, "expected label name after 'goto'")
		if !ok {
			p.synchronizeStatement()
			return nil
		}
	} else if kind != ast.FallthroughBranch && !p.lineBreakBeforeNext() && p.at(token.Identifier) {
		label = p.advance()
	}
	end, ok := p.expectTerminator("expected ';' after branch statement")
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
	end, ok := p.expectTerminator("expected ';' after call")
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
	end, ok := p.expectTerminator("expected ';' after detached task")
	if !ok {
		p.synchronizeStatement()
		end = p.previous()
	}
	return &ast.DetachStmt{Value: value, Span: start.Span.Merge(end.Span)}
}
