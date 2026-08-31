package parser

import (
	"reflect"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func parseSource(t *testing.T, input string) (*ast.Program, int) {
	t.Helper()
	tokens, lexerDiagnostics := lexer.Lex("test.otm", input)
	if len(lexerDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics: %v", lexerDiagnostics)
	}
	program, diagnostics := Parse(tokens)
	return program, len(diagnostics)
}

func TestParsesNativeRestParameters(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function collect<T>(prefix: string, ...values: T[]): int { return len(values); }
interface Joiner { function join(...parts: string[]): string; }
function arrow(): (...values: int[]) => int { return (...values: int[]): int => len(values); }
`)
	if diagnosticCount != 0 || program == nil || len(program.Declarations) != 3 {
		t.Fatalf("program = %#v, diagnostics = %d", program, diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	if len(function.Parameters) != 2 || !function.Parameters[1].Variadic || !function.Parameters[1].Type.IsSlice() {
		t.Fatalf("function parameters = %#v", function.Parameters)
	}
	contract := program.Declarations[1].(*ast.InterfaceDecl)
	if len(contract.Methods) != 1 || len(contract.Methods[0].Parameters) != 1 || !contract.Methods[0].Parameters[0].Variadic {
		t.Fatalf("interface methods = %#v", contract.Methods)
	}
	arrowFunction := program.Declarations[2].(*ast.FunctionDecl)
	if !arrowFunction.ReturnType.Variadic || len(arrowFunction.ReturnType.Parameters) != 1 || !arrowFunction.ReturnType.Parameters[0].IsSlice() {
		t.Fatalf("function type = %#v", arrowFunction.ReturnType)
	}
	statement := arrowFunction.Body.Statements[0].(*ast.ReturnStmt)
	arrow := statement.Value.(*ast.ArrowExpr)
	if len(arrow.Parameters) != 1 || !arrow.Parameters[0].Variadic {
		t.Fatalf("arrow parameters = %#v", arrow.Parameters)
	}
}

func TestParsesFunctionControlFlowAndCalls(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function max(left: int, right: int): int {
  if (left > right) {
    return left;
  } else {
    return right;
  }
}
function main(): void {
  const result: int = max(20, 22);
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	if len(program.Declarations) != 2 {
		t.Fatalf("declarations = %d", len(program.Declarations))
	}
	max := program.Declarations[0].(*ast.FunctionDecl)
	if max.Name != "max" || len(max.Parameters) != 2 {
		t.Fatalf("unexpected function: %#v", max)
	}
	if _, ok := max.Body.Statements[0].(*ast.IfStmt); !ok {
		t.Fatalf("first statement = %T", max.Body.Statements[0])
	}
}

func TestRecoversAtNextDeclaration(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function broken(value int): int { return value; }
function valid(): int { return 42; }
`)
	if diagnosticCount == 0 {
		t.Fatal("expected parser diagnostic")
	}
	if len(program.Declarations) != 1 {
		var names []string
		for _, decl := range program.Declarations {
			if fn, ok := decl.(*ast.FunctionDecl); ok {
				names = append(names, fn.Name)
			}
		}
		t.Fatalf("declarations = %d (%v), want 1", len(program.Declarations), names)
	}
	if got := program.Declarations[0].(*ast.FunctionDecl).Name; got != "valid" {
		t.Fatalf("recovered function = %q", got)
	}
}

func TestParsesArrowFunctionsAndFunctionTypes(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function apply(value: int, operation: (value: int) => int): int {
  return operation(value);
}
function compute(): int {
  const double = (value: int) => value * 2;
  const increment = (value: int): int => { return value + 1; };
  return increment(apply(21, double));
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	apply := program.Declarations[0].(*ast.FunctionDecl)
	if !apply.Parameters[1].Type.IsFunction() {
		t.Fatal("operation parameter is not a function type")
	}
	compute := program.Declarations[1].(*ast.FunctionDecl)
	variable := compute.Body.Statements[0].(*ast.VariableDecl)
	if _, ok := variable.Value.(*ast.ArrowExpr); !ok {
		t.Fatalf("initializer = %T", variable.Value)
	}
}

func TestParsesVariadicSliceExpansionOnCall(t *testing.T) {
	program, diagnosticCount := parseSource(t, `function join(parts: string[]): string { return path.Join(parts...); }`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	returned := function.Body.Statements[0].(*ast.ReturnStmt)
	call, ok := returned.Value.(*ast.CallExpr)
	if !ok || !call.Expanded || len(call.Arguments) != 1 {
		t.Fatalf("return value = %#v", returned.Value)
	}
}

func TestParsesAssignmentsAndLoops(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function sum(limit: int): int {
  let total = 0;
  for (let index: int = 0; index < limit; index = index + 1) {
    if (index === 2) { continue; }
    total = total + index;
  }
  while (total < 10) {
    total = total + 1;
    break;
  }
  return total;
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	if _, ok := function.Body.Statements[1].(*ast.ForStmt); !ok {
		t.Fatalf("statement = %T", function.Body.Statements[1])
	}
	if _, ok := function.Body.Statements[2].(*ast.WhileStmt); !ok {
		t.Fatalf("statement = %T", function.Body.Statements[2])
	}
}

func TestParsesCollectionRangeBindingShapes(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function ranges(items: int[]): void {
  for (const value: int of items) {}
  for (let [index: int, value: int] of items) {}
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	single, ok := function.Body.Statements[0].(*ast.ForRangeStmt)
	if !ok || !single.Constant || len(single.Bindings) != 1 || single.Bindings[0].Name != "value" || single.Bindings[0].Type.Name != "int" || single.Source == nil {
		t.Fatalf("single range = %#v", function.Body.Statements[0])
	}
	pair, ok := function.Body.Statements[1].(*ast.ForRangeStmt)
	if !ok || pair.Constant || len(pair.Bindings) != 2 || pair.Bindings[0].Name != "index" || pair.Bindings[1].Name != "value" || pair.Bindings[1].Type.Name != "int" {
		t.Fatalf("pair range = %#v", function.Body.Statements[1])
	}
}

func TestParsesSelectCommunicationShapes(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function choose(input: GoReceiveChannel<int>, output: GoSendChannel<int>): void {
  let value = 0;
  let open = false;
  select {
    case const received = <-input {}
    case let [received, ok] = <-input {}
    case value = <-input {}
    case [value, open] = <-input {}
    case output <- value {}
    case <-input {}
    default {}
  }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	selected, ok := function.Body.Statements[2].(*ast.SelectStmt)
	if !ok || len(selected.Cases) != 7 {
		t.Fatalf("select = %#v", function.Body.Statements[2])
	}
	if first := selected.Cases[0]; first.Kind != ast.SelectReceive || !first.Declare || !first.Constant || len(first.Bindings) != 1 || first.Bindings[0].Name != "received" {
		t.Fatalf("single declaration = %#v", first)
	}
	if checked := selected.Cases[1]; checked.Kind != ast.SelectReceive || !checked.Declare || checked.Constant || len(checked.Bindings) != 2 {
		t.Fatalf("checked declaration = %#v", checked)
	}
	if assignment := selected.Cases[2]; assignment.Declare || len(assignment.Targets) != 1 {
		t.Fatalf("single assignment = %#v", assignment)
	}
	if checked := selected.Cases[3]; checked.Declare || len(checked.Targets) != 2 {
		t.Fatalf("checked assignment = %#v", checked)
	}
	if sent := selected.Cases[4]; sent.Kind != ast.SelectSend || sent.Channel == nil || sent.Value == nil {
		t.Fatalf("send = %#v", sent)
	}
	if discarded := selected.Cases[5]; discarded.Kind != ast.SelectReceive || len(discarded.Bindings) != 0 || len(discarded.Targets) != 0 {
		t.Fatalf("discarded receive = %#v", discarded)
	}
	if fallback := selected.Cases[6]; fallback.Kind != ast.SelectDefault || fallback.Channel != nil {
		t.Fatalf("default = %#v", fallback)
	}
}

func TestParsesTypeSwitchCaseShapes(t *testing.T) {
	program, diagnosticCount := parseSource(t, `import go io from "io"; import go strings from "strings"; function classify(value: io.Reader): int { switch (value) { case const reader as *strings.Reader { return reader.Len(); } case let writer as io.Writer { return 2; } case const _ as io.Reader { return 3; } case nil { return 4; } default { return 5; } } }`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	switched, ok := function.Body.Statements[0].(*ast.TypeSwitchStmt)
	if !ok || len(switched.Cases) != 5 {
		t.Fatalf("type switch = %#v", function.Body.Statements[0])
	}
	if first := switched.Cases[0]; !first.Constant || first.Name != "reader" || !first.Type.IsPointer() || first.Nil || first.Default {
		t.Fatalf("first case = %#v", first)
	}
	if second := switched.Cases[1]; second.Constant || second.Name != "writer" || second.Type.Qualifier != "io" {
		t.Fatalf("second case = %#v", second)
	}
	if blank := switched.Cases[2]; blank.Name != "_" || blank.Type.Name != "Reader" {
		t.Fatalf("blank case = %#v", blank)
	}
	if !switched.Cases[3].Nil || !switched.Cases[4].Default {
		t.Fatalf("nil/default cases = %#v", switched.Cases[3:])
	}
}

func TestParsesValueSwitchCaseShapes(t *testing.T) {
	program, diagnosticCount := parseSource(t, `function classify(value: int): int { switch (value) { case 0 { return 1; } case 1, 2 + 1 { fallthrough; } default { return 4; } } return 0; }`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	switched, ok := function.Body.Statements[0].(*ast.ValueSwitchStmt)
	if !ok || len(switched.Cases) != 3 {
		t.Fatalf("value switch = %#v", function.Body.Statements[0])
	}
	if first := switched.Cases[0]; first.Default || len(first.Values) != 1 {
		t.Fatalf("first case = %#v", first)
	}
	if second := switched.Cases[1]; second.Default || len(second.Values) != 2 {
		t.Fatalf("multi-value case = %#v", second)
	}
	branch, ok := switched.Cases[1].Body.Statements[0].(*ast.BranchStmt)
	if !ok || branch.Kind != ast.FallthroughBranch || branch.Label != "" {
		t.Fatalf("fallthrough branch = %#v", switched.Cases[1].Body.Statements[0])
	}
	if fallback := switched.Cases[2]; !fallback.Default || len(fallback.Values) != 0 {
		t.Fatalf("default case = %#v", fallback)
	}
}

func TestFallthroughParserFailureMatrix(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"missing semicolon", `function bad(value: int): void { switch (value) { case 0 { fallthrough } default {} } }`},
		{"cannot name target", `function bad(value: int): void { switch (value) { case 0 { fallthrough next; } default {} } }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, diagnostics := parseSource(t, test.source); diagnostics == 0 {
				t.Fatal("expected parser diagnostic")
			}
		})
	}
}

func TestParsesRelativeImports(t *testing.T) {
	tokens, lexerDiagnostics := lexer.Lex("main.otm", `import { add, meaning } from "./math"; function main(): void {}`)
	if len(lexerDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics: %v", lexerDiagnostics)
	}
	program, diagnostics := Parse(tokens)
	if len(diagnostics) != 0 {
		t.Fatalf("parser diagnostics: %v", diagnostics)
	}
	if len(program.Imports) != 1 {
		t.Fatalf("imports = %d", len(program.Imports))
	}
	imported := program.Imports[0]
	if imported.Path != "./math" || len(imported.Names) != 2 || len(imported.NameSpans) != 2 {
		t.Fatalf("import = %#v", imported)
	}
	if imported.NameSpans[0].Start.Offset != 9 || imported.NameSpans[0].End.Offset != 12 || imported.NameSpans[1].Start.Offset != 14 || imported.NameSpans[1].End.Offset != 21 {
		t.Fatalf("import name spans = %#v", imported.NameSpans)
	}
}

func TestParsesCollectionsObjectsAndClasses(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Counter {
  constructor(private value: int) {}
  public function increment(): void { this.value = this.value + 1; }
  public function current(): int { return this.value; }
}
function compute(values: int[], lookup: Map<string, int>): int {
  const counter = new Counter(values[0]);
  const dto = { count: counter.current(), label: "ok" };
  return dto.count + lookup["extra"];
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	class, ok := program.Declarations[0].(*ast.ClassDecl)
	if !ok || class.Constructor == nil || len(class.Methods) != 2 {
		t.Fatalf("class = %#v", program.Declarations[0])
	}
	function := program.Declarations[1].(*ast.FunctionDecl)
	if !function.Parameters[0].Type.IsArray() {
		t.Fatal("first parameter is not an array")
	}
	if len(function.Parameters[1].Type.GenericArguments) != 2 {
		t.Fatal("Map arguments were not parsed")
	}
}

func TestObjectTypeSyntaxMatrix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"empty", `function value(input: {}): {} { return input; }`, true},
		{"fields", `function value(input: { message: string, count: int }): { count: int, message: string } { return input; }`, true},
		{"nested and suffix", `function value(input: { child: { values: int[] }, }[]): void {}`, true},
		{"missing name", `function value(input: { : string }): void {}`, false},
		{"missing colon", `function value(input: { message string }): void {}`, false},
		{"missing field type", `function value(input: { message: }): void {}`, false},
		{"missing close", `function value(input: { message: string): void {}`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program, diagnostics := parseSource(t, test.input)
			if test.valid && diagnostics != 0 {
				t.Fatalf("diagnostics=%d", diagnostics)
			}
			if !test.valid && diagnostics == 0 {
				t.Fatal("expected diagnostics")
			}
			if test.valid && len(program.Declarations) == 0 {
				t.Fatal("missing declaration")
			}
		})
	}
}

func TestParsesCABIExportMetadata(t *testing.T) {
	program, diagnostics := parseSource(t, `export c("ontama_add") function add(left: int32, right: int32): int32 { return left + right; }`)
	if diagnostics != 0 || len(program.Declarations) != 1 {
		t.Fatalf("diagnostics=%d declarations=%d", diagnostics, len(program.Declarations))
	}
	function, ok := program.Declarations[0].(*ast.FunctionDecl)
	if !ok || !function.CABIExport || function.CABISymbol != "ontama_add" || len(function.Parameters) != 2 || function.CABIExportSpan.Start.Column != 1 || function.CABISymbolSpan.Start.Column == 0 {
		t.Fatalf("function=%#v", program.Declarations[0])
	}
}

func TestParsesCABIExportListMetadata(t *testing.T) {
	program, diagnostics := parseSource(t, `
function add(left: int32, right: int32): int32 { return left + right; }
const sub = (left: int32, right: int32): int32 => left - right;
export c(
  "ontama_add",
  "ontama_sub",
) {
  add,
  sub,
};
`)
	if diagnostics != 0 || len(program.Declarations) != 3 {
		t.Fatalf("diagnostics=%d declarations=%d", diagnostics, len(program.Declarations))
	}
	exports, ok := program.Declarations[2].(*ast.CABIExportDecl)
	if !ok {
		t.Fatalf("declaration=%T", program.Declarations[2])
	}
	if got, want := exports.Symbols, []string{"ontama_add", "ontama_sub"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("symbols=%v want=%v", got, want)
	}
	if got, want := exports.Names, []string{"add", "sub"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names=%v want=%v", got, want)
	}
	if len(exports.SymbolSpans) != 2 || len(exports.NameSpans) != 2 || exports.Span.Start.Line != 4 || exports.Span.End.Line != 10 {
		t.Fatalf("export spans=%#v symbol spans=%#v name spans=%#v", exports.Span, exports.SymbolSpans, exports.NameSpans)
	}
}

func TestCABIExportListParserFailureMatrix(t *testing.T) {
	for _, test := range []struct {
		name, source string
	}{
		{"empty symbols", `export c() { add };`},
		{"empty names", `export c("ontama_add") {};`},
		{"missing symbol comma", `export c("ontama_add" "ontama_sub") { add, sub };`},
		{"missing name comma", `export c("ontama_add", "ontama_sub") { add sub };`},
		{"missing semicolon", `export c("ontama_add") { add }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, diagnostics := parseSource(t, test.source)
			if diagnostics == 0 {
				t.Fatal("expected parser diagnostic")
			}
		})
	}
}

func TestParsesInterfaceMatrix(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
interface Empty {}
interface Formatter {
  function format(value: string): string;
  function size(): int;
}
interface Named { function name(): string; }
class Document implements Formatter, Named {
  public function format(value: string): string { return value; }
  public function size(): int { return 0; }
  public function name(): string { return "document"; }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	if len(program.Declarations) != 4 {
		t.Fatalf("declarations = %d", len(program.Declarations))
	}
	empty := program.Declarations[0].(*ast.InterfaceDecl)
	formatter := program.Declarations[1].(*ast.InterfaceDecl)
	class := program.Declarations[3].(*ast.ClassDecl)
	if len(empty.Methods) != 0 || len(formatter.Methods) != 2 || len(class.Implements) != 2 {
		t.Fatalf("empty=%#v formatter=%#v implements=%#v", empty, formatter, class.Implements)
	}
}

func TestReportsMalformedInterfaceMethod(t *testing.T) {
	_, diagnosticCount := parseSource(t, `interface Broken { function format(value: string): string }`)
	if diagnosticCount == 0 {
		t.Fatal("expected parser diagnostic")
	}
}

func TestParsesNamedGoPackageImport(t *testing.T) {
	program, diagnosticCount := parseSource(t, `import go text from "strings"; function value(): string { return text.TrimSpace(" x "); }`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	if len(program.Imports) != 1 {
		t.Fatalf("imports = %d", len(program.Imports))
	}
	imported := program.Imports[0]
	if !imported.Go || imported.Alias != "text" || imported.Path != "strings" || len(imported.Names) != 0 {
		t.Fatalf("Go import = %#v", imported)
	}
	if imported.AliasSpan.Start.Offset != 10 || imported.AliasSpan.End.Offset != 14 {
		t.Fatalf("Go alias span = %#v", imported.AliasSpan)
	}
}

func TestParsesGoQualifiedTypesPointersAndStructLiterals(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
import go http from "net/http";
function use(value: http.Client, pointer: *http.Client): *http.Client {
  let client: http.Client = http.Client{ Timeout: 1 };
  pointer = &client;
  return pointer;
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	qualified := function.Parameters[0].Type
	if qualified.Qualifier != "http" || qualified.Name != "Client" || !qualified.Go {
		t.Fatalf("qualified type = %#v", qualified)
	}
	if !function.Parameters[1].Type.IsPointer() || function.Parameters[1].Type.Pointee.Qualifier != "http" {
		t.Fatalf("pointer type = %#v", function.Parameters[1].Type)
	}
	variable := function.Body.Statements[0].(*ast.VariableDecl)
	literal, ok := variable.Value.(*ast.GoCompositeLiteralExpr)
	if !ok || literal.Type.Qualifier != "http" || len(literal.Fields) != 1 || literal.Fields[0].Name != "Timeout" {
		t.Fatalf("Go struct literal = %#v", variable.Value)
	}
	assignment := function.Body.Statements[1].(*ast.AssignmentStmt)
	if unary, ok := assignment.Value.(*ast.UnaryExpr); !ok || unary.Operator != "&" {
		t.Fatalf("address expression = %#v", assignment.Value)
	}
}

func TestParsesMultipleBindingsAndAssignments(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function parse(value: string): int {
  const [number, err] = strconv.Atoi(value);
  let [discarded, _] = strconv.Atoi(value);
  [number, err] = strconv.Atoi("42");
  return number + discarded;
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	declaration := function.Body.Statements[0].(*ast.MultiVariableDecl)
	if !declaration.Constant || len(declaration.Bindings) != 2 || declaration.Bindings[0].Name != "number" || declaration.Bindings[1].Name != "err" {
		t.Fatalf("multiple declaration = %#v", declaration)
	}
	mutable := function.Body.Statements[1].(*ast.MultiVariableDecl)
	if mutable.Constant || mutable.Bindings[1].Name != "_" {
		t.Fatalf("mutable multiple declaration = %#v", mutable)
	}
	assignment := function.Body.Statements[2].(*ast.MultiAssignmentStmt)
	if len(assignment.Bindings) != 2 || assignment.Bindings[0].Name != "number" || assignment.Bindings[1].Name != "err" {
		t.Fatalf("multiple assignment = %#v", assignment)
	}
}

func TestMultipleBindingParserFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"top level", `const [value, err] = call(); function valid(): int { return 1; }`},
		{"empty binding", `function value(): void { const [] = call(); }`},
		{"missing close", `function value(): void { const [value, err = call(); }`},
		{"missing assignment", `function value(): void { const [value, err] call(); }`},
		{"non-name target", `function value(): void { [value, item.field] = call(); }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, diagnosticCount := parseSource(t, test.source)
			if diagnosticCount == 0 {
				t.Fatal("expected parser diagnostic")
			}
		})
	}
}
