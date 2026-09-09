package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"testing"
)

func TestParsesInterfaceInheritance(t *testing.T) {
	program, count := parseSource(t, `interface Combined<T> extends Reader<T>, Writer<T[]> { function close(): void; }`)
	if count != 0 {
		t.Fatalf("diagnostics = %d", count)
	}
	decl := program.Declarations[0].(*ast.InterfaceDecl)
	if len(decl.Bases) != 2 || decl.Bases[0].Name != "Reader" || decl.Bases[1].GenericArguments[0].Element.Name != "T" || len(decl.Methods) != 1 {
		t.Fatalf("interface = %#v", decl)
	}
	for _, input := range []string{`interface Bad extends {}`, `interface Bad extends Reader, {}`, `interface Bad extends Reader<int {}`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("accepted %q", input)
		}
	}
}
