package compiler

import "testing"

var classPropertySeeds = []string{
	`class C{public static const n:int=D.n+1;}class D{public static const n:int=2;}function f():int{return C.n;}`,
	`class C{public static const n:int=C.n;}`,
	`class C{public static const n:byte=255;}function f():byte{return C.n+1;}`,
	`class C{public static const n:int=1;}function f():void{C.n++;}`,
	`class C{public static const n:int=1;}function f():void{const p=&C.n;}`,
	`class C{public static const function f():int{return 1;}}`,
	`class C<T>{public static n:int=0;public static get x():int{return C.n;}public static set x(v:int){C.n=v;}}function f():int{C.x++;return C.n;}`,
	`class B{public static n:int=0;}class C extends B{}function f():void{C.n++;const p=&B.n;*p=2;}`,
	`class C{public static current:C=new C();constructor(){const c=C.current;}}`,
	`class C{public static first:int=C.next+1;public static next:int=2;}`,
	`class C<T>{public static value:T=1;}`,
	`let n=0;class C{public static get x():int{return n;}public static set x(v:int){n=v;}}function f():int{C.x++;return C.x;}`,
	`class B<T>{public static get x():int{return 1;}public static set x(v:int){}}class C<T> extends B<T>{}function f():void{C.x+=2;}`,
	`class C<T>{public static get x():T{throw new Exception("bad");}}`,
	`class C{public static get x():int{return 1;}}function f(c:C):int{return c.x;}`,
	`class C{public static get x():int{return 1;}public set x(v:int){}}`,
	`interface R<T>{get x():T;}interface W<T>{set x(v:T);}interface I<T> extends R<T>,W<T>{}function f(c:I<int>):int{c.x++;return c.x;}`,
	`interface I{get x():int;set x(v:int);}class B{public get x():int{return 1;}public set x(v:int){}}class C extends B implements I{}`,
	`interface I<T>{get x():T;}abstract class B<T> implements I<T>{public abstract get x():T;}class C extends B<int>{public override get x():int{return 1;}}`,
	`interface I{get x():int;function getX():int;}`,
	`interface R<T>{get x():T;}interface W<T>{set x(v:T);}interface I extends R<int>,W<string>{}`,
	`interface I{get x(:int;}`,
	`abstract class B<T>{public abstract get x():T;public abstract set x(v:T);}class C extends B<int>{private n:int=0;public override get x():int{return this.n;}public override set x(v:int){this.n=v;}}function f(c:C):int{const b:B<int>=c;b.x++;return b.x;}`,
	`class B{public virtual get x():int{return 1;}public virtual set x(v:int){}}class C extends B{public override get x():int{return super.x+1;}}function f(c:C):void{c.x++;}`,
	`abstract class B{constructor(){this.x++;}public abstract get x():int;public set x(v:int){}}`,
	`abstract class B{public abstract get x():int;}class C extends B{public override get x():int{return super.x;}}`,
	`class C{private n:int=0;public get x():int{return this.n;}public set x(v:int){this.n=v;}}function f(c:C):int{c.x+=1;c.x++;return c.x;}`,
	`class C<T>{constructor(private v:T){}public get x():T{return this.v;}protected set x(v:T){this.v=v;}}class D extends C<int>{constructor(){super(0);}public function f():int{super.x++;return this.x;}}`,
	`class C{public get value():int{return 1;}}function f(c:C):void{c.value=2;}`,
	`class C{public set value(v:int){}}function f(c:C):void{c.value=2;}`,
	`class C{get x():int{return 1;}set x(v:string){}}`,
	`class C{public get x():int[]{return [1];}}function f(c:C):int{c.x[0]=2;return c.x[0];}`,
	`class C{public get x():(n:int)=>int{return (n:int):int=>n;}}function f(c:C):int{return c.x(1);}`,
	`class C{public get x():int|null{return null;}}`,
	`class C{get x(:int{}`,
}

func TestClassPropertyPipelineProperties(t *testing.T) {
	generated := 0
	for _, seed := range classPropertySeeds {
		_, reached, err := compilePipelineProperty(seed)
		if err != nil {
			t.Fatal(err)
		}
		if reached {
			generated++
		}
	}
	if generated < 5 {
		t.Fatalf("only %d property seeds reached codegen", generated)
	}
}

func FuzzClassPropertyPipeline(f *testing.F) {
	for _, seed := range classPropertySeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 64<<10 {
			t.Skip()
		}
		if _, _, err := compilePipelineProperty(input); err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
	})
}
