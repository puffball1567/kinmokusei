package sema

import (
	"strings"
	"testing"
)

func TestStaticFieldContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"updates", `class C{public static count:int=0;}function f():int{C.count=1;C.count+=2;C.count++;const p=&C.count;*p=3;return C.count;}`, ""},
		{"generic shared", `class C<T>{public static n:int=1;constructor(public value:T){}}class D<U> extends C<U>{constructor(v:U){super(v);}}function f():int{D.n++;return C.n;}`, ""},
		{"private backing", `class C{private static n:int=0;public static get value():int{return C.n;}public static set value(v:int){C.n=v;}}`, ""},
		{"protected inherited", `class B{protected static n:int=0;}class C extends B{public static function f():int{C.n++;return B.n;}}`, ""},
		{"missing initializer", `class C{public static n:int;}`, "explicit initializer"},
		{"wrong type", `class C{public static n:int="text";}`, "cannot use"},
		{"overflow", `class C{public static n:byte=256;}`, "represented"},
		{"no generic type", `class C<T>{public static value:T=1;}`, "unknown type"},
		{"no generic initializer", `class C<T>{public static value:int=len(make<T[]>(1));}`, "unknown type"},
		{"no this", `class C{public static value:int=this.value;}`, "cannot reference this"},
		{"no constructor parameter", `class C{public static value:int=n;constructor(n:int){}}`, "undefined name"},
		{"no instance access", `class C{public static n:int=0;}function f(c:C):int{return c.n;}`, "through a class name"},
		{"no instance write", `class C{public static n:int=0;}function f(c:C):void{c.n=1;}`, "through a class name"},
		{"private access", `class C{private static n:int=0;}function f():int{return C.n;}`, "private"},
		{"private inherited", `class B{private static n:int=0;}class C extends B{public static function f():int{return C.n;}}`, "private"},
		{"protected access", `class C{protected static n:int=0;}function f():int{return C.n;}`, "protected"},
		{"duplicate", `class C{public static n:int=0;public n:int=0;}`, "duplicate field"},
		{"hide inherited", `class B{public static n:int=0;}class C extends B{public static n:int=1;}`, "duplicate field"},
		{"method collision", `class C{public static n:int=0;public static function n():int{return 1;}}`, "conflicts"},
		{"property collision", `class C{public static n:int=0;public static get n():int{return 1;}}`, "conflicts"},
		{"generated collision", `class C{public static getX:int=0;public static get x():int{return 1;}}`, "collides"},
		{"separate Go namespaces", `class C{public static getX:int=0;public get x():int{return 1;}}`, ""},
		{"global collision", `let CValue=0;class C{public static value:int=0;}`, "collides"},
		{"local collision", `class C{public static value:int=0;}function f():int{let CValue=9;return C.value;}`, "shadows generated static member"},
		{"local class shadow", `class C{public static value:int=0;}class V{public value:int=2;}function f():int{const C=new V();return C.value;}`, ""},
		{"class shadow cannot select static", `class C{public static value:int=0;}function f(C:C):int{return C.value;}`, "through a class name"},
		{"self cycle", `class C{public static value:int=C.value;}`, "initialization cycle"},
		{"indirect cycle", `class A{public static x:int=B.x;}class B{public static x:int=A.x;}`, "initialization cycle"},
		{"global cycle", `const n:int=C.x;class C{public static x:int=n;}`, "initialization cycle"},
		{"accessor cycle", `class C{public static x:int=C.value;public static get value():int{return C.x;}}`, "initialization cycle"},
		{"method cycle", `class C{public static x:int=C.read();public static function read():int{return C.x;}}`, "initialization cycle"},
		{"closure cycle", `class C{public static x:()=>int=():int=>C.x();}`, "initialization cycle"},
		{"constructor cycle", `class C{public static current:C=new C();constructor(){const c=C.current;}}`, "initialization cycle"},
		{"base constructor cycle", `class B{constructor(){const c=C.current;}}class C extends B{public static current:C=new C();}`, "initialization cycle"},
		{"instance initializer cycle", `class C{public static current:C=new C();public other:C=C.current;}`, "initialization cycle"},
		{"forward initialization", `class C{public static first:int=C.next+1;public static next:int=2;}`, ""},
		{"self object without reads", `class C{public static current:C=new C();}`, ""},
		{"nullable", `class V{public n:int=1;}class C{public static item:V|null=null;}function f():int{if(C.item!==null){return C.item.n;}return 0;}`, "nullable"},
		{"Result", `class C{public static x:Result<int>=ok(1);}`, "fields"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
