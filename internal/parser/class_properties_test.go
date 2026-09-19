package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"testing"
)

func TestClassPropertiesParse(t *testing.T) {
	program, count := parseSource(t, `class Box<T>{private raw:T;public get value():T{return this.raw;}protected set value(v:T){this.raw=v;}public get:int;public set:int;}`)
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	if len(class.Methods) != 2 || len(class.Fields) != 3 || class.Methods[0].Accessor != "get" || class.Methods[1].Accessor != "set" || class.Methods[1].ReturnType.Name != "void" || class.Methods[1].Visibility != ast.Protected {
		t.Fatalf("class=%#v", class)
	}
	for _, input := range []string{`class C{get x(){return 1;}}`, `class C{get x(:int{}`, `class C{set x(v:){}}`, `class C{get x():int;}`, `class C{set x(v:int)`, `class C{get x<T>():T{}}`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("accepted %q", input)
		}
	}
}
