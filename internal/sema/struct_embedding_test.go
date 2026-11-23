package sema

import (
	"strings"
	"testing"
)

func TestNativeStructEmbedding(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"field and methods", `struct Leaf{public n:int;public function get():int{return this.n;}public pointer function bump():void{this.n++;}} struct Outer{Leaf;} function run():int{let o=Outer{Leaf:Leaf{n:1}};o.n+=2;o.bump();const get=o.get;return get();}`, ""},
		{"forward value embedding", `struct Outer{Leaf;} struct Leaf{public n:int;} function run(o:Outer):int{return o.n;}`, ""},
		{"pointer embedding and method set", `struct Leaf{public n:int;public pointer function bump():void{this.n++;}} struct Outer{*Leaf;} function call<T extends interface{bump():void;}>(x:T):void{x.bump();}function run(o:Outer):void{call(o);const f=Outer.bump;f(o);}`, ""},
		{"pointer method set needs pointer for value embedding", `struct Leaf{public pointer function bump():void{}} struct Outer{Leaf;} function call<T extends interface{bump():void;}>(x:T):void{x.bump();} function run(o:Outer):void{call(o);}`, "constraint"},
		{"pointer method on temporary", `struct Leaf{public pointer function bump():void{}} struct Outer{Leaf;} function run():void{Outer{Leaf:Leaf{}}.bump();}`, "requires an addressable"},
		{"generic source alias", `struct Leaf<T>{public n:T;} alias View<T>=Leaf<T>;struct Outer{View<int>;} function run():int{return Outer{View:Leaf<int>{n:1}}.n;}`, ""},
		{"shadow", `struct Leaf{public n:string;} struct Outer{Leaf;public n:int;} function run(o:Outer):int{return o.n;}`, ""},
		{"ambiguous diamond", `struct Leaf{public n:int;} struct Left{Leaf;} struct Right{Leaf;} struct Outer{Left;Right;} function run(o:Outer):int{return o.n;}`, "ambiguous promoted"},
		{"ambiguous method", `struct Left{public function get():int{return 1;}} struct Right{public function get():int{return 2;}} struct Outer{Left;Right;} function run(o:Outer):int{return o.get();}`, "ambiguous promoted"},
		{"recursive pointer", `struct Node{*Node;public n:int;} function run(v:Node):int{return v.n;}`, ""},
		{"recursive pointer negative lookup", `struct Node{*Node;} function run(v:Node):int{return v.missing;}`, "no field"},
		{"recursive value", `struct Left{Right;} struct Right{Left;}`, "recursively contains itself by value"},
		{"non-struct embedding", `struct Outer{int;}`, "embedded field must name a Kinmokusei struct"},
		{"literal must use the embedded name", `struct Leaf{public n:int;} struct Outer{Leaf;} function run():Outer{return Outer{n:1};}`, "has no field"},
		{"emitted name collision", `struct Leaf{} struct Outer{Leaf;public leaf:int;}`, "share the generated Go name"},
		{"emitted method collision", `struct Leaf{} struct Outer{Leaf;public function leaf():void{}}`, "conflicts with embedded field"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
