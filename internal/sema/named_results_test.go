package sema

import (
	"strings"
	"testing"
)

func TestNamedResults(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"multiple results", `function f(n:int):(value:int,err:error){value=n;return;}`, ""},
		{"nullable zero", `class Item{} function f():(item:Item|null){return;}`, ""},
		{"raw Go zero", `function f():(value:*int, err:error){return;}`, ""},
		{"ordinary explicit and blank results", `function f():(_:int,value:int){return 6,7;}`, ""},
		{"generic result", `function f<T extends Number>(x:T):(value:T){value=x;return;}constraint Number=~int|~int16;`, ""},
		{"block arrow", `const f=(x:int):(value:int)=>{value=x;return;};`, ""},
		{"function type annotation", `function f():(x:int)=>int{return (x:int)=>x;} const g=():(x:int)=>int=>(x:int)=>x;`, ""},
		{"shadowed bare return", `function f():(value:int){if(true){const value=2;return;}return;}`, "shadowed named result"},
		{"explicit return under shadow", `function f():(value:int){if(true){const value=2;return value;}return;}`, ""},
		{"duplicate parameter", `function f(value:int):(value:int){return;}`, "duplicate local name"},
		{"duplicate result", `function f():(value:int,value:int){return;}`, "duplicate local name"},
		{"type parameter collision", `function f<T extends Number>():(T:T){return;}constraint Number=~int;`, "conflicts with a type parameter"},
		{"non-null zero", `class Item{} function f():(item:Item){return;}`, "zero value"},
		{"non-null record zero", `class Item{} struct Box{public item:Item;} function f():(box:Box){return;}`, "zero value"},
		{"open generic zero", `function f<T>():(value:T){return;}`, "zero value"},
		{"Result effect", `function f():(value:Result<int>){return ok(0);}`, "ordinary value types"},
		{"fallthrough", `function f():(value:int){value=1;}`, "may complete without returning"},
		{"nested bare return", `function f():(value:int){const g=():int=>{return;};return;}`, "expected return value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
