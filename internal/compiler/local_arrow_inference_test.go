package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalArrowResultInferencePipeline(t *testing.T) {
	t.Parallel()
	for index, input := range []string{
		`function run():int{const a=()=>b();const b=()=>42;return a();}`,
		`function run():int{const a=()=>b();const b=()=>c();const c=()=>42;return a();}`,
		`function run():int{const value=42;const a=(value:string)=>b();const b=()=>value;return a("shadow");}`,
		`function run():int{const value=42;const a=()=>{const value="shadow";return b();};const b=()=>value;return a();}`,
		`function run():int{const a=()=>{const b=()=>42;return b();};const b=()=>a();return b();}`,
		`function run():int{const a=()=>{const inner=()=>b();return inner();};const b=()=>42;return a();}`,
		`function run<T>(value:T):T{const a=()=>b();const b=()=>value;return a();}`,
		`class Box{constructor(public value:int){}public function read():int{const a=()=>b();const b=()=>this.value;return a();}}`,
		`function run():int{const a=()=>b();const b=()=>{try{throw new Exception("x");}catch(e:error){return 42;}finally{}};return a();}`,
		`function value():int{return 42;}function run():int{const a=()=>b();const b=()=>{const task=go value();return await task;};return a();}`,
		`function apply<T,U>(v:T,f:(v:T)=>U):U{return f(v);}function run():int{const a=()=>apply(b(),(x)=>x*2);const b=()=>21;return a();}`,
		`function run():int{const a=()=>b;const b=(n:int)=>n+1;return a()(41);}`,
		`function run():int{const a=()=>{b=()=>7;return b();};let b=()=>1;return a();}`,
		`function run():boolean{const even=(n:int)=>{if(n==0){return true;}return odd(n-1);};const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};return even(8);}`,
		`class Box{public value:int=42;}function run(input:Box|null):int{const saved=input;if(saved===null){return 0;}const a=()=>b();const b=()=>saved.value;return a();}`,
	} {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			t.Log(input)
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "inference.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestLocalArrowResultInferenceDiagnostics(t *testing.T) {
	t.Parallel()
	for index, test := range []struct{ input, want string }{
		{`function run():int{const a=()=>b();const b=()=>a();return a();}`, "needs an explicit return type"},
		{`function run():int{const a=()=>b();const b=()=>c();const c=()=>a();return a();}`, "needs an explicit return type"},
		{`function run():int{const a=(secret:int)=>b();const b=()=>secret;return a(1);}`, "undefined name"},
		{`function run():int{const a=()=>b(1);const b=(x)=>x;return a();}`, "cannot infer arrow parameter"},
		{`function run():int{const a=()=>b(true);const b=(x:boolean)=>{if(x){return 42;}};return a();}`, "may complete"},
		{`function run():string{const a=():string=>b();const b=()=>42;return a();}`, "cannot use"},
		{`class Box{public value:int=42;}function run(x:Box|null):int{if(x===null){return 0;}const a=()=>b();const b=()=>{x=null;return 1;};a();return x.value;}`, "nullable"},
		{`class Box{public value:int=42;}function run(x:Box|null):int{if(x===null){return 0;}const a=()=>{const b=()=>c();const c=()=>{x=null;return 1;};return b();};a();return x.value;}`, "nullable"},
		{`class Item{public value:int=42;}class Box{public item:Item|null=null;}function run(box:Box):int{if(box.item===null){return 0;}const first=()=>last();const last=()=>{box.item=null;return 1;};return box.item.value;}`, "nullable"},
		{`class Item{public value:int=42;}class Box{public item:Item|null=null;}function run(box:Box):int{if(box.item===null){return 0;}const outer=()=>{const first=()=>last();const last=()=>{box.item=null;return 1;};return first();};return box.item.value;}`, "nullable"},
	} {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			t.Log(test.input)
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
			t.Fatalf("%v, want %s", checked.Diagnostics, test.want)
		})
	}
}

func TestLocalArrowInferenceChecksDependencyOnce(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "invalid.km")
	input := `function run():void{const first=()=>last();const second=()=>last();const last=()=>missing;}`
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

func TestLocalArrowInferenceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"library.km": `export const second=()=>999;class Hidden{public value:int=42;}
export function Private():int{const first=()=>last();const last=()=>new Hidden();return first().value;}
export function Capture(n:int):int{let trace=0;const first=(n:string)=>second();let second=()=>{trace=trace*10+1;return n;};const saved=first;const before=saved("shadow");second=()=>{trace=trace*10+2;return n+1;};return before*100+saved("shadow")*10+trace;}
export function Factory(n:int):()=>int{const first=()=>second();const second=()=>{n++;return n;};return first;}
export function Generic<T>(value:T):T{const first=()=>last();const last=()=>value;return first();}
export function Recover():int{const first=()=>last();const last=()=>{try{throw new Exception("expected");}catch(e:error){return 7;}finally{}};return first();}`,
		"main.km": `import { Private,Capture,Factory,Generic,Recover } from "./library";
export function Run(n:int):int{const next=Factory(n);return Capture(n)+next()*10+next()+Private()+Generic(2)+Recover();}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "localinference")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "local-arrow-inference.test", generated,
		`package reference
func capture(n int)int{trace:=0;var second func()int;first:=func(n string)int{return second()};second=func()int{trace=trace*10+1;return n};saved:=first;before:=saved("shadow");second=func()int{trace=trace*10+2;return n+1};return before*100+saved("shadow")*10+trace}
func factory(n int)func()int{var second func()int;first:=func()int{return second()};second=func()int{n++;return n};return first}
func Run(n int)int{next:=factory(n);return capture(n)+next()*10+next()+42+2+7}`,
		`package localinference
import("testing";reference "local-arrow-inference.test/reference")
func TestBehavior(t *testing.T){for _,n:=range []int{-2,0,3,7}{if got,want:=Run(n),reference.Run(n);got!=want{t.Fatalf("Run(%d)=%d want %d",n,got,want)}}}`)
}
