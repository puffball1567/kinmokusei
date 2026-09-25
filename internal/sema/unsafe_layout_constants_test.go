package sema

import (
	gotypes "go/types"
	"strings"
	"testing"
)

func TestUnsafeLayoutConstantContexts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"size alias address", `function f(x:int):void{const n=u.Sizeof(x);const alias=n;const p=&alias;}`, "addressable"},
		{"alignment address", `function f(x:int64):void{const n=u.Alignof(x);const p=&n;}`, "addressable"},
		{"offset address", `import go r from "reflect";function f(x:r.StringHeader):void{const n=u.Offsetof(x.Len);const p=&n;}`, "addressable"},
		{"typed size", `function f(x:int):byte{return u.Sizeof(x);}`, "cannot use"},
		{"negative length", `function f(x:int64):int[]{const n=int(u.Sizeof(x))-9;return makeSlice<int>(n);}`, "negative"},
		{"bounds", `function f(x:int64,a:[8]int):int{return a[u.Sizeof(x)];}`, "out of bounds"},
		{"switch duplicate", `function f(n:int):void{switch(n){case int(u.Sizeof(int64(0))){return;}case 8{return;}}}`, "duplicate"},
		{"scalar overflow", `function f():int{return int(u.Sizeof(1<<100));}`, "overflows"},
		{"untyped null", `function f():int{return int(u.Sizeof(null));}`, "requires a typed value"},
		{"fractional shift", `function f(n:uint):int{return int(u.Sizeof(1.0<<n));}`, "shifted operand"},
		{"generic size runtime", `function f<T>(x:T):void{const n=u.Sizeof(x);const p=&n;}`, ""},
		{"generic alignment runtime", `function f<T>(x:T):void{const n=u.Alignof(x);const p=&n;}`, ""},
		{"generic array runtime", `function f<T>(x:[2]T):void{const n=u.Sizeof(x);const p=&n;}`, ""},
		{"constrained size runtime", `constraint I=~int;function f<T extends I>(x:T):void{const n=u.Sizeof(x);const p=&n;}`, ""},
		{"generic slice header constant", `function f<T>(x:T[]):void{const n=u.Sizeof(x);const p=&n;}`, "addressable"},
		{"generic pointer constant", `function f<T>(x:*T):void{const n=u.Sizeof(x);const p=&n;}`, "addressable"},
		{"nullable pointer constant", `function f(x:*int|null):void{const n=u.Sizeof(x);const p=&n;}`, "addressable"},
		{"mutable source constant", `function f():void{let value=1;const n=u.Sizeof(value);const p=&n;}`, "addressable"},
		{"mutable result runtime", `function f():void{let n=u.Sizeof(1);const copy=n;const p=&copy;}`, ""},
		{"class constant", `class C{public static const size:int=int(u.Sizeof(int64(0)));}function f():byte{return byte(C.size);}`, ""},
		{"call operand unevaluated", `function value():int{return 1;}function f():void{const n=u.Sizeof(value());const p=&n;}`, "addressable"},
		{"len sees nested call", `function value():int{return 1;}function f(a:[1][2]int,i:int):void{const n=len(a[i+int(u.Sizeof(value()))]);const p=&n;}`, ""},
		{"len constant without nested call", `function f(a:[1][2]int,i:int):void{const n=len(a[i+int(u.Sizeof(i))]);const p=&n;}`, "addressable"},
		{"receive operand unevaluated", `function f(c:GoChannel<int>):void{const n=u.Alignof(<-c);const p=&n;}`, "addressable"},
		{"named import", `import go {Sizeof as size} from "unsafe";const n=size(int64(0));function f():void{const p=&n;}`, "addressable"},
		{"named import shadow", `import go {Sizeof as size} from "unsafe";function f():void{const size=(x:int):int=>x;const n=size(1);const p=&n;}`, ""},
		{"size through source struct", `struct S{public x:int64;}function f(s:S):void{const n=u.Sizeof(s);const p=&n;}`, "addressable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSourceWithPolicy(t, `import go u from "unsafe";`+test.source, GoInteropPolicy{AllowUnsafe: true}), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}

func TestUnsafeLayoutTargetSizes(t *testing.T) {
	t.Parallel()
	for _, arch := range []string{"386", "amd64", "arm", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			policy := GoInteropPolicy{AllowUnsafe: true, Sizes: gotypes.SizesFor("gc", arch)}
			for _, expression := range []string{"u.Sizeof(int(0))", "u.Alignof(int64(0))", "u.Offsetof(header.Len)"} {
				source := `import go u from "unsafe";import go r from "reflect";let header:r.StringHeader=r.StringHeader{};const size=` + expression + `;function f():byte{return byte(248+size);}`
				diagnostics := checkSourceWithPolicy(t, source, policy)
				if (len(diagnostics) == 0) != (arch == "386" || arch == "arm") {
					t.Fatalf("%s: %v", expression, diagnostics)
				}
			}
		})
	}
}
