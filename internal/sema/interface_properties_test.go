package sema

import (
	"strings"
	"testing"
)

func TestInterfacePropertyContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"generic DI", `interface Cell<T>{get value():T;set value(v:T);}class Box<T> implements Cell<T>{constructor(private raw:T){}public get value():T{return this.raw;}public set value(v:T){this.raw=v;}}function use<T>(c:Cell<T>,v:T):T{c.value=v;return c.value;}function f():int{return use<int>(new Box<int>(1),2);}`, ""},
		{"split diamond", `interface Read<T>{get value():T;}interface Write<T>{set value(v:T);}interface Left<T> extends Read<T>{}interface Right<T> extends Read<T>{}interface Cell<T> extends Left<T>,Right<T>,Write<T>{}function f(c:Cell<int>):int{c.value+=2;c.value++;return c.value;}`, ""},
		{"inherited implementation", `interface I{get x():int;set x(v:int);}class B{public get x():int{return 1;}public set x(v:int){}}class C extends B implements I{}`, ""},
		{"abstract implementation", `interface I<T>{get x():T;set x(v:T);}abstract class B<T> implements I<T>{public abstract get x():T;public abstract set x(v:T);}class C extends B<int>{public override get x():int{return 1;}public override set x(v:int){}}function f():I<int>{return new C();}`, ""},
		{"setter only", `interface I{set x(v:int);}function f(c:I):void{c.x=2;}`, ""},
		{"readonly", `interface I{get x():int;}function f(c:I):void{c.x=2;}`, "has no setter"},
		{"writeonly", `interface I{set x(v:int);}function f(c:I):int{return c.x;}`, "has no getter"},
		{"writeonly update", `interface I{set x(v:int);}function f(c:I):void{c.x++;}`, "accessible getter"},
		{"missing getter", `interface I{get x():int;}class C implements I{}`, "missing method get x"},
		{"missing setter", `interface I{get x():int;set x(v:int);}class C implements I{public get x():int{return 1;}}`, "missing method set x"},
		{"private getter", `interface I{get x():int;}class C implements I{private get x():int{return 1;}}`, "must be public"},
		{"private setter", `interface I{set x(v:int);}class C implements I{protected set x(v:int){}}`, "must be public"},
		{"field is not property", `interface I{get x():int;}class C implements I{public x:int=1;}`, "missing method get x"},
		{"method is not property", `interface I{get x():int;}class C implements I{public function getX():int{return 1;}}`, "missing method get x"},
		{"mismatched implementation", `interface I{get x():int;}class C implements I{public get x():string{return "x";}}`, "incompatible signature"},
		{"pair mismatch", `interface I{get x():int;set x(v:string);}`, "identical types"},
		{"inherited pair mismatch", `interface R<T>{get x():T;}interface W<T>{set x(v:T);}interface I extends R<int>,W<string>{}`, "identical types"},
		{"nullable pair mismatch", `class V{}interface R<T>{get x():T;}interface W<T>{set x(v:T);}interface I extends R<V>,W<V|null>{}`, "identical types"},
		{"nullable implementation mismatch", `class V{}interface I{get x():V|null;}class C implements I{public get x():V{return new V();}}`, "incompatible signature"},
		{"inherited getter mismatch", `interface R<T>{get x():T;}interface I extends R<int>{get x():string;}`, "incompatible signatures"},
		{"duplicate", `interface I{get x():int;get x():int;}`, "duplicate"},
		{"source collision", `interface I{get x():int;function x():int;}`, "conflicts with a method"},
		{"inherited source collision", `interface R{get x():int;}interface I extends R{function x():int;}`, "conflicts with a method"},
		{"generated collision", `interface I{get x():int;function getX():int;}`, "generated property method"},
		{"inherited generated collision", `interface R{get x():int;}interface M{function getX():int;}interface I extends R,M{}`, "generated property method"},
		{"capitalization collision", `interface I{get x():int;get X():int;}`, "generated property method"},
		{"void getter", `interface I{get x():void;}`, "getter must"},
		{"getter parameter", `interface I{get x(v:int):int;}`, "getter must"},
		{"setter arity", `interface I{set x();}`, "setter must"},
		{"setter result", `interface I{set x(v:int):int;}`, "setter must"},
		{"rest setter", `interface I{set x(...v:int[]);}`, "setter must"},
		{"Result getter", `interface I{get x():Result<int>;}`, "properties"},
		{"Task getter", `interface I{get x():Task<int>;}`, "properties"},
		{"Result setter", `interface I{set x(v:Result<int>);}`, "properties"},
		{"Task setter", `interface I{set x(v:Task<int>);}`, "properties"},
		{"not addressable", `interface I{get x():int;set x(v:int);}function f(c:I):void{const p=&c.x;}`, "address"},
		{"returned array", `interface I{get x():[1]int;}function f(c:I):void{c.x[0]=2;}`, "not assignable"},
		{"no property narrowing", `class V{public n:int=1;}interface I{get x():V|null;}function f(c:I):int{if(c.x!==null){return c.x.n;}return 0;}`, "nullable"},
		{"nullable receiver", `interface I{get x():int;}function f(c:I|null):int{return c.x;}`, "nullable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
