package sema

import (
	"strings"
	"testing"
)

func TestClassPropertyContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"pair", `class C{private n:int=0;public get value():int{return this.n;}public set value(v:int){this.n=v;}}function f(c:C):int{c.value=2;c.value+=3;c.value++;return c.value;}`, ""},
		{"generic inheritance", `class C<T>{constructor(private n:T){}public get value():T{return this.n;}protected set value(v:T){this.n=v;}}class D extends C<int>{constructor(){super(1);}public function run():int{this.value=2;super.value+=3;return super.value;}}`, ""},
		{"setter only", `class C{public set value(v:int){}}function f(c:C):void{c.value=2;}`, ""},
		{"readonly", `class C{public get value():int{return 1;}}function f(c:C):void{c.value=2;}`, "has no setter"},
		{"writeonly", `class C{public set value(v:int){}}function f(c:C):int{return c.value;}`, "has no getter"},
		{"writeonly update", `class C{public set value(v:int){}}function f(c:C):void{c.value++;}`, "accessible getter"},
		{"private setter", `class C{public get value():int{return 1;}private set value(v:int){}}function f(c:C):void{c.value=2;}`, "private"},
		{"private getter", `class C{private get value():int{return 1;}public set value(v:int){}}function f(c:C):void{c.value+=2;}`, "accessible getter"},
		{"mismatch", `class C{public get value():int{return 1;}public set value(v:string){}}`, "identical types"},
		{"nullable mismatch", `class V{}class C{public get value():V|null{return null;}public set value(v:V){}}`, "identical types"},
		{"duplicate", `class C{get x():int{return 1;}get x():int{return 2;}}`, "duplicate"},
		{"field collision", `class C{x:int;get x():int{return 1;}}`, "conflicts with a field"},
		{"method collision", `class C{get x():int{return 1;}function x():int{return 2;}}`, "conflicts with a property"},
		{"reverse method collision", `class C{function x():int{return 2;}get x():int{return 1;}}`, "conflicts with a method"},
		{"generated collision", `class C{public get x():int{return 1;}public function getX():int{return 2;}}`, "conflicts"},
		{"inherited collision", `class B{public function getX():int{return 1;}}class C extends B{public get x():int{return 2;}}`, "conflicts"},
		{"inherited accessor shadow", `class B{public get x():int{return 1;}}class C extends B{public function getX():int{return 2;}}`, "conflicts"},
		{"inherited accessor field shadow", `class B{public get x():int{return 1;}}class C extends B{public getX:int=2;}`, "conflicts"},
		{"hide property", `class B{public get x():int{return 1;}}class C extends B{public x:int=2;}`, "inherited property"},
		{"add inherited setter", `class B{public get x():int{return 1;}}class C extends B{public set x(v:int){}}`, "cannot be redeclared"},
		{"address", `class C{public get x():int{return 1;}public set x(v:int){}}function f(c:C):void{const p=&c.x;}`, "address"},
		{"returned struct not storage", `struct S{public n:int;}class C{public get x():S{return S{n:1};}}function f(c:C):void{c.x.n=2;}`, "not assignable"},
		{"returned array not storage", `class C{public get x():[1]int{return [1];}}function f(c:C):void{c.x[0]=2;}`, "not assignable"},
		{"void getter", `class C{get x():void{}}`, "getter must"},
		{"getter argument", `class C{get x(v:int):int{return v;}}`, "getter must"},
		{"setter arity", `class C{set x(){}}`, "setter must"},
		{"setter result", `class C{set x(v:int):int{return v;}}`, "setter must"},
		{"rest setter", `class C{set x(...v:int[]){}}`, "setter must"},
		{"result getter", `class C{get x():Result<int>{return ok(1);}}`, "properties"},
		{"static", `class C{static get x():int{return 1;}}`, "static properties"},
		{"virtual", `class C{public virtual get x():int{return 1;}}`, ""},
		{"constructor not initialized by setter", `class V{}class C{private raw:V;public set value(v:V){this.raw=v;}constructor(){this.value=new V();}}`, "initialized"},
		{"no property narrowing", `class V{public n:int=1;}class C{public get value():V|null{return null;}}function f(c:C):int{if(c.value!==null){return c.value.n;}return 0;}`, "nullable"},
		{"getter invalidates field proof", `class V{public n:int=1;}class C{public v:V|null=null;public get x():int{this.v=null;return 1;}}function f(c:C):int{if(c.v!==null){const x=c.x;return c.v.n;}return 0;}`, "nullable"},
		{"setter invalidates field proof", `class V{public n:int=1;}class C{public v:V|null=null;public set x(n:int){this.v=null;}}function f(c:C):int{if(c.v!==null){c.x=1;return c.v.n;}return 0;}`, "nullable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
