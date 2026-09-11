package codegen

import (
	"reflect"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestConstructorFactoryParameterNames(t *testing.T) {
	t.Parallel()
	class := &ast.ClassDecl{Name: "Box", TypeParameters: []ast.TypeParameter{{Name: "Box_"}}}
	parameters := []ast.Parameter{{Name: "Box"}, {Name: "Box__"}, {Name: "type"}, {Name: "value"}}
	want := []string{"Box___", "Box__", "type_", "value"}
	for i := 0; i < 2; i++ {
		if got := constructorFactoryParameterNames(class, parameters); !reflect.DeepEqual(got, want) {
			t.Fatalf("names = %v, want %v", got, want)
		}
	}
	if parameters[0].Name != "Box" || class.TypeParameters[0].Name != "Box_" {
		t.Fatal("factory name generation mutated source names")
	}
	if got := constructorFactoryParameterNames(class, nil); len(got) != 0 {
		t.Fatalf("parameterless constructor names = %v", got)
	}
}
