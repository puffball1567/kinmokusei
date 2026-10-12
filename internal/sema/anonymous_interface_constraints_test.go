package sema

import (
	"strings"
	"testing"
)

func TestAnonymousInterfaceConstraints(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"struct", `struct S{public function read():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}const n=use(S{});`, ""},
		{"class verified text", `class S{public function read():string{return "ok";}}function use<T extends interface{read():string;}>(v:T):string{return v.read();}const n=use(new S());`, ""},
		{"dependent text inference", `struct S{public function read():string{return "ok";}}function use<E,T extends interface{read():E;}>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"dependent nullable inference", `class Item{}struct S{public function read():Item|null{return null;}}function use<E,T extends interface{read():E;}>(v:T):E{return v.read();}const n:Item|null=use(S{});`, ""},
		{"inference conflict", `struct S{public function read():string{return "ok";}public function write(v:bstring):void{}}function use<E,T extends interface{read():E;write(v:E):void;}>(v:T):E{return v.read();}const n=use(S{});`, "cannot infer"},
		{"pointer", `struct S{public pointer function read():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}function f(v:*S):int{return use(v);}`, ""},
		{"pointer on value", `struct S{public pointer function read():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"distinct", `type S=distinct int;public function read(this:S):int{return int(this);}function use<T extends interface{read():int;}>(v:T):int{return v.read();}const n=use(S(1));`, ""},
		{"named interface forwarding", `interface Reader{function read():string;}function use<T extends interface{read():string;}>(v:T):string{return v.read();}function f(v:Reader):string{return use(v);}`, ""},
		{"anonymous value forwarding", `function use<T extends interface{read():string;}>(v:T):string{return v.read();}function f(v:interface{read():string;}):string{return use(v);}`, ""},
		{"generic forwarding", `function use<T extends interface{read():string;}>(v:T):string{return v.read();}function f<U extends interface{read():string;}>(v:U):string{return use(v);}`, ""},
		{"method owner specialization", `struct S{public function read():string{return "ok";}}class Box<E>{public function use<T extends interface{read():E;}>(v:T):E{return v.read();}}const n:string=new Box<string>().use(S{});`, ""},
		{"Result", `struct S{public function read():Result<string>{return ok("ok");}}function use<T extends interface{read():Result<string>;}>(v:T):Result<string>{return v.read();}function f():Result<string>{return use(S{});}`, ""},
		{"Result checked", `function use<T extends interface{read():Result<int>;}>(v:T):void{v.read();}`, "Result"},
		{"multiple results", `struct S{public function read():(int,string){return 1,"ok";}}function use<T extends interface{read():(int,string);}>(v:T):(int,string){return v.read();}function f():(int,string){return use(S{});}`, ""},
		{"variadic", `struct S{public function read(...v:string[]):int{return len(v);}}function use<T extends interface{read(...v:string[]):int;}>(v:T):int{return v.read("a","b");}const n=use(S{});`, ""},
		{"function value", `struct S{public function read():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}const f=use<S>;const n=f(S{});`, ""},
		{"alias", `alias Reader=interface{read():string;};struct S{public function read():string{return "ok";}}function use<T extends Reader>(v:T):string{return v.read();}const n=use(S{});`, ""},
		{"generic alias", `alias Reader<E>=interface{read():E;};struct S{public function read():string{return "ok";}}function use<E,T extends Reader<E>>(v:T):E{return v.read();}const n:string=use(S{});`, ""},
		{"generic struct bound", `struct Box<T extends interface{read():string;}>{public v:T;public function read():string{return this.v.read();}}struct S{public function read():string{return "ok";}}const n=Box<S>{v:S{}}.read();`, ""},
		{"early bound rejection", `struct Holder{public v:Box<S>;}struct Box<T extends interface{read():int;}>{public v:T;}struct S{}`, "does not satisfy"},
		{"defined type bound", `type List<T extends interface{read():int;}>=distinct T[];struct S{public function read():int{return 1;}}function use(v:List<S>):int{return v[0].read();}`, ""},
		{"recursive method bound", `function use<T extends interface{same(v:T):boolean;}>(v:T):boolean{return v.same(v);}`, ""},
		{"Go raw compatible", `import go fmt from "fmt";function use<T extends interface{String():bstring;}>(v:T):bstring{return v.String();}function f(v:fmt.Stringer):bstring{return use(v);}`, ""},
		{"Go verified text rejected", `import go fmt from "fmt";function use<T extends interface{String():string;}>(v:T):string{return v.String();}function f(v:fmt.Stringer):string{return use(v);}`, "does not satisfy"},
		{"raw text rejected", `struct S{public function read():bstring{return b"ok";}}function use<T extends interface{read():string;}>(v:T):string{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"nullable result rejected", `class Item{}struct S{public function read():Item|null{return null;}}function use<T extends interface{read():Item;}>(v:T):Item{return v.read();}const n=use(S{});`, "does not satisfy"},
		{"nullable argument rejected", `class Item{}struct S{public function write(v:Item):void{}}function use<T extends interface{write(v:Item|null):void;}>(v:T):T{return v;}const n=use(S{});`, "does not satisfy"},
		{"nullable receiver", `class S{public function read():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}function f(v:S|null):int{return use(v);}`, "does not satisfy"},
		{"private", `class S{private function Read():int{return 1;}}function use<T extends interface{Read():int;}>(v:T):int{return v.Read();}const n=use(new S());`, "does not satisfy"},
		{"static", `class S{public static function read():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}const n=use(new S());`, "does not satisfy"},
		{"generic method", `class S{public function read<U>():int{return 1;}}function use<T extends interface{read():int;}>(v:T):int{return v.read();}const n=use(new S());`, "does not satisfy"},
		{"method spelling", `function use<T extends interface{read():int;}>(v:T):int{return v.Read();}`, "has no member"},
		{"duplicate name", `function use(v:interface{read():int;read():int;}):void{}`, "conflicting Go method name"},
		{"lowered name collision", `function use(v:interface{read():int;Read():int;}):void{}`, "conflicting Go method name"},
		{"invalid parameter storage", `function use(v:interface{read(v:void):int;}):void{}`, "cannot be represented"},
		{"runtime generic method rejected", `class S{public function read<T>():int{return 1;}}function use(v:interface{read():int;}):int{return v.read();}const n=use(new S());`, "cannot use"},
		{"runtime private rejected", `class S{private function Read():int{return 1;}}function use(v:interface{Read():int;}):int{return v.Read();}const n=use(new S());`, "cannot use"},
		{"runtime struct", `struct S{public function read():string{return "ok";}}function use(v:interface{read():string;}):string{return v.read();}const n=use(S{});`, ""},
		{"runtime source interface", `interface Reader{function read():string;}function use(v:interface{read():string;}):string{return v.read();}function f(v:Reader):string{return use(v);}`, ""},
		{"runtime anonymous forwarding", `function use(v:interface{read():string;}):string{return v.read();}function f(v:interface{read():string;other():void;}):string{return use(v);}`, ""},
		{"runtime pointer", `struct S{public pointer function read():int{return 1;}}function use(v:interface{read():int;}):int{return v.read();}function f(v:*S):int{return use(v);}`, ""},
		{"runtime pointer on value", `struct S{public pointer function read():int{return 1;}}function use(v:interface{read():int;}):int{return v.read();}const n=use(S{});`, "cannot use"},
		{"runtime raw Go compatible", `import go fmt from "fmt";function use(v:interface{String():bstring;}):bstring{return v.String();}function f(v:fmt.Stringer):bstring{return use(v);}`, ""},
		{"runtime raw Go cannot promise text", `import go fmt from "fmt";function use(v:interface{String():string;}):string{return v.String();}function f(v:fmt.Stringer):string{return use(v);}`, "cannot use"},
		{"runtime bound cannot erase raw text", `function use<T extends interface{read():bstring;}>(v:T):interface{read():string;}{return v;}`, "cannot use"},
		{"runtime source interface cannot erase raw text", `interface Reader{function read():bstring;}function use(v:Reader):interface{read():string;}{return v;}`, "cannot use"},
		{"runtime bound cannot erase nullability", `class Item{}function use<T extends interface{read():Item|null;}>(v:T):interface{read():Item;}{return v;}`, "cannot use"},
		{"checked interface conversion", `alias View=interface{read():string;};function use(v:View):View{return View(v);}`, ""},
		{"Go raw interface conversion", `import go fmt from "fmt";alias View=interface{String():bstring;};function use(v:fmt.Stringer):View{return View(v);}`, ""},
		{"Go conversion cannot promise text", `import go fmt from "fmt";alias View=interface{String():string;};function use(v:fmt.Stringer):View{return View(v);}`, "cannot convert"},
		{"source conversion cannot promise text", `alias View=interface{read():string;};function use(v:interface{read():bstring;}):View{return View(v);}`, "cannot convert"},
		{"source conversion cannot erase nullability", `class Item{}alias View=interface{read():Item;};function use(v:interface{read():Item|null;}):View{return View(v);}`, "cannot convert"},
		{"container conversion cannot promise text", `alias Views=interface{read():string;}[];function use(v:interface{read():bstring;}[]):Views{return Views(v);}`, "cannot convert"},
		{"nested method text contract", `struct S{public function read():interface{text():bstring;}{return nil;}}function use<T extends interface{read():interface{text():string;};}>(v:T):boolean{return true;}const n=use(S{});`, "does not satisfy"},
		{"nested method nullable contract", `class Item{}struct S{public function read():interface{item():Item|null;}{return nil;}}function use<T extends interface{read():interface{item():Item;};}>(v:T):boolean{return true;}const n=use(S{});`, "does not satisfy"},
		{"nested Go raw contract", `import go fmt from "fmt";struct S{public function read():fmt.Stringer{return nil;}}function use<T extends interface{read():interface{String():bstring;};}>(v:T):boolean{return true;}const n=use(S{});`, ""},
		{"nested Go cannot promise text", `import go fmt from "fmt";struct S{public function read():fmt.Stringer{return nil;}}function use<T extends interface{read():interface{String():string;};}>(v:T):boolean{return true;}const n=use(S{});`, "does not satisfy"},
		{"composition remains diagnosed", `constraint Bad=interface{read():string;}&comparable;`, "use a direct interface bound"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
