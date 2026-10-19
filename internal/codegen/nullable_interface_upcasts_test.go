package codegen

import (
	"bytes"
	goast "go/ast"
	"go/format"
	"go/token"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestNullableInterfaceUpcastEvaluatesOutsideHelperScope(t *testing.T) {
	t.Parallel()
	expr := &ast.ClassUpcastExpr{
		Value: &ast.CallExpr{Callee: &ast.IdentifierExpr{Name: "value"}}, NullableInterface: true,
		SourceType: ast.TypeRef{Name: "Box", GenericArguments: []ast.TypeRef{{Name: "int"}}},
		TargetType: ast.TypeRef{Name: "Reader", Interface: true, Nullable: true, GenericArguments: []ast.TypeRef{{Name: "int"}}},
	}
	generated, err := generateExpression(expr)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), generated); err != nil {
		t.Fatal(err)
	}
	want := `func(value *Box[int]) Reader[int] {
	if value == nil {
		return nil
	}
	return value
}(value())`
	if got := out.String(); got != want {
		t.Fatalf("upcast = %s; want %s", got, want)
	}
	call := generated.(*goast.CallExpr)
	if len(call.Args) != 1 {
		t.Fatalf("source must be evaluated as one outside argument: %#v", call)
	}
}
