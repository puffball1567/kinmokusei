package parser

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

func TestParsesNullableTypesAndNullLiteral(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class User {}
function maybe(): User | null { return null; }
function pointer(): *User | null { return null; }
function values(): User[] | null { return null; }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	maybe := program.Declarations[1].(*ast.FunctionDecl)
	if !maybe.ReturnType.Nullable || maybe.ReturnType.Name != "User" {
		t.Fatalf("nullable class type = %#v", maybe.ReturnType)
	}
	returned := maybe.Body.Statements[0].(*ast.ReturnStmt)
	if literal, ok := returned.Value.(*ast.LiteralExpr); !ok || literal.Kind != ast.NullLiteral {
		t.Fatalf("return value = %#v", returned.Value)
	}
	pointer := program.Declarations[2].(*ast.FunctionDecl).ReturnType
	if !pointer.Nullable || !pointer.IsPointer() || pointer.Pointee.Nullable {
		t.Fatalf("nullable pointer precedence = %#v", pointer)
	}
	values := program.Declarations[3].(*ast.FunctionDecl).ReturnType
	if !values.Nullable || !values.IsArray() || values.Element.Nullable {
		t.Fatalf("nullable slice precedence = %#v", values)
	}
}

func TestRejectsNilAndMissingNullableTypeSuffix(t *testing.T) {
	for _, source := range []string{
		`class User {} function bad(): User | nil { return nil; }`,
		`class User {} function bad(): User | { return null; }`,
	} {
		if _, diagnosticCount := parseSource(t, source); diagnosticCount == 0 {
			t.Fatalf("expected nullable parser diagnostic for %q", source)
		}
	}
}
