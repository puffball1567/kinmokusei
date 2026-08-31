package lexer

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/puffball1567/onsentamago/internal/token"
)

func TestTokenMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		kind   token.Kind
	}{
		{"identifier", "name_2", token.Identifier},
		{"integer", "42", token.Integer},
		{"float", "42.5", token.Float},
		{"string", `"text"`, token.String},
		{"function", "function", token.Function},
		{"const", "const", token.Const},
		{"let", "let", token.Let},
		{"return", "return", token.Return},
		{"if", "if", token.If},
		{"else", "else", token.Else},
		{"true", "true", token.True},
		{"false", "false", token.False},
		{"import", "import", token.Import},
		{"from", "from", token.From},
		{"while", "while", token.While},
		{"for", "for", token.For},
		{"of", "of", token.Of},
		{"break", "break", token.Break},
		{"continue", "continue", token.Continue},
		{"new", "new", token.New},
		{"class", "class", token.Class},
		{"struct", "struct", token.Struct},
		{"constructor", "constructor", token.Constructor},
		{"public", "public", token.Public},
		{"private", "private", token.Private},
		{"protected", "protected", token.Protected},
		{"final", "final", token.Final},
		{"static", "static", token.Static},
		{"this", "this", token.This},
		{"interface", "interface", token.Interface},
		{"implements", "implements", token.Implements},
		{"go", "go", token.Go},
		{"defer", "defer", token.Defer},
		{"nil", "nil", token.Nil},
		{"null", "null", token.Null},
		{"try", "try", token.Try},
		{"catch", "catch", token.Catch},
		{"finally", "finally", token.Finally},
		{"throw", "throw", token.Throw},
		{"as", "as", token.As},
		{"export", "export", token.Export},
		{"left parenthesis", "(", token.LeftParen},
		{"right parenthesis", ")", token.RightParen},
		{"left brace", "{", token.LeftBrace},
		{"right brace", "}", token.RightBrace},
		{"colon", ":", token.Colon},
		{"comma", ",", token.Comma},
		{"semicolon", ";", token.Semicolon},
		{"left bracket", "[", token.LeftBracket},
		{"right bracket", "]", token.RightBracket},
		{"dot", ".", token.Dot},
		{"ellipsis", "...", token.Ellipsis},
		{"assign", "=", token.Assign},
		{"plus assign", "+=", token.PlusAssign},
		{"minus assign", "-=", token.MinusAssign},
		{"star assign", "*=", token.StarAssign},
		{"slash assign", "/=", token.SlashAssign},
		{"percent assign", "%=", token.PercentAssign},
		{"and assign", "&=", token.AndAssign},
		{"or assign", "|=", token.OrAssign},
		{"xor assign", "^=", token.XorAssign},
		{"and-not assign", "&^=", token.AndNotAssign},
		{"shift-left assign", "<<=", token.ShlAssign},
		{"shift-right assign", ">>=", token.ShrAssign},
		{"increment", "++", token.Increment},
		{"decrement", "--", token.Decrement},
		{"arrow", "=>", token.FatArrow},
		{"plus", "+", token.Plus},
		{"minus", "-", token.Minus},
		{"star", "*", token.Star},
		{"slash", "/", token.Slash},
		{"percent", "%", token.Percent},
		{"bang", "!", token.Bang},
		{"equal", "==", token.Equal},
		{"strict equal", "===", token.StrictEqual},
		{"not equal", "!=", token.NotEqual},
		{"strict unequal", "!==", token.StrictUnequal},
		{"less", "<", token.Less},
		{"left arrow", "<-", token.LeftArrow},
		{"less equal", "<=", token.LessEqual},
		{"greater", ">", token.Greater},
		{"greater equal", ">=", token.GreaterEqual},
		{"and", "&&", token.And},
		{"or", "||", token.Or},
		{"ampersand", "&", token.Ampersand},
		{"pipe", "|", token.Pipe},
		{"caret", "^", token.Caret},
		{"shift left", "<<", token.ShiftLeft},
		{"shift right", ">>", token.ShiftRight},
		{"and not", "&^", token.AndNot},
		{"question", "?", token.Question},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, diagnostics := Lex("matrix.otm", test.source)
			if len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
			if len(tokens) != 2 || tokens[0].Kind != test.kind || tokens[0].Lexeme != test.source || tokens[1].Kind != token.EOF {
				t.Fatalf("tokens = %#v, want %s followed by EOF", tokens, test.kind)
			}
		})
	}
}

func TestLexerBoundaryAndFailureMatrix(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantKind    token.Kind
		wantMessage string
	}{
		{"empty", "", token.EOF, ""},
		{"whitespace only", " \t\r\n", token.EOF, ""},
		{"trailing decimal point", "1.", token.Float, ""},
		{"escaped string", `"line\nquote\""`, token.String, ""},
		{"invalid escape", `"\q"`, token.String, "invalid string literal escape"},
		{"unterminated string at eof", `"open`, token.String, "unterminated string literal"},
		{"unterminated block comment", "/* open", token.EOF, "unterminated block comment"},
		{"backslash", `\`, token.Illegal, "unexpected character"},
		{"double dot", "..", token.Illegal, "unexpected character sequence"},
		{"unknown punctuation", "@", token.Illegal, "unexpected character"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, diagnostics := Lex("failure.otm", test.source)
			if len(tokens) == 0 || tokens[0].Kind != test.wantKind || tokens[len(tokens)-1].Kind != token.EOF {
				t.Fatalf("tokens = %#v", tokens)
			}
			if test.wantMessage == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("diagnostics = %v", diagnostics)
				}
			} else if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, test.wantMessage) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.wantMessage)
			}
		})
	}
}

func TestAmbiguousOperatorBoundaryMatrix(t *testing.T) {
	tokens, diagnostics := Lex("operators.otm", `&& & &^ ^ | || << <- <= < >> >= >`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	want := []token.Kind{
		token.And, token.Ampersand, token.AndNot, token.Caret, token.Pipe, token.Or,
		token.ShiftLeft, token.LeftArrow, token.LessEqual, token.Less,
		token.ShiftRight, token.GreaterEqual, token.Greater, token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %#v", tokens)
	}
	for index := range want {
		if tokens[index].Kind != want[index] {
			t.Errorf("token %d = %s, want %s", index, tokens[index].Kind, want[index])
		}
	}
}

func TestUpdateOperatorBoundaryMatrix(t *testing.T) {
	tokens, diagnostics := Lex("updates.otm", `+ += ++ - -= -- * *= / /= % %= & &= && &^ &^= | |= || ^ ^= < << <<= <- <= > >> >>= >=`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	want := []token.Kind{
		token.Plus, token.PlusAssign, token.Increment,
		token.Minus, token.MinusAssign, token.Decrement,
		token.Star, token.StarAssign, token.Slash, token.SlashAssign, token.Percent, token.PercentAssign,
		token.Ampersand, token.AndAssign, token.And, token.AndNot, token.AndNotAssign,
		token.Pipe, token.OrAssign, token.Or, token.Caret, token.XorAssign,
		token.Less, token.ShiftLeft, token.ShlAssign, token.LeftArrow, token.LessEqual,
		token.Greater, token.ShiftRight, token.ShrAssign, token.GreaterEqual, token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %#v", tokens)
	}
	for index := range want {
		if tokens[index].Kind != want[index] {
			t.Errorf("token %d = %s, want %s", index, tokens[index].Kind, want[index])
		}
	}
}

func TestTracksRuneColumnsAndByteOffsets(t *testing.T) {
	tokens, diagnostics := Lex("positions.otm", "α\n  β")
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	second := tokens[1]
	if second.Span.Start.Line != 2 || second.Span.Start.Column != 3 {
		t.Fatalf("second start = %#v", second.Span.Start)
	}
	wantOffset := len("α\n  ")
	if second.Span.Start.Offset != wantOffset || second.Span.End.Offset != wantOffset+len("β") {
		t.Fatalf("second span = %#v", second.Span)
	}
}

func FuzzLexNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"", "function main(): void {}", `export c("ontama_value") function value(): int32 { return 1; }`, "const λ = (x: int) => x + 1;", "const task: Task<Result<int>> = go load(); const value = await task?; detach go notify();", "try { throw err; } catch (caught: error) {} finally {}", "call(values...);", "copyArray[[3]int](values);", "viewArray[[3]int](values);", "select { case <-channel {} default {} }", "switch (value) { case const typed as Type {} case nil {} default {} }", "^value & mask | other &^ cleared << 2 >> 1", "value += 1; value &^= 3; value <<= 2; value++; value--;", "Outer<Middle<Inner<int>>>", "&& & &= &^ &^= ^ ^= | |= || << <<= <- <= < >> >>= >= >", "..", "\xff\x00", "/*", `"\q"`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		tokens, diagnostics := Lex("fuzz.otm", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != token.EOF {
			t.Fatalf("lexer result does not end in EOF: %#v", tokens)
		}
		previousEnd := 0
		for i, item := range tokens {
			if item.Span.Start.Offset < previousEnd || item.Span.Start.Offset > item.Span.End.Offset || item.Span.End.Offset > len(input) {
				t.Fatalf("invalid span at token %d: %#v for %d-byte input", i, item.Span, len(input))
			}
			if utf8.ValidString(input) && item.Span.Start.Offset < len(input) && !utf8.RuneStart(input[item.Span.Start.Offset]) {
				t.Fatalf("token %d starts inside a UTF-8 sequence: %#v", i, item.Span)
			}
			previousEnd = item.Span.End.Offset
		}
		for i, item := range diagnostics {
			if item.Span.Start.Offset < 0 || item.Span.Start.Offset > item.Span.End.Offset || item.Span.End.Offset > len(input) {
				t.Fatalf("invalid diagnostic span at %d: %#v", i, item.Span)
			}
		}
	})
}
