package parser

import (
	"testing"

	"ontama.local/ontama/internal/ast"
)

func TestParsesSingleInheritanceModifiersAndSuper(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Base { public virtual function value(): int { return 1; } }
class Derived extends Base implements Reader {
  constructor() { super(); }
  public override function value(): int { return super.value(); }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	base := program.Declarations[0].(*ast.ClassDecl)
	derived := program.Declarations[1].(*ast.ClassDecl)
	if !base.Methods[0].Virtual || base.Methods[0].Override {
		t.Fatalf("base method modifiers = virtual:%v override:%v", base.Methods[0].Virtual, base.Methods[0].Override)
	}
	if derived.Base == nil || derived.Base.Name != "Base" || len(derived.Implements) != 1 || derived.Implements[0].Name != "Reader" {
		t.Fatalf("derived header = %#v", derived)
	}
	if !derived.Methods[0].Override || derived.Methods[0].Virtual {
		t.Fatalf("derived method modifiers = virtual:%v override:%v", derived.Methods[0].Virtual, derived.Methods[0].Override)
	}
	constructorCall := derived.Constructor.Body.Statements[0].(*ast.ExpressionStmt).Value.(*ast.CallExpr)
	if got := constructorCall.Callee.(*ast.IdentifierExpr).Name; got != "super" {
		t.Fatalf("constructor callee = %q", got)
	}
	methodCall := derived.Methods[0].Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.CallExpr)
	if got := methodCall.Callee.(*ast.MemberExpr).Object.(*ast.IdentifierExpr).Name; got != "super" {
		t.Fatalf("method receiver = %q", got)
	}
}

func TestParsesProtectedClassMembersAndConstructorFields(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Base {
  constructor(protected value: int) {}
  protected field: string;
  protected virtual function read(): int { return this.value; }
}
class Child extends Base {
  protected override function read(): int { return super.read(); }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	base := program.Declarations[0].(*ast.ClassDecl)
	child := program.Declarations[1].(*ast.ClassDecl)
	if parameter := base.Constructor.Parameters[0]; !parameter.IsField || parameter.Visibility != ast.Protected {
		t.Fatalf("constructor parameter = %#v", parameter)
	}
	if base.Fields[0].Visibility != ast.Protected || base.Methods[0].Visibility != ast.Protected || !base.Methods[0].Virtual {
		t.Fatalf("protected base members = %#v / %#v", base.Fields[0], base.Methods[0])
	}
	if child.Methods[0].Visibility != ast.Protected || !child.Methods[0].Override {
		t.Fatalf("protected override = %#v", child.Methods[0])
	}
}

func TestParsesFinalClassAndFinalOverrideInEitherModifierOrder(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Base {
  protected virtual function first(): int { return 1; }
  public virtual function second(): int { return 2; }
}
final class Child extends Base {
  protected final override function first(): int { return 3; }
  public override final function second(): int { return 4; }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	child := program.Declarations[1].(*ast.ClassDecl)
	if !child.Final {
		t.Fatal("final class modifier was not retained")
	}
	for index, method := range child.Methods {
		if !method.Override || !method.Final {
			t.Fatalf("method %d modifiers = override:%v final:%v", index, method.Override, method.Final)
		}
	}
}
