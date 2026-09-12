package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRecursiveArrowsPipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function run():int{const f=(n:int):int=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}`,
		`function run():int{const f:(n:int)=>int=(n)=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}`,
		`function run():void{const f=():void=>{f();};}`,
		`function run():void{let f=():void=>{f=():void=>{};};f();}`,
		`function run():void{let f=():void=>{const p=&f;};f();}`,
		`function run():int{const f=(n:int):int=>{const nested=():int=>f(n-1);if(n<=1){return 1;}return n*nested();};return f(5);}`,
		`function run():int{const f=(f:int):int=>f+1;return f(5);}`,
		`function run():int{const f=(n:int):int=>{const f=(n:int):int=>n+1;return f(n);};return f(5);}`,
		`function f(n:int):int{return 99;} function run():int{const f=(n:int):int=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}`,
		`import go { Abs } from "math"; function run():int{const Abs=(n:int):int=>{if(n<=1){return 1;}return n*Abs(n-1);};return Abs(5);}`,
		`import go http from "net/http"; function run():void{const h:http.HandlerFunc=(w,r)=>{if(r.Method=="again"){h(w,r);}};}`,
		`alias Callback=(n:int)=>int; function run():int{const f:Callback=(n)=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}`,
		`function run<T>(value:T):T{const f=(n:int):T=>{if(n<=1){return value;}return f(n-1);};return f(5);}`,
		`class Box<T>{constructor(public value:T){}} function run():Box<int>{const f=(n:int):Box<int>=>{if(n<=1){return new Box<int>(n);}return f(n-1);};return f(5);}`,
		`function run():int{try{const f=(n:int):int=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}finally{}}`,
		`function run():void{switch(1){case 1{const f=():void=>{f();};f();}}}`,
		`class C{public run:()=>int;constructor(){const f=(n:int):int=>{if(n<=1){return 1;}return n*f(n-1);};this.run=()=>f(5);}}`,
		`function run():void{for(const f=():int=>1;f()>0;){break;}}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "arrow.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestLocalRecursiveArrowDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`function run():void{const f=(n:int)=>f(n-1);}`, "needs an explicit return type"},
		{`const f=():int=>1;function run():void{const f=()=>f();}`, "needs an explicit return type"},
		{`function run():void{const f=():void=>{f=():void=>{};};}`, "cannot assign to const"},
		{`function run():void{const f=(n:int):int=>f("bad");}`, "cannot use string"},
		{`function run():void{const f:int=():int=>1;}`, "cannot use"},
		{`function run():void{const f=():int=>1;const f=():int=>2;}`, "duplicate local name"},
		{`function run():void{const f=():int=>g();f();const g=():int=>1;}`, "undefined"},
		{`function run():void{for(const f=():int=>f();false;){}}`, "before the for loop"},
		{`function run():void{const f=(n:int):Result<int>=>{if(n<=1){return ok(1);}const value=f(n-1)?;return ok(n*value);};}`, "Result may only be used"},
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			found = found || strings.Contains(d.Message, test.want)
		}
		if !found {
			t.Fatalf("%s: %v, want %s", test.input, result.Diagnostics, test.want)
		}
	}
}

func TestLocalRecursiveArrowsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"library.km": `export const same=(n:int):int=>999;
export function Factorial(n:int):int{const same=(n:int):int=>{if(n<=1){return 1;}return n*same(n-1);};return same(n);}
export function Reassign():int{let f=(n:int):int=>{if(n==0){return 1;}return f(n-1)+1;};const saved=f;f=(n:int):int=>10;return saved(2)*100+f(2);}
export function ReplaceInside():int{let f=():int=>{f=():int=>7;return f();};return f()*10+f();}
export function Escape():(n:int)=>int{let calls=0;const f=(n:int):int=>{calls++;if(n<=1){return calls;}return f(n-1);};return f;}
export function Shadow():int{const f=():int=>42;{const f=(f:int):int=>f+1;f(2);}return f();}
export function OrdinaryInitializer():int{const n=7;{const n=n+1;return n;}}`,
		"main.km": `import { Factorial, Reassign, ReplaceInside, Escape, Shadow, OrdinaryInitializer } from "./library";
export const Run=(n:int):int=>Factorial(n);
export const Mutable=():int=>Reassign()+ReplaceInside();
export const Captured=():int=>{const f=Escape();return f(3)*10+f(2);};
export const Shadows=():int=>Shadow()+OrdinaryInitializer();`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "localarrows")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "local-arrow-bindings.test", generated,
		`package reference
func Run(n int)int{if n<=1{return 1};return n*Run(n-1)}
func Mutable()int{return 11*100+10+77}
func Captured()int{return 3*10+5}
func Shadows()int{return 42+8}`,
		`package localarrows
import("testing"; reference "local-arrow-bindings.test/reference")
func TestBehavior(t *testing.T){for _,n:=range []int{0,1,2,5,8}{if got,want:=Run(n),reference.Run(n);got!=want{t.Errorf("Run(%d)=%d want %d",n,got,want)}};if Mutable()!=reference.Mutable(){t.Fatal("mutable self binding",Mutable())};if Captured()!=reference.Captured(){t.Fatal("escaped closure",Captured())};if Shadows()!=reference.Shadows(){t.Fatal("shadowing",Shadows())}}`)
}
