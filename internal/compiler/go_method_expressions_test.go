package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoMethodExpressionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module go-method-expressions.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
import "errors"
type Value struct{N int}
func(v Value)Add(n int)int{return v.N+n}
func(v *Value)Set(n int){v.N=n}
func(v Value)All(ns ...int)int{for _,n:=range ns{v.N+=n};return v.N}
func(v Value)Pair()(int,error){if v.N<0{return v.N,errors.New("negative")};return v.N,nil}
func(v *Value)Nil()bool{return v==nil}
type Reader interface{Add(int)int}
type Embedded struct{Value}
type Box[T any]struct{N T}
func(v Box[T])Get()T{return v.N}
type IntBox = Box[int]
func Apply(f func(Value,int)int,v Value,n int)int{return f(v,n)}
`,
		"helper.km": `import go { Value } from "go-method-expressions.test/contracts";
export function callback():(v:Value,n:int)=>int{return Value.Add;}`,
		"entry.km": `import {callback} from "./helper";
import go api from "go-method-expressions.test/contracts";
import go { IntBox } from "go-method-expressions.test/contracts";
import go bytes from "bytes";
export function Run(n:int):int[]{
 let v:api.Value=api.Value{N:n};
 const add=api.Value.Add;const set=(*api.Value).Set;
 const pointerAdd=(*api.Value).Add;const read=api.Reader.Add;
 const old=add(v,2);set(&v,n+3);
 const promoted=api.Embedded.Add;const embedded=api.Embedded{Value:v};
 const all=api.Value.All;const extras=[1,2,3];
 const pair=api.Value.Pair;const [value,pairFailure]=pair(v);
 const generic=IntBox.Get;
 let b:bytes.Buffer=bytes.Buffer{};const write=(*bytes.Buffer).WriteString;
 const [written,failure]=write(&b,"abc");
 return [old,pointerAdd(&v,4),read(v,5),promoted(embedded,6),all(v,extras...),value,generic(IntBox{N:n}),api.Apply(callback(),v,7),written];
}
export function NilPointer():boolean{const f=(*api.Value).Nil;return f(nil);}
export function NilValueWrapper():int{const f=(*api.Value).Add;return f(nil,1);}
export function NilInterface():int{const f=api.Reader.Add;return f(nil,1);}
export function Checked(n:int):Result<int>{const pair=api.Value.Pair;const value=pair(api.Value{N:n})?;return ok(value);}
export function PointerConversion(v:*api.Value):*api.Value{return (*api.Value)(v);}
export function Evaluation():int[]{
 let calls=0;let v:api.Value=api.Value{N:1};
 const receiver=():*api.Value=>{calls++;return &v;};
 const argument=():int=>{calls++;return calls;};
 (*api.Value).Set(receiver(),argument());
 defer (*api.Value).Set(&v,100);
 const task=go api.Value.Add(v,5);
 return [calls,v.N,await task];
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "methods")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("bytes";"errors")
type counter struct{number int}
func(v counter)add(n int)int{return v.number+n}
func(v *counter)set(n int){v.number=n}
func(v counter)all(ns ...int)int{for _,n:=range ns{v.number+=n};return v.number}
func(v counter)pair()(int,error){if v.number<0{return v.number,errors.New("negative")};return v.number,nil}
func(v *counter)isNil()bool{return v==nil}
type contract interface{add(int)int}
type embedded struct{counter}
type box[T any]struct{number T}
func(v box[T])get()T{return v.number}
type intBox=box[int]
func apply(f func(counter,int)int,v counter,n int)int{return f(v,n)}
func Run(n int)[]int{
 v:=counter{n};add:=counter.add;set:=(*counter).set;pointerAdd:=(*counter).add;read:=contract.add
 old:=add(v,2);set(&v,n+3);promoted:=embedded.add;composite:=embedded{v}
 all:=counter.all;extras:=[]int{1,2,3};pair:=counter.pair;value,_:=pair(v);generic:=intBox.get
 var b bytes.Buffer;write:=(*bytes.Buffer).WriteString;written,_:=write(&b,"abc")
 return []int{old,pointerAdd(&v,4),read(v,5),promoted(composite,6),all(v,extras...),value,generic(intBox{n}),apply(counter.add,v,7),written}
}
func NilPointer()bool{return (*counter).isNil(nil)}
func NilValueWrapper()int{return (*counter).add(nil,1)}
func NilInterface()int{return contract.add(nil,1)}
func Checked(n int)(int,error){pair:=counter.pair;v,e:=pair(counter{n});if e!=nil{return 0,e};return v,nil}
func Evaluation()[]int{
 calls:=0;v:=counter{1};receiver:=func()*counter{calls++;return &v};argument:=func()int{calls++;return calls}
 (*counter).set(receiver(),argument());defer (*counter).set(&v,100)
 result:=make(chan int,1);copy:=v;go func(){result<-counter.add(copy,5)}()
 return []int{calls,v.number,<-result}
}
`
	comparison := `package methods_test
import("reflect";"testing";api "go-method-expressions.test/contracts";g "go-method-expressions.test";r "go-method-expressions.test/reference")
func panics(f func()int)(did bool){defer func(){did=recover()!=nil}();f();return}
func TestMethods(t *testing.T){
 for _,n:=range []int{-10,0,17}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal("methods",n,a,b)}}
 for _,n:=range []int{-10,0,17}{a,ae:=g.Checked(n);b,be:=r.Checked(n);if a!=b||(ae==nil)!=(be==nil){t.Fatal("Result forwarding",a,ae,b,be)};if ae!=nil&&ae.Error()!=be.Error(){t.Fatal("error identity",ae,be)}}
 value:=api.Value{N:7};if g.PointerConversion(&value)!=&value{t.Fatal("pointer conversion identity")}
 if !g.NilPointer()||g.NilPointer()!=r.NilPointer(){t.Fatal("nil pointer method")}
 for _,pair:=range [][2]func()int{{g.NilValueWrapper,r.NilValueWrapper},{g.NilInterface,r.NilInterface}}{if !panics(pair[0])||!panics(pair[1]){t.Fatal("missing Go panic")}}
 if a,b:=g.Evaluation(),r.Evaluation();!reflect.DeepEqual(a,b){t.Fatal("evaluation",a,b)}
}
`
	runGeneratedGoDifferentialTest(t, root, "go-method-expressions.test", generated, reference, comparison)
}
