package sema

import (
	"strings"
	"testing"
)

func TestClassConstantContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"scalar", `class C{public static const n:int=2;public static const s:string="ab";public static const b:boolean=true;}function f():int{return C.n+len(C.s);}`, ""},
		{"forward", `const result=C.n;class C{public static const n:int=D.n+1;}class D{public static const n:int=2;}`, ""},
		{"module dependency", `class C{public static const n:int=next+1;}const next=2;`, ""},
		{"generic inherited", `class C<T>{public static const n:int=2;}class D extends C<string>{}function f():int{return D.n;}`, ""},
		{"module type scope", `alias T=int;class C<T>{public static const n:T=2;}function f<T>():int{const next=9;return C.n;}`, ""},
		{"private backing", `class C{private static const n:int=2;public static const m:int=C.n+1;}`, ""},
		{"protected", `class C{protected static const n:int=2;}class D extends C{public static const m:int=C.n+1;}`, ""},
		{"nonconstant", `function get():int{return 2;}class C{public static const n:int=get();}`, "compile-time constant"},
		{"mutable", `let n=2;class C{public static const m:int=n;}`, "compile-time constant"},
		{"static mutable", `class C{public static n:int=2;public static const m:int=C.n;}`, "compile-time constant"},
		{"getter", `class C{public static get n():int{return 2;}public static const m:int=C.n;}`, "compile-time constant"},
		{"reference", `class V{}class C{public static const v:V=new V();}`, "numeric, string, or boolean"},
		{"nullable", `class V{}class C{public static const v:V|null=null;}`, "numeric, string, or boolean"},
		{"missing", `class C{public static const n:int;}`, "explicit initializer"},
		{"generic", `class C<T>{public static const n:T=1;}`, "unknown type"},
		{"overflow", `class C{public static const n:byte=256;}`, "represented"},
		{"overflow use", `class C{public static const n:byte=255;}function f():byte{return C.n+1;}`, "overflows"},
		{"wrong type", `class C{public static const n:int="a";}`, "cannot use"},
		{"assign", `class C{public static const n:int=1;}function f():void{C.n=2;}`, "cannot assign"},
		{"increment", `class C{public static const n:int=1;}function f():void{C.n++;}`, "cannot assign"},
		{"compound", `class C{public static const n:int=1;}function f():void{C.n+=2;}`, "cannot assign"},
		{"address", `class C{public static const n:int=1;}function f():void{const p=&C.n;}`, "addressable"},
		{"alias address", `class C{public static const n:int=1;}function f():void{const n=C.n;const p=&n;}`, "addressable"},
		{"private", `class C{private static const n:int=1;}function f():int{return C.n;}`, "private"},
		{"instance", `class C{public static const n:int=1;}function f(c:C):int{return c.n;}`, "class name"},
		{"self cycle", `class C{public static const n:int=C.n;}`, "initialization cycle"},
		{"cross cycle", `class C{public static const n:int=D.n;}class D{public static const n:int=C.n;}`, "initialization cycle"},
		{"global cycle", `const n=C.n;class C{public static const n:int=n;}`, "initialization cycle"},
		{"bounds", `class C{public static const n:int=3;}function f(a:[3]int):int{return a[C.n];}`, "out of bounds"},
		{"switch duplicate", `class C{public static const n:int=2;}function f(n:int):void{switch(n){case C.n{}case 2{}}}`, "duplicate"},
		{"float rounding duplicate", `class C{public static const n:float32=16777217;}function f(n:float32):void{switch(n){case C.n{}case 16777216{}}}`, "duplicate"},
		{"unevaluated length", `class C{public static a:[2]int=[1,2];public static const n:int=len(C.a);}`, ""},
		{"constant length cycle", `class C{public static a:[2]int=[C.n,0];public static const n:int=len(C.a);}`, "initialization cycle"},
		{"global length cycle", `const n=len(C.a);class C{public static a:[2]int=[n,0];}`, "initialization cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
