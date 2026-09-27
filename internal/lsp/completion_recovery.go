package lsp

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/token"
)

// Analyze the receiver without an incomplete selector. Blank the whole member
// token, including any suffix after the cursor, keeping original byte offsets.
// This text is a request-local overlay; the editor document is never changed.
func memberCompletionAnalysisText(value string, offset int, prefix string) string {
	start := offset - len(prefix) - 1
	if start < 0 || offset > len(value) || value[start] != '.' {
		return value
	}
	end := offset
	for end < len(value) {
		r, size := utf8.DecodeRuneInString(value[end:])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		end += size
	}
	result := []byte(value)
	for index := start; index < end; index++ {
		if result[index] != '\r' && result[index] != '\n' {
			result[index] = ' '
		}
	}
	recovered := string(result)
	if strings.TrimSpace(value[end:]) == "" {
		recovered = closeCompletionDelimiters(recovered)
	}
	return recovered
}

// At EOF an unfinished call/index/body can hide the receiver's declaration
// from parser recovery. Only append matching closers; do not guess how to
// repair mismatched delimiters or incomplete strings/comments.
func closeCompletionDelimiters(value string) string {
	tokens, diagnostics := lexer.Lex("<completion-recovery>", value)
	if len(diagnostics) != 0 {
		return value
	}
	var closers []token.Kind
	for _, item := range tokens {
		switch item.Kind {
		case token.LeftParen:
			closers = append(closers, token.RightParen)
		case token.LeftBracket:
			closers = append(closers, token.RightBracket)
		case token.LeftBrace:
			closers = append(closers, token.RightBrace)
		case token.RightParen, token.RightBracket, token.RightBrace:
			if len(closers) == 0 || closers[len(closers)-1] != item.Kind {
				return value
			}
			closers = closers[:len(closers)-1]
		}
	}
	if len(closers) == 0 {
		return value
	}
	var result strings.Builder
	result.WriteString(value)
	result.WriteByte('\n')
	for index := len(closers) - 1; index >= 0; index-- {
		switch closers[index] {
		case token.RightParen:
			result.WriteByte(')')
		case token.RightBracket:
			result.WriteByte(']')
		case token.RightBrace:
			result.WriteByte('}')
		}
	}
	return result.String()
}
