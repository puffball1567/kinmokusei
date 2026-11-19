package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstrainedCallbackCallsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `import go errors from "errors";
type Increment=distinct (v:int)=>int;
type Double=distinct (v:int)=>int;
constraint Transform=Increment|Double;
constraint Variadic=~(...v:int[])=>int;
constraint Pair=~(v:int)=>(int,string);
constraint Binary=~(a:int,b:int)=>int;
constraint Apply=~(v:int)=>void;
constraint Checked=~(n:int)=>Result<int>;
class Item{constructor(public value:int){}}
constraint Load=~()=>Result<Item>;
alias Reader<E>=()=>E;
constraint Read<E>=~Reader<E>;
constraint TryRead<E>=~()=>Result<E>;
alias FindItem=()=>Result<Item|null>;

function transform<F extends Transform>(fn:F,n:int):int{return fn(n);}
function variadic<F extends Variadic>(fn:F,n:int):int{const xs=[n,n+2];return fn(xs...);}
function pair<F extends Pair>(fn:F,n:int):(int,string){return fn(n);}
function values(n:int):(int,int){return n,n+2;}
function binary<F extends Binary>(fn:F,n:int):int{return fn(values(n));}
function deferred<F extends Apply>(fn:F,n:int):void{defer fn(n);n+=1;}
function launched<F extends Transform>(fn:F,n:int):int{const task=go fn(n);return await task;}
export function CheckedCall<F extends Checked>(fn:F,n:int):Result<int>{const value=fn(n)?;return ok(value+1);}
export function CheckedPayload<F extends Load>(fn:F):Result<Item>{return fn();}
function checked(n:int):Result<int>{if(n<0){return fail(errors.New(b"negative"));}return ok(n*2);}
export function ResultValue(n:int):Result<int>{return CheckedCall(checked,n);}
function read<E,F extends Read<E>>(fn:F):E{return fn();}
function forward<A,G extends Read<A>>(fn:G):A{return read(fn);}
function tryRead<E,F extends TryRead<E>>(fn:F):Result<E>{return fn();}
export function Text():string{return forward(():string=>"金木犀");}
export function Find(n:int):Result<Item|null>{
 const fn:FindItem=():Result<Item|null>=>{
  if(n<0){return fail(errors.New(b"negative"));}
  if(n===0){return ok(null);}
  return ok(new Item(n));
 };
 return tryRead(fn);
}

export function Values(n:int):int[]{
 const inc=Increment((v:int):int=>v+1);const double=Double((v:int):int=>v*2);
 const sum=(...values:int[]):int=>{let result=0;for(const v of values){result+=v;}return result;};
 const labeled=(v:int):(int,string)=>{return v,"tag";};
 const [value,label]=pair(labeled,n);
 let observed=0;const write=(v:int):void=>{observed=v;};deferred(write,n);
 return [transform(inc,n),transform(double,n),variadic(sum,n),value,len(label),binary((a:int,b:int):int=>a-b,n),observed,launched(inc,n)];
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, ds, err := EmitGo([]string{path}, "callbackcalls")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	reference := `package reference
import "errors"
type inc func(int)int
type double func(int)int
type transform interface{inc|double}
func run[F transform](fn F,n int)int{return fn(n)}
func variadic[F ~func(...int)int](fn F,n int)int{return fn([]int{n,n+2}...)}
func pair[F ~func(int)(int,string)](fn F,n int)(int,string){return fn(n)}
func values(n int)(int,int){return n,n+2}
func binary[F ~func(int,int)int](fn F,n int)int{return fn(values(n))}
func deferred[F ~func(int)](fn F,n int){defer fn(n);n++}
func launched[F transform](fn F,n int)int{done:=make(chan int,1);go func(){done<-fn(n)}();return <-done}
func checked[F ~func(int)(int,error)](fn F,n int)(int,error){v,e:=fn(n);if e!=nil{return 0,e};return v+1,nil}
func ResultValue(n int)(int,error){return checked(func(v int)(int,error){if v<0{return 0,errors.New("negative")};return v*2,nil},n)}
func read[E any,F ~func()E](fn F)E{return fn()}
func forward[A any,G ~func()A](fn G)A{return read(fn)}
func tryRead[E any,F ~func()(E,error)](fn F)(E,error){return fn()}
func Text()string{return forward(func()string{return "金木犀"})}
type Item struct{Value int}
func Find(n int)(*Item,error){return tryRead(func()(*Item,error){if n<0{return nil,errors.New("negative")};if n==0{return nil,nil};return &Item{Value:n},nil})}
func Values(n int)[]int{
 in:=inc(func(v int)int{return v+1});dou:=double(func(v int)int{return v*2})
 sum:=func(xs ...int)int{var n int;for _,x:=range xs{n+=x};return n}
 v,label:=pair(func(v int)(int,string){return v,"tag"},n)
 observed:=0;deferred(func(v int){observed=v},n)
 return []int{run(in,n),run(dou,n),variadic(sum,n),v,len(label),binary(func(a,b int)int{return a-b},n),observed,launched(in,n)}
}
`
	comparison := `package callbackcalls_test
import("errors";"fmt";"reflect";"strings";"testing";g "constrained-callback-calls.test";r "constrained-callback-calls.test/reference")
func capture(f func())(p any){defer func(){p=recover()}();f();return nil}
func TestCalls(t *testing.T){
 if a,b:=g.Text(),r.Text();a!=b{t.Fatal(a,b)}
 for _,n:=range []int{-7,0,12}{
  if a,b:=g.Values(n),r.Values(n);!reflect.DeepEqual(a,b){t.Fatal(n,a,b)}
  a,ae:=g.ResultValue(n);b,be:=r.ResultValue(n);if a!=b||fmt.Sprint(ae)!=fmt.Sprint(be){t.Fatal(n,a,ae,b,be)}
  ga,ge:=g.Find(n);ra,re:=r.Find(n)
  if fmt.Sprint(ge)!=fmt.Sprint(re)||(ga==nil)!=(ra==nil)||ga!=nil&&ga.Value!=ra.Value{t.Fatal(n,ga,ge,ra,re)}
 }
}
func TestBoundaries(t *testing.T){
 var missing func(int)(int,error)
 if p:=capture(func(){g.CheckedCall(missing,7)});p==nil||!strings.Contains(fmt.Sprint(p),"non-null parameter"){t.Fatal(p)}
 if p:=capture(func(){g.CheckedPayload(func()(*g.Item,error){return nil,nil})});p==nil||!strings.Contains(fmt.Sprint(p),"non-null result"){t.Fatal(p)}
 failure:=errors.New("fail");v,e:=g.CheckedPayload(func()(*g.Item,error){return nil,failure});if v!=nil||e!=failure{t.Fatal(v,e)}
}
`
	runGeneratedGoDifferentialTest(t, root, "constrained-callback-calls.test", generated, reference, comparison)
}
