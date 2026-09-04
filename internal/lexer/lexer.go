package lexer

import (
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

type Lexer struct {
	path        string
	input       string
	offset      int
	line        int
	column      int
	diagnostics []diagnostic.Diagnostic
}

func Lex(path, input string) ([]token.Token, []diagnostic.Diagnostic) {
	l := &Lexer{path: path, input: input, line: 1, column: 1}
	var tokens []token.Token
	for {
		tok := l.next()
		tokens = append(tokens, tok)
		if tok.Kind == token.EOF {
			break
		}
	}
	return tokens, l.diagnostics
}

func (l *Lexer) next() token.Token {
	l.skipTrivia()
	start := l.position()
	if l.eof() {
		return l.makeToken(token.EOF, start)
	}

	r, _ := l.peek()
	if isIdentifierStart(r) {
		l.advance()
		for !l.eof() {
			r, _ = l.peek()
			if !isIdentifierContinue(r) {
				break
			}
			l.advance()
		}
		text := l.input[start.Offset:l.offset]
		return token.Token{Kind: token.LookupIdentifier(text), Lexeme: text, Span: l.span(start)}
	}
	if unicode.IsDigit(r) {
		kind := token.Integer
		l.advance()
		for !l.eof() {
			r, _ = l.peek()
			if !unicode.IsDigit(r) {
				break
			}
			l.advance()
		}
		if r, _ = l.peek(); r == '.' {
			kind = token.Float
			l.advance()
			for !l.eof() {
				r, _ = l.peek()
				if !unicode.IsDigit(r) {
					break
				}
				l.advance()
			}
		}
		return token.Token{Kind: kind, Lexeme: l.input[start.Offset:l.offset], Span: l.span(start)}
	}

	switch r {
	case '"':
		return l.scanString(start)
	case '(':
		l.advance()
		return l.makeToken(token.LeftParen, start)
	case ')':
		l.advance()
		return l.makeToken(token.RightParen, start)
	case '{':
		l.advance()
		return l.makeToken(token.LeftBrace, start)
	case '}':
		l.advance()
		return l.makeToken(token.RightBrace, start)
	case ':':
		l.advance()
		return l.makeToken(token.Colon, start)
	case ',':
		l.advance()
		return l.makeToken(token.Comma, start)
	case ';':
		l.advance()
		return l.makeToken(token.Semicolon, start)
	case '[':
		l.advance()
		return l.makeToken(token.LeftBracket, start)
	case ']':
		l.advance()
		return l.makeToken(token.RightBracket, start)
	case '.':
		l.advance()
		if l.match('.') {
			if l.match('.') {
				return l.makeToken(token.Ellipsis, start)
			}
			tok := l.makeToken(token.Illegal, start)
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: "unexpected character sequence " + tok.Lexeme, Span: tok.Span})
			return tok
		}
		return l.makeToken(token.Dot, start)
	case '+':
		l.advance()
		if l.match('+') {
			return l.makeToken(token.Increment, start)
		}
		if l.match('=') {
			return l.makeToken(token.PlusAssign, start)
		}
		return l.makeToken(token.Plus, start)
	case '-':
		l.advance()
		if l.match('-') {
			return l.makeToken(token.Decrement, start)
		}
		if l.match('=') {
			return l.makeToken(token.MinusAssign, start)
		}
		return l.makeToken(token.Minus, start)
	case '*':
		l.advance()
		if l.match('=') {
			return l.makeToken(token.StarAssign, start)
		}
		return l.makeToken(token.Star, start)
	case '/':
		l.advance()
		if l.match('=') {
			return l.makeToken(token.SlashAssign, start)
		}
		return l.makeToken(token.Slash, start)
	case '%':
		l.advance()
		if l.match('=') {
			return l.makeToken(token.PercentAssign, start)
		}
		return l.makeToken(token.Percent, start)
	case '=':
		l.advance()
		if l.match('>') {
			return l.makeToken(token.FatArrow, start)
		}
		if l.match('=') {
			if l.match('=') {
				return l.makeToken(token.StrictEqual, start)
			}
			return l.makeToken(token.Equal, start)
		}
		return l.makeToken(token.Assign, start)
	case '!':
		l.advance()
		if l.match('=') {
			if l.match('=') {
				return l.makeToken(token.StrictUnequal, start)
			}
			return l.makeToken(token.NotEqual, start)
		}
		return l.makeToken(token.Bang, start)
	case '<':
		l.advance()
		if l.match('-') {
			return l.makeToken(token.LeftArrow, start)
		}
		if l.match('<') {
			if l.match('=') {
				return l.makeToken(token.ShlAssign, start)
			}
			return l.makeToken(token.ShiftLeft, start)
		}
		if l.match('=') {
			return l.makeToken(token.LessEqual, start)
		}
		return l.makeToken(token.Less, start)
	case '>':
		l.advance()
		if l.match('>') {
			if l.match('=') {
				return l.makeToken(token.ShrAssign, start)
			}
			return l.makeToken(token.ShiftRight, start)
		}
		if l.match('=') {
			return l.makeToken(token.GreaterEqual, start)
		}
		return l.makeToken(token.Greater, start)
	case '&':
		l.advance()
		if l.match('&') {
			return l.makeToken(token.And, start)
		}
		if l.match('^') {
			if l.match('=') {
				return l.makeToken(token.AndNotAssign, start)
			}
			return l.makeToken(token.AndNot, start)
		}
		if l.match('=') {
			return l.makeToken(token.AndAssign, start)
		}
		return l.makeToken(token.Ampersand, start)
	case '^':
		l.advance()
		if l.match('=') {
			return l.makeToken(token.XorAssign, start)
		}
		return l.makeToken(token.Caret, start)
	case '?':
		l.advance()
		return l.makeToken(token.Question, start)
	case '|':
		l.advance()
		if l.match('|') {
			return l.makeToken(token.Or, start)
		}
		if l.match('=') {
			return l.makeToken(token.OrAssign, start)
		}
		return l.makeToken(token.Pipe, start)
	default:
		l.advance()
	}

	tok := l.makeToken(token.Illegal, start)
	l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: "unexpected character " + tok.Lexeme, Span: tok.Span})
	return tok
}

func (l *Lexer) scanString(start source.Position) token.Token {
	l.advance()
	terminated := false
	for !l.eof() {
		r, _ := l.peek()
		if r == '"' {
			l.advance()
			terminated = true
			break
		}
		if r == '\n' || r == '\r' {
			break
		}
		if r == '\\' {
			l.advance()
			if !l.eof() {
				l.advance()
			}
			continue
		}
		l.advance()
	}
	tok := l.makeToken(token.String, start)
	if !terminated {
		l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: "unterminated string literal", Span: tok.Span})
	} else if _, err := strconv.Unquote(tok.Lexeme); err != nil {
		l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: "invalid string literal escape", Span: tok.Span})
	}
	return tok
}

func (l *Lexer) skipTrivia() {
	for !l.eof() {
		r, _ := l.peek()
		if unicode.IsSpace(r) {
			l.advance()
			continue
		}
		if r == '/' && l.peekNext('/') {
			for !l.eof() {
				r, _ = l.peek()
				if r == '\n' {
					break
				}
				l.advance()
			}
			continue
		}
		if r == '/' && l.peekNext('*') {
			start := l.position()
			l.advance()
			l.advance()
			closed := false
			for !l.eof() {
				if r, _ = l.peek(); r == '*' && l.peekNext('/') {
					l.advance()
					l.advance()
					closed = true
					break
				}
				l.advance()
			}
			if !closed {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: "unterminated block comment", Span: l.span(start)})
			}
			continue
		}
		break
	}
}

func (l *Lexer) eof() bool { return l.offset >= len(l.input) }

func (l *Lexer) peek() (rune, int) {
	if l.eof() {
		return 0, 0
	}
	return utf8.DecodeRuneInString(l.input[l.offset:])
}

func (l *Lexer) peekNext(want rune) bool {
	_, size := l.peek()
	if size == 0 || l.offset+size >= len(l.input) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.offset+size:])
	return r == want
}

func (l *Lexer) match(want rune) bool {
	r, _ := l.peek()
	if r != want {
		return false
	}
	l.advance()
	return true
}

func (l *Lexer) advance() {
	r, size := l.peek()
	if size == 0 {
		return
	}
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
}

func (l *Lexer) position() source.Position {
	return source.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *Lexer) span(start source.Position) source.Span {
	return source.Span{Path: l.path, Start: start, End: l.position()}
}

func (l *Lexer) makeToken(kind token.Kind, start source.Position) token.Token {
	return token.Token{Kind: kind, Lexeme: l.input[start.Offset:l.offset], Span: l.span(start)}
}

func isIdentifierStart(r rune) bool    { return r == '_' || unicode.IsLetter(r) }
func isIdentifierContinue(r rune) bool { return isIdentifierStart(r) || unicode.IsDigit(r) }
