package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func TestParsesEnumDeclarationMatrix(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
enum Status {
  Pending,
  Running = 4,
  Complete,
};
enum WireCode: uint16 { Empty = 0, Ready = 65535, }
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	if len(program.Declarations) != 2 {
		t.Fatalf("declarations = %d", len(program.Declarations))
	}
	status := program.Declarations[0].(*ast.EnumDecl)
	if status.Name != "Status" || status.Underlying.Name != "int" || len(status.Members) != 3 || status.Members[0].Value != nil || status.Members[1].Value == nil {
		t.Fatalf("status enum = %#v", status)
	}
	wire := program.Declarations[1].(*ast.EnumDecl)
	if wire.Name != "WireCode" || wire.Underlying.Name != "uint16" || len(wire.Members) != 2 {
		t.Fatalf("wire enum = %#v", wire)
	}
}

func TestEnumSyntaxFailureMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"missing name", `enum { Value }`, "expected enum name"},
		{"missing body", `enum Status: int;`, "expected '{' after enum name"},
		{"missing member", `enum Status { = 1 }`, "expected enum member name"},
		{"missing separator", `enum Status { Pending Running }`, "expected ',' or '}'"},
		{"missing close", `enum Status { Pending,`, "expected '}' after enum body"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("enum_failure.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			_, diagnostics := Parse(tokens)
			messages := make([]string, len(diagnostics))
			for index, item := range diagnostics {
				messages[index] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestEnumRemainsContextualIdentifier(t *testing.T) {
	_, diagnosticCount := parseSource(t, `function use(enum: int): int { const value = enum; return value; }`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
}
