package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceResultForwardingMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `import go strings from "strings";
import go strconv from "strconv";
import go errors from "errors";
import go cmp from "cmp";
import go slices from "slices";
import {Split,Tail} from "./types";
function Cut(s:string):(string,string,boolean){return strings.Cut(s,":");}
function apply(f:(s:string)=>(string,string,boolean),s:string):(string,string,boolean){return f(s);}
function Callback(s:string):(string,string,boolean){return apply((text)=>strings.Cut(text,":"),s);}
const Arrow=(s:string):(string,string,boolean)=>strings.Cut(s,":");
function generic<T>(f:(s:T)=>(T,T,boolean),s:T):(T,T,boolean){return f(s);}
function Generic(s:string):(string,string,boolean){return generic(Cut,s);}
function Alias(s:string):(string,string,boolean){const split:Split<string>=(text)=>strings.Cut(text,":");return split(s);}
function Block(s:string):(string,string,boolean){const split:Split<string>=(text)=>{return strings.Cut(text,":");};return split(s);}
interface Cutter<T>{function cut(s:T):(T,T,boolean);}
class Splitter implements Cutter<string> {public function cut(s:string):(string,string,boolean){return Cut(s);}}
function Method(s:string):(string,string,boolean){const splitter=new Splitter();return splitter.cut(s);}
function Interface(s:string):(string,string,boolean){const splitter:Cutter<string>=new Splitter();return splitter.cut(s);}
function Anonymous(s:string):(string,string,boolean){const splitter:interface{cut(s:string):(string,string,boolean);}=new Splitter();return splitter.cut(s);}
function Destructure(s:string):string{const [left,right,found]=Cut(s);if(found){return right+left;}return left;}
class Echo<T>{public function echo(value:T):T{return value;}}
function EchoValue(s:string):string{const value:interface{echo(value:string):string;}=new Echo<string>();return value.echo(s);}
function Parse(s:string):(int,error){return strconv.Atoi(s);}
function Twice(s:string):Result<int>{const value=Parse(s)?;return ok(value*2);}
function Manual(s:string):(string,string,boolean){return s,Tail(),true;}
function ManualArrow(s:string):(string,string,boolean){const f:Split<string>=(text)=>{return text,Tail(),true;};return f(s);}
let trace:int=0;
function bump(n:int):int{trace=trace*10+n;return trace;}
function Ordered():(int,int,int){trace=0;return bump(1),bump(2),trace;}
class Base {public function value():int{return 7;}}
class Derived extends Base {}
function construct():(Base,int){return new Derived(),9;}
function Upcast():int{const [obj,value]=construct();return obj.value()+value;}
function Narrow():(byte,float32){return 255,16777217;}
function TryForward(s:string):(string,string,boolean){try{return Cut(s);}finally{trace++;}}
function TryTyped():(byte,error){try{return 255,nil;}finally{trace++;}}
function TryCatch(fail:boolean):(int,string){try{if(fail){throw errors.New("bad");}return 1,"ok";}catch(e:error){return 2,e.Error();}finally{trace++;}}
function Nested():(int,int){trace=0;try{try{return bump(1),bump(2);}finally{trace=trace*10+3;}}finally{trace=trace*10+4;}}
function Override():(int,int){try{return 1,2;}finally{return 3,4;}}
function Trace():int{return trace;}
function genericTry<T>(value:T):(T,T){try{return value,value;}finally{trace++;}}
function GenericTry(s:string):(string,string){return genericTry(s);}
function FinallyThrow():(int,string){try{try{return 1,"ignored";}finally{throw errors.New("override");}}catch(e:error){return 2,e.Error();}}
function RuntimePanic():(int,int){try{let xs:int[]=[];return xs[0],2;}catch(e:error){return 9,9;}finally{trace++;}}
const InferredForward=(s:string)=>Cut(s);
const InferredValues=(s:string)=>{return s,Tail(),true;};
const InferredDependency=(s:string)=>Later(s);
const Later=(s:string)=>{return s,Tail(),true;};
function InferredLocal(s:string):(string,string,boolean){const make=()=>{return s,Tail(),true;};return make();}
const InferredTry=(s:string)=>{try{return s,s;}finally{trace++;}};
const InferredForwardTry=(s:string)=>{try{return Cut(s);}finally{trace++;}};
function input(s:string):(string,int){trace++;return s,2;}
function consume(s:string,n:int):string{return strings.Repeat(s,n);}
class Repeater{public function repeat(s:string,n:int):string{return consume(s,n);}}
function NestedCall(s:string):string{trace=0;const value=consume(input(s));return value+strconv.Itoa(trace);}
function GoNestedCall(s:string):string{trace=0;const value=strings.Repeat(input(s));return value+strconv.Itoa(trace);}
function MethodNestedCall(s:string):string{trace=0;const r=new Repeater();const value=r.repeat(input(s));return value+strconv.Itoa(trace);}
function numbers():(int,int,int){trace++;return 1,2,3;}
function sum(...values:int[]):int{let total=0;for(const n of values){total+=n;}return total;}
function VariadicCall():int{trace=0;return sum(numbers())*10+trace;}
function twoNumbers():(int,int){trace++;return 3,7;}
function first<T>(a:T,b:T):T{return a;}
function firstRest<T>(...values:T[]):T{return values[0];}
function GenericArguments():int{trace=0;const a=first(twoNumbers());const b=first<int>(twoNumbers());const c=firstRest(numbers());return a*1000+b*100+c*10+trace;}
function GoGenericArguments():int{trace=0;const a=cmp.Compare(twoNumbers());const b=cmp.Compare<int>(twoNumbers());return a*100+b*10+trace;}
function twoSlices():(int[],int[]){return [1,2],[3];}
function GoGenericVariadic():int[]{return slices.Concat(twoSlices());}
function GoGenericExplicitVariadic():int[]{return slices.Concat<int[]>(twoSlices());}
`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(filepath.Join(root, "types.km"), []byte(`alias Split<T>=(s:T)=>(T,T,boolean); function Tail():string{return "tail";}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module source-results.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "results")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import ("strings";"strconv";"cmp";"slices")
func Cut(s string)(string,string,bool){return strings.Cut(s,":")}
func EchoValue(s string)string{return s}
func Twice(s string)(int,error){value,err:=strconv.Atoi(s);if err!=nil{return 0,err};return value*2,nil}
func Manual(s string)(string,string,bool){return s,"tail",true}
var trace int
func bump(n int)int{trace=trace*10+n;return trace}
func Ordered()(int,int,int){trace=0;return bump(1),bump(2),trace}
func Upcast()int{return 7+9}
func Narrow()(byte,float32){return 255,16777217}
func TryForward(s string)(string,string,bool){defer func(){trace++}();return Cut(s)}
func TryTyped()(byte,error){defer func(){trace++}();return 255,nil}
func TryCatch(fail bool)(int,string){defer func(){trace++}();if fail{return 2,"bad"};return 1,"ok"}
func Nested()(int,int){trace=0;defer func(){trace=trace*10+4}();return func()(int,int){defer func(){trace=trace*10+3}();return bump(1),bump(2)}()}
func Override()(a,b int){defer func(){a,b=3,4}();return 1,2}
func Trace()int{return trace}
func GenericTry(s string)(string,string){defer func(){trace++}();return s,s}
func FinallyThrow()(int,string){return 2,"override"}
func RuntimePanic()(int,int){defer func(){trace++}();xs:=[]int{};return xs[0],2}
func input(s string)(string,int){trace++;return s,2}
func NestedCall(s string)string{trace=0;v:=strings.Repeat(input(s));return v+strconv.Itoa(trace)}
func numbers()(int,int,int){trace++;return 1,2,3}
func sum(values ...int)int{total:=0;for _,n:=range values{total+=n};return total}
func VariadicCall()int{trace=0;return sum(numbers())*10+trace}
func twoNumbers()(int,int){trace++;return 3,7}
func first[T any](a,b T)T{return a}
func firstRest[T any](v ...T)T{return v[0]}
func GenericArguments()int{trace=0;a:=first(twoNumbers());b:=first[int](twoNumbers());c:=firstRest(numbers());return a*1000+b*100+c*10+trace}
func GoGenericArguments()int{trace=0;a:=cmp.Compare(twoNumbers());b:=cmp.Compare[int](twoNumbers());return a*100+b*10+trace}
func twoSlices()([]int,[]int){return []int{1,2},[]int{3}}
func GoGenericVariadic()[]int{return slices.Concat(twoSlices())}
`
	comparison := `package results_test
import("testing";"reflect";g "source-results.test";r "source-results.test/reference")
func TestForwarding(t *testing.T){for _,s:=range []string{"a:b","missing",":","a:b:c","日本語:値"}{
 a,b,c:=r.Cut(s)
 for _,f:=range []func(string)(string,string,bool){g.Cut,g.Callback,g.Arrow,g.Method,g.Alias,g.Block,g.Interface,g.Anonymous,g.Generic,g.InferredForward}{
  x,y,z:=f(s);if x!=a||y!=b||z!=c{t.Fatalf("%q: got %q %q %v",s,x,y,z)}
 }
 want:=a;if c{want=b+a};if got:=g.Destructure(s);got!=want{t.Fatalf("destructure: %q != %q",got,want)}
 if g.EchoValue(s)!=r.EchoValue(s){t.Fatal("generic structural interface")}
}}
func TestErrors(t *testing.T){for _,s:=range []string{"42","-7","bad",""}{a,ae:=g.Twice(s);b,be:=r.Twice(s);if a!=b||(ae==nil)!=(be==nil){t.Fatalf("%q: %v %v != %v %v",s,a,ae,b,be)}}}
func TestExplicitReturns(t *testing.T){
 for _,s:=range []string{"","value"}{a,b,c:=r.Manual(s);for _,f:=range []func(string)(string,string,bool){g.Manual,g.ManualArrow,g.InferredValues,g.InferredDependency,g.InferredLocal}{x,y,z:=f(s);if x!=a||y!=b||z!=c{t.Fatal("manual return")}}}
 a,b,c:=g.Ordered();x,y,z:=r.Ordered();if a!=x||b!=y||c!=z{t.Fatal("evaluation order")}
 if g.Upcast()!=r.Upcast(){t.Fatal("class upcast")}
 n,f:=g.Narrow();m,h:=r.Narrow();if n!=m||f!=h{t.Fatal("numeric context")}
}
func TestExceptionReturns(t *testing.T){
 a,b:=g.Nested();x,y:=r.Nested();if a!=x||b!=y||g.Trace()!=r.Trace(){t.Fatal("nested finally and evaluation order")}
 a,b=g.Override();x,y=r.Override();if a!=x||b!=y{t.Fatal("finally override")}
 n,e:=g.TryTyped();m,f:=r.TryTyped();if n!=m||e!=f{t.Fatal("typed nil return")}
 for _,s:=range []string{"a:b","plain"}{a,b,c:=g.TryForward(s);x,y,z:=r.TryForward(s);if a!=x||b!=y||c!=z{t.Fatal("forward through finally")};a,b=g.GenericTry(s);x,y=r.GenericTry(s);if a!=x||b!=y{t.Fatal("generic payload")}}
 for _,fail:=range []bool{false,true}{a,b:=g.TryCatch(fail);x,y:=r.TryCatch(fail);if a!=x||b!=y{t.Fatal("catch return")}}
 if g.Trace()!=r.Trace(){t.Fatal("finally execution count")}
 first,second:=g.InferredTry("value");left,right:=r.GenericTry("value");if first!=left||second!=right{t.Fatal("inferred try values")}
 p,q,found:=g.InferredForwardTry("a:b");u,v,ok:=r.TryForward("a:b");if p!=u||q!=v||found!=ok{t.Fatal("inferred try forwarding")}
 a,text:=g.FinallyThrow();x,want:=r.FinallyThrow();if a!=x||text!=want{t.Fatal("throw overrides return")}
 panics:=func(f func()(int,int))(yes bool){defer func(){yes=recover()!=nil}();f();return}
 gp,rp:=panics(g.RuntimePanic),panics(r.RuntimePanic);if !gp||gp!=rp||g.Trace()!=r.Trace(){t.Fatal("runtime panic must bypass catch and execute finally")}
}
func TestNestedCalls(t *testing.T){for _,s:=range []string{"","abc","日本語"}{for _,f:=range []func(string)string{g.NestedCall,g.GoNestedCall,g.MethodNestedCall}{if got,want:=f(s),r.NestedCall(s);got!=want{t.Fatalf("%q: %q != %q",s,got,want)}}};if g.VariadicCall()!=r.VariadicCall(){t.Fatal("variadic expansion or repeated evaluation")}}
func TestGenericNestedCalls(t *testing.T){if g.GenericArguments()!=r.GenericArguments(){t.Fatal("native inference, explicit args or evaluation")};if g.GoGenericArguments()!=r.GoGenericArguments(){t.Fatal("Go inference or explicit args")};want:=r.GoGenericVariadic();if !reflect.DeepEqual(g.GoGenericVariadic(),want)||!reflect.DeepEqual(g.GoGenericExplicitVariadic(),want){t.Fatal("variadic inference")}}
`
	runGeneratedGoDifferentialTest(t, root, "source-results.test", generated, reference, comparison)
}
