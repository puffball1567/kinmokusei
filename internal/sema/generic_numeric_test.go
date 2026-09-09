package sema

import (
	"strings"
	"testing"
)

func TestGenericNumericConstantArguments(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"native explicit float", `function keep<T>(v:T):T{return v;}function f():float32{return keep<float32>(1.25);}`, ""},
		{"native explicit integer", `function keep<T>(v:T):T{return v;}function f():byte{return keep<byte>(255);}`, ""},
		{"native integer overflow", `function keep<T>(v:T):T{return v;}function f():byte{return keep<byte>(256);}`, "overflows"},
		{"native fractional", `function keep<T>(v:T):T{return v;}function f():int{return keep<int>(1.5);}`, "truncated"},
		{"native float overflow", `function keep<T>(v:T):T{return v;}function f():float32{return keep<float32>(1e40);}`, "overflows"},
		{"native complex", `function keep<T>(v:T):T{return v;}function f():complex64{return keep<complex64>(1+2i);}`, ""},
		{"native complex overflow", `function keep<T>(v:T):T{return v;}function f():complex64{return keep<complex64>(1e40i);}`, "overflows"},
		{"native typed second", `function pick<T>(a:T,b:T):T{return a;}function f(b:float32):float32{return pick(1.25,b);}`, ""},
		{"native typed first", `function pick<T>(a:T,b:T):T{return b;}function f(a:float32):float32{return pick(a,1.25);}`, ""},
		{"native numeric promotion", `function pick<T>(a:T,b:T):T{return b;}function f():float{return pick(1,2.5);}`, ""},
		{"native complex promotion", `function pick<T>(a:T,b:T):T{return b;}function f():complex128{return pick(1.5,2i);}`, ""},
		{"native integer binding", `const n=255;function keep<T>(v:T):T{return v;}function f():byte{return keep<byte>(n);}`, ""},
		{"native runtime alias", `const n=255;const alias=n;function keep<T>(v:T):T{return v;}function f():byte{return keep<byte>(alias);}`, "cannot"},
		{"Go explicit float", `import go cmp from "cmp";function f():int{return cmp.Compare<float32>(1.25,2.0);}`, ""},
		{"Go inferred float", `import go cmp from "cmp";function f():int{return cmp.Compare(1,2.5);}`, ""},
		{"Go integer overflow", `import go cmp from "cmp";function f():int{return cmp.Compare<byte>(256,0);}`, "overflows"},
		{"Go float overflow", `import go cmp from "cmp";function f():int{return cmp.Compare<float32>(1e40,0);}`, "overflows"},
		{"Go fraction", `import go cmp from "cmp";function f():int{return cmp.Compare<int>(1.5,0);}`, "truncated"},
		{"Go typed float variable", `import go cmp from "cmp";function f(a:float):int{return cmp.Compare<float32>(a,0);}`, "cannot"},
		{"Go shifted value overflow", `import go cmp from "cmp";function f():int{return cmp.Compare<byte>(1<<8,0);}`, "overflows"},
		{"Go negative unsigned", `import go cmp from "cmp";function f():int{return cmp.Compare<byte>(-1,0);}`, "overflows"},
		{"Go inferred overflow", `import go cmp from "cmp";function f():int{return cmp.Compare(1e400,0);}`, "overflows"},
		{"native inferred overflow", `function keep<T>(v:T):T{return v;}function f():float{return keep(1e400);}`, "overflows"},
		{"Go imported constant", `import go cmp from "cmp";import go math from "math";const pi=math.Pi;function f():int{return cmp.Compare<float32>(pi,math.Pi+1);}`, ""},
		{"Go imported constant overflow", `import go cmp from "cmp";import go math from "math";function f():int{return cmp.Compare<float32>(math.MaxFloat64,0);}`, "overflows"},
		{"native typed float mismatch", `function pick<T>(a:T,b:T):T{return b;}function f(a:float32,b:float):float{return pick(a,b);}`, "already inferred"},
		{"constraint does not narrow default", `constraint Small=~int8;function keep<T extends Small>(v:T):T{return v;}function f():int8{return keep(1);}`, "does not satisfy"},
		{"imported rune default", `import go unicode from "unicode";function keep<T>(v:T):T{return v;}function f():int32{return keep(unicode.ReplacementChar);}`, ""},
		{"imported rune promotion", `import go unicode from "unicode";function pick<T>(a:T,b:T):T{return b;}function f():int32{return pick(1,unicode.ReplacementChar);}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
