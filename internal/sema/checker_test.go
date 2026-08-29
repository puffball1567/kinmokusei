package sema

import (
	"go/importer"
	"strings"
	"testing"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/lexer"
	"ontama.local/ontama/internal/parser"
	"ontama.local/ontama/internal/source"
)

func TestResolvedDeclarationIdentityMatrix(t *testing.T) {
	input := `function helper(value: int): int { return value; }
function compute(input: int): int {
  let local: int = 1;
  local += input;
  return helper(local);
}`
	tokens, lexDiagnostics := lexer.Lex("identity.otm", input)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics: %v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics: %v", parseDiagnostics)
	}
	if diagnostics := Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics: %v", diagnostics)
	}
	helper := program.Declarations[0].(*ast.FunctionDecl)
	compute := program.Declarations[1].(*ast.FunctionDecl)
	local := compute.Body.Statements[0].(*ast.VariableDecl)
	update := compute.Body.Statements[1].(*ast.AssignmentStmt)
	returned := compute.Body.Statements[2].(*ast.ReturnStmt).Value.(*ast.CallExpr)
	if returned.Signature == nil || len(returned.Signature.ParameterTypes) != 1 || returned.Signature.ParameterTypes[0] != "int" || returned.Signature.Result != "int" {
		t.Fatalf("resolved call signature = %#v", returned.Signature)
	}
	tests := []struct {
		name       string
		identifier *ast.IdentifierExpr
		wantStart  int
	}{
		{"compound target", update.Target.(*ast.IdentifierExpr), local.NameSpan.Start.Offset},
		{"parameter use", update.Value.(*ast.IdentifierExpr), compute.Parameters[0].Span.Start.Offset},
		{"function call", returned.Callee.(*ast.IdentifierExpr), helper.NameSpan.Start.Offset},
		{"call argument", returned.Arguments[0].(*ast.IdentifierExpr), local.NameSpan.Start.Offset},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.identifier.ResolvedDeclaration.Start.Offset; got != test.wantStart || test.identifier.ResolvedDeclaration.Path != "identity.otm" {
				t.Fatalf("resolved declaration = %#v, want offset %d", test.identifier.ResolvedDeclaration, test.wantStart)
			}
		})
	}
}

func TestResolvedTypeAndMemberDeclarationIdentityMatrix(t *testing.T) {
	input := `interface Reader { function read(): int; }
class Box implements Reader {
  public value: int;
  constructor(value: int) { this.value = value; }
  public function read(): int { return this.value; }
}
function use(box: Box, reader: Reader): int {
  const copy: Box = new Box(box.value);
  return copy.read() + reader.read();
}`
	tokens, lexDiagnostics := lexer.Lex("type_identity.otm", input)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics: %v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics: %v", parseDiagnostics)
	}
	if diagnostics := Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics: %v", diagnostics)
	}
	contract := program.Declarations[0].(*ast.InterfaceDecl)
	class := program.Declarations[1].(*ast.ClassDecl)
	use := program.Declarations[2].(*ast.FunctionDecl)
	copy := use.Body.Statements[0].(*ast.VariableDecl)
	construction := copy.Value.(*ast.NewExpr)
	result := use.Body.Statements[1].(*ast.ReturnStmt).Value.(*ast.BinaryExpr)
	classCall := result.Left.(*ast.CallExpr).Callee.(*ast.MemberExpr)
	interfaceCall := result.Right.(*ast.CallExpr).Callee.(*ast.MemberExpr)
	constructorAssignment := class.Constructor.Body.Statements[0].(*ast.AssignmentStmt).Target.(*ast.MemberExpr)

	tests := []struct {
		name string
		got  source.Span
		want source.Span
	}{
		{"implements type", class.Implements[0].ResolvedDeclaration, contract.NameSpan},
		{"class parameter type", use.Parameters[0].Type.ResolvedDeclaration, class.NameSpan},
		{"interface parameter type", use.Parameters[1].Type.ResolvedDeclaration, contract.NameSpan},
		{"local class type", copy.Type.ResolvedDeclaration, class.NameSpan},
		{"construction", construction.ResolvedDeclaration, class.NameSpan},
		{"field selection", constructorAssignment.ResolvedDeclaration, class.Fields[0].NameSpan},
		{"class method selection", classCall.ResolvedDeclaration, class.Methods[0].NameSpan},
		{"interface method selection", interfaceCall.ResolvedDeclaration, contract.Methods[0].NameSpan},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got.Path != test.want.Path || test.got.Start.Offset != test.want.Start.Offset || test.got.End.Offset != test.want.End.Offset {
				t.Fatalf("resolved declaration = %#v, want %#v", test.got, test.want)
			}
		})
	}
}

func TestResolvedBindingTypeMetadataForGoValues(t *testing.T) {
	input := `import go web from "net/http";
function use(): string {
  const client = web.DefaultClient;
  client.CloseIdleConnections();
  const [request, err] = web.NewRequest(web.MethodGet, "https://example.com", nil);
  if (err != nil) { return ""; }
  return request.Method;
}`
	tokens, lexDiagnostics := lexer.Lex("go_binding_types.otm", input)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics: %v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics: %v", parseDiagnostics)
	}
	if diagnostics := Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics: %v", diagnostics)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	client := function.Body.Statements[0].(*ast.VariableDecl)
	request := function.Body.Statements[2].(*ast.MultiVariableDecl).Bindings[0]
	for name, ref := range map[string]ast.TypeRef{"client": client.ResolvedType, "request": request.ResolvedType} {
		if !ref.IsPointer() || ref.Pointee == nil || ref.Pointee.Name == "" || ref.Pointee.Qualifier != "web" || !ref.Pointee.Go {
			t.Fatalf("%s resolved type = %#v", name, ref)
		}
	}
	if client.Type.IsSpecified() {
		t.Fatalf("inferred source type was rewritten: %#v", client.Type)
	}
}

func checkSource(t *testing.T, input string) []string {
	return checkSourceWithPolicy(t, input, GoInteropPolicy{})
}

func checkSourceWithPolicy(t *testing.T, input string, policy GoInteropPolicy) []string {
	t.Helper()
	tokens, lexDiagnostics := lexer.Lex("test.otm", input)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics: %v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics: %v", parseDiagnostics)
	}
	diagnostics := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), policy)
	messages := make([]string, len(diagnostics))
	for i, item := range diagnostics {
		messages[i] = item.Message
	}
	return messages
}

func TestNumberAndFloatAreAliases(t *testing.T) {
	diagnostics := checkSource(t, `
function identity(value: float): number { return value; }
function main(): void { const result: float64 = identity(1.5); }
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsImplicitIntFloatConversion(t *testing.T) {
	diagnostics := checkSource(t, `function bad(value: int): float { return value; }`)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "cannot use int as float") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestReportsUndefinedNamesAndMissingReturn(t *testing.T) {
	diagnostics := checkSource(t, `
function bad(flag: boolean): int {
  if (flag) { return missing; }
}
`)
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics: %v", len(diagnostics), diagnostics)
	}
	joined := strings.Join(diagnostics, "\n")
	if !strings.Contains(joined, "undefined name") || !strings.Contains(joined, "may complete") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestIntegerLiteralUsesTargetNumericType(t *testing.T) {
	diagnostics := checkSource(t, `function value(): float { return 1; }`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestChecksInferredAndContextualArrowFunctions(t *testing.T) {
	diagnostics := checkSource(t, `
function apply(value: int, operation: (value: int) => int): int {
  return operation(value);
}
function compute(): int {
  const double = (value: int) => value * 2;
  return apply(21, double);
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestBlockArrowRequiresReturnType(t *testing.T) {
	diagnostics := checkSource(t, `
function compute(): int {
  const invalid = (): int => { return 42; };
  const missing = () => { return 42; };
  return invalid();
}
`)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "block body require") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestChecksMutableAssignmentsAndLoops(t *testing.T) {
	diagnostics := checkSource(t, `
function sum(limit: int): int {
  let total = 0;
  for (let index: int = 0; index < limit; index = index + 1) {
    total = total + index;
  }
  while (total < 10) { total = total + 1; }
  return total;
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsConstAssignmentAndBranchOutsideLoop(t *testing.T) {
	diagnostics := checkSource(t, `
function invalid(): int {
  const value = 1;
  value = 2;
  break;
  return value;
}
`)
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	joined := strings.Join(diagnostics, "\n")
	if !strings.Contains(joined, "cannot assign to const") || !strings.Contains(joined, "inside a loop") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestChecksCollectionsObjectsAndClasses(t *testing.T) {
	diagnostics := checkSource(t, `
class Counter {
  constructor(private value: int) {}
  public function increment(): void { this.value = this.value + 1; }
  public function current(): int { return this.value; }
}
function compute(values: int[], lookup: Map<string, int>): int {
  const counter = new Counter(values[0]);
  counter.increment();
  const dto = { count: counter.current(), label: "ok" };
  return dto.count + lookup["extra"];
}
function empty(): int[] { return []; }
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsPrivateClassAccess(t *testing.T) {
	diagnostics := checkSource(t, `
class Secret { constructor(private value: int) {} }
function reveal(secret: Secret): int { return secret.value; }
`)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "private") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestRejectsNonComparableMapKey(t *testing.T) {
	diagnostics := checkSource(t, `function invalid(values: Map<int[], string>): void {}`)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "Map key") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestChecksInterfacePolymorphismAndComposition(t *testing.T) {
	diagnostics := checkSource(t, `
interface Formatter { function format(value: string): string; }
interface Named { function name(): string; }
interface Empty {}
class PrefixFormatter implements Formatter, Named, Empty {
  constructor(private prefix: string) {}
  public function format(value: string): string { return this.prefix + value; }
  public function name(): string { return "prefix"; }
}
class Renderer {
  constructor(private formatter: Formatter) {}
  public function render(value: string): string { return this.formatter.format(value); }
}
function makeFormatter(): Formatter { return new PrefixFormatter("x:"); }
function render(formatter: Formatter, value: string): string { return formatter.format(value); }
function example(): string {
  const formatter: Formatter = new PrefixFormatter("x:");
  const renderer = new Renderer(formatter);
  return renderer.render(formatter.format("ok"));
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestInterfaceFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"unknown interface", `class Value implements Missing {}`, `unknown interface "Missing"`},
		{"duplicate interface", `interface A {} class Value implements A, A {}`, `duplicate implemented interface "A"`},
		{"missing method", `interface A { function value(): int; } class Value implements A {}`, "missing method value"},
		{"private method", `interface A { function value(): int; } class Value implements A { private function value(): int { return 1; } }`, "must be public"},
		{"static method", `interface A { function value(): int; } class Value implements A { public static function value(): int { return 1; } }`, "cannot be static"},
		{"parameter mismatch", `interface A { function value(input: int): int; } class Value implements A { public function value(input: string): int { return 1; } }`, "incompatible signature"},
		{"return mismatch", `interface A { function value(): int; } class Value implements A { public function value(): string { return "x"; } }`, "incompatible signature"},
		{"implicit implementation rejected", `interface A { function value(): int; } class Value { public function value(): int { return 1; } } function use(value: A): int { return value.value(); } function bad(): int { return use(new Value()); }`, "cannot use Value as A"},
		{"unknown member", `interface A {} function bad(value: A): int { return value.missing(); }`, "has no method"},
		{"new interface", `interface A {} function bad(): A { return new A(); }`, `unknown class "A"`},
		{"duplicate method", `interface A { function value(): int; function value(): int; }`, "duplicate interface method"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if len(diagnostics) == 0 || !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want substring %q", diagnostics, test.want)
			}
		})
	}
}
