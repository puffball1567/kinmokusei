package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestInterfacePropertiesParse(t *testing.T) {
	program, count := parseSource(t, "interface Cell<T>{\nget value():T\nset value(v:T)\nfunction get():int\nfunction set():void\n}")
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	decl := program.Declarations[0].(*ast.InterfaceDecl)
	if len(decl.Methods) != 4 || decl.Methods[0].Accessor != "get" || decl.Methods[1].Accessor != "set" || decl.Methods[1].ReturnType.Name != "void" || decl.Methods[2].Accessor != "" || decl.Methods[3].Accessor != "" {
		t.Fatalf("interface=%#v", decl)
	}
	for _, input := range []string{`interface I{get x(){}}`, `interface I{get x():int{return 1;}}`, `interface I{set x(v:int){}}`, `interface I{get x<T>():T;}`, `interface I{private get x():int;}`, `interface I{static get x():int;}`, `interface I{get x():int set x(v:int);}`, `interface I{set x(v:);}`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("accepted %q", input)
		}
	}
}
