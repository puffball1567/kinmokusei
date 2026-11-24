package parser

import (
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func regexExpression(tok token.Token) *ast.RegexExpr {
	text := tok.Lexeme[3:]
	text = strings.TrimSuffix(text, "\"")
	var pattern strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) {
			if text[i+1] != '"' {
				pattern.WriteByte('\\')
			}
			i++
		}
		pattern.WriteByte(text[i])
	}
	return &ast.RegexExpr{Pattern: pattern.String(), Span: tok.Span}
}

// Keep existing identifiers named match, including calls in their own statements.
func (p *Parser) looksLikeMatch() bool {
	if p.current+1 >= len(p.tokens) || p.tokens[p.current+1].Kind != token.LeftParen {
		return false
	}
	depth := 0
	for i := p.current + 1; i < len(p.tokens)-1; i++ {
		switch p.tokens[i].Kind {
		case token.LeftParen:
			depth++
		case token.RightParen:
			depth--
			if depth == 0 {
				if p.tokens[i+1].Kind != token.LeftBrace {
					return false
				}
				if i+2 < len(p.tokens) && (p.tokens[i+2].Kind == token.Case || p.tokens[i+2].Kind == token.Default) {
					return true
				}
				return !p.disallowCompositeBeforeBlock && p.tokens[i+1].Span.Start.Line == p.tokens[i].Span.End.Line
			}
		}
	}
	return false
}

func (p *Parser) parseMatch() ast.Expression {
	start := p.advance()
	value := p.parseMatchParenthesized("match value")
	if value == nil {
		return nil
	}
	if _, ok := p.expect(token.LeftBrace, "expected '{' after match value"); !ok {
		return nil
	}
	match := &ast.MatchExpr{Value: value}
	for !p.at(token.RightBrace) && !p.at(token.EOF) {
		start := p.advance()
		var arm ast.MatchArm
		switch start.Kind {
		case token.Case:
			pattern := p.parseMatchPattern()
			if pattern == nil {
				return nil
			}
			arm.Pattern = *pattern
			if p.match(token.If) {
				arm.Guard = p.parseMatchParenthesized("match guard")
				if arm.Guard == nil {
					return nil
				}
			}
		case token.Default:
			arm.Pattern = ast.MatchPattern{Kind: ast.MatchWildcard, Span: start.Span}
		default:
			p.report(start, "expected 'case' or 'default' in match")
			return nil
		}
		if _, ok := p.expect(token.FatArrow, "expected '=>' before match arm value"); !ok {
			return nil
		}
		arm.Value = p.parseExpression()
		if arm.Value == nil {
			return nil
		}
		arm.Span = start.Span.Merge(arm.Value.GetSpan())
		match.Arms = append(match.Arms, arm)
		if !p.match(token.Comma) && !p.implicitTerminator() {
			p.report(p.peek(), "expected ',' or a newline after match arm")
			return nil
		}
	}
	end, ok := p.expect(token.RightBrace, "expected '}' after match arms")
	if !ok {
		return nil
	}
	match.Span = start.Span.Merge(end.Span)
	return match
}

func (p *Parser) parseMatchParenthesized(context string) ast.Expression {
	if _, ok := p.expect(token.LeftParen, "expected '(' before "+context); !ok {
		return nil
	}
	unqualified, beforeBlock := p.disallowUnqualifiedComposite, p.disallowCompositeBeforeBlock
	p.disallowUnqualifiedComposite, p.disallowCompositeBeforeBlock = false, false
	value := p.parseExpression()
	p.disallowUnqualifiedComposite, p.disallowCompositeBeforeBlock = unqualified, beforeBlock
	if _, ok := p.expect(token.RightParen, "expected ')' after "+context); !ok {
		return nil
	}
	return value
}

func (p *Parser) parseMatchPattern() *ast.MatchPattern {
	if !p.enterNesting() {
		return nil
	}
	defer p.leaveNesting()
	start := p.peek()
	pattern := &ast.MatchPattern{Span: start.Span}
	switch start.Kind {
	case token.Identifier:
		p.advance()
		if start.Lexeme == "_" {
			return pattern
		}
		if (start.Lexeme == "ok" || start.Lexeme == "fail") && p.match(token.LeftParen) {
			pattern.Kind = ast.MatchResultOK
			if start.Lexeme == "fail" {
				pattern.Kind = ast.MatchResultFail
			}
			if !p.at(token.RightParen) {
				pattern.Payload = p.parseMatchPattern()
				if pattern.Payload == nil {
					return nil
				}
			}
			end, ok := p.expect(token.RightParen, "expected ')' after Result pattern")
			if !ok {
				return nil
			}
			pattern.Span = start.Span.Merge(end.Span)
			return pattern
		}
		if !p.match(token.Dot) {
			p.report(start, "expected a literal, 'Enum.Member', Result pattern, '_', or a 'const' binding in match pattern")
			return nil
		}
		name := p.peek()
		if !token.IsIdentifierName(name.Kind) {
			p.report(name, "expected enum member name after '.' in match pattern")
			return nil
		}
		p.advance()
		pattern.Kind = ast.MatchLiteral
		pattern.Literal = &ast.MemberExpr{
			Object: &ast.IdentifierExpr{Name: start.Lexeme, Span: start.Span},
			Name:   name.Lexeme, NameSpan: name.Span, Span: start.Span.Merge(name.Span),
		}
	case token.Const:
		p.advance()
		name, ok := p.expect(token.Identifier, "expected binding name after 'const' in match pattern")
		if !ok {
			return nil
		}
		pattern.Kind = ast.MatchBinding
		pattern.Binding = ast.Binding{Name: name.Lexeme, Span: name.Span}
		if p.match(token.Bang) {
			pattern.Kind = ast.MatchNonNullBinding
		}
	case token.LeftBrace:
		p.advance()
		pattern.Kind = ast.MatchRecord
		for !p.at(token.RightBrace) && !p.at(token.EOF) {
			name := p.peek()
			if !token.IsIdentifierName(name.Kind) {
				p.report(name, "expected field name in record pattern")
				return nil
			}
			p.advance()
			if _, ok := p.expect(token.Colon, "expected ':' after record pattern field"); !ok {
				return nil
			}
			field := p.parseMatchPattern()
			if field == nil {
				return nil
			}
			pattern.Fields = append(pattern.Fields, ast.MatchField{Name: name.Lexeme, NameSpan: name.Span, Pattern: *field})
			if !p.match(token.Comma) {
				break
			}
		}
		if _, ok := p.expect(token.RightBrace, "expected '}' after record pattern"); !ok {
			return nil
		}
	case token.LeftBracket:
		p.advance()
		pattern.Kind = ast.MatchSequence
		for !p.at(token.RightBracket) && !p.at(token.EOF) {
			element := p.parseMatchPattern()
			if element == nil {
				return nil
			}
			pattern.Elements = append(pattern.Elements, *element)
			if !p.match(token.Comma) {
				break
			}
		}
		if _, ok := p.expect(token.RightBracket, "expected ']' after sequence pattern"); !ok {
			return nil
		}
	case token.String, token.ByteString, token.Regex, token.Integer, token.Float, token.True, token.False, token.Null:
		pattern.Kind = ast.MatchLiteral
		pattern.Literal = p.parsePrimary()
	case token.Minus:
		p.advance()
		if !p.at(token.Integer) && !p.at(token.Float) {
			p.report(p.peek(), "expected a numeric literal after '-' in match pattern")
			return nil
		}
		literal := p.parsePrimary()
		pattern.Kind = ast.MatchLiteral
		pattern.Literal = &ast.UnaryExpr{Operator: "-", Operand: literal, Span: start.Span.Merge(literal.GetSpan())}
	default:
		p.report(start, "expected match pattern")
		return nil
	}
	pattern.Span = start.Span.Merge(p.previous().Span)
	return pattern
}
