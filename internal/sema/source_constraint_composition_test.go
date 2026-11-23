package sema

import (
	"strings"
	"testing"
)

func TestSourceConstraintComposition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"forward", `constraint Readable=Reader&comparable;interface Reader{function read():int;}struct S{public n:int;public function read():int{return this.n;}}function use<T extends Readable>(v:T):int{return v.read();}const n=use(S{n:7});`, ""},
		{"class", `interface Reader{function read():string;}constraint Readable=Reader&comparable;class S{public function read():string{return "ok";}}function use<T extends Readable>(v:T):string{return v.read();}const n=use(new S());`, ""},
		{"noncomparable", `interface Reader{function read():int;}constraint Readable=Reader&comparable;struct S{public xs:int[];public function read():int{return 1;}}function use<T extends Readable>(v:T):int{return v.read();}const n=use(S{xs:[]});`, "does not satisfy"},
		{"exact struct", `constraint Readable=Reader&Row;interface Reader{function read():int;}struct Row{public n:int;public function read():int{return this.n;}}function use<T extends Readable>(v:T):int{return v.read();}const n=use(Row{n:7});`, ""},
		{"underlying distinct", `constraint Readable=Reader&~int;interface Reader{function read():int;}type S=distinct int;public function read(this:S):int{return int(this);}function use<T extends Readable>(v:T):int{return v.read()+int(v);}const n=use(S(7));`, ""},
		{"dependent text", `constraint Readable<E>=Reader<E>&comparable;interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}function use<E,T extends Readable<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"dependent nullable", `class Item{}constraint Readable<E>=Reader<E>&comparable;interface Reader<E>{function read():E;}struct S{public function read():Item|null{return null;}}function use<E,T extends Readable<E>>(v:T):E{return v.read();}const n:Item|null=use(S{});`, ""},
		{"dependent nominal", `class Item{}constraint Readable<E>=Reader<E>&comparable;interface Reader<E>{function read():E;}struct S{public function read():Item{return new Item();}}function use<E,T extends Readable<E>>(v:T):E{return v.read();}const n:Item=use(S{});`, ""},
		{"constraint reference", `constraint A<E>=Reader<E>&comparable;constraint B<E>=A<E>&A<E>;interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}function use<E,T extends B<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"generic alias", `alias Bound<E>=Reader<E>;constraint A<E>=Bound<E>&comparable;interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}function use<E,T extends A<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"inherited", `constraint Readable=Reader&comparable;interface Reader extends Base{}interface Base{function read():int;}function use<T extends Readable>(v:T):int{return v.read();}`, ""},
		{"anonymous", `constraint Readable<E>=interface{read():E;}&comparable;struct S{public function read():string{return "ok";}}function use<E,T extends Readable<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"anonymous alias", `alias Reader<E>=interface{read():E;};constraint Readable<E>=Reader<E>&comparable;struct S{public function read():string{return "ok";}}function use<E,T extends Readable<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"source and Go", `import go fmt from "fmt";interface Reader{function read():int;}constraint Readable=Reader&fmt.Stringer;struct S{public function read():int{return 1;}public function String():bstring{return b"s";}}function use<T extends Readable>(v:T):bstring{_=v.read();return v.String();}const n=use(S{});`, ""},
		{"same Go method alias", `import go fmt from "fmt";interface Reader{function string():bstring;}constraint Readable=Reader&fmt.Stringer;function use<T extends Readable>(v:T):bstring{return v.string()+v.String();}`, ""},
		{"Go raw mismatch", `import go fmt from "fmt";interface Reader{function String():string;}constraint Readable=Reader&fmt.Stringer;`, "incompatible"},
		{"nullable intersection mismatch", `class Item{}interface A{function read():Item;}interface B{function read():Item|null;}constraint C=A&B;`, "incompatible source signatures"},
		{"Result intersection mismatch", `interface A{function read():Result<int>;}interface B{function read():(int,error);}constraint C=A&B;`, "incompatible source signatures"},
		{"nested Result mismatch", `interface A{function read(fn:()=>Result<int>):void;}interface B{function read(fn:()=>(int,error)):void;}constraint C=A&B;`, "incompatible source signatures"},
		{"source properties", `interface Read{get value():int;}interface Write{set value(v:int):void;}constraint Counter=Read&Write;class S{public get value():int{return 1;}public set value(v:int):void{}}function use<T extends Counter>(v:T):int{v.value=2;return v.value;}const n=use(new S());`, ""},
		{"property mismatch", `interface Read{get value():int;}interface Write{set value(v:string):void;}constraint Counter=Read&Write;`, "getter and setter"},
		{"property namespace", `interface Read{get value():int;}interface Method{function value():int;}constraint Counter=Read&Method;`, "conflicts with a method"},
		{"Result", `interface Reader{function read():Result<int>;}constraint Readable=Reader&comparable;struct S{public function read():Result<int>{return ok(1);}}function use<T extends Readable>(v:T):Result<int>{return v.read();}function f():Result<int>{return use(S{});}`, ""},
		{"Result discard", `interface Reader{function read():Result<int>;}constraint Readable=Reader&comparable;function use<T extends Readable>(v:T):void{v.read();}`, "Result"},
		{"multiple results", `interface Reader{function read():(int,bstring);}constraint Readable=Reader&comparable;function use<T extends Readable>(v:T):(int,bstring){return v.read();}`, ""},
		{"variadic", `interface Reader{function read(...v:int[]):int;}constraint Readable=Reader&comparable;struct S{public function read(...v:int[]):int{return v[0];}}function use<T extends Readable>(v:T):int{return v.read(1,2);}const n=use(S{});`, ""},
		{"owner", `constraint Readable<E>=Reader<E>&comparable;interface Reader<E>{function read():E;}struct S{public function read():string{return "ok";}}struct Box<E>{public function use<T extends Readable<E>>(v:T):E{return v.read();}}const n=Box<string>{}.use(S{});`, ""},
		{"owner mismatch", `constraint Readable<E>=Reader<E>&comparable;interface Reader<E>{function read():E;}struct S{public function read():bstring{return b"ok";}}struct Box<E>{public function use<T extends Readable<E>>(v:T):E{return v.read();}}const n=Box<string>{}.use(S{});`, "does not satisfy"},
		{"early application", `struct Holder{public value:Box<S>;}struct Box<T extends Readable>{public value:T;}constraint Readable=Reader&comparable;interface Reader{function read():int;}struct S{public function read():int{return 1;}}`, ""},
		{"early application rejected", `struct Holder{public value:Box<S>;}struct Box<T extends Readable>{public value:T;}constraint Readable=Reader&comparable;interface Reader{function read():int;}struct S{}`, "does not satisfy"},
		{"recursive method", `interface Equal<E>{function same(v:E):boolean;}constraint C<E>=Equal<E>&comparable;function use<T extends C<T>>(v:T):boolean{return v.same(v);}`, ""},
		{"constraint cycle", `interface Reader{function read():int;}constraint A=Reader&B;constraint B=A;`, "constraint declaration cycle"},
		{"union rejected", `interface Reader{function read():int;}constraint C=Reader|int;`, "use '&'"},
		{"underlying rejected", `interface Reader{function read():int;}constraint C=~Reader;`, "not interfaces"},
		{"nullable implementation", `interface Reader{function read():int;}constraint C=Reader&comparable;class S{public function read():int{return 1;}}function use<T extends C>(v:T):int{return v.read();}function f(v:S|null):int{return use(v);}`, "does not satisfy"},
		{"source spelling", `interface Reader{function read():int;}constraint C=Reader&comparable;function use<T extends C>(v:T):int{return v.Read();}`, "has no member"},
		{"pointer method rejected on value", `interface Reader{function read():int;}constraint C=Reader&comparable;struct S{public pointer function read():int{return 1;}}function use<T extends C>(v:T):int{return v.read();}const n=use(S{});`, "does not satisfy"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
