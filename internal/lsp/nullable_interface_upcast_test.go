package lsp

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestNullableInterfaceUpcastCompletionPreservesContract(t *testing.T) {
	t.Parallel()
	target := ast.TypeRef{Name: "Reader", Nullable: true, Interface: true, GenericArguments: []ast.TypeRef{{Name: "int"}}}
	expression := &ast.ClassUpcastExpr{NullableInterface: true, TargetType: target}
	got, ok := expressionCompletionType(&ast.Program{}, expression)
	if !ok || got.Name != "Reader" || !got.Nullable || !got.Interface || len(got.GenericArguments) != 1 || got.GenericArguments[0].Name != "int" {
		t.Fatalf("lost nullable generic interface contract: %#v, %v", got, ok)
	}
}
