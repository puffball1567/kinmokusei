package sema

import (
	"strings"
	"testing"
)

func TestNumericConstantAliases(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, input, want string }{
		{"integer", `const a=254;const b=a+1;function use():byte{return b;}`, ""},
		{"float", `const a=2.;const b=a;const c=b+1.;function use(xs:int[]):int{return xs[c];}`, ""},
		{"huge", `const a=1e400;const b=a;const c=b/b;function use():float{return c;}`, ""},
		{"complex", `const a=1+2i;const b=a;function use():complex64{return b;}`, ""},
		{"local", `function use():byte{const a=200;const b=a+1;const c=-(-b);return c;}`, ""},
		{"forward", `const b=a+1;const a=254;function use():byte{return b;}`, ""},
		{"rounding", `const a:float32=16777217.;const b=a;function use():float32{return b-16777216.;}`, ""},
		{"typed integer", `const a:int=1;const b=a;function use():byte{return b;}`, "cannot"},
		{"typed operation overflow", `const a:int8=120;const b=a+8;`, "overflows"},
		{"typed negation overflow", `const a:int8=-128;const b=-a;`, "overflows"},
		{"constant address", `function use():void{const a=1;const b=a;const pointer=&b;}`, "addressable"},
		{"runtime address", `function get():int{return 1;}function use():void{const a=get();const b=a;const pointer=&b;}`, ""},
		{"converted constant address", `function use():void{const a=1;const b=int8(a);const pointer=&b;}`, "addressable"},
		{"converted alias", `const a=120;const b=int8(a);const c=b;function use():int8{return c;}`, ""},
		{"converted alias retains type", `const a=120;const b=int8(a);const c=b;function use():int{return c;}`, "cannot"},
		{"generic conversion is runtime", `constraint Number=~int;function use<T extends Number>():void{const a=T(1);const pointer=&a;}`, ""},
		{"overflow", `const a=255;const b=a+1;function use():byte{return b;}`, "cannot be represented"},
		{"float overflow", `const a=1e40;const b=a;function use():float32{return b;}`, "overflows"},
		{"fraction", `const a=2.5;const b=a;function use(xs:int[]):int{return xs[b];}`, "index must be an integer"},
		{"mutable", `let a=2.;const b=a;function use(xs:int[]):int{return xs[b];}`, "index must be an integer"},
		{"call", `function get():float{return 2.;}const a=get();const b=a;function use(xs:int[]):int{return xs[b];}`, "index must be an integer"},
		{"loop storage", `function use(xs:int[]):int{for(const a=2.;false;){const b=a;return xs[b];}return 0;}`, "index must be an integer"},
		{"captured loop storage", `function use():void{for(const a=2.;false;){const read=():int=>{const b=a;return b;};}}`, "cannot"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.input)
			if test.want == "" && len(diagnostics) > 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
