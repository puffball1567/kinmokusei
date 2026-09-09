package lexer

import (
	"go/scanner"
	gotoken "go/token"

	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func isDecimalDigit(r rune) bool { return '0' <= r && r <= '9' }

// Bound the candidate before asking Go's scanner to validate numeric syntax.
// Passing the whole remaining source to a fresh scanner for every number would
// repeatedly copy that suffix. All characters consumed here are ASCII.
func (l *Lexer) scanNumber(start source.Position) token.Token {
	hex := false
	if l.input[l.offset] == '0' && l.offset+1 < len(l.input) {
		switch l.input[l.offset+1] {
		case 'x', 'X':
			hex = true
			l.advance()
			l.advance()
		case 'b', 'B', 'o', 'O':
			l.advance()
			l.advance()
		}
	}
	digits := func(allowHex bool) {
		for {
			r, _ := l.peek()
			if !isDecimalDigit(r) && r != '_' && !(allowHex && ('a' <= r && r <= 'f' || 'A' <= r && r <= 'F')) {
				return
			}
			l.advance()
		}
	}
	digits(hex)
	// Keep Kinmokusei's suffix spread token intact, including after a number.
	if r, _ := l.peek(); r == '.' && !l.peekNext('.') {
		l.advance()
		digits(hex)
	}
	if r, _ := l.peek(); r == 'e' || r == 'E' || r == 'p' || r == 'P' {
		l.advance()
		if r, _ := l.peek(); r == '+' || r == '-' {
			l.advance()
		}
		digits(false)
	}
	if r, _ := l.peek(); r == 'i' {
		l.advance()
	}
	text := l.input[start.Offset:l.offset]
	var s scanner.Scanner
	file := gotoken.NewFileSet().AddFile(l.path, -1, len(text))
	s.Init(file, []byte(text), func(pos gotoken.Position, message string) {
		point := start
		point.Offset += pos.Offset
		point.Column += pos.Offset
		l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: message, Span: source.Span{Path: l.path, Start: point, End: l.position()}})
	}, 0)
	_, goKind, _ := s.Scan()
	kind := token.Integer
	switch goKind {
	case gotoken.FLOAT:
		kind = token.Float
	case gotoken.IMAG:
		kind = token.Imaginary
	}
	return l.makeToken(kind, start)
}
