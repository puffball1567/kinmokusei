package sema

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
	"github.com/puffball1567/onsentamago/internal/parser"
)

func TestCABIExportSuccessMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"void without parameters", `export c("ontama_ping") function ping(): void {}`},
		{"all fixed scalar types", `export c("ontama_scalars") function scalars(a: byte, b: uint8, c: int8, d: int16, e: int32, f: int64, g: uint16, h: uint32, i: uint64, j: float32, k: float, l: float64, m: number): float64 { return k; }`},
		{"normalized boolean", `export c("ontama_not") function logicalNot(value: boolean): boolean { return !value; }`},
		{"multiple exports", `export c("ontama_left") function left(value: int32): int32 { return value; } export c("ontama_right") function right(value: int64): int64 { return value; }`},
		{"export list functions", `function add(left: int32, right: int32): int32 { return left + right; } function sub(left: int32, right: int32): int32 { return left - right; } export c("ontama_add", "ontama_sub") {add, sub};`},
		{"export list forward references", `export c("ontama_add", "ontama_sub") {add, sub}; function add(left: int32, right: int32): int32 { return left + right; } const sub = (left: int32, right: int32): int32 => left - right;`},
		{"export list arrows", `const ping = (): void => {}; const scale = (value: float64): float64 => value * 2.0; export c("ontama_ping", "ontama_scale") {ping, scale};`},
		{"export list multiline trailing commas", "function add(left: int32, right: int32): int32 { return left + right; }\nconst sub = (left: int32, right: int32): int32 => left - right;\nexport c(\n  \"ontama_add\",\n  \"ontama_sub\",\n) {\n  add,\n  sub,\n};"},
		{"fixed-width enum", `enum Level: int16 { Minimum = -32768, Normal = 4, Maximum = 32767 } export c("ontama_next") function next(value: Level): Level { return Level(int16(value) + int16(1)); }`},
		{"fixed-width enum through defined underlying", `type CodeBase = distinct uint32; enum Code: CodeBase { Empty, Ready = 41 } export c("ontama_code") function code(value: Code): Code { return value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		})
	}
}

func TestCABIExportFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty symbol", `export c("") function value(): void {}`, "must start with an ASCII letter"},
		{"leading underscore", `export c("_value") function value(): void {}`, "must start with an ASCII letter"},
		{"hyphen", `export c("bad-value") function value(): void {}`, "contain only ASCII"},
		{"Unicode", `export c("温泉") function value(): void {}`, "must start with an ASCII letter"},
		{"Go keyword", `export c("func") function value(): void {}`, "is a Go keyword"},
		{"reserved main", `export c("main") function value(): void {}`, "is reserved by the generated Go package"},
		{"reserved init", `export c("init") function value(): void {}`, "is reserved by the generated Go package"},
		{"duplicate symbol", `export c("same") function left(): void {} export c("same") function right(): void {}`, `duplicate C ABI symbol "same"`},
		{"implementation collision", `function same(): void {} export c("same") function other(): void {}`, `generated Go name "same" collides`},
		{"machine width int", `export c("value") function value(input: int): void {}`, `parameter "input" has unsupported type int`},
		{"machine width uint", `export c("value") function value(input: uint): void {}`, `parameter "input" has unsupported type uint`},
		{"machine width enum int", `enum Status { Pending } export c("value") function value(input: Status): Status { return input; }`, `parameter "input" has unsupported type Status`},
		{"machine width enum uint", `enum Status: uint { Pending } export c("value") function value(input: Status): Status { return input; }`, `parameter "input" has unsupported type Status`},
		{"string", `export c("value") function value(input: string): void {}`, `parameter "input" has unsupported type string`},
		{"slice", `export c("value") function value(input: byte[]): void {}`, `parameter "input" has unsupported type byte[]`},
		{"object result", `export c("value") function value(): { item: int32 } { return { item: 1 }; }`, `result has unsupported type non-scalar type`},
		{"error result", `export c("value") function value(): error { return nil; }`, `result has unsupported type error`},
		{"symbol and name count mismatch", `function add(): void {} export c("ontama_add", "ontama_extra") {add};`, `counts must match positionally`},
		{"undefined target", `export c("ontama_missing") {missing};`, `undefined C ABI export target "missing"`},
		{"mutable arrow", `let value = (input: int32): int32 => input; export c("ontama_value") {value};`, `must be a const arrow function`},
		{"const scalar", `const value: int32 = 1; export c("ontama_value") {value};`, `must be a top-level function or const arrow function`},
		{"arrow inferred result", `const value = (input: int32) => input; export c("ontama_value") {value};`, `requires an explicit return type`},
		{"arrow unsupported parameter", `const value = (input: string): int32 => 1; export c("ontama_value") {value};`, `parameter "input" has unsupported type string`},
		{"arrow unsupported result", `const value = (): string => "value"; export c("ontama_value") {value};`, `result has unsupported type string`},
		{"duplicate list symbol", `function left(): void {} function right(): void {} export c("same", "same") {left, right};`, `duplicate C ABI symbol "same"`},
		{"duplicate list target", `function value(): void {} export c("first", "second") {value, value};`, `duplicate C ABI export target "value"`},
		{"inline and list duplicate target", `export c("first") function value(): void {} export c("second") {value};`, `duplicate C ABI export target "value"`},
		{"list implementation collision", `function same(): void {} function other(): void {} export c("same") {other};`, `generated Go name "same" collides`},
		{"list generic function", `function identity<T>(value: T): T { return value; } export c("identity") {identity};`, `generic functions cannot be exported through the C ABI`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}

func TestCABIExportListResolvesPositionalTargets(t *testing.T) {
	tokens, lexDiagnostics := lexer.Lex("cabi-list.otm", `
function add(left: int32, right: int32): int32 { return left + right; }
const sub = (left: int32, right: int32): int32 => left - right;
export c("ontama_sub", "ontama_add") {sub, add};
`)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics=%v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics=%v", parseDiagnostics)
	}
	if diagnostics := Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics=%v", diagnostics)
	}
	if len(program.CABIExports) != 2 {
		t.Fatalf("resolved exports=%#v", program.CABIExports)
	}
	for index, want := range []struct{ symbol, name string }{{"ontama_sub", "sub"}, {"ontama_add", "add"}} {
		got := program.CABIExports[index]
		if got.Symbol != want.symbol || got.Name != want.name || len(got.Parameters) != 2 || got.ReturnType.Name != "int32" {
			t.Fatalf("export[%d]=%#v want symbol=%q name=%q", index, got, want.symbol, want.name)
		}
	}
	declaration := program.Declarations[2].(*ast.CABIExportDecl)
	if len(declaration.ResolvedDeclarations) != 2 || declaration.ResolvedDeclarations[0] != program.Declarations[1].(*ast.VariableDecl).NameSpan || declaration.ResolvedDeclarations[1] != program.Declarations[0].(*ast.FunctionDecl).NameSpan {
		t.Fatalf("resolved declarations=%#v", declaration.ResolvedDeclarations)
	}
}
