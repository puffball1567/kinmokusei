package sema

import (
	"strings"
	"testing"
)

func TestMultipleCallArgumentBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"type", `function pair():(int,string){return 1,"x";} function f(a:int,b:int):void{} function use():void{f(pair());}`, "cannot pass result 2"},
		{"variadic type", `function pair():(int,string){return 1,"x";} function f(...a:int[]):void{} function use():void{f(pair());}`, "cannot pass result 2"},
		{"mixed", `function pair():(int,int){return 1,2;} function f(a:int,b:int):void{} function use():void{f(1,pair());}`, "require destructuring"},
		{"spread", `function pair():(int,int){return 1,2;} function f(...a:int[]):void{} function use():void{f(pair()...);}`, "require destructuring"},
		{"Result effect", `function pair():Result<int>{return ok(1);} function f(a:int,b:error):void{} function use():void{f(pair());}`, "Result values must"},
		{"class coercion", `class A{} class B extends A{} function pair():(B,int){return new B(),1;} function f(a:A,n:int):void{} function use():void{f(pair());}`, "cannot pass result 1"},
		{"scalar overflow", `function f(a:byte):void{} function use():void{f(max(256,257));}`, "cannot"},
		{"generic count", `function pair():(int,int){return 1,2;} function f<T>(a:T):T{return a;} function use():void{f(pair());}`, "argument count mismatch"},
		{"generic conflicting types", `function pair():(int,string){return 1,"x";} function f<T>(a:T,b:T):T{return a;} function use():void{f(pair());}`, "cannot"},
		{"generic explicit mismatch", `function pair():(int,int){return 1,2;} function f<T>(a:T,b:T):T{return a;} function use():void{f<string>(pair());}`, "cannot"},
		{"generic constraint", `constraint N=~int; function pair():(string,string){return "a","b";} function f<T extends N>(a:T,b:T):T{return a;} function use():void{f(pair());}`, "does not satisfy"},
		{"generic instance method", `function pair():(int,int){return 1,2;} class C{public function f<T>(a:T,b:T):T{return a;}} function use():void{const c=new C();c.f(pair());}`, "generic instance methods require explicit destructuring"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(checkSource(t, tc.source), "\n"); !strings.Contains(got, tc.want) {
				t.Fatalf("want %q, got %s", tc.want, got)
			}
		})
	}
}
