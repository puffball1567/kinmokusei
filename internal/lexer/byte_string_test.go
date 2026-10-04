package lexer

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/token"
)

func TestByteStringPrefixAndSpan(t *testing.T) {
	tokens, diagnostics := Lex("bytes.km", `b"\xff\x00" "湯" b "text" binary`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	want := []token.Kind{token.ByteString, token.String, token.Identifier, token.String, token.Identifier, token.EOF}
	if len(tokens) != len(want) {
		t.Fatalf("tokens=%v", tokens)
	}
	for i, kind := range want {
		if tokens[i].Kind != kind {
			t.Fatalf("token %d=%v, want %v", i, tokens[i], kind)
		}
	}
	if tokens[0].Lexeme != `b"\xff\x00"` || tokens[0].Span.Start.Offset != 0 || tokens[0].Span.End.Offset != 11 {
		t.Fatalf("raw literal span=%v", tokens[0])
	}
}

func TestByteStringsValidateEscapesAndRecover(t *testing.T) {
	for _, input := range []string{`b"\xGG"; const good=b"ok";`, "b\"missing\nconst good=b\"ok\";"} {
		tokens, diagnostics := Lex("bytes.km", input)
		if len(diagnostics) != 1 {
			t.Fatalf("%q: %v", input, diagnostics)
		}
		found := false
		for _, tok := range tokens {
			found = found || tok.Kind == token.Const
		}
		if !found {
			t.Fatalf("failed to recover: %v", tokens)
		}
	}
}
