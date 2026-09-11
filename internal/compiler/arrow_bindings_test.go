package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArrowBindingsPipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`const f=(n:int)=>{if(n>0){return n;}return 0;};`,
		`const f=()=>{};`,
		`const f=()=>{return;};`,
		`const f=()=>{const nested=()=>{return 1;};return nested();};`,
		`const f=()=>{try{return 1;}finally{}};`,
		`const f=()=>{try{throw new Exception("x");}catch(e:error){return 2;}finally{}};`,
		`function f():int{const run=()=>{try{return 1;}finally{const local=()=>{return "x";};local();}};return run();}`,
		`const main=():void=>{};`,
		`const main=()=>{};`,
		`import go { Println } from "fmt"; const main=()=>{Println(double(21))}; const double=(n:int):int=>n*2;`,
		`const factorial=(n:int):int=>{if(n<=1){return 1;}return n*factorial(n-1);};`,
		`const even=(n:int):boolean=>{if(n==0){return true;}return odd(n-1);}; const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};`,
		`const read=():int=>later; const later=42;`,
		`const result=twice(21); const twice=(n:int):int=>n*2;`,
		`const twice:(n:int)=>int=(n)=>{return n*2;};`,
		`function f():int{const twice:(n:int)=>int=(n)=>{return n*2;};return twice(2);}`,
		`function apply(f:(n:int)=>int):int{return f(2);} const result=apply((n)=>{return n*2;});`,
		`class Box { public run:(n:int)=>int=(n)=>{return n+1;}; }`,
		`function make():(n:int)=>int{return (n)=>{return n+1;};}`,
		`function f():int{let run:(n:int)=>int=(n)=>n+1;run=(n)=>{return n+2;};return run(1);}`,
		`const sum:(...values:int[])=>int=(...values)=>{let n=0;for(const v of values){n+=v;}return n;};`,
		`import go sort from "sort"; function f():void{const values:int[]=[2,1];sort.Slice(values,(i,j)=>{return values[i]<values[j];});}`,
		`import go http from "net/http"; const handler:http.HandlerFunc=(w,r)=>{};`,
		`import go http from "net/http"; const handler:http.HandlerFunc=(w,r)=>{w.Header().Set("X-Method",r.Method);};`,
		`const make=()=>{return (n:int)=>n+1;}; const main=()=>{const fn=make();fn(1);};`,
		`const f=():int=>1; const g=f; function run():int{return g();}`,
		`const value:(n:int)=>byte=(n)=>1;`,
		`alias Callback=(n:int)=>int; const f:Callback=(n)=>n+1;`,
		`class C{public x:int=1;public run:()=>int;constructor(){this.run=()=>{return this.x;};}}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				t.Fatalf("accepted=%v err=%v", accepted, err)
			}
		})
	}
}

func TestArrowBindingDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`const f=f;`, "initialization cycle"},
		{`const f:int=f;`, "initialization cycle"},
		{`let a:int=b;let b:int=a;`, "initialization cycle"},
		{`const value=read();const read=():int=>value;`, "initialization cycle"},
		{`const value:int=read();function read():int{return value;}`, "initialization cycle"},
		{`const value:int=read();function read():int{value=1;return 1;}`, "initialization cycle"},
		{`const f=()=>{return nil;};`, "cannot infer this arrow return type"},
		{`const f=(b:boolean)=>{if(b){return;}return 1;};`, "cannot mix bare"},
		{`const f=(b:boolean)=>{if(b){return 1;}return;};`, "cannot mix bare"},
		{`const f=(b:boolean)=>{if(b){return 1;}return "x";};`, "cannot use"},
		{`const f=(b:boolean)=>{if(b){return 1;}};`, "may complete"},
		{`const f=()=>f();`, "needs an explicit return type"},
		{`const f=()=>g();const g=()=>1;`, "needs an explicit return type"},
		{`const main=(n:int):void=>{};`, "main must be"},
		{`const main=():int=>1;`, "main must be"},
		{`let main=():void=>{};`, "main must be"},
		{`const f=(n)=>n;`, "cannot infer arrow parameter"},
		{`const f:(n:int)=>int=(a,b)=>1;`, "cannot infer arrow parameter"},
		{`const f:(n:int)=>int=(n:string)=>1;`, "cannot use"},
		{`const f:(n:int)=>int=(n)=>{return "bad";};`, "cannot use"},
		{`const f:(n:int)=>int=(n)=>{if(n>0){return n;}};`, "may complete"},
		{`const f=():int=>1; function g():void{f=():int=>2;}`, "cannot assign to const"},
		{`const f=():int=>1; function g():void{const p=&f;}`, "addressable"},
		{`const f:(n:int)=>byte=(n)=>256;`, "cannot be represented"},
		{`const f:()=>void=()=>{return 1;};`, "void function cannot return"},
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			if strings.Contains(d.Message, test.want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: %v, want %s", test.input, result.Diagnostics, test.want)
		}
	}
}

func TestArrowBindingsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"math.km": `export const factorial=(n:int):int=>{if(n<=1){return 1;}return n*factorial(n-1);};
export const twice:(n:int)=>int=(n)=>{return n*2;};`,
		"main.km": `import { factorial, twice } from "./math";
export const Run=(n:int):int=>{let step:(v:int)=>int=(v)=>v+1; const saved=step; step=(v)=>v+2; return factorial(n)+twice(step(n))+saved(n)+initial;};
const initial=twice(3);
const even=(n:int):boolean=>{if(n==0){return true;}return odd(n-1);};
const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};
export const Even=(n:int):boolean=>even(n);
class Counter{public value:int=0;public next:()=>int;constructor(){this.next=()=>{this.value++;return this.value;};}}
export const Capture=():int=>{const counter=new Counter();const fn=counter.next;return fn()*10+fn();};`,
		"inference.km": `let trace=0;
export const Inferred=(n:int)=>{const nested=()=>{return "inner";};try{if(n<0){throw new Exception(nested());}return n+1;}catch(e:error){return -1;}finally{trace++;}};
export const Trace=():int=>trace;`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km"), filepath.Join(root, "inference.km")}, "arrowbindings")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "func Run(") || !strings.Contains(string(generated), "func factorial(") {
		t.Fatalf("not callable declarations:\n%s", generated)
	}
	runGeneratedGoDifferentialTest(t, root, "arrow-bindings.test", generated,
		`package reference
func factorial(n int)int{if n<=1{return 1};return n*factorial(n-1)}
func Run(n int)int{step:=func(v int)int{return v+1};saved:=step;step=func(v int)int{return v+2};return factorial(n)+2*step(n)+saved(n)+6}
func Even(n int)bool{return n%2==0}
func Capture()int{value:=0;next:=func()int{value++;return value};return next()*10+next()}
var trace int
func Inferred(n int)int{defer func(){trace++}();if n<0{return -1};return n+1}
func Trace()int{return trace}`,
		`package arrowbindings
import("testing"; reference "arrow-bindings.test/reference")
func TestBehavior(t *testing.T){for _,n:=range []int{0,1,2,5,8}{if got,want:=Run(n),reference.Run(n);got!=want{t.Errorf("Run(%d)=%d want %d",n,got,want)};if Even(n)!=reference.Even(n){t.Fatal(n)}};if Capture()!=reference.Capture(){t.Fatal("capture")};for _,n:=range []int{-1,0,3}{if Inferred(n)!=reference.Inferred(n)||Trace()!=reference.Trace(){t.Fatal("inferred try/finally",n)}}}`)
}

func TestArrowContextKeepsPrivateDependencyType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		entry:                             `import { apply } from "./library"; const Run=():int=>apply((item)=>{return item.value;});`,
		filepath.Join(root, "library.km"): `class Hidden{public value:int=42;} export function apply(callback:(item:Hidden)=>int):int{return callback(new Hidden());}`,
	})
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("%v %v", err, result.Diagnostics)
	}
}
