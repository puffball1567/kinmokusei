package sema

import (
	"strings"
	"testing"
)

func TestNumericConstantContexts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"slice index", `function f(a:int[]):int{return a[2.0];}`, ""},
		{"array index", `function f(a:[4]int):int{return a[0x1p1];}`, ""},
		{"pointer index", `function f(a:*[4]int):int{return a[2e0];}`, ""},
		{"string index", `function f():byte{return "日本"[3.0];}`, ""},
		{"complex zero component", `function f(a:int[]):int{return a[2+0i];}`, ""},
		{"constant index", `const n=2.0;function f(a:int[]):int{return a[n];}`, ""},
		{"slice bounds", `const n=3e0;function f(a:int[]):int[]{return a[1.0:n:4e0];}`, ""},
		{"make slice", `const n=2e0;function f():int[]{return makeSlice<int>(n,4.0);}`, ""},
		{"make map", `function f():Map<int,int>{return makeMap<int,int>(2.0);}`, ""},
		{"channel", `function f():int{const c=goChannel<int>(2.0);return cap(c);}`, ""},
		{"return constant overflow", `const n=1e400;function f():float{return n;}`, "overflows"},
		{"narrow constant", `const n=2.0;function f():float32{return n;}`, ""},
		{"constant arithmetic", `const n=1e400;function f():float{return n/n;}`, ""},
		{"complex arithmetic overflow", `const n=1e40i;function f():complex64{return n+n;}`, "overflows"},
		{"runtime alias overflow", `const n=1e400;const alias=n;`, "overflows"},
		{"typed float index", `const n:float=2.0;function f(a:int[]):int{return a[n];}`, "index must be an integer"},
		{"float variable index", `function f(a:int[],n:float):int{return a[n];}`, "index must be an integer"},
		{"runtime alias index", `const n=2.0;const alias=n;function f(a:int[]):int{return a[alias];}`, "index must be an integer"},
		{"fraction index", `function f(a:int[]):int{return a[1.5];}`, "index must be an integer"},
		{"negative index", `function f(a:int[]):int{return a[-1.0];}`, "index cannot be negative"},
		{"large index", `function f(a:int[]):int{return a[1e40];}`, "index is out of range"},
		{"array overflow", `function f(a:[4]int):int{return a[4.0];}`, "out of bounds"},
		{"pointer overflow", `function f(a:*[4]int):int{return a[4e0];}`, "out of bounds"},
		{"string byte bound", `function f():byte{return "日本"[6.0];}`, "out of bounds"},
		{"string constant bound", `const text="abc";function f():byte{return text[3.0];}`, "out of bounds"},
		{"string runtime alias", `const text="abc";const alias=text;function f():byte{return alias[3.0];}`, ""},
		{"negative bound", `const n=-1.0;function f(a:int[]):int[]{return a[n:];}`, "bound cannot be negative"},
		{"bound ordering", `const n=3.0;function f(a:int[]):int[]{return a[n:2.0];}`, "out of order"},
		{"fixed bound", `function f(a:[4]int):int[]{return a[:5.0];}`, "exceeds fixed array length"},
		{"typed float size", `function f(n:float):int[]{return makeSlice<int>(n);}`, "size must be an integer"},
		{"fraction size", `function f():int[]{return makeSlice<int>(1.5);}`, "size must be an integer"},
		{"negative size", `const n=-2e0;function f():int[]{return makeSlice<int>(n);}`, "size cannot be negative"},
		{"size overflow", `function f():int[]{return makeSlice<int>(1e40);}`, "size is out of range"},
		{"capacity ordering", `const n=3.0;function f():int[]{return makeSlice<int>(n,2.0);}`, "capacity cannot be smaller"},
		{"channel overflow", `function f():void{const c=goChannel<int>(1e40);}`, "capacity is out of range"},
		{"map key overflow", `function f(a:Map<float32,int>):int{return a[1e40];}`, "overflows"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
