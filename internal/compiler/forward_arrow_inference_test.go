package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForwardArrowInferencePipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`const f=()=>g();const g=()=>1;`,
		`const f=()=>g();const g=()=>h();const h=()=>{return 42;};`,
		`const answer=twice(21);const twice=(n:int)=>n*2;`,
		`const read=()=>later;const later=42;`,
		`const read=()=>later;const later=last;const last=42;`,
		`const f=(later:string)=>g();const g=()=>later;const later=42;function run():int{return f("shadow");}`,
		`const f=()=>{const later="shadow";return g();};const g=()=>later;const later=42;function run():int{return f();}`,
		`const f=()=>{const g=()=>"local";return g();};const g=()=>f();function run():string{return g();}`,
		`const even=(n:int)=>{if(n==0){return true;}return odd(n-1);};const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};`,
		`const f=()=>g;const g=(n:int)=>n+1;function run():int{return f()(2);}`,
		`function map<T,U>(v:T,f:(x:T)=>U):U{return f(v);}const run=()=>map(value(),(n)=>n+1);const value=()=>41;`,
		`const f=()=>g();const g=()=>{try{throw new Exception("x");}catch(e:error){return 42;}};`,
		`class Box{public value:int=42;}const f=()=>g(null);const g=(x:Box|null)=>{if(x===null){return 0;}return x.value;};`,
		`const f=()=>{later=2;return later;};let later=1;`,
		`const first=()=>{return last();};const middle=()=>last();const last=()=>{return 42;};`,
		`const small:byte=later;const later:byte=255;`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "forward.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestForwardArrowDependencyCheckedOnce(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "invalid.km")
	input := `const first=()=>last();const second=()=>last();const last=()=>missing;`
	checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range checked.Diagnostics {
		if strings.Contains(d.Message, `undefined name "missing"`) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("dependency checked %d times: %v", count, checked.Diagnostics)
	}
}

func TestForwardArrowsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"library.km": `class Hidden{public value:int=42;} export const make=()=>create(); const create=()=>new Hidden();`,
		"main.km": `import { make } from "./library";
let trace=0;
const first=record(1);
const answer=double(21);
const last=record(2);
const double=(n:int)=>n*2;
const record=(n:int)=>{trace=trace*10+n;return trace;};
export const Run=()=>answer*100+trace+first+last;
export const Private=()=>make().value;
export const Shadow=(value:string)=>read();
const read=()=>value;
const value=42;
export const Even=(n:int)=>even(n);
const even=(n:int)=>{if(n==0){return true;}return odd(n-1);};
const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};
export const Nested=()=>factory()(2);
const factory=()=>{return (n:int)=>double(n);};
export const Recover=()=>recover();
const recover=()=>{try{throw new Exception("contained");}catch(e:error){return 7;}};`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "forwardarrows")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "forward-arrows.test", generated,
		`package reference
var trace int
var first=record(1)
var answer=double(21)
var last=record(2)
func record(n int)int{trace=trace*10+n;return trace}
func double(n int)int{return n*2}
func Run()int{return answer*100+trace+first+last}
func Private()int{return 42}
func Shadow(value string)int{return 42}
func Even(n int)bool{return n%2==0}
func Nested()int{return double(2)}
func Recover()int{return 7}`,
		`package forwardarrows
import("testing";reference "forward-arrows.test/reference")
func TestBehavior(t *testing.T){if Run()!=reference.Run(){t.Fatal("initialization order",Run(),reference.Run())};if Private()!=reference.Private(){t.Fatal("private identity")};if Shadow("local")!=reference.Shadow("local"){t.Fatal("lexical scope")};for _,n:=range []int{0,1,2,5,8}{if Even(n)!=reference.Even(n){t.Fatal("recursion",n)}};if Nested()!=reference.Nested(){t.Fatal("nested callable result")};if Recover()!=reference.Recover(){t.Fatal("exception runtime")}}`)
}

func TestForwardArrowInferenceDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`const f=()=>g();const g=()=>f();`, "needs an explicit return type"},
		{`const f=()=>g();const g=()=>h();const h=()=>f();`, "needs an explicit return type"},
		{`const value=read();const read=()=>value;`, "initialization cycle"},
		{`const first=last;const last=first;`, "initialization cycle"},
		{`const f=(secret:int)=>g();const g=()=>secret;`, "undefined name"},
		{`const f=()=>g();const g=(x)=>x;`, "cannot infer arrow parameter"},
		{`const f=()=>g();const g=(b:boolean)=>{if(b){return 1;}};`, "may complete"},
		{`const f=():string=>g();const g=()=>42;`, "cannot use"},
		{`const small:byte=later;const later=256;`, "cannot use"},
	} {
		t.Run(test.input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
			checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range checked.Diagnostics {
				if strings.Contains(d.Message, test.want) {
					return
				}
			}
			t.Fatalf("diagnostics=%v, want %s", checked.Diagnostics, test.want)
		})
	}
}
