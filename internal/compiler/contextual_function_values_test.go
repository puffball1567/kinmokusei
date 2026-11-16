package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextualFunctionValuesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `import go errors from "errors";
export function Identity<T>(v:T):T{return v;}
export function Pair<T,U>(a:T,b:U):(T,U){return a,b;}
export function First<T>(...values:T[]):T{return values[0];}
export function Checked<T>(v:T,success:boolean):Result<T>{if(!success){return fail(errors.New("failure"));}return ok(v);}
export class Item{constructor(public value:int){}}
export interface View{function read():int;}
export function Produce<T>(produce:()=>T):T{return produce();}
export function CheckedProduce<T>(produce:()=>T,success:boolean):Result<T>{if(!success){return fail(errors.New("failure"));}return ok(produce());}
export function PairProduce<T>(produce:()=>T):(T,int){return produce(),7;}
export function Forward<T>(produce:()=>Result<T>):Result<T>{return produce();}
`,
		"entry.km": `import {Identity as Id,Pair,First,Checked,Item,Produce,CheckedProduce,PairProduce,Forward,View} from "./models";
import go slices from "slices";
import go {Compare as compare} from "cmp";
alias Callback=(value:int)=>int;
const global:Callback=Id;
constraint Mixed=Callback|int;
function mixed<T extends Mixed>(value:T):T{const f:(v:T)=>T=Id;return f(value);}
export function MixedInt(value:int):int{return mixed(value);}
export function MixedCallback():int{const f:Callback=(v)=>v+7;return mixed(f)(0);}
class Box{public callback:Callback=Id;}
function make():Callback{return Id;}
function apply<T>(callback:(value:T)=>T,value:T):T{return callback(value);}
function applyLast<T>(value:T,callback:(value:T)=>T):T{return callback(value);}
let trace=0;
function tick(value:int):int{trace=trace*10+value;return value;}
function outer<T>(value:T):T{const f:(v:T)=>T=Id;return f(value);}
export function Run(n:int):int[]{
 trace=0;
 let local:Callback=(value)=>value+100;local=Id;
 const callbacks:Callback[]=[Id,Id];const holder:{f:Callback}={f:Id};
 const pair:(a:int,b:bstring)=>(int,bstring)=Pair<int>;const [a,b]=pair(n,b"raw");
 const factory:()=>Callback=()=>Id;const first:(...values:int[])=>int=First;
 const values=[n+3,n,n+1];const clone:(values:int[])=>int[]=slices.Clone<int[]>;
 const copied=clone(values);copied[0]+=10;slices.SortFunc(values,compare);
 const x=apply(Id,tick(1));const y=applyLast(tick(2),Id);
 const task=go global(n+6);
 return [global(n),local(n),callbacks[1](n+1),holder.f(n+2),a,len(b),factory()(n+3),first(n+4,n+5),make()(n+5),new Box().callback(n+7),copied[0],values[0],x,y,trace,outer(n+8),await task];
}

export function Text(value:string):string{const f:(v:string)=>string=Id;return f(value);}
export function ResultValue(value:int,success:boolean):Result<int>{const f:(v:int,success:boolean)=>Result<int>=Checked;return f(value,success);}
export function Nullable(value:Item|null):Item|null{const f:(v:Item|null)=>Item|null=Id;return f(value);}
export function MakeIdentity():(value:Item)=>Item{return Id;}
export function MakeExplicit():(value:Item)=>Item{return Id<Item>;}
export function MakeProducer():(produce:()=>Item)=>Item{return Produce;}
export function MakeCheckedProducer():(produce:()=>Item,success:boolean)=>Result<Item>{return CheckedProduce;}
export function MakePairProducer():(produce:()=>Item)=>(Item,int){return PairProduce;}
export function MakeFirst():(...values:Item[])=>Item{return First;}
export function MakeForwarder():(produce:()=>Result<Item>)=>Result<Item>{return Forward;}
export function MakeViewIdentity():(value:View)=>View{return Id;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := []string{filepath.Join(root, "entry.km")}
	generated, ds, err := EmitGo(entry, "contextual")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	mapped, ds, err := EmitGoWithSourceMap(entry, "contextual")
	if err != nil || len(ds) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("source map: %v %v", err, ds)
	}
	reference := `package reference
import("errors";"slices";"cmp")
func id[T any](v T)T{return v}
func pair[T,U any](a T,b U)(T,U){return a,b}
func first[T any](vs ...T)T{return vs[0]}
func checked[T any](v T,success bool)(T,error){if !success{var zero T;return zero,errors.New("failure")};return v,nil}
func MixedInt(v int)int{return id(v)}
func MixedCallback()int{return id(func(v int)int{return v+7})(0)}
type callback=func(int)int
var global callback=id
type box struct{callback callback}
func makeCallback()callback{return id}
func apply[T any](callback func(T)T,value T)T{return callback(value)}
func applyLast[T any](value T,callback func(T)T)T{return callback(value)}
var trace int
func tick(v int)int{trace=trace*10+v;return v}
func outer[T any](v T)T{var f func(T)T=id;return f(v)}
func Run(n int)[]int{trace=0;local:=func(v int)int{return v+100};local=id;callbacks:=[]callback{id[int],id[int]};holder:=struct{f callback}{id[int]};var p func(int,string)(int,string)=pair[int];a,b:=p(n,"raw");var factory func()callback=func()callback{return id};var f func(...int)int=first;values:=[]int{n+3,n,n+1};var clone func([]int)[]int=slices.Clone[[]int];copied:=clone(values);copied[0]+=10;slices.SortFunc(values,cmp.Compare);x:=apply(id,tick(1));y:=applyLast(tick(2),id);task:=make(chan int,1);go func(){task<-global(n+6)}();return []int{global(n),local(n),callbacks[1](n+1),holder.f(n+2),a,len(b),factory()(n+3),f(n+4,n+5),makeCallback()(n+5),box{id[int]}.callback(n+7),copied[0],values[0],x,y,trace,outer(n+8),<-task}}
func Text(v string)string{var f func(string)string=id;return f(v)}
func ResultValue(v int,success bool)(int,error){var f func(int,bool)(int,error)=checked;return f(v,success)}
type Item struct{Value int}
type View interface{Read()int}
func MakeViewIdentity()func(View)View{return func(v View)View{if v==nil{panic("non-null")};return id(v)}}
func Nullable(v *Item)*Item{var f func(*Item)*Item=id;return f(v)}
func MakeIdentity()func(*Item)*Item{return func(v *Item)*Item{if v==nil{panic("non-null")};return id(v)}}
func MakeForwarder()func(func()(*Item,error))(*Item,error){return func(produce func()(*Item,error))(*Item,error){if produce==nil{panic("non-null")};v,e:=produce();if e==nil&&v==nil{panic("non-null")};return v,e}}
func MakeProducer()func(func()*Item)*Item{return func(produce func()*Item)*Item{if produce==nil{panic("non-null")};v:=produce();if v==nil{panic("non-null")};return v}}
func MakeCheckedProducer()func(func()*Item,bool)(*Item,error){return func(produce func()*Item,success bool)(*Item,error){if produce==nil{panic("non-null")};if !success{return nil,errors.New("failure")};return MakeProducer()(produce),nil}}
func MakePairProducer()func(func()*Item)(*Item,int){return func(produce func()*Item)(*Item,int){return MakeProducer()(produce),7}}
func MakeFirst()func(...*Item)*Item{return func(values ...*Item)*Item{for _,v:=range values{if v==nil{panic("non-null")}};return values[0]}}
`
	comparison := `package contextual_test
import("reflect";"testing";g "contextual-functions.test";r "contextual-functions.test/reference")
type view struct{}
func(*view)Read()int{return 7}
type typedError struct{}
func(*typedError)Error()string{return "typed error"}
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return false}
func TestValues(t *testing.T){
 for _,n:=range []int{-12,0,31}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal(n,a,b)};for _,success:=range []bool{false,true}{a,ae:=g.ResultValue(n,success);b,be:=r.ResultValue(n,success);if a!=b||(ae==nil)!=(be==nil){t.Fatal(a,ae,b,be)};if ae!=nil&&ae.Error()!=be.Error(){t.Fatal(ae,be)}}}
 for _,s:=range []string{"","hello","金木犀🍊"}{if g.Text(s)!=r.Text(s){t.Fatal(s)}}
 if g.MixedInt(0)!=r.MixedInt(0)||g.MixedInt(9)!=r.MixedInt(9)||g.MixedCallback()!=r.MixedCallback(){t.Fatal("mixed bounds")}
 if g.Nullable(nil)!=nil||r.Nullable(nil)!=nil{t.Fatal("absence")}
 a,b:=g.NewItem(7),&r.Item{Value:7};if g.Nullable(a)!=a||r.Nullable(b)!=b||g.MakeIdentity()(a)!=a||r.MakeIdentity()(b)!=b{t.Fatal("identity")}
 if a,b:=panics(func(){g.MakeIdentity()(nil)}),panics(func(){r.MakeIdentity()(nil)});a!=b||!a{t.Fatal("native contract",a,b)}
 checks:=[]struct{got,want func()}{
 {func(){g.MakeExplicit()(nil)},func(){r.MakeIdentity()(nil)}},
 {func(){g.MakeViewIdentity()(nil)},func(){r.MakeViewIdentity()(nil)}},
 {func(){g.MakeProducer()(nil)},func(){r.MakeProducer()(nil)}},
 {func(){g.MakeProducer()(func()*g.Item{return nil})},func(){r.MakeProducer()(func()*r.Item{return nil})}},
 {func(){g.MakeCheckedProducer()(func()*g.Item{return nil},true)},func(){r.MakeCheckedProducer()(func()*r.Item{return nil},true)}},
 {func(){g.MakePairProducer()(func()*g.Item{return nil})},func(){r.MakePairProducer()(func()*r.Item{return nil})}},
 {func(){g.MakeFirst()(a,nil)},func(){r.MakeFirst()(b,nil)}},
 {func(){g.MakeForwarder()(func()(*g.Item,error){return nil,nil})},func(){r.MakeForwarder()(func()(*r.Item,error){return nil,nil})}},
 }
 for i,c:=range checks{if x,y:=panics(c.got),panics(c.want);x!=y||!x{t.Fatal("boundary",i,x,y)}}
 if x,y:=g.MakeProducer()(func()*g.Item{return a}),r.MakeProducer()(func()*r.Item{return b});x!=a||y!=b{t.Fatal("producer identity")}
 if x,xe:=g.MakeCheckedProducer()(func()*g.Item{return nil},false);x!=nil||xe==nil{t.Fatal("failure zero",x,xe)}
 if y,ye:=r.MakeCheckedProducer()(func()*r.Item{return nil},false);y!=nil||ye==nil{t.Fatal("reference failure zero",y,ye)}
 if x,xn:=g.MakePairProducer()(func()*g.Item{return a});x!=a||xn!=7{t.Fatal("pair")}
 if x,y:=g.MakeFirst()(a),r.MakeFirst()(b);x!=a||y!=b{t.Fatal("rest")}
 v:=(*view)(nil);if g.MakeViewIdentity()(v)!=v||r.MakeViewIdentity()(v)!=v{t.Fatal("typed nil interface")}
 x,xe:=g.MakeForwarder()(func()(*g.Item,error){return nil,(*typedError)(nil)});y,ye:=r.MakeForwarder()(func()(*r.Item,error){return nil,(*typedError)(nil)});if x!=nil||y!=nil||xe==nil||ye==nil||xe.Error()!=ye.Error(){t.Fatal("typed nil error",x,xe,y,ye)}
}
`
	runGeneratedGoDifferentialTest(t, root, "contextual-functions.test", generated, reference, comparison)
}

func TestContextualFunctionValueRejectsPrivateGoArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":           "module contextual-private.test\n\ngo 1.23\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod":    "module example.com/native\n\ngo 1.23\n",
		"native/native.go": "package native\ntype hidden int\nfunc Accept(f func(hidden)hidden)int{return int(f(7))}\n",
		"entry.km":         `import go native from "example.com/native";function id<T>(v:T):T{return v;}const value=native.Accept(id);`,
	}
	for name, input := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, ds, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "contextual")
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range ds {
		if strings.Contains(diagnostic.Message, "inferred generic function type argument contains a private Go type") {
			return
		}
	}
	t.Fatalf("missing source diagnostic: %v", ds)
}
