package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResultFunctionValuesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module result-function-values.test\n\ngo 1.23\n",
		"api/api.go": `package api
type Callback func(int)(int,error)
func Apply(n int, f Callback)(int,error){return f(n)}
func ApplyVoid(f func()error)error{return f()}
func Factory(base int)Callback{return func(n int)(int,error){return base+n,nil}}
func Generic[T any](n T,f func(T)(T,error))(T,error){return f(n)}
`,
		"library.km": `import go errors from "errors";
import go strconv from "strconv";
import go api from "result-function-values.test/api";
alias Load=(n:int)=>Result<int>;
type Named<T>=distinct (n:T)=>Result<T>;
export function Factory(base:int):Load{
  let calls=0;
  return (n)=>{calls++;if(n<0){return fail(errors.New("negative"));}return ok(base+n+calls);};
}
export function Parse(text:string):Result<int>{const f:(text:string)=>Result<int>=strconv.Atoi;return f(text);}
export function Recursive(n:int):Result<int>{
  const f=(n:int):Result<int>=>{if(n<0){return fail(errors.New("negative"));}if(n<=1){return ok(1);}const value=f(n-1)?;return ok(n*value);};
  return f(n);
}
export function Reassign():Result<int>{
  let f=(n:int):Result<int>=>{if(n==0){return ok(1);}const value=f(n-1)?;return ok(value+1);};
  const saved=f;f=(n)=>{return ok(10);};const a=saved(2)?;const b=f(2)?;return ok(a*100+b);
}
export function NamedCall(n:int):Result<int>{const f:Named<int>=(value)=>{return ok(value+1);};return f(n);}
export function Callbacks(n:int):Result<int>{
  const fs:Load[]=[Factory(10),Factory(20)];
  const values=makeMap<string,Load>();values["first"]=fs[0];
  const a=values["first"](n)?;const b=fs[1](n)?;return ok(a+b);
}
export function GoBoundary(n:int):Result<int>{
  const f:Load=api.Factory(10);
  const a=api.Apply(n,f)?;
  const b=api.Generic(n,(value:int):Result<int>=>{if(value<0){return fail(errors.New("negative"));}return ok(value+1);})?;
  return ok(a+b);
}
export function VoidBoundary(reject:boolean):Result<void>{
  const f=():Result<void>=>{if(reject){return fail(errors.New("void failure"));}return ok();};
  api.ApplyVoid(f)?;return ok();
}
interface Loader{function load(n:int):Result<int>;}
class Base implements Loader{public virtual function load(n:int):Result<int>{return ok(n);}}
class Child extends Base{public override function load(n:int):Result<int>{return ok(n*2);}}
export function MethodValue(n:int):Result<int>{const value:Loader=new Child();const load=value.load;return load(n);}
class Hidden{constructor(public value:int){}}
struct Holder{load:()=>Result<Hidden>;}
class ObjectLoader{constructor(private value:int){}public function factory():()=>Result<Hidden>{return ()=>{return ok(new Hidden(this.value));};}}
export function Objects(n:int):Result<int>{
  const loader=new ObjectLoader(n);const holder=Holder{load:loader.factory()};
  const channel=goChannel<()=>Result<Hidden>>(1);channel<-holder.load;
  const f=<-channel;const p=&f;const object=(*p)()?;return ok(object.value);
}
export function Async(n:int):Result<int>{const f=Factory(10);const task=go f(n);const result=await task?;return ok(result);}
export function Finally(reject:boolean):int{
  let trace=0;
  const f=():Result<int>=>{try{trace=trace*10+1;if(reject){return fail(errors.New("failed"));}return ok(42);}finally{trace=trace*10+2;}};
  const [value,err]=f();if(err!==nil){return trace*1000-1;}return trace*1000+value;
}
export function ReturnedFunction():Result<()=>Result<int>>{return ok(()=>{return ok(42);});}
export function EvaluateOnce():int{
  let trace=0;
  const callee=():Load=>{trace=trace*10+1;return (n)=>{trace=trace*10+3;return ok(n);};};
  const arg=():int=>{trace=trace*10+2;return 7;};const [value,err]=callee()(arg());return trace*10+value;
}
`,
		"entry.km": `import {Factory,Parse,Recursive,Reassign,NamedCall,Callbacks,GoBoundary,VoidBoundary,MethodValue,Objects,Async,Finally,ReturnedFunction,EvaluateOnce} from "./library";
export function Scalar(n:int):Result<int>{const a=Recursive(n)?;const b=NamedCall(n)?;const c=Callbacks(n)?;const d=GoBoundary(n)?;const e=MethodValue(n)?;const f=Objects(n)?;const g=Async(n)?;return ok(a+b+c+d+e+f+g);}
export function Captures():Result<int>{const f=Factory(10);const a=f(1)?;const b=f(2)?;const c=Reassign()?;const g=ReturnedFunction()?;const d=g()?;return ok(a+b+c+d);}
export function Parsing(text:string):Result<int>{return Parse(text);}
export function Notify(reject:boolean):Result<void>{return VoidBoundary(reject);}
export function Control(reject:boolean):int{return Finally(reject)+EvaluateOnce();}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "resultfunctions")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("errors";"strconv")
func factorial(n int)(int,error){if n<0{return 0,errors.New("negative")};if n<=1{return 1,nil};v,e:=factorial(n-1);if e!=nil{return 0,e};return n*v,nil}
func Scalar(n int)(int,error){a,e:=factorial(n);if e!=nil{return 0,e};return a+(n+1)+(32+2*n)+(11+2*n)+(2*n)+n+(11+n),nil}
func Captures()(int,error){return 12+14+1110+42,nil}
func Parsing(text string)(int,error){return strconv.Atoi(text)}
func Notify(reject bool)error{if reject{return errors.New("void failure")};return nil}
func Control(reject bool)int{trace:=0;f:=func()(int,error){defer func(){trace=trace*10+2}();trace=trace*10+1;if reject{return 0,errors.New("failed")};return 42,nil};value,err:=f();if err!=nil{return trace*1000-1+1237};return trace*1000+value+1237}
`
	comparison := `package resultfunctions_test
import("testing";"fmt";g "result-function-values.test";r "result-function-values.test/reference")
func TestContracts(t *testing.T){
  for _,n:=range []int{-3,0,1,2,5,8}{gv,ge:=g.Scalar(n);rv,re:=r.Scalar(n);if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Errorf("Scalar(%d)=(%d,%v) want (%d,%v)",n,gv,ge,rv,re)}}
  gv,ge:=g.Captures();rv,re:=r.Captures();if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Errorf("Captures=(%d,%v) want (%d,%v)",gv,ge,rv,re)}
  for _,s:=range []string{"0","42","-8","bad","999999999999999999999999999999"}{gv,ge:=g.Parsing(s);rv,re:=r.Parsing(s);if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Errorf("Parsing(%q)=(%d,%v) want (%d,%v)",s,gv,ge,rv,re)}}
  for _,reject:=range []bool{false,true}{if got,want:=fmt.Sprint(g.Notify(reject)),fmt.Sprint(r.Notify(reject));got!=want{t.Errorf("Notify=%v want %v",got,want)};if got,want:=g.Control(reject),r.Control(reject);got!=want{t.Errorf("Control=%d want %d",got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "result-function-values.test", generated, reference, comparison)
}
