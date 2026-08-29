package codegen

import (
	"bytes"
	goast "go/ast"
	"go/format"
	goParser "go/parser"
	"go/token"
	"strings"
	"testing"

	ontamaAST "ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/lexer"
	ontamaParser "ontama.local/ontama/internal/parser"
	"ontama.local/ontama/internal/sema"
)

func formatGoExpression(t *testing.T, expression goast.Expr) string {
	t.Helper()
	var output bytes.Buffer
	if err := format.Node(&output, token.NewFileSet(), expression); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func generateCheckedSource(t *testing.T, source string) []byte {
	t.Helper()
	tokens, lexDiagnostics := lexer.Lex("matrix.otm", source)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
	}
	program, parseDiagnostics := ontamaParser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics = %v", parseDiagnostics)
	}
	if diagnostics := sema.Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics = %v", diagnostics)
	}
	generated, err := Generate(program, "matrix")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = goParser.ParseFile(token.NewFileSet(), "generated.go", generated, goParser.AllErrors); err != nil {
		t.Fatalf("generated invalid Go: %v\n%s", err, generated)
	}
	return generated
}

func TestGeneratedGoSyntaxMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			"declarations and primitive expressions",
			`const compileTime: int = 2; const runtime = compute(); let mutable = 1; function compute(): int { const text = "a" + "b"; const flag = !false && 1 < 2; return -1 + +2 * 3 / 1 % 2; }`,
			[]string{"const compileTime int = 2", "var runtime = compute()", "var mutable = 1", `const text = "a" + "b"`, "const flag = !false && 1 < 2"},
		},
		{
			"conditionals loops and assignments",
			`function control(limit: int): int { let value = 0; for (let index = 0; index < limit; index = index + 1) { if (index === 2) { continue; } value = value + index; } while (value < 10) { value = value + 1; if (value > limit) { break; } } return value; }`,
			[]string{"for index := 0; index < limit; index = index + 1", "if index == 2", "continue", "for value < 10", "break"},
		},
		{
			"function types and arrow bodies",
			`function apply(value: int, operation: (value: int) => int): int { return operation(value); } function arrows(): int { const expression = (value: int) => value + 1; const block = (value: int): int => { return expression(value); }; return apply(1, block); }`,
			[]string{"operation func(int) int", "var expression = func(value int) int", "var block = func(value int) int"},
		},
		{
			"collections and object fields",
			`function collect(values: int[], lookup: Map<string, int>): int { let items: int[] = []; items = [values[0], lookup["x"]]; const dto = { count: items[0], label: "ok" }; return dto.count; }`,
			[]string{"values []int", "lookup map[string]int", "var items = []int{}", "struct {", "Count int", "`json:\"count\"`", "dto.Count"},
		},
		{
			"class interface and visibility",
			`interface Reader { function read(): int; } class Value implements Reader { constructor(public initial: int, private hidden: int) {} public function read(): int { return this.initial + this.hidden; } private function secret(): int { return this.hidden; } public static function create(): Value { return new Value(1, 2); } } function use(reader: Reader): int { return reader.read(); }`,
			[]string{"type Reader interface", "Read() int", "type Value struct", "Initial int", "hidden int", "var _ Reader = &Value{}", "func NewValue", "func (this *Value) Read() int", "func ValueCreate() *Value", "reader.Read()"},
		},
		{
			"empty types and trailing lists",
			`interface Empty {} class Unit implements Empty { constructor() {} } function empty(): int[] { return []; }`,
			[]string{"type Empty interface", "type Unit struct", "var _ Empty = &Unit{}", "return []int{}"},
		},
		{
			"Go multiple results and error",
			`import go strconv from "strconv"; function parse(value: string): int { const [parsed, err] = strconv.Atoi(value); if (err != nil) { return -1; } let copy = 0; let copyError: error = nil; [copy, copyError] = strconv.Atoi(value); const [unused, _] = strconv.Atoi(value); return parsed + copy; }`,
			[]string{"var parsed, err = strconv.Atoi(value)", "var copyError error = nil", "copy_, copyError = strconv.Atoi(value)", "var unused, _ = strconv.Atoi(value)", "_ = unused"},
		},
		{
			"Go generic inferred and explicit calls",
			`import go slices from "slices"; function inferred(items: int[]): int[] { return slices.Clone(items); } function explicit(items: int[]): int[] { return slices.Clone[int[]](items); } function concat(): int[] { return slices.Concat[int[]](); }`,
			[]string{"slices.Clone(items)", "slices.Clone[[]int](items)", "slices.Concat[[]int]()"},
		},
		{
			"Go generic named types and methods",
			`import go atomic from "sync/atomic"; import go time from "time"; import go unique from "unique"; function pointer(): atomic.Pointer<time.Duration> { return atomic.Pointer<time.Duration>{}; } function handles(value: string): unique.Handle<string>[] { return [unique.Make(value)]; }`,
			[]string{"func pointer() atomic.Pointer[time.Duration]", "return atomic.Pointer[time.Duration]{}", "func handles(value string) []unique.Handle[string]", "unique.Make(value)"},
		},
		{
			"Go checked and unchecked type assertions",
			`import go io from "io"; import go strings from "strings"; function force(reader: io.Reader): *strings.Reader { return reader as! *strings.Reader; } function probe(reader: io.Reader): boolean { const [typed, ok] = reader as? *strings.Reader; return ok; }`,
			[]string{"return reader.(*strings.Reader)", "var typed, ok = reader.(*strings.Reader)"},
		},
		{
			"Go interface type switch",
			`import go io from "io"; import go strings from "strings"; function classify(value: io.Reader): int { switch (value) { case const reader as *strings.Reader { return reader.Len(); } case let writer as io.Writer { writer = writer; return 2; } case const _ as io.Reader { return 3; } case nil { return 4; } default { return 5; } } }`,
			[]string{"switch __ontama_type_switch_", ":= value.(type)", "case *strings.Reader:", "reader := __ontama_type_switch_", "case io.Writer:", "writer := __ontama_type_switch_", "case io.Reader:", "case nil:", "default:"},
		},
		{
			"value switch",
			`function classify(value: int): int { switch (value) { case 0 { return 1; } case 1, 2 + 1 { break; } default { return 4; } } return 0; }`,
			[]string{"switch value {", "case 0:", "case 1, 2 + 1:", "break", "default:"},
		},
		{
			"defer and goroutine calls",
			`function target(value: int): int { return value; } function run(callback: (value: int) => int): void { defer target(1); go callback(2); }`,
			[]string{"defer target(1)", "go callback(2)"},
		},
		{
			"raw Go channel direction send and receive",
			`function relay(input: GoReceiveChannel<int>, output: GoSendChannel<int>, bidirectional: GoChannel<int>): int { output <- <-input + 1; bidirectional <- 2; <-bidirectional; return <-bidirectional; }`,
			[]string{"input <-chan int", "output chan<- int", "bidirectional chan int", "output <- <-input + 1", "<-bidirectional", "return <-bidirectional"},
		},
		{
			"raw Go channel creation close and checked receive",
			`function channels(): boolean { const channel = goChannel[int](1); channel <- 42; const [value, open] = <-channel; closeGoChannel(channel); return open; }`,
			[]string{"var channel = make(chan int, 1)", "channel <- 42", "var value, open = <-channel", "close(channel)"},
		},
		{
			"raw Go channel range and unused binding",
			`function ranges(channel: GoReceiveChannel<int>): int { let total = 0; for (const value of channel) { total = total + value; } for (const unused of channel) {} return total; }`,
			[]string{"for value := range channel", "for range channel"},
		},
		{
			"raw Go channel select communication matrix",
			`function choose(input: GoReceiveChannel<int>, output: GoSendChannel<int>, channel: GoChannel<int>): int { let value = 0; let open = false; select { case const received = <-input { return received; } case const [unused, ok] = <-channel { if (ok) { return 1; } } case const [_, _] = <-channel {} case let mutable = <-channel { mutable = mutable + 1; return mutable; } case value = <-channel {} case [value, open] = <-channel {} case [_, open] = <-channel {} case output <- value {} case <-input {} default { break; } } return value; } function waitForever(): void { select {} }`,
			[]string{"select {", "case received := <-input:", "case _, ok := <-channel:", "case mutable := <-channel:", "case value = <-channel:", "case value, open = <-channel:", "case _, open = <-channel:", "case output <- value:", "case <-input:", "default:", "select {}"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generated := string(generateCheckedSource(t, test.source))
			for _, want := range test.want {
				if !strings.Contains(generated, want) {
					t.Errorf("generated Go does not contain %q:\n%s", want, generated)
				}
			}
		})
	}
}

func TestGenerationIsDeterministicAndDefaultsPackageName(t *testing.T) {
	source := `interface Value { function get(): int; } class Item implements Value { public function get(): int { return 1; } } function value(): Value { return new Item(); }`
	tokens, _ := lexer.Lex("stable.otm", source)
	program, parseDiagnostics := ontamaParser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics = %v", parseDiagnostics)
	}
	if diagnostics := sema.Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics = %v", diagnostics)
	}
	first, err := Generate(program, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(program, "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("generation is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if !strings.Contains(string(first), "package main") {
		t.Fatalf("default package missing:\n%s", first)
	}
}

func TestGoTypeMatrix(t *testing.T) {
	integer := ontamaAST.TypeRef{Name: "int"}
	stringType := ontamaAST.TypeRef{Name: "string"}
	void := ontamaAST.TypeRef{Name: "void"}
	qualified := ontamaAST.TypeRef{Name: "Duration", Qualifier: "time", Go: true}
	fixedLength := int64(3)
	tests := []struct {
		name string
		ref  ontamaAST.TypeRef
		want string
	}{
		{"boolean", ontamaAST.TypeRef{Name: "boolean"}, "bool"},
		{"number", ontamaAST.TypeRef{Name: "number"}, "float64"},
		{"class", ontamaAST.TypeRef{Name: "Value"}, "*Value"},
		{"interface", ontamaAST.TypeRef{Name: "Reader", Interface: true}, "Reader"},
		{"Go qualified", qualified, "time.Duration"},
		{"Go pointer", ontamaAST.TypeRef{Pointee: &qualified}, "*time.Duration"},
		{"Go basic", ontamaAST.TypeRef{Name: "uint64", Go: true}, "uint64"},
		{"Go error", ontamaAST.TypeRef{Name: "error", Go: true}, "error"},
		{"empty anonymous Go struct", ontamaAST.TypeRef{GoStruct: true, Go: true}, "struct {\n}"},
		{"array", ontamaAST.TypeRef{Element: &integer}, "[]int"},
		{"fixed array", ontamaAST.TypeRef{Element: &integer, FixedLength: &fixedLength}, "[3]int"},
		{"map", ontamaAST.TypeRef{Name: "Map", GenericArguments: []ontamaAST.TypeRef{stringType, integer}}, "map[string]int"},
		{"bidirectional channel", ontamaAST.TypeRef{Name: "GoChannel", GenericArguments: []ontamaAST.TypeRef{integer}, Go: true}, "chan int"},
		{"send channel", ontamaAST.TypeRef{Name: "GoSendChannel", GenericArguments: []ontamaAST.TypeRef{integer}, Go: true}, "chan<- int"},
		{"receive channel", ontamaAST.TypeRef{Name: "GoReceiveChannel", GenericArguments: []ontamaAST.TypeRef{integer}, Go: true}, "<-chan int"},
		{"function result", ontamaAST.TypeRef{Parameters: []ontamaAST.TypeRef{integer}, Return: &integer}, "func(int) int"},
		{"void function", ontamaAST.TypeRef{Parameters: []ontamaAST.TypeRef{integer}, Return: &void}, "func(int)"},
		{
			"object fields are canonical and retain JSON names",
			ontamaAST.TypeRef{Object: true, ObjectFields: []ontamaAST.ObjectTypeField{
				{Name: "Zebra", JSONName: "zebra", Type: stringType},
				{Name: "Alpha", JSONName: "alpha", Type: integer},
			}},
			"struct {\n\tAlpha int    `json:\"alpha\"`\n\tZebra string `json:\"zebra\"`\n}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatGoExpression(t, goType(test.ref)); got != test.want {
				t.Fatalf("goType() = %q, want %q", got, test.want)
			}
		})
	}
	goStruct := ontamaAST.TypeRef{GoStruct: true, Go: true, ObjectFields: []ontamaAST.ObjectTypeField{
		{Name: "Second", GoTag: `json:"second"`, Type: stringType},
		{Name: "First", Type: integer},
	}}
	formattedStruct := formatGoExpression(t, goType(goStruct))
	if second, first := strings.Index(formattedStruct, "Second"), strings.Index(formattedStruct, "First"); second < 0 || first < 0 || second >= first || !strings.Contains(formattedStruct, `"json:\"second\""`) {
		t.Fatalf("anonymous Go struct lost order or tag: %s", formattedStruct)
	}
}

func TestOperatorAndNameMappingMatrix(t *testing.T) {
	operators := map[string]token.Token{
		"+": token.ADD, "-": token.SUB, "*": token.MUL, "/": token.QUO, "%": token.REM, "!": token.NOT, "&": token.AND,
		"==": token.EQL, "===": token.EQL, "!=": token.NEQ, "!==": token.NEQ,
		"<": token.LSS, "<=": token.LEQ, ">": token.GTR, ">=": token.GEQ, "&&": token.LAND, "||": token.LOR,
	}
	for operator, want := range operators {
		if got := goToken(operator); got != want {
			t.Errorf("goToken(%q) = %s, want %s", operator, got, want)
		}
	}
	if got := goToken("unknown"); got != token.ILLEGAL {
		t.Errorf("unknown operator = %s", got)
	}
	if goName("type") != "type_" || goName("len") != "len_" || goName("copy") != "copy_" || goName("value") != "value" {
		t.Errorf("Go keyword mangling is incorrect")
	}
	if memberName("value", ontamaAST.Public) != "Value" || memberName("value", ontamaAST.Private) != "value" || memberName("", ontamaAST.Public) != "" {
		t.Errorf("member visibility mapping is incorrect")
	}
}

func TestGoConstantClassificationMatrix(t *testing.T) {
	literal := &ontamaAST.LiteralExpr{Kind: ontamaAST.IntegerLiteral, Text: "1"}
	nilLiteral := &ontamaAST.LiteralExpr{Kind: ontamaAST.NilLiteral, Text: "nil"}
	identifier := &ontamaAST.IdentifierExpr{Name: "value"}
	tests := []struct {
		name string
		expr ontamaAST.Expression
		want bool
	}{
		{"literal", literal, true},
		{"nil", nilLiteral, false},
		{"unary literal", &ontamaAST.UnaryExpr{Operator: "-", Operand: literal}, true},
		{"binary literals", &ontamaAST.BinaryExpr{Operator: "+", Left: literal, Right: literal}, true},
		{"binary with value", &ontamaAST.BinaryExpr{Operator: "+", Left: literal, Right: identifier}, false},
		{"numeric conversion", &ontamaAST.CallExpr{Callee: &ontamaAST.IdentifierExpr{Name: "int64"}, Arguments: []ontamaAST.Expression{literal}}, true},
		{"conversion arity", &ontamaAST.CallExpr{Callee: &ontamaAST.IdentifierExpr{Name: "int64"}}, false},
		{"ordinary call", &ontamaAST.CallExpr{Callee: &ontamaAST.IdentifierExpr{Name: "compute"}, Arguments: []ontamaAST.Expression{literal}}, false},
		{"value", identifier, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isGoConstant(test.expr); got != test.want {
				t.Fatalf("isGoConstant(%T) = %v, want %v", test.expr, got, test.want)
			}
		})
	}
	for _, name := range []string{"int", "int32", "int64", "uint16", "uint32", "uint64", "float32", "float", "number", "float64", "byte"} {
		if !isBuiltinConversion(name) {
			t.Errorf("%q is not recognized as a conversion", name)
		}
	}
	if isBuiltinConversion("string") {
		t.Fatal("string unexpectedly recognized as a numeric conversion")
	}
}

func TestUnusedLocalsAreLoweredToValidGo(t *testing.T) {
	generated := string(generateCheckedSource(t, `
function sideEffect(): int { return 1; }
function value(): void {
  const compileTime = 1;
  const runtime = sideEffect();
  for (let index = 0; ; ) { break; }
}
`))
	for _, want := range []string{"_ = compileTime", "_ = runtime", "for _ = 0; ;"} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
}

func TestGeneratedGoValidationRejectsTypeErrors(t *testing.T) {
	err := validateGeneratedGo([]byte("package broken\nfunc value() int { return \"wrong\" }\n"))
	if err == nil || !strings.Contains(err.Error(), "cannot use") {
		t.Fatalf("validation error = %v", err)
	}
}
