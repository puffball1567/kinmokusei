package codegen

import (
	goast "go/ast"
	"testing"

	kmast "github.com/puffball1567/kinmokusei/internal/ast"
)

func TestObjectLiteralSeparatesTypeAndInitializerOrder(t *testing.T) {
	t.Parallel()
	for _, names := range [][]string{{"Zeta", "Alpha"}, {"", ""}} {
		expr := &kmast.ObjectLiteralExpr{
			Fields: []kmast.ObjectField{
				{Name: "zeta", Value: &kmast.IdentifierExpr{Name: "first"}},
				{Name: "alpha", Value: &kmast.IdentifierExpr{Name: "second"}},
			},
			ResolvedFieldTypes: []kmast.TypeRef{{Name: "int"}, {Name: "int"}},
			ResolvedFieldNames: names,
		}
		generated, err := generateExpression(expr)
		if err != nil {
			t.Fatal(err)
		}
		literal := generated.(*goast.CompositeLit)
		fields := literal.Type.(*goast.StructType).Fields.List
		for index, sourceIndex := range []int{1, 0} {
			want := names[sourceIndex]
			if want == "" {
				want = expr.Fields[sourceIndex].Name
			}
			if got := fields[index].Names[0].Name; got != want {
				t.Fatalf("type field %d = %q, want %q", index, got, want)
			}
		}
		for index, element := range literal.Elts {
			field := element.(*goast.KeyValueExpr)
			wantName := names[index]
			if wantName == "" {
				wantName = expr.Fields[index].Name
			}
			if field.Key.(*goast.Ident).Name != wantName || field.Value.(*goast.Ident).Name != expr.Fields[index].Value.(*kmast.IdentifierExpr).Name {
				t.Fatalf("initializer %d lost source order or checked name", index)
			}
		}
	}
}
