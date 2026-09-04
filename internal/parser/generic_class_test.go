package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestParsesGenericClassAndExplicitConstruction(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Pair<K extends comparable, V> {
  constructor(public key: K, public value: V) {}
  public function read(): V { return this.value; }
}
function use(): Pair<string, int> { return new Pair<string, int>("answer", 42); }
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	if class.Name != "Pair" || len(class.TypeParameters) != 2 || class.TypeParameters[0].Name != "K" || class.TypeParameters[0].Constraint == nil || class.TypeParameters[0].Constraint.Name != "comparable" || class.TypeParameters[1].Name != "V" {
		t.Fatalf("generic class = %#v", class)
	}
	function := program.Declarations[1].(*ast.FunctionDecl)
	returned := function.Body.Statements[0].(*ast.ReturnStmt)
	constructed := returned.Value.(*ast.NewExpr)
	if constructed.ClassName != "Pair" || len(constructed.TypeArguments) != 2 || constructed.TypeArguments[0].Name != "string" || constructed.TypeArguments[1].Name != "int" || len(constructed.Arguments) != 2 {
		t.Fatalf("generic construction = %#v", constructed)
	}
}

func TestGenericClassSyntaxFailureMatrix(t *testing.T) {
	for _, source := range []string{
		`class Empty<> {}`,
		`class Missing<T,> {}`,
		`class Box<T {}`,
		`class Box<T> {} function use(): void { new Box<int(1); }`,
	} {
		if _, diagnostics := parseSource(t, source); diagnostics == 0 {
			t.Fatalf("source unexpectedly parsed without diagnostics: %s", source)
		}
	}
}
