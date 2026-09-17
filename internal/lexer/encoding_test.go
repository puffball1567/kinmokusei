package lexer

import (
	"fmt"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/token"
)

func TestInvalidUTF8SourceMatrix(t *testing.T) {
	t.Parallel()
	for _, context := range []struct{ name, prefix, suffix string }{
		{"string", `"日本`, `"`},
		{"line comment", "// 日本", "\n"},
		{"block comment", "/* 日本", " */"},
		{"identifier boundary", "日本", " "},
		{"after escape", `"\`, `"`},
		{"end of file", "", ""},
	} {
		for _, invalid := range []string{"\xd4", "\x80", "\xff", "\xc0\xaf", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xe2\x82"} {
			t.Run(fmt.Sprintf("%s/%x", context.name, invalid), func(t *testing.T) {
				prefix := "\n" + context.prefix
				input := prefix + invalid + context.suffix
				if context.name != "end of file" {
					input += "\nconst recovered = 1;"
				}
				tokens, diagnostics := Lex("encoding.km", input)
				count := 0
				for _, diagnostic := range diagnostics {
					if diagnostic.Message != "invalid UTF-8 encoding" {
						continue
					}
					span := diagnostic.Span
					if span.Path != "encoding.km" || span.Start.Offset != len(prefix)+count || span.End.Offset != span.Start.Offset+1 || span.Start.Line != 2 || span.Start.Column != len([]rune(context.prefix))+1+count || span.End.Column != span.Start.Column+1 {
						t.Fatalf("incorrect malformed-byte location: %#v", span)
					}
					count++
				}
				if count != len(invalid) {
					t.Fatalf("got %d encoding errors, want %d: %v", count, len(invalid), diagnostics)
				}
				if tokens[len(tokens)-1].Kind != token.EOF {
					t.Fatal("lexer did not reach EOF")
				}
				if context.name != "end of file" {
					found := false
					for _, item := range tokens {
						found = found || item.Kind == token.Identifier && item.Lexeme == "recovered"
					}
					if !found {
						t.Fatal("lexer did not recover after malformed bytes")
					}
				}
			})
		}
	}
}

func TestValidUTF8AndByteEscapes(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`"日本語 😀 �"`, `"\xFF\324\uFFFD\U0001F600"`, `"\x00\000\u0000\U00000000"`, "// � 日本語\nconst 名 = 1;", "/* � 😀 */ const 名 = 1;"} {
		if _, diagnostics := Lex("valid.km", input); len(diagnostics) != 0 {
			t.Fatalf("valid source %q rejected: %v", input, diagnostics)
		}
	}
}

func TestNULSourceDiagnostics(t *testing.T) {
	t.Parallel()
	for _, context := range []struct{ prefix, suffix string }{
		{`"日本`, `"`}, {"// 日本", "\n"}, {"/* 日本", " */"}, {"日本", " "}, {"", ""},
	} {
		prefix := "\n" + context.prefix
		input := prefix + "\x00" + context.suffix + "\nconst recovered=1;"
		tokens, diagnostics := Lex("nul.km", input)
		found := 0
		for _, d := range diagnostics {
			if d.Message != "NUL character is not allowed in source text; use a string escape" {
				continue
			}
			found++
			if d.Span.Path != "nul.km" || d.Span.Start.Offset != len(prefix) || d.Span.End.Offset != len(prefix)+1 || d.Span.Start.Line != 2 || d.Span.Start.Column != len([]rune(context.prefix))+1 {
				t.Fatalf("incorrect NUL location: %+v", d.Span)
			}
		}
		recovered := false
		for _, item := range tokens {
			recovered = recovered || item.Kind == token.Identifier && item.Lexeme == "recovered"
		}
		if found != 1 || !recovered || tokens[len(tokens)-1].Kind != token.EOF {
			t.Fatalf("diagnostics=%v recovered=%v", diagnostics, recovered)
		}
	}
}
