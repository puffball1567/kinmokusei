package sema

import (
	"strings"
	"testing"
)

func TestComplexSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"construction", `function f(a:float32,b:float32):complex64{return complex(a,b);}`, ""},
		{"wide construction", `function f(a:float,b:float):complex128{return complex(a,b);}`, ""},
		{"constant components", `function f(a:float32):complex64{return complex(a,1.5);}`, ""},
		{"constant narrow", `function f():complex64{return complex(1,2);}`, ""},
		{"projection", `function f(a:complex64):float32{return real(a)+imag(a);}`, ""},
		{"constant projection", `function f():int{return int(real(complex(3,4)));}`, ""},
		{"arithmetic", `function f(a:complex128,b:complex128):complex128{let r=-a+b*2;r/=b;r++;return r;}`, ""},
		{"aliases", `alias C=complex64;type Named=distinct complex64;function f(a:Named):float32{return real(a);}function g():C{return C(complex(1,2));}`, ""},
		{"constant expressions", `function f():complex64{return complex(1.5+2.5,-3.0);}`, ""},
		{"constant bindings", `const a=1.5;const b=2.5;function f():complex64{return complex(a,b);}`, ""},
		{"conversion literal", `function f():complex64{return complex64(2);}`, ""},
		{"conversion width", `function f(a:complex128):complex64{return complex64(a);}`, ""},
		{"map", `function f(a:complex128):int{const m=makeMap<complex128,int>();m[a]=1;return m[a];}`, ""},
		{"integer variables", `function f(a:int,b:int):complex128{return complex(a,b);}`, "expected floating-point"},
		{"nonconstant shift construction", `function f(n:uint):complex128{return complex(1<<n,0);}`, "expected floating-point"},
		{"nonconstant shift projection", `function f(n:uint):float{return real(1<<n);}`, "expected complex"},
		{"generic projection restriction", `constraint C=~complex128;function f<T extends C>(v:T):float{return real(v);}`, "not supported as argument"},
		{"mixed widths", `function f(a:float32,b:float):complex128{return complex(a,b);}`, "mismatched types"},
		{"real variable", `function f(a:float):float{return real(a);}`, "expected complex"},
		{"ordering", `function f(a:complex128,b:complex128):boolean{return a<b;}`, "not defined"},
		{"modulo", `function f(a:complex128,b:complex128):complex128{return a%b;}`, "not defined"},
		{"complex to real variable", `function f(a:complex128):float{return float(a);}`, "cannot convert"},
		{"real to complex variable", `function f(a:float):complex128{return complex128(a);}`, "cannot convert"},
		{"overflow conversion", `function f():complex64{return complex64(complex(1<<200,0));}`, "cannot convert"},
		{"fractional projection conversion", `function f():int{return int(real(complex(1.5,0)));}`, "cannot convert"},
		{"overflow assignment", `function f():complex64{return complex(1<<200,0);}`, "overflows"},
		{"division zero", `function f():complex128{return complex(1,2)/complex(0,0);}`, "division by zero"},
		{"arity", `function f():complex128{return complex(1);}`, "expects 2 arguments"},
		{"spread", `function f(a:complex128[]):float{return real(a...);}`, "does not accept spread"},
		{"types", `function f():complex128{return complex<int>(1,2);}`, "expects 0 type arguments"},
		{"shadowing", `function complex():int{return 1;}function use():int{return complex();}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
