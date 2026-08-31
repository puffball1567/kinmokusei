package lsp

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

func TestGenericConstraintDeclarationDetailsPreserveSourceSyntax(t *testing.T) {
	constraint := ast.TypeRef{Name: "comparable"}
	parameters := []ast.TypeParameter{{Name: "T", Constraint: &constraint}, {Name: "U"}}
	if got, want := formatTypeParameters(parameters), "<T extends comparable, U>"; got != want {
		t.Fatalf("formatTypeParameters() = %q, want %q", got, want)
	}
	function := &ast.FunctionDecl{
		Name: "equal", TypeParameters: parameters,
		Parameters: []ast.Parameter{{Name: "left", Type: ast.TypeRef{Name: "T"}}, {Name: "right", Type: ast.TypeRef{Name: "T"}}},
		ReturnType: ast.TypeRef{Name: "boolean"},
	}
	if got, want := functionDeclarationDetail(function), "function equal<T extends comparable, U>(left: T, right: T): boolean"; got != want {
		t.Fatalf("functionDeclarationDetail() = %q, want %q", got, want)
	}
}
