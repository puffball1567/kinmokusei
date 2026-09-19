package sema

import (
	"strings"
	"testing"
)

func TestVirtualPropertyContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"generic abstract DI", `abstract class B<T>{public abstract get value():T;protected abstract set value(v:T);}class C extends B<int>{private n:int=0;public override get value():int{return this.n;}protected override set value(v:int){this.n=v;}public function inc():void{this.value++;}}function f(b:B<int>):int{return b.value;}`, ""},
		{"one accessor override", `class B{public virtual get x():int{return 1;}public virtual set x(v:int){}}class C extends B{public override get x():int{return super.x+1;}}function f(c:C):void{c.x++;}`, ""},
		{"reabstract", `class B{public virtual get x():int{return 1;}}abstract class M extends B{public abstract override get x():int;}class C extends M{public final override get x():int{return 3;}}`, ""},
		{"missing getter", `abstract class B{public abstract get x():int;}class C extends B{}`, "must implement abstract method get x"},
		{"missing setter", `abstract class B{public get x():int{return 1;}public abstract set x(v:int);}class C extends B{}`, "must implement abstract method set x"},
		{"not abstract class", `class B{public abstract get x():int;}`, "require an abstract class"},
		{"private abstract", `abstract class B{private abstract get x():int;}`, "public or protected"},
		{"abstract final", `abstract class B{public final abstract get x():int;}`, "static or final"},
		{"private virtual", `class B{private virtual get x():int{return 1;}}`, "public or protected"},
		{"missing override", `class B{public virtual get x():int{return 1;}}class C extends B{public get x():int{return 2;}}`, "add override"},
		{"nonvirtual override", `class B{public get x():int{return 1;}}class C extends B{public override get x():int{return 2;}}`, "not virtual"},
		{"unmatched override", `class C{public override get x():int{return 2;}}`, "no inherited method"},
		{"final override", `class B{public virtual get x():int{return 1;}}class C extends B{public final override get x():int{return 2;}}class D extends C{public override get x():int{return 3;}}`, "final and cannot"},
		{"visibility narrowing", `class B{public virtual get x():int{return 1;}}class C extends B{protected override get x():int{return 2;}}`, "preserve inherited visibility"},
		{"visibility widening", `class B{protected virtual set x(v:int){}}class C extends B{public override set x(v:int){}}`, "preserve inherited visibility"},
		{"nullable contract", `class V{}abstract class B{public abstract get x():V|null;}class C extends B{public override get x():V{return new V();}}`, "incompatible signature"},
		{"setter pair mismatch", `class B{public get x():int{return 1;}public virtual set x(v:int){}}class C extends B{public override set x(v:string){}}`, "identical types"},
		{"direct abstract getter constructor", `abstract class B{constructor(){const n=this.x;}public abstract get x():int;}`, "abstract getter on this during construction"},
		{"direct abstract setter constructor", `abstract class B{constructor(){this.x=1;}public abstract set x(v:int);}`, "abstract setter on this during construction"},
		{"abstract getter compound constructor", `abstract class B{constructor(){this.x++;}public abstract get x():int;public set x(v:int){}}`, "abstract getter on this during construction"},
		{"abstract getter not read on write", `abstract class B{constructor(){this.x=1;}public abstract get x():int;public set x(v:int){}}`, ""},
		{"super abstract getter", `abstract class B{public abstract get x():int;}class C extends B{public override get x():int{return super.x;}}`, "super cannot access abstract getter"},
		{"super abstract setter", `abstract class B{public abstract set x(v:int);}class C extends B{public override set x(v:int){super.x=v;}}`, "super cannot access abstract setter"},
		{"super abstract compound getter", `abstract class B{public abstract get x():int;public set x(v:int){}}class C extends B{public override get x():int{return 1;}public function run():void{super.x++;}}`, "super cannot access abstract getter"},
		{"nonproperty method contract", `interface I{function x():int;}class C implements I{public get x():int{return 1;}}`, "missing method x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
