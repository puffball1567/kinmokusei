package parser

import (
	"testing"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/lexer"
)

func TestParsesNativeStructDeclarationsAndLiterals(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
struct Point {
  public x: int;
  private label: string;
  public function sum(extra: int): int { return this.x + extra; }
  public pointer function move(delta: int): void { this.x += delta; }
}
function make(): Point {
  return Point { x: 3, label: "hot", };
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	declaration, ok := program.Declarations[0].(*ast.StructDecl)
	if !ok || declaration.Name != "Point" || len(declaration.Fields) != 2 || len(declaration.Methods) != 2 {
		t.Fatalf("struct declaration = %#v", program.Declarations[0])
	}
	if declaration.Fields[0].Visibility != ast.Public || declaration.Fields[1].Visibility != ast.Private {
		t.Fatalf("field visibility = %#v", declaration.Fields)
	}
	if declaration.Methods[0].PointerReceiver || !declaration.Methods[1].PointerReceiver || declaration.Methods[1].Name != "move" {
		t.Fatalf("struct methods = %#v", declaration.Methods)
	}
	returned := program.Declarations[1].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt)
	literal, ok := returned.Value.(*ast.GoCompositeLiteralExpr)
	if !ok || literal.Type.Name != "Point" || literal.Type.Go || len(literal.Fields) != 2 {
		t.Fatalf("struct literal = %#v", returned.Value)
	}
}

func TestNativeStructLiteralDoesNotConsumeSelectOrSwitchBodies(t *testing.T) {
	_, diagnosticCount := parseSource(t, `
struct Signal { public value: int; }
function choose(input: GoReceiveChannel<int>, output: GoSendChannel<Signal>, signal: Signal): int {
  select {
    case const value = <-input { return value; }
    case output <- (Signal { value: 1 }) {}
    default {}
  }
  switch (signal) {
    case (Signal { value: 1 }) { return 1; }
    default { return 0; }
  }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
}

func TestParsesExternalNativeStructReceiverMethods(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
struct Point { public x: int; }
public function copied(this: Point, delta: int): Point { this.x += delta; return this; }
private function move(this: *Point, delta: int): void { this.x += delta; }
function read(this: Point): int { return this.x; }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	if len(program.Declarations) != 4 {
		t.Fatalf("declarations = %#v", program.Declarations)
	}
	value := program.Declarations[1].(*ast.MethodDecl)
	pointer := program.Declarations[2].(*ast.MethodDecl)
	private := program.Declarations[3].(*ast.MethodDecl)
	if !value.External || value.ReceiverName != "this" || value.ReceiverType.Name != "Point" || value.Visibility != ast.Public {
		t.Fatalf("value receiver = %#v", value)
	}
	if len(value.Parameters) != 1 || value.Parameters[0].Name != "delta" {
		t.Fatalf("receiver must not remain in callable parameters: %#v", value.Parameters)
	}
	if !pointer.ReceiverType.IsPointer() || pointer.ReceiverName != "this" || pointer.Visibility != ast.Private {
		t.Fatalf("pointer receiver = %#v", pointer)
	}
	if private.Visibility != ast.Private || private.Name != "read" {
		t.Fatalf("default visibility = %#v", private)
	}
}

func TestNativeStructSyntaxFailureMatrix(t *testing.T) {
	tests := []string{
		`struct { public value: int; } function recovered(): void {}`,
		`struct Broken function recovered(): void {}`,
		`struct Broken { public value int; } function recovered(): void {}`,
		`struct Broken { constructor() {} } function recovered(): void {}`,
		`struct Broken { public pointer value: int; } function recovered(): void {}`,
		`struct Broken { public pointer function value(): int; } function recovered(): void {}`,
		`struct Point { public x: int; } public function copied(): Point { return Point { x: 1 }; }`,
		`struct Point { public x: int; } public function copied(point: Point): Point { return point; }`,
		`struct Point { public x: int; } public function copied(this Point): Point { return this; }`,
		`struct Point { public x: int; } function copied(value: Point, this: Point): Point { return this; }`,
		`struct Point { public x: int; } function (this: Point) copied(): Point { return this; }`,
	}
	for _, source := range tests {
		tokens, lexDiagnostics := lexer.Lex("struct_failure.otm", source)
		if len(lexDiagnostics) != 0 {
			t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
		}
		_, diagnostics := Parse(tokens)
		if len(diagnostics) == 0 {
			t.Errorf("expected parser diagnostics for %q", source)
		}
	}
}
