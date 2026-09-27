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

func TestAbstractPropertiesParse(t *testing.T) {
	program, count := parseSource(t, "abstract class B<T>{\npublic abstract get value():T\nprotected abstract set value(v:T)\n}\nclass C extends B<int>{public final override get value():int{return 1;}protected override set value(v:int){}}")
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	base := program.Declarations[0].(*ast.ClassDecl)
	if len(base.Methods) != 2 || !base.Methods[0].Abstract || !base.Methods[1].Abstract || base.Methods[0].Body == nil || base.Methods[1].ReturnType.Name != "void" {
		t.Fatalf("base=%#v", base)
	}
	for _, input := range []string{`abstract class B{public abstract get x():int{return 1;}}`, `abstract class B{public abstract set x(v:int){}}`, `class B{public get x():int;}`, `abstract class B{public abstract get x():int public abstract set x(v:int);}`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestStaticPropertiesParse(t *testing.T) {
	program, count := parseSource(t, "class C<T>{\npublic static get value():int{return 1;}\nprotected static set value(v:int):void{}\n}")
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	decl := program.Declarations[0].(*ast.ClassDecl)
	if len(decl.Methods) != 2 || !decl.Methods[0].Static || !decl.Methods[1].Static || decl.Methods[0].Accessor != "get" || decl.Methods[1].Accessor != "set" || decl.Methods[1].Visibility != ast.Protected {
		t.Fatalf("class=%#v", decl)
	}
}
