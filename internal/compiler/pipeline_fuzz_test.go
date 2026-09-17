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
	`constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):E{return v[0];}`,
	`constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):T{return v[0:1:2];}`,
	`constraint A<E>=~*[2]E;function f<E,T extends A<E>>(v:T):E[]{return v[:];}`,
	`constraint M=~Map<byte,int>;function f<T extends M>(v:T):int{v[256]=1;const [value,ok]=v[1];return value;}`,
	`class C{public value:int=1;}alias Maybe=C|null;constraint S=~Maybe[];function f<T extends S>(v:T):int{return v[:][0].value;}`,
	`constraint S=~byte[]|~string;function f<T extends S>(v:T):T{return v[:];}`,
	`const load=():Result<int>=>{return ok(1);};const main=():void=>{const [value,err]=load();if(err!==nil){return;}};`,
	`function load():Result<int>{return ok(1);}const first=()=>second();const second=()=>{const [value,err]=load();return value;};`,
	`constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):[2]E{return copyArray[[2]E](v);}`,
	`constraint S=~int[]|~string[];function f<T extends S>(v:T):*[2]int{return viewArray[[2]int](v);}`,
	`class C{}alias Maybe=C|null;function pair<E>(v:E[]):*[1]E{return viewArray[[1]E](v);}function f(v:Maybe[]):C{return pair(v)[0];}`,
	`class C{public value:int=1;}function pair<E>(v:E[]):[1]E{return copyArray[[1]E](v);}function f(v:C[]):int{const a=pair(v);return a[:][0].value;}`,
	`type Pair<E>=distinct [2]E;constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):Pair<E>{return copyArray[Pair<E>](v);}`,
	`class C{}function f(v:C[]):void{const a=viewArray[[2]C](v);const n=cap(a);const p=&n;}`,
	`constraint S<E>=~E[];function f<E,T extends S<E>>(v:T,e:E):T{return append(v,e);}`,
	`constraint S=~byte[];constraint Text=~string;function f<T extends S,U extends Text>(v:T,s:U):T{copy(v,s);return append(v,s...);}`,
	`constraint S=~int[]|~string[];function f<T extends S>(v:T):T{return append(v);}`,
	`class Base{}class Child extends Base{}constraint S=~Base[];function f<T extends S>(v:T):T{return append(v,new Child());}`,
	`class Base{}alias Maybe=Base|null;type A<E>=distinct E[];type B<E>=distinct E[];constraint S=A<Maybe>|B<Base>;function f<T extends S>(v:T):T{return append(v,null);}`,
	`type Items=distinct int[];class Box<E>{constructor(public value:E){}}function f(v:Box<int[]>[],s:Box<Items>[]):int{return copy(v,s);}`,
	`alias Maybe=int[]|null;function f(v:Maybe[],s:int[][]):int{return copy(v,s);}`,
	`constraint C=~int[]|~Map<string,int>;function f<T extends C>(v:T):void{clear(v);}`,
	`constraint C=~int[]|~int;function f<T extends C>(v:T):void{clear(v);}`,
	`constraint C<K extends comparable>=~Map<K,int>|~Map<K,string>;function f<K extends comparable,T extends C<K>>(v:T,k:K):void{delete(v,k);}`,
	`constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,256);}`,
	`constraint C=~Map<string,int>|~Map<int,int>;function f<T extends C>(v:T):void{delete(v,"x");}`,
	`class Key{}constraint C=~Map<Key|null,int>|~Map<Key|null,string>;function f<T extends C>(v:T):void{delete(v,null);}`,
	`class Key{}constraint C=~Map<Key,int>|~Map<Key|null,string>;function f<T extends C>(v:T,k:Key):void{delete(v,k);}`,
	`const a=min(255,256);const b=a;function run():byte{return b;}`,
	`const a=max(1e400,2e400);function run():float{return a/1e400;}`,
	`type Text=distinct string;function run(s:Text):Text{return min("a",s);}`,
	`constraint Ordered=~int|~string;function run<T extends Ordered>(a:T,b:T):T{return max(a,b);}`,
	`function run(n:int,v:byte):byte{return min(+(1<<n)+2,v);}`,
	`function run(n:int,v:byte):byte{return max(300<<n,v);}`,
	`function run():void{const n=max(1,2);const p=&n;}`,
	`function run(a:[3]int):int{const n=len(a);return a[n];}`,
	`function run(a:*[3]int):void{const n=cap(a);const p=&n;}`,
	`constraint A=~[3]int|~int[]|~string;function run<T extends A>(a:T):int{return len(a);}`,
	`constraint A=~[3]int|~string;function run<T extends A>(a:T):int{return cap(a);}`,
	`function run(c:GoChannel<[3]int>):int{return len(<-c);}`,
	`function run(a:int[]):int{const n=len(copyArray[[3]int](a));return n;}`,
	`type Text=distinct string;const a="温";const b=a+"泉";const n=len(b);function run():Text{return b;}`,
	`type Flag=distinct boolean;const a:Flag=true;const b=!a;function run():Flag{return true&&b;}`,
	`type Text=distinct string;function choose<T>(a:T,b:T):T{return a;}function run(v:Text):Text{const a="x";return choose(a,v);}`,
	`const a="ab";const b=a;function run():string{return b[:3];}`,
	`const a="abc";const n=len(a);function run():byte{return n;}`,
	`function run():void{const a=true;const b=a;const pointer=&b;}`,
	`abstract class Base<T>{public abstract function read():T;}class Leaf extends Base<int>{public override function read():int{return 1;}}function run():int{const base:Base<int>=new Leaf();return base.read();}`,
	`abstract class Base{public abstract function read():int;}class Leaf extends Base{}`,
	`abstract final class Base{private static abstract function read():int{return 1;}}`,
	`struct Holder<E>{value:E;}constraint Box<E>=comparable&Holder<E>;function use<A extends Box<A>>():void{}`,
	`const a=1e400;const b=a;const c=b+b;function run():float{return c/a;}`,
	`const a=254;const b=a+1;function run():byte{return b;}`,
	`const a:int8=120;const b=a+8;`,
	`function run(xs:int[]):int{for(const n=2.;false;){const m=n;return xs[m];}return 0;}`,
	`function run():void{const a=1;const b=a;const pointer=&b;}`,
	`function load():Result<int>{return ok(1);}function run():void{const _=load();_=load();const [_,err]=load();_=err;}`,
	`function load():Result<int>{return ok(1);}function run():void{const [value,err]=load();}`,
	`function run():Result<void>{const load=():Result<int>=>{return ok(1);};const _=load()?;return ok();}`,
	`function run():void{const _=()=>1;const _=()=>2;for(const _:error=nil;false;){} }`,
	`function run():void{const _=null;_=nil;}`,
	`import go cmp from "cmp"; constraint A=cmp.Ordered&~int; function twice<T extends A>(x:T):T{return x*2;} function use():int{return twice(21);}`,
	`constraint Key=comparable; constraint A=~int|~int[]; constraint B=Key&A; function twice<T extends B>(x:T):T{return x*2;}`,
	`constraint A=comparable&~int; constraint Bad=A|~string;`,
	`constraint Pair<E>=comparable&~[2]E; function keep<T extends Pair<E>,E>(x:T):T{return x;} function bad(x:[2]int):[2]int{return keep(x);}`,
	`class Leaf{} interface Reader{function read():Leaf;} class Bad implements Reader{public function read():Leaf|null{return null;}}`,
	`class Leaf{} interface Reader<T>{function read():T;} interface A extends Reader<Leaf>{} interface B extends Reader<Leaf|null>{} interface Bad extends A,B{}`,
	`class Leaf{} alias Maybe=Leaf|null; interface Reader{function read(callback:(value:Maybe)=>void):Maybe[];} class Good implements Reader{public function read(callback:(value:Maybe)=>void):Maybe[]{callback(null);return [null];}}`,
	`class Leaf{} interface Reader{function read():Result<Leaf>;} class Bad implements Reader{public function read():Result<Leaf|null>{return ok(null);}}`,
	`const value=1;export {value as publicName,value as another};`,
	`export {value as renamed} from "./library";`,
	`export {value as};`,
	`export { value } from "./library";`,
	`export {} from "./library";`,
	`export { value } from ""; const value=1;`,
	`function run():Result<int>{const f=():Result<int>=>{return ok(42);};return f();}`,
	`function run():Result<int>{const f=(n:int):Result<int>=>{if(n==0){return ok(1);}const value=f(n-1)?;return ok(n*value);};return f(5);}`,
	`type Load<T>=distinct ()=>Result<T>;function run():Result<int>{const f:Load<int>=()=>{return ok(42);};return f();}`,
	`function run():Result<void>{const f=():Result<void>=>{return ok();};f()?;return ok();}`,
	`function bad():void{const f=():Result<int>=>{return ok(1);};f();}`,
	`function run():void{let i=0;goto again;again:for(const f=():int=>f();i<3;i++){if(i==0){i++;goto again;}if(i==1){continue again;}break again;}}`,
	`function run():int{for(const f=(n:int):int=>{if(n==0){return 1;}return f(n-1);}; ;){return f(2);}return 0;}`,
	`function run():void{let i=0;outer:for(let f=():int=>f();i<2;i++){continue outer;}}`,
	`function run():void{for(const f=()=>f();false;){}}`,
	`function run():int{const first=()=>last();const last=()=>42;return first();}`,
	`function run():int{const value=42;const first=(value:string)=>last();const last=()=>value;return first("shadow");}`,
	`function run():int{const first=()=>{last=()=>7;return last();};let last=()=>1;return first();}`,
	`function run<T>(value:T):T{const first=()=>last();const last=()=>value;return first();}`,
	`function run():int{const first=()=>last();const last=()=>first();return first();}`,
	`function run():int{const first=():int=>last();const last=():int=>42;return first();}`,
	`function run():int{const first=():int=>last();let last=():int=>1;last=():int=>7;return first();}`,
	`function run():int{const first=():int=>last();first();const last=():int=>42;return last();}`,
	`function run():int{const first=()=>last();const last=()=>first();return first();}`,
	`function run():boolean{const even=(n:int):boolean=>{if(n==0){return true;}return odd(n-1);};const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};return even(8);}`,
	`const first=()=>last();const last=()=>42;`,
	`const first=(value:string)=>last();const last=()=>value;const value=42;`,
	`const first=()=>last();const last=()=>first();`,
	`const value=read();const read=()=>value;`,
	`const even=(n:int)=>{if(n==0){return true;}return odd(n-1);};const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};`,
	`function apply<T,U>(v:T,f:(x:T)=>U):U{return f(v);} const run=()=>apply(21,(x)=>x*2);`,
	`function chain<T,U,V>(v:T,g:(u:U)=>V,f:(t:T)=>U):V{return g(f(v));} const run=()=>chain(1,(s)=>len(s),(n)=>"yes");`,
	`import go slices from "slices"; const run=()=>slices.IndexFunc([1,2],(x)=>x==2);`,
	`function apply<T>(f:(x:T)=>T):void{} const run=()=>apply((x)=>x);`,
	`function pick<T>(v:T,f:()=>T):T{return f();} const run=()=>pick(1,()=>1.5);`,
	`class Box<T>{constructor(public value:T){}} function run():Box<int>{const f=(n:int):Box<int>=>{if(n<=1){return new Box<int>(n);}return f(n-1);};return f(5);}`,
	`function run():int{const f=(n:int):int=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}`,
	`function run():void{let f=():void=>{f=():void=>{};};f();}`,
	`function run():void{const f=()=>f();}`,
	`function run():void{for(const f=():int=>f();false;){}}`,
	`const main=()=>{};`,
	`const f=(n:int):int=>{if(n==0){return 1;}return n*f(n-1);};`,
	`const f=()=>f();`,
	`const f:(n:int)=>int=(n)=>{return n+1;};`,
	`const f:(...values:int[])=>int=(...values)=>{return len(values);};`,
	`const f=()=>{try{return 1;}finally{const nested=()=>{return "x";};nested();}};`,
	`const f=(b:boolean)=>{if(b){return;}return 1;};`,
	`import go http from "net/http"; const handler:http.HandlerFunc=(w,r)=>{};`,
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
	`constraint A = ~int | ~string; constraint B = A & ~int; function twice<T extends B>(x:T):T{return x*2;}`,
	`constraint A<E>=~E[]; constraint B<E>=A<E>&~E[];`,
	`constraint A=B&int; constraint B=A&int;`,
	`constraint Bad=int&string;`,
	`import go fmt from "fmt"; constraint A=~int&fmt.Stringer; constraint B=A&~int; function show<T extends B>(value:T):string{return value.String();}`,
	`import go io from "io"; constraint A=io.Reader&io.Closer; constraint B=A; function close<T extends B>(value:T):error{return value.Close();}`,
	`import go fmt from "fmt"; constraint A=fmt.Stringer; constraint B=A|int;`,
	`constraint A<E>=~E[]|int; constraint B<E>=A<E>&int;`,
	`constraint Bad=int&string|bool;`,
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
