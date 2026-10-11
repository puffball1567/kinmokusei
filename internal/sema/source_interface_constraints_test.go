package sema

import (
	"strings"
	"testing"
)

func TestSourceInterfaceConstraints(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"struct", `interface Reader{function read():int;}struct S{public function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const n=use(S{});`, ""},
		{"class", `interface Reader{function read():string;}class S{public function read():string{return "ok";}}function use<T extends Reader>(v:T):string{return v.read();}const n=use(new S());`, ""},
		{"pointer", `interface Reader{function read():int;}struct S{public pointer function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}function call(v:*S):int{return use(v);}`, ""},
		{"distinct", `interface Reader{function read():int;}type S=distinct int;public function read(this:S):int{return int(this);}function use<T extends Reader>(v:T):int{return v.read();}const n=use(S(1));`, ""},
		{"interface forwarding", `interface Reader{function read():int;}function use<T extends Reader>(v:T):int{return v.read();}function forward<U extends Reader>(v:U):int{return use(v);}function call(v:Reader):int{return forward(v);}`, ""},
		{"inherited", `interface Base{function read():int;}interface Reader extends Base{}struct S{public function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const n=use(S{});`, ""},
		{"dependent text", `interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}function use<E,T extends Reader<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"method owner specialization", `interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}class Box<E>{public function use<T extends Reader<E>>(v:T):E{return v.read();}}const n:string=new Box<string>().use(S{});`, ""},
		{"method owner specialization rejection", `interface Reader<E>{function read():E;}struct S{public function read():bstring{return b"ok";}}class Box<E>{public function use<T extends Reader<E>>(v:T):E{return v.read();}}const n=new Box<string>().use(S{});`, "does not satisfy"},
		{"method owner substitutions isolated", `interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}struct I{public function read():int{return 1;}}class Box<E>{public function use<T extends Reader<E>>(v:T):E{return v.read();}}const a:string=new Box<string>().use(S{});const b:int=new Box<int>().use(I{});const c:string=new Box<string>().use(S{});`, ""},
		{"source generic constraint parameter", `interface Reader{function read():int;}constraint Slice<E extends Reader>=~E[];struct S{public function read():int{return 1;}}function use<T extends Slice<S>>(v:T):int{return v[0].read();}`, ""},
		{"dependent nullable", `class Item{}interface Reader<E>{function read():E;}struct S{public function read():Item|null{return null;}}function use<E,T extends Reader<E>>(v:T):E{return v.read();}const n:Item|null=use(S{});`, ""},
		{"explicit function value", `interface Reader{function read():int;}struct S{public function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const f=use<S>;const n=f(S{});`, ""},
		{"generic owner forward bound", `class Box<T extends Reader>{constructor(public v:T){}public function read():int{return this.v.read();}}interface Reader{function read():int;}struct S{public function read():int{return 1;}}const b=new Box<S>(S{});`, ""},
		{"generic struct bound", `struct Box<T extends Reader>{public v:T;public function read():int{return this.v.read();}}interface Reader{function read():int;}struct S{public function read():int{return 1;}}const b=Box<S>{v:S{}};`, ""},
		{"early field application", `struct Holder{public v:Box<S>;}struct Box<T extends Reader>{public v:T;}interface Reader{function read():int;}struct S{public function read():int{return 1;}}`, ""},
		{"early field rejection", `struct Holder{public v:Box<S>;}struct Box<T extends Reader>{public v:T;}interface Reader{function read():int;}struct S{}`, "does not satisfy"},
		{"early interface application rejection", `interface Holder{function read():Box<S>;}interface Box<T extends Reader>{function value():T;}interface Reader{function read():int;}struct S{}`, "does not satisfy"},
		{"early class base rejection", `class Holder extends Box<S>{}class Box<T extends Reader>{}interface Reader{function read():int;}class S{}`, "does not satisfy"},
		{"generic defined type", `interface Reader{function read():int;}struct S{public function read():int{return 1;}}type List<T extends Reader>=distinct T[];function use(v:List<S>):int{return v[0].read();}`, ""},
		{"early defined type rejection", `struct Holder{public v:List<S>;}type List<T extends Reader>=distinct T[];interface Reader{function read():int;}type S=distinct int;`, "does not satisfy"},
		{"alias", `interface Reader{function read():int;}alias Bound=Reader;function use<T extends Bound>(v:T):int{return v.read();}`, ""},
		{"forward owner alias", `struct Box<T extends Bound>{public v:T;public function read():int{return this.v.read();}}alias Bound=Reader;interface Reader{function read():int;}struct S{public function read():int{return 1;}}const b=Box<S>{v:S{}};`, ""},
		{"generic forward owner alias", `struct Box<T extends Bound<string>>{public v:T;public function read():string{return this.v.read();}}alias Bound<E>=Reader<E>;interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}const b=Box<S>{v:S{}};`, ""},
		{"generic source constraint parameter rejection", `interface Reader{function read():int;}constraint Slice<E extends Reader>=~E[];type S=distinct int;function use<T extends Slice<S>>(v:T):void{}`, "does not satisfy"},
		{"multiple results", `interface Reader{function read():(int,bstring);}struct S{public function read():(int,bstring){return 1,b"x";}}function use<T extends Reader>(v:T):(int,bstring){return v.read();}function call():(int,bstring){return use(S{});}`, ""},
		{"dependent method conflict", `interface Reader<E>{function read():E;function write(v:E):void;}struct S{public function read():string{return "ok";}public function write(v:bstring):void{}}function use<E,T extends Reader<E>>(v:T):E{return v.read();}const n=use(S{});`, "cannot infer"},
		{"invariant nullable argument", `class Item{}interface Reader{function write(v:Item|null):void;}struct S{public function write(v:Item):void{}}function use<T extends Reader>(v:T):T{return v;}const n=use(S{});`, "does not satisfy"},
		{"composition remains diagnosed", `interface Reader{function read():int;}constraint Combined=Reader&comparable;`, "constraint"},
		{"generated method collision", `interface Reader{function read():int;function Read():int;}function use<T extends Reader>(v:T):int{return v.read();}`, "conflicting Go method name"},
		{"generated accessor collision", `interface Reader{get value():int;function getValue():int;}function use<T extends Reader>(v:T):int{return v.value;}`, "conflicting Go method name"},
		{"recursive source bound", `interface Equal<E>{function same(v:E):boolean;}function use<T extends Equal<T>>(v:T):boolean{return v.same(v);}`, ""},
		{"comparable inferred owner", `interface Reader{function read():int;}type Values<T extends Reader>=distinct Map<T,int>;struct S{public function read():int{return 1;}}function use(v:Values<S>):int{return len(v);}`, ""},
		{"inferred comparable enforced", `interface Reader{function read():int;}type Values<T extends Reader>=distinct Map<T,int>;struct S{public xs:int[];public function read():int{return 1;}}function use(v:Values<S>):void{}`, "does not satisfy"},
		{"Go raw method", `import go fmt from "fmt";interface Reader{function String():bstring;}function use<T extends Reader>(v:T):bstring{return v.String();}function call(v:fmt.Stringer):bstring{return use(v);}`, ""},
		{"property", `interface Counter{get value():int;set value(v:int):void;}class S{public get value():int{return 1;}public set value(v:int):void{}}function use<T extends Counter>(v:T):int{v.value=2;return v.value;}const n=use(new S());`, ""},
		{"Result", `interface Reader{function read():Result<int>;}struct S{public function read():Result<int>{return ok(1);}}function use<T extends Reader>(v:T):Result<int>{return v.read();}function call():Result<int>{return use(S{});}`, ""},
		{"variadic", `interface Reader{function read(...v:int[]):int;}struct S{public function read(...v:int[]):int{return v[0];}}function use<T extends Reader>(v:T):int{return v.read(1,2);}const n=use(S{});`, ""},
		{"missing", `interface Reader{function read():int;}struct S{}function use<T extends Reader>(v:T):int{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"private", `interface Reader{function read():int;}class S{private function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const n=use(new S());`, "does not satisfy"},
		{"static", `interface Reader{function read():int;}class S{public static function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const n=use(new S());`, "does not satisfy"},
		{"generic method", `interface Reader{function read():int;}struct S{public function read<U>():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"pointer on value", `interface Reader{function read():int;}struct S{public pointer function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"raw text mismatch", `interface Reader{function read():string;}struct S{public function read():bstring{return b"ok";}}function use<T extends Reader>(v:T):string{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"Go cannot promise verified text", `import go fmt from "fmt";interface Reader{function String():string;}function use<T extends Reader>(v:T):string{return v.String();}function call(v:fmt.Stringer):string{return use(v);}`, "does not satisfy"},
		{"nullable receiver", `interface Reader{function read():int;}class S{public function read():int{return 1;}}function use<T extends Reader>(v:T):int{return v.read();}function call(v:S|null):int{return use(v);}`, "does not satisfy"},
		{"nullable return mismatch", `class Item{}interface Reader{function read():Item;}struct S{public function read():Item|null{return null;}}function use<T extends Reader>(v:T):Item{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"Go spelling is not source spelling", `interface Reader{function read():int;}function use<T extends Reader>(v:T):int{return v.Read();}`, "has no member"},
		{"Result cannot be ignored", `interface Reader{function read():Result<int>;}function use<T extends Reader>(v:T):void{v.read();}`, "Result"},
		{"readonly property", `interface Counter{get value():int;}function use<T extends Counter>(v:T):void{v.value=2;}`, "no setter"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
