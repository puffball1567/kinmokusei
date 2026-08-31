package lexer

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/token"
)

func TestLexesUnicodeIdentifiersAndOperators(t *testing.T) {
	tokens, diagnostics := Lex("example.otm", "const 合計: int = 20 + 22;")
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	want := []token.Kind{token.Const, token.Identifier, token.Colon, token.Identifier, token.Assign, token.Integer, token.Plus, token.Integer, token.Semicolon, token.EOF}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(want))
	}
	for i, kind := range want {
		if tokens[i].Kind != kind {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].Kind, kind)
		}
	}
	if tokens[1].Lexeme != "合計" {
		t.Errorf("identifier = %q", tokens[1].Lexeme)
	}
}

func TestReportsUnterminatedStringAndContinues(t *testing.T) {
	tokens, diagnostics := Lex("broken.otm", "\"missing\nconst ok: boolean = true;")
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Span.Start.Line != 1 {
		t.Errorf("diagnostic line = %d", diagnostics[0].Span.Start.Line)
	}
	foundConst := false
	for _, tok := range tokens {
		if tok.Kind == token.Const {
			foundConst = true
		}
	}
	if !foundConst {
		t.Error("lexer did not recover after unterminated string")
	}
}

func TestSkipsComments(t *testing.T) {
	tokens, diagnostics := Lex("comments.otm", "/* header */ // line\nfunction main(): void {}")
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	if tokens[0].Kind != token.Function {
		t.Fatalf("first token = %s", tokens[0].Kind)
	}
}

func TestLexesSelectKeywords(t *testing.T) {
	tokens, diagnostics := Lex("select.otm", "select { case <-channel {} default {} }")
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	want := []token.Kind{token.Select, token.LeftBrace, token.Case, token.LeftArrow, token.Identifier, token.LeftBrace, token.RightBrace, token.Default, token.LeftBrace, token.RightBrace, token.RightBrace, token.EOF}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(want))
	}
	for i, kind := range want {
		if tokens[i].Kind != kind {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].Kind, kind)
		}
	}
}

func TestLexesTypeSwitchKeyword(t *testing.T) {
	tokens, diagnostics := Lex("switch.otm", "switch (value) { case const typed as Type {} case nil {} default {} }")
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	want := []token.Kind{token.Switch, token.LeftParen, token.Identifier, token.RightParen, token.LeftBrace, token.Case, token.Const, token.Identifier, token.As, token.Identifier, token.LeftBrace, token.RightBrace, token.Case, token.Nil, token.LeftBrace, token.RightBrace, token.Default, token.LeftBrace, token.RightBrace, token.RightBrace, token.EOF}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(want))
	}
	for i, kind := range want {
		if tokens[i].Kind != kind {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].Kind, kind)
		}
	}
}
