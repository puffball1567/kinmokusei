package compiler

import "testing"

var classPropertySeeds = []string{
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
