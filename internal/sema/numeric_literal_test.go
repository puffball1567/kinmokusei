package sema

import (
	"strings"
	"testing"
)

func TestNumericLiteralSemanticMatrix(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"bases", `function f():int{return 0b10+0o10+010+0x10+1_000;}`, ""},
		{"array length", `function f(a:[0x_10]int):int{return a[0b11];}`, ""},
		{"narrow integer", `function f():uint8{return 0xFF;}`, ""},
		{"integer overflow", `function f():uint8{return 0x100;}`, "cannot be represented"},
		{"octal array length", `function f(a:[8]int):[010]int{return a;}`, ""},
		{"exponents", `function f():float{return .5+1e2+0x1.fp-2;}`, ""},
		{"narrow float", `function f():float32{return 1.25e2;}`, ""},
		{"float overflow", `function f():float32{return 1e40;}`, "overflows"},
		{"float negative overflow", `function f():float{return -1e400;}`, "overflows"},
		{"large intermediate", `function f():float{return 1e400/1e400;}`, ""},
		{"inferred float overflow", `function f():void{let x=1e400;}`, "overflows"},
		{"inferred complex overflow", `let x=1e400i;`, "overflows"},
		{"inferred array overflow", `function f():void{const x=[1e400];}`, "overflows"},
		{"inferred array later overflow", `function f():void{const x=[1.,1e400];}`, "overflows"},
		{"inferred object overflow", `function f():void{const x={value:1e400i};}`, "overflows"},
		{"fraction conversion", `function f():int{return int(1e-2);}`, "cannot convert"},
		{"imaginary", `function f():complex64{return 1+2i;}`, ""},
		{"imaginary bases", `function f():complex128{return 0123i+0xFi+0b11i+0o10i+0x1.fp2i;}`, ""},
		{"imaginary projection", `function f():float{return imag(.5e2i);}`, ""},
		{"imaginary inference", `function f():complex128{let z=2i;z+=1;return z;}`, ""},
		{"imaginary overflow", `function f():complex64{return 1e40i;}`, "overflows"},
		{"imaginary ordered", `function f():boolean{return 2i<3i;}`, "not defined"},
		{"imaginary modulo", `function f():complex128{return 2i%3i;}`, "not defined"},
		{"imaginary real conversion", `function f():float{return float(2i);}`, "cannot convert"},
		{"duplicate base", `function f(v:int):int{switch(v){case 010{return 1;}case 0x8{return 2;}}return 0;}`, "duplicate"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
