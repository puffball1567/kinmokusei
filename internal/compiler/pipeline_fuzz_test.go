package compiler

import (
	"bytes"
	"fmt"
	"go/importer"
	goparser "go/parser"
	"go/token"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	kinmokuseiParser "github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/sema"
)

var pipelineFuzzSeeds = []string{
	`export function value():int{return hidden()} function hidden():int{return 2}`,
	`export {}; const hidden=1;`,
	`export { value }; const value=(n:int):int=>n+1;`,
	`export { missing, missing };`,
	`export type`,
	`import go { Compare, Ordered } from "cmp"; function f<T extends Ordered>(x:T,y:T):int{return Compare(x,y);}`,
	`import go { Duration, Second } from "time"; function f():Duration{return Duration(2)*Second;}`,
	`import go { Pi, MaxUint8 } from "math"; function f():byte{return MaxUint8;} function g():float32{return Pi;}`,
	"import go strings from \"strings\"\nfunction f():string {\nreturn strings.TrimSpace\n(\" x \")\n}",
	"function f():void {\nconst call = ():void => { return }\nfor (let i=0; i<2; i++) { call() }\n}",
	"function f():int { return\n1 }",
	`function f():int{return 1;{}}`,
	`function f():int{switch(1){default{break;return 1;}}}`,
	`function pick<T>(a:T,b:T):T{return b;}function f(v:float32):float32{return pick(1.25,v);}`,
	`function keep<T>(v:T):T{return v;}function f():byte{return keep<byte>(256);}`,
	`import go cmp from "cmp";function f():int{return cmp.Compare<float32>(1e40,0);}`,
	`const offset=2.0; function use(values:int[]):int{return values[offset];}`,
	`const huge=1e400; function use():float{return huge/huge;}`,
	`function sizes():int[]{return makeSlice<int>(2+0i,4e0);}`,
	`function invalid(values:[4]int):int{return values[1e40];}`,
	`const first=2.;const second=first;function use(values:int[]):int{return values[second];}`,
	`constraint Slice<E> = ~E[]; function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use(values:int[]):int[]{return elements(values);}`,
	`constraint Lookup<K extends comparable,V> = ~Map<K,V>; class Store<K extends comparable,V,M extends Lookup<K,V>>{constructor(public values:M){}}`,
	`constraint A<E extends B<E>> = ~E[]; constraint B<E extends A<E>> = ~E[];`,
	`constraint Bad<E> = ~E;`,
	`constraint Slice<E>=~E[]; alias Items<E>=E[]; function use(values:Items<int>):int{return len(values);}`,
	`function compare(a:int,b:int,c:uint):boolean{return a<b>>c;}`,
	`class Box<T>{constructor(public value:T){}} function use(value:int):int{const box:Box<Box<int>>=new Box<Box<int>>(new Box<int>(value));return box.value.value;}`,
	`constraint Slice<E> = Storage<E>; constraint Storage<T> = ~T[]; function size<S extends Slice<int>>(values:S):int{return len(values);}`,
	`constraint A<E> = B<E>; constraint B<T> = A<T>;`,
	`constraint A = ~int; constraint B = A | int;`,
	`function bad<T extends U,U extends T>(value:T):T{return value;}`,
	`interface Bound<E> {function read():E;} function use<E,S extends Bound<E>>(value:S):void{}`,
	`constraint Number=~int; class Box<E extends Number>{public function keep<S extends E>(value:S):S{return value;}}`,
	`constraint Values = ~int[]; function sum<T extends Values>(values:T):int {let total=0;for(const value of values){total+=value;}return total;}`,
	`class Leaf { public value:int=1; } constraint Leaves=~Leaf[]; function sum<T extends Leaves>(values:T):int {let total=0;for(const value of values){total+=value.value;}return total;}`,
	`constraint Mixed=~int[]|~[2]int; function use<T extends Mixed>(values:T):void {for(const value of values){}}`,
	`class Leaf {} class Box<T> { public leaf: Leaf = new Leaf(); public items: T[] = []; public callback: (value: int) => int = (value: int): int => { return value + 1; }; }`,
	`class Incomplete { public value: int =`,
	`function add(left: int, right: int): int { return left + right; }`,
	`struct Point { public x: int; public y: int; pointer function move(dx: int, dy: int): void { this.x += dx; this.y += dy; } }`,
	`class Box<T extends comparable> { constructor(public value: T) {} public function get(): T { return this.value; } } function use(): string { return new Box<string>("value").get(); }`,
	`function parse(okay: boolean): Result<int> { if (okay) { return ok(42); } return err(new Exception("no value")); }`,
	`function collect(values: int[]): int { let total = 0; for (const value of values) { total += value; } return total; }`,
	`function count(n: int): int { let total = 0; for (const i of n) { total += i; } return total; }`,
	`constraint Integer = ~int8 | ~int64; function convert<T extends Integer>(value: int): T { return T(value); }`,
	`function values(yield: (value: int) => boolean): void { yield(1); } function sum(): int { let result = 0; for (const value of values) { result += value; } return result; }`,
	`function ticks(yield: () => boolean): void { yield(); } function use(): void { for (const _ of ticks) {} }`,
	`interface Child<T> extends Root<T> {} interface Root<T> { function read(): T; } class Value implements Child<int> { public function read(): int { return 1; } } function read<T>(value: Root<T>): T { return value.read(); } function use(): int { return read(new Value()); }`,
	`import go fmt from "fmt"; interface Named extends fmt.Stringer {} class Value implements Named {public function string():string{return "x";}} function use():fmt.Stringer{return new Value();}`,
	`import go fmt from "fmt"; interface Named extends fmt.Stringer {function string():int;}`,
	`import go io from "io"; interface Reader extends io.Reader {} class Value implements Reader {public function read(buffer:byte[]):Result<int>{return ok(len(buffer));}}`,
	`interface A<T> extends B<T[]> {} interface B<T> extends A<T> {}`,
	`import go strings from "strings"; function upper(value: string): string { return strings.ToUpper(value); }`,
	`function broken<T extends>(value: T): T { return value; }`,
	`class Incomplete { private value: string; constructor(flag: boolean) { if (flag) { this.value = "set"; } } }`,
	`class Switched { private value: string; constructor(values: int[]) { switch (len(values)) { case 0 { this.value = "empty"; } default { for (const item of values) { this.value = "set"; } } } } }`,
	`class NestedGuard { private value: string; constructor(values: int[], enabled: boolean) { if (len(values) > 0) { if (enabled) { for (const item of values) { this.value = "set"; } } else { for (const item of values) { this.value = "disabled"; } } } else { this.value = "empty"; } } }`,
	`for (((`,
	`class Box<T> { constructor(public value: T) {} public function keep<U>(marker: U): T { return this.value; } } function use<U>(box: Box<U>): U { return box.keep<int>(1); }`,
	`struct Cell<T> { public value: T; } function get<T>(cell: Cell<T>): T { return cell.value; } function use(cell: Cell<int>): int { return get(cell); }`,
	`class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) > 0) { const count = len(values); for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`,
	`class Holder { constructor() { const callback = (): int => { return 1; }; const value = callback(); } }`,
	"\xff\x00",
}

func compilePipelineProperty(input string) ([]byte, bool, error) {
	tokens, lexDiagnostics := lexer.Lex("fuzz.km", input)
	program, parseDiagnostics := kinmokuseiParser.Parse(tokens)
	if program == nil {
		return nil, false, fmt.Errorf("parser returned a nil program")
	}
	if len(lexDiagnostics) != 0 || len(parseDiagnostics) != 0 {
		return nil, false, nil
	}
	goImporter := importer.Default()
	if diagnostics := sema.CheckScopedWithGoImporter(program, nil, goImporter); len(diagnostics) != 0 {
		return nil, false, nil
	}
	first, err := codegen.GenerateWithImporter(program, "fuzzpkg", goImporter)
	if err != nil {
		return nil, true, err
	}
	second, err := codegen.GenerateWithImporter(program, "fuzzpkg", goImporter)
	if err != nil {
		return nil, true, err
	}
	if !bytes.Equal(first, second) {
		return nil, true, fmt.Errorf("code generation is not deterministic")
	}
	if _, err := goparser.ParseFile(token.NewFileSet(), "generated.go", first, goparser.AllErrors); err != nil {
		return nil, true, fmt.Errorf("generated Go does not parse: %w", err)
	}
	return first, true, nil
}

func TestCompilePipelineProperties(t *testing.T) {
	generated := 0
	for index, seed := range pipelineFuzzSeeds {
		output, reachedCodegen, err := compilePipelineProperty(seed)
		if err != nil {
			t.Fatalf("seed %d: %v", index, err)
		}
		if reachedCodegen {
			generated++
			if len(output) == 0 {
				t.Fatalf("seed %d reached codegen with empty output", index)
			}
		}
	}
	if generated < 6 {
		t.Fatalf("only %d seeds reached semantic analysis and code generation; want at least 6", generated)
	}
}

func FuzzCompilePipelineNeverPanics(f *testing.F) {
	for _, seed := range pipelineFuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 64<<10 {
			t.Skip()
		}
		if _, _, err := compilePipelineProperty(input); err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
	})
}
