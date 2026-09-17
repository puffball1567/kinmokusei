package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericCallbackInferencePipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=():int=>apply(21,(x)=>x*2);`,
		`function apply<T>(f:(x:T)=>T,v:T):T{return f(v);} const run=():int=>apply((x)=>x*2,21);`,
		`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=():byte=>apply<byte>(21,(x)=>1);`,
		`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=():byte=>apply(21,(x:byte)=>x);`,
		`function map<T,U>(v:T,f:(x:T)=>U):U{return f(v);} const run=():string=>map(21,(x)=>{if(x>0){return "yes";}return "no";});`,
		`function map<T,U>(v:T,f:(x:T)=>U):U{return f(v);} const run=():string=>map<int>(21,(x)=>"yes");`,
		`function chain<T,U,V>(value:T,next:(u:U)=>V,first:(t:T)=>U):V{return next(first(value));} const run=():int=>chain(21,(s)=>len(s),(n)=>"yes");`,
		`alias Callback<T>=(x:T)=>T; function apply<T>(v:T,...fs:Callback<T>[]):T{for(const f of fs){v=f(v);}return v;} const run=():int=>apply(21,(x)=>x+1,(x)=>x*2);`,
		`function sum<T>(v:T,f:(...xs:T[])=>T):T{return f(v);} const run=():int=>sum(2,(...xs)=>xs[0]);`,
		`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} function outer<T>(v:T):T{return apply(v,(x)=>x);}`,
		`class Box<T>{constructor(public value:T){} public function map<U>(f:(x:T)=>U):U{return f(this.value);}} const run=():int=>new Box<string>("yes").map((x)=>len(x));`,
		`constraint Slice<E>=~E[]; function each<S extends Slice<E>,E>(s:S,f:(x:E)=>E):void{for(const x of s){f(x);}} const run=():void=>each([1,2],(x)=>x+1);`,
		`import go slices from "slices"; const run=():int=>slices.IndexFunc([1,2,3],(x)=>x==2);`,
		`import go { IndexFunc } from "slices"; const run=():int=>IndexFunc<int[]>([1,2,3],(x)=>x==2);`,
		`import go slices from "slices"; function run():int{let values:int[]=[2,1];slices.SortFunc(values,(a,b)=>a-b);return values[0];}`,
		`import go slices from "slices"; const run=():int=>slices.CompareFunc([1,2],["a","bb"],(n,s)=>n-len(s));`,
		`import go slices from "slices"; type Numbers=distinct int[]; const run=():int=>slices.IndexFunc(Numbers([1,2]),(x)=>x==2);`,
		`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} class User{public value:int=1;} function run(input:User|null):int{const user=input;if(user===null){return 0;}return apply(1,(x)=>user.value+x);}`,
		`function pick<T>(v:T,f:()=>T):T{return f();} const run=():float=>pick(1,()=>1.5);`,
		`function pick<T,U>(v:T,f:(x:U)=>T):T{return v;} const run=():int32=>pick(1,(x:int32)=>x);`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "callback.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestGenericCallbackInferenceDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=()=>apply(1,(x)=>"bad");`, "cannot use"},
		{`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=()=>apply(1,(x,y)=>x);`, "cannot infer arrow parameter"},
		{`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=()=>apply<byte>(1,(x)=>256);`, "cannot be represented"},
		{`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=()=>apply(256,(x:byte)=>x);`, "overflows"},
		{`function unknown<T>(f:(x:T)=>T):void{} const run=()=>unknown((x)=>x);`, "cannot infer"},
		{`function apply<T extends comparable,U extends comparable>(v:T,f:(x:T)=>U):U{return f(v);} const run=()=>apply(1,(x)=>[x]);`, "constraint"},
		{`import go slices from "slices"; const run=()=>slices.IndexFunc([1,2],(x)=>"bad");`, "cannot use"},
		{`import go slices from "slices"; const run=()=>slices.IndexFunc([1,2],(x)=>{if(x>0){return true;}});`, "may complete"},
		{`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} class User{public value:int=1;} function run(input:User|null):int{let user=input;if(user===null){return 0;}apply(1,(x)=>{user=null;return x;});return user.value;}`, "nullable"},
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

func TestImportedGenericCallbackReturnInference(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"go.mod": "module generic-callback-return.test\n\ngo 1.23\n",
		"api/api.go": `package api
type Callback[T,U any] func(T)U
func Map[S ~[]E,E,R any](values S, f Callback[E,R])R{return f(values[0])}
func Before[T,U any](f func(T)U,value T)U{return f(value)}
func Pair[T any](value T,f func(T,T)T)T{return f(value,value)}`,
		"main.km": `import go api from "generic-callback-return.test/api";
export function Run():int{return api.Map(["yes"],(s)=>len(s))+api.Before((n)=>n*2,3);}
export function Narrow():byte{return api.Pair(2,(a,b:byte)=>a+b);}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "genericcallbackreturn")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "generic-callback-return.test", generated,
		`package reference
func Run()int{return len("yes")+3*2}
func Narrow()byte{return 4}`,
		`package genericcallbackreturn
import("testing";reference "generic-callback-return.test/reference")
func TestBehavior(t *testing.T){if Run()!=reference.Run(){t.Fatal("callback result inference")};if Narrow()!=reference.Narrow(){t.Fatal("partial parameter inference")}}`)
}

func TestGenericCallbacksMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"library.km": `export function map<T,U>(v:T,f:(x:T)=>U):U{return f(v);} export function before<T>(f:(x:T)=>T,v:T):T{return f(v);}
class Hidden{public value:int=42;} export function privateInput<T>(f:(v:Hidden)=>T):T{return f(new Hidden());}`,
		"main.km": `import { map, before } from "./library";
import { privateInput } from "./library";
import go { IndexFunc, SortFunc } from "slices";
let trace=0;
function input():int{trace=trace*10+1;return 3;}
export function Run():int{trace=0;const answer=before((x)=>{trace=trace*10+2;return x*2;},input());return trace*100+answer;}
export function Map(n:int):int{return map(n,(x)=>{const next=(y:int)=>x+y;return next(2);});}
export function Go():int{let values:int[]=[3,1,2];SortFunc(values,(a,b)=>a-b);return IndexFunc(values,(x)=>x==2)*10+values[0];}
export function Nested():int{return map(3,(x)=>map(x,(y)=>x+y));}
export function Private():int{return privateInput((v)=>v.value);}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "genericcallbacks")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "generic-callbacks.test", generated,
		`package reference
import "slices"
func Run()int{trace:=0;input:=func()int{trace=trace*10+1;return 3};f:=func(x int)int{trace=trace*10+2;return x*2};answer:=f(input());return trace*100+answer}
func Map(n int)int{return n+2}
func Go()int{values:=[]int{3,1,2};slices.SortFunc(values,func(a,b int)int{return a-b});return slices.IndexFunc(values,func(x int)bool{return x==2})*10+values[0]}
func Nested()int{return 6}
func Private()int{return 42}`,
		`package genericcallbacks
import("testing";reference "generic-callbacks.test/reference")
func TestBehavior(t *testing.T){if Run()!=reference.Run(){t.Fatal("evaluation order",Run())};for _,n:=range []int{-2,0,3}{if Map(n)!=reference.Map(n){t.Fatal("capture",n)}};if Go()!=reference.Go(){t.Fatal("Go callbacks")};if Nested()!=reference.Nested(){t.Fatal("nested inference")};if Private()!=reference.Private(){t.Fatal("private dependency identity")}}`)
}
