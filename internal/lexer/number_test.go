package lexer

import (
	"go/scanner"
	gotoken "go/token"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/token"
)

func TestNumericLiteralSyntaxMatchesGo(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"0", "42", "4_2", "0600", "0_600", "0o600", "0O600", "0b_101", "0B101", "0xBad_Face", "0X_FF",
		".5", "1.", "1.e2", "1e6", "1E-6", "1_2.3_4e+5_6", "0x1.fp2", "0X_1.FP-2", "0x.8p0", "0x1p4",
		"2i", "0123i", "08i", "0_123i", ".25i", "2.e3i", "1E-2i", "0b11i", "0o10i", "0xFi", "0x1.fp2i",
		"0b2", "0o8", "08", "0x", "0b", "1_", "1__2", "1e", "1e+", "1e_2", "1._2", "0x1.2", "0b1.0", "1p2", "0x1p", "2_i",
	} {
		t.Run(input, func(t *testing.T) {
			var reference scanner.Scanner
			file := gotoken.NewFileSet().AddFile("number.go", -1, len(input))
			reference.Init(file, []byte(input), func(gotoken.Position, string) {}, 0)
			_, goKind, literal := reference.Scan()
			tokens, diagnostics := Lex("number.km", input)
			want := map[gotoken.Token]token.Kind{gotoken.INT: token.Integer, gotoken.FLOAT: token.Float, gotoken.IMAG: token.Imaginary}[goKind]
			if len(tokens) != 2 || tokens[0].Kind != want || tokens[0].Lexeme != literal || (len(diagnostics) == 0) != (reference.ErrorCount == 0) {
				t.Fatalf("tokens=%v diagnostics=%v; Go kind=%v literal=%q errors=%d", tokens, diagnostics, goKind, literal, reference.ErrorCount)
			}
		})
	}
}

func TestNumericLiteralBoundariesAndPositions(t *testing.T) {
	tokens, diagnostics := Lex("number.km", "名前\n  1e-2+3i .5 ... 1... 0xF.member １２")
	want := []token.Kind{token.Identifier, token.Float, token.Plus, token.Imaginary, token.Float, token.Ellipsis, token.Integer, token.Ellipsis, token.Float, token.Identifier, token.Illegal, token.Illegal, token.EOF}
	if len(tokens) != len(want) {
		t.Fatalf("tokens=%v diagnostics=%v", tokens, diagnostics)
	}
	if len(diagnostics) != 3 { // Hex float without exponent, then two non-ASCII digits.
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	for i, kind := range want {
		if tokens[i].Kind != kind {
			t.Fatalf("token %d=%v want=%v", i, tokens[i], kind)
		}
	}
	if pos := tokens[1].Span.Start; pos.Offset != 9 || pos.Line != 2 || pos.Column != 3 {
		t.Fatalf("number position=%v", pos)
	}
	_, diagnostics = Lex("number.km", "名前\n  1e+")
	if len(diagnostics) != 1 || diagnostics[0].Span.Start.Line != 2 || diagnostics[0].Span.Start.Column != 6 || diagnostics[0].Span.Start.Offset != 12 {
		t.Fatalf("diagnostic positions=%v", diagnostics)
	}
}

func FuzzNumericLiteralLexer(f *testing.F) {
	for _, input := range []string{"1e+", "0x1.fp2i", "0b_101", "1...", ".5", "1_2e-3i", "0x", "１２"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		tokens, diagnostics := Lex("fuzz.km", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != token.EOF {
			t.Fatal("missing EOF")
		}
		for _, tok := range tokens {
			if tok.Span.Start.Offset < 0 || tok.Span.End.Offset > len(input) || tok.Span.Start.Offset > tok.Span.End.Offset {
				t.Fatalf("invalid token span: %v", tok)
			}
		}
		for _, d := range diagnostics {
			if d.Span.Start.Offset < 0 || d.Span.End.Offset > len(input) || d.Span.Start.Offset > d.Span.End.Offset {
				t.Fatalf("invalid diagnostic span: %v", d)
			}
		}
	})
}
