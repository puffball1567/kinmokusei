package parser

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"testing"
)

func TestAbstractClassSyntax(t *testing.T) {
	t.Parallel()
	program, count := parseSource(t, "export abstract class Base<T>{\npublic abstract function get():T\nprotected abstract function set(v:T):void;\n}")
	if count != 0 || len(program.Declarations) != 1 {
		t.Fatalf("program=%#v diagnostics=%d", program, count)
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	if !class.Abstract || len(class.Methods) != 2 || !class.Methods[0].Abstract || class.Methods[0].Body == nil {
		t.Fatalf("class=%#v", class)
	}
	for _, source := range []string{
		`abstract abstract class Base{}`,
		`abstract function f():void{}`,
		`abstract class Base{public abstract abstract function get():int;}`,
		`abstract class Base{public abstract function get():int{return 1;}}`,
		`abstract class Base{abstract constructor(){}}`,
		`abstract class Base{public abstract value:int;}`,
		`abstract class Base{public abstract function ():void;}`,
		`abstract class Base{public abstract function get():int public function f():void{}}`,
	} {
		if _, count := parseSource(t, source); count == 0 {
			t.Errorf("accepted %s", source)
		}
	}
}
