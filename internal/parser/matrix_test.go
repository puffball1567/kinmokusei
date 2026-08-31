package parser

import (
	"testing"

	"ontama.local/ontama/internal/lexer"
)

func TestValidSyntaxMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"empty program", ""},
		{"top-level variables", `const enabled: boolean = true; let count = 0;`},
		{"primitive literals and unary", `function value(): int { const text = "x"; const ratio = 1.5; const flag = !false; return -1 + +2; }`},
		{"binary precedence", `function value(): boolean { return 1 + 2 * 3 === 7 && 4 % 2 === 0 || false; }`},
		{"nested conditional", `function value(flag: boolean): int { if (flag) { return 1; } else if (!flag) { return 2; } else { return 3; } }`},
		{"while and branches", `function loop(flag: boolean): void { while (flag) { if (flag) { continue; } break; } }`},
		{"goto and labeled branches", `function loop(value: int): int { goto dispatch; dispatch: if (value > 0) { return value; } outer: for (;;) { if (value === 0) { break outer; } continue outer; } return 0; }`},
		{"for all clauses", `function loop(): void { for (let i = 0; i < 2; i = i + 1) {} }`},
		{"for empty clauses", `function loop(): void { for (;;) { break; } }`},
		{"channel range const let typed and blank", `function ranges(channel: GoReceiveChannel<int>): void { for (const value of channel) { use(value); } for (let value: int of channel) { value = value + 1; } for (const _ of channel) {} }`},
		{"collection range value key index typed and blank", `function ranges(items: int[], table: Map<string, int>, text: string): void { for (const value of items) { use(value); } for (let [index: int, value: int] of items) { use(index + value); } for (const [key, value] of table) { use(key); } for (const [offset, rune] of text) { use(offset + int(rune)); } for (const [_, _] of items) {} }`},
		{"channel select receive send assign discard default and empty", `function choose(input: GoReceiveChannel<int>, output: GoSendChannel<int>, channel: GoChannel<int>): void { let value = 0; let open = false; select { case const item = <-input { use(item); } case let [item, ok] = <-channel { item = item + 1; use(ok); } case value = <-channel { use(value); } case [value, open] = <-channel { use(open); } case output <- value {} case <-input {} default {} } select {} }`},
		{"Go interface type switch const let blank nil and default", `import go io from "io"; import go strings from "strings"; function classify(value: io.Reader): int { switch (value) { case const reader as *strings.Reader { return reader.Len(); } case let writer as io.Writer { writer = writer; return 2; } case const _ as io.Reader { return 3; } case nil { return 4; } default { return 5; } } }`},
		{"value switch single multi nil default and break", `function classify(value: int, pointer: *int): int { switch (value) { case 0 { return 1; } case 1, 2 { break; } default { return 3; } } switch (pointer) { case nil { return 4; } default {} } return 0; }`},
		{"function type and arrows", `function apply(fn: (left: int, right: int) => int): int { const add = (left: int, right: int): int => { return left + right; }; return fn(add(1, 2)); }`},
		{"trailing commas", `function call(value: int,): int { return call(value,); } function arrays(): int[] { return [1, 2,]; }`},
		{"nested collection types", `function lookup(values: Map<string, int[]>): int { return values["x"][0]; }`},
		{"object and member chain", `function value(): int { const dto = { child: { value: 1 }, label: "ok", }; return dto.child.value; }`},
		{"class members", `class Value { public name: string; private count: int; constructor(public initial: int, label: string,) { this.count = initial; } public function get(): int { return this.count; } public static function create(): Value { return new Value(1, "x",); } }`},
		{"interfaces and implementations", `interface Reader { function read(index: int): string; } interface Empty {} class Text implements Reader, Empty { public function read(index: int): string { return "x"; } }`},
		{"qualified Go interface implementation", `import go sort from "sort"; class Values implements sort.Interface { public function len(): int { return 0; } public function less(left: int, right: int): boolean { return false; } public function swap(left: int, right: int): void {} }`},
		{"relative import", `import { A, value, } from "../dependency"; function use(): void {}`},
		{"Go package import", `import go strings from "strings"; function use(): string { return strings.TrimSpace(" x "); }`},
		{"Go qualified types and pointers", `import go time from "time"; function copy(value: time.Duration): time.Duration { let item: time.Duration = value; const pointer: *time.Duration = &item; return *pointer; }`},
		{"Go generic named types", `import go atomic from "sync/atomic"; function load(value: *atomic.Pointer<string>): *string { return value.Load(); }`},
		{"Go checked and unchecked assertions", `import go io from "io"; import go strings from "strings"; function force(value: io.Reader): *strings.Reader { return value as! *strings.Reader; } function probe(value: io.Reader): boolean { const [reader, ok] = value as? *strings.Reader; return ok; }`},
		{"defer and goroutine calls", `function invoke(value: int): int { return value; } function run(callback: (value: int) => int): void { defer invoke(1); go callback(2); }`},
		{"raw Go channel types send and receive", `function relay(input: GoReceiveChannel<int>, output: GoSendChannel<int>, bidirectional: GoChannel<int>): int { output <- <-input + 1; bidirectional <- 2; return <-bidirectional; }`},
		{"raw Go channel creation close and checked receive", `function use(): boolean { const channel = goChannel[int](1); channel <- 42; const [value, open] = <-channel; closeGoChannel(channel); return open; }`},
		{"Go struct and nil pointer", `import go http from "net/http"; function client(): boolean { let value: http.Client = http.Client{ Timeout: 0, }; let pointer: *http.Client = nil; return pointer == nil; }`},
		{"variadic slice expansion", `import go path from "path"; function join(parts: string[]): string { return path.Join(parts...); }`},
		{"native rest declarations", `function sum(prefix: int, ...values: int[]): int { return prefix; } interface Joiner { function join(...parts: string[]): string; } class Batch { constructor(public ...values: int[]) {} public function add(...values: int[]): void {} } struct Values { public function add(...values: int[]): void {} } function arrows(): void { const sum = (...values: int[]): int => len(values); new Batch([1, 2]...); }`},
		{"explicit Go generic arguments", `import go slices from "slices"; function clone(items: int[]): int[] { return slices.Clone[int[]](items); }`},
		{"explicit C ABI export", `export c("ontama_add") function add(left: int32, right: int32): int32 { return left + right; }`},
		{"qualified member index remains indexing", `function value(holder: Map<string, int[]>): int { return holder.values[0]; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("valid.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			program, diagnostics := Parse(tokens)
			if program == nil || len(diagnostics) != 0 {
				t.Fatalf("program = %#v, diagnostics = %v", program, diagnostics)
			}
		})
	}
}

func TestParserFailureAndRecoveryMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"unknown top level", `return 1; function recovered(): void {}`},
		{"empty import", `import {} from "./x"; function recovered(): void {}`},
		{"missing import path", `import { value } from; function recovered(): void {}`},
		{"missing function name", `function (): void {} function recovered(): void {}`},
		{"missing parameter type", `function broken(value): void {} function recovered(): void {}`},
		{"missing return type", `function broken(): {} function recovered(): void {}`},
		{"goto missing label", `function broken(): void { goto; }`},
		{"label missing statement", `function broken(): void { empty: }`},
		{"missing expression", `function broken(): void { const value = ; } function recovered(): void {}`},
		{"missing closing block", `function broken(): void { if (true) { return; }`},
		{"invalid assignment target", `function broken(): void { (1 + 2) = 3; }`},
		{"missing array close", `function broken(): int[] { return [1, 2; }`},
		{"missing object colon", `function broken(): int { const x = { value 1 }; return 0; }`},
		{"missing member name", `function broken(value: string): void { value.; }`},
		{"incomplete arrow", `function broken(): void { const fn = (value: int) =>; }`},
		{"class missing body", `class Broken function recovered(): void {}`},
		{"duplicate constructors", `class Broken { constructor() {} constructor() {} }`},
		{"static constructor", `class Broken { static constructor() {} }`},
		{"static field", `class Broken { static value: int; }`},
		{"interface missing semicolon", `interface Broken { function value(): int }`},
		{"interface method body", `interface Broken { function value(): int {} }`},
		{"implements missing type", `class Broken implements {}`},
		{"Go import missing alias", `import go from "strings"; function recovered(): void {}`},
		{"Go import missing from", `import go strings "strings"; function recovered(): void {}`},
		{"Go import missing path", `import go strings from; function recovered(): void {}`},
		{"Go import missing semicolon", `import go strings from "strings" function recovered(): void {}`},
		{"Go qualified type missing name", `import go time from "time"; function broken(value: time.): void {} function recovered(): void {}`},
		{"Go pointer missing pointee", `function broken(value: *): void {} function recovered(): void {}`},
		{"Go struct field missing colon", `import go time from "time"; function broken(): void { const value = time.Time{ Location nil }; }`},
		{"spread argument not final", `function broken(values: int[]): void { call(values..., 1); }`},
		{"multiple spread arguments", `function broken(left: int[], right: int[]): void { call(left..., right...); }`},
		{"rest parameter not slice", `function broken(...values: int): void {}`},
		{"rest parameter not final", `function broken(...values: int[], tail: int): void {}`},
		{"rest receiver", `struct Value {} function broken(...this: Value): void {}`},
		{"empty explicit Go type arguments", `import go slices from "slices"; function broken(items: int[]): int[] { return slices.Clone[](items); }`},
		{"unterminated explicit Go type arguments", `import go slices from "slices"; function broken(items: int[]): int[] { return slices.Clone[int(items); }`},
		{"assertion missing failure marker", `import go io from "io"; function broken(value: io.Reader): io.Reader { return value as io.Reader; }`},
		{"assertion missing type", `import go io from "io"; function broken(value: io.Reader): io.Reader { return value as!; }`},
		{"defer missing call", `function broken(): void { defer; }`},
		{"defer non-call", `function broken(): void { defer 1; }`},
		{"go non-call", `function broken(value: int): void { go value; }`},
		{"channel send missing value", `function broken(channel: GoChannel<int>): void { channel <-; }`},
		{"channel receive missing operand", `function broken(): int { return <-; }`},
		{"channel factory empty type arguments", `function broken(): void { const channel = goChannel[](1); }`},
		{"channel factory unterminated type arguments", `function broken(): void { const channel = goChannel[int(1); }`},
		{"channel range missing binding", `function broken(channel: GoChannel<int>): void { for (const of channel) {} }`},
		{"channel range missing source", `function broken(): void { for (const value of) {} }`},
		{"channel range missing close parenthesis", `function broken(channel: GoChannel<int>): void { for (const value of channel {} }`},
		{"channel range missing body", `function broken(channel: GoChannel<int>): void { for (const value of channel) }`},
		{"range binding list too short", `function broken(items: int[]): void { for (const [value] of items) {} }`},
		{"range binding list too long", `function broken(items: int[]): void { for (const [index, value, extra] of items) {} }`},
		{"range binding list missing close", `function broken(items: int[]): void { for (const [index, value of items) {} }`},
		{"select missing open brace", `function broken(): void { select case <-channel {} }`},
		{"select unexpected clause", `function broken(): void { select { return; } }`},
		{"select declaration missing binding", `function broken(channel: GoChannel<int>): void { select { case const = <-channel {} } }`},
		{"select declaration missing assignment", `function broken(channel: GoChannel<int>): void { select { case const value <-channel {} } }`},
		{"select declaration requires receive", `function broken(channel: GoChannel<int>): void { select { case const value = channel {} } }`},
		{"select assignment requires receive", `function broken(channel: GoChannel<int>): void { let value = 0; select { case value = channel {} } }`},
		{"select invalid communication", `function broken(): void { select { case call() {} } }`},
		{"select case missing body", `function broken(channel: GoChannel<int>): void { select { case <-channel } }`},
		{"select default missing body", `function broken(): void { select { default } }`},
		{"select checked declaration too few bindings", `function broken(channel: GoChannel<int>): void { select { case const [value] = <-channel {} } }`},
		{"select checked assignment invalid target", `function broken(channel: GoChannel<int>): void { let value = 0; select { case [value, 1] = <-channel {} } }`},
		{"type switch missing open parenthesis", `function broken(value: error): void { switch value) {} }`},
		{"type switch missing value", `function broken(): void { switch () {} }`},
		{"type switch missing close parenthesis", `function broken(value: error): void { switch (value {} }`},
		{"type switch missing body", `function broken(value: error): void { switch (value) }`},
		{"type switch unexpected clause", `function broken(value: error): void { switch (value) { return; } }`},
		{"type switch mixed value case", `function broken(value: error): void { switch (value) { case const typed as error {} case value {} } }`},
		{"type switch case missing binding", `function broken(value: error): void { switch (value) { case const as error {} } }`},
		{"type switch case missing as", `function broken(value: error): void { switch (value) { case const typed error {} } }`},
		{"type switch case missing type", `function broken(value: error): void { switch (value) { case const typed as {} } }`},
		{"type switch case missing body", `function broken(value: error): void { switch (value) { case const typed as error } }`},
		{"type switch nil missing body", `function broken(value: error): void { switch (value) { case nil } }`},
		{"type switch default missing body", `function broken(value: error): void { switch (value) { default } }`},
		{"value switch missing case value", `function broken(value: int): void { switch (value) { case {} } }`},
		{"value switch trailing case comma", `function broken(value: int): void { switch (value) { case 1, {} } }`},
		{"C ABI export missing boundary", `export function recovered(): void {}`},
		{"C ABI export unknown boundary", `export wasm("value") function recovered(): void {}`},
		{"C ABI export missing open parenthesis", `export c "value") function recovered(): void {}`},
		{"C ABI export missing symbol", `export c() function recovered(): void {}`},
		{"C ABI export missing close parenthesis", `export c("value" function recovered(): void {}`},
		{"C ABI export missing function", `export c("value") const recovered = 1;`},
		{"C ABI export receiver parameter", `struct Point { public x: int; } export c("move") function move(this: *Point): void {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("invalid.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("unexpected lexer diagnostics = %v", lexDiagnostics)
			}
			program, diagnostics := Parse(tokens)
			if program == nil || len(diagnostics) == 0 {
				t.Fatalf("program = %#v, expected parser diagnostics", program)
			}
		})
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"", "function main(): void {}", `export c("ontama_value") function value(input: int32): int32 { return input; }`, "const fn = (value: int) => value + 1;", "function f(): Result<int> { const task: Task<Result<int>> = go load(); const value = await task?; detach go notify(); return ok(value); }", "function f(err: error): void { try { throw err; } catch (caught: error) { throw caught; } finally {} }", "class C implements I {", "for (((", "select { case const [value, open] = <-channel {", "switch (value) { case const typed as", "function f(value: [2][3]int): [0]byte { return []; }", "function f(value: [999999999999999999999]int): void {", "function f(values: int[]): int[] { return values[1:2:3]; }", "function f(values: int[]): int[] { return values[1::]; }", "function f(): void { makeSlice[int](1, 2); makeMap[string, int](); append([1], [2]...); }", "function f(values: int[]): void { copyArray[[2]int](values); viewArray[[2]int](values); }", "function f(values: int[]): void { copyArray[[2]int](values...); viewArray[](values); }", "function f(): void { makeSlice[](1); makeMap[string,](1); }", "interface I { function f(): int; }", `import go strings from "strings";`, `function f(): void { const [value, err] = call(); [value, err] = call(); }`, `function f(values: int[]): void { call(values...); }`, `function f(value: error): void { const [typed, ok] = value as? error; }`, `function f(value: int): int { return ^value & 7 | value &^ 3 << 2 >> 1; }`, `function f(value: int, items: int[]): void { value += 1; items[0] &^= value; for (; value < 3; value++) {} value--; }`, `function f(value: Outer<Middle<Inner<int>>>): void {}`, `struct Point { public x: int; } public function move(this: *Point, delta: int): void { this.x += delta; }`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		tokens, _ := lexer.Lex("fuzz.otm", input)
		program, _ := Parse(tokens)
		if program == nil {
			t.Fatal("parser returned a nil program")
		}
	})
}
