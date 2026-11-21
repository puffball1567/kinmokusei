package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeStructGoInterfacesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module struct-go-interfaces.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Getter[T any] interface{Get() T}
func Read[T any](v Getter[T])T{return v.Get()}
func ReadPair[T any](v Getter[T])(T,int){return v.Get(),1}
func Convert[U,T any](v Getter[T], f func(T)U)U{return f(v.Get())}
func Sum[T ~int](values ...Getter[T])T{var x T;for _,v:=range values{x+=v.Get()};return x}
func Take[T any](v T)T{return v}
func Combine[T any](v Getter[T], fallback T)T{_ = fallback;return v.Get()}
type Callback func(int)int
type Caller interface{Call(Callback)int}
`,
		"models.km": `import go errors from "errors";
export struct Counter {
 public count:int;
 public pointer function write(data:byte[]):Result<int> {
  if(len(data)==0){return fail(errors.New("empty"));}
  this.count+=len(data);
  return ok(len(data));
 }
}
export struct Getter<T>{public value:T;public function get():T{return this.value;}}
export struct PointerGetter<T>{public value:T;public pointer function get():T{return this.value;}}
export struct Tag{public value:bstring; public function string():bstring{return this.value;}}
`,
		"entry.km": `import {Counter,Getter,PointerGetter,Tag} from "./models";
import go fmt from "fmt";
import go io from "io";
import go contracts from "struct-go-interfaces.test/contracts";
struct ValueWriter{Counter;}
struct SharedWriter{*Counter;}
struct CallbackRunner{public function call(fn:contracts.Callback):int{return fn(5);}}
struct GetterView{Getter<int>;}
struct SharedGetter{*PointerGetter<int>;}
struct Count{public value:int;}
function run(w:io.Writer,data:byte[]):(int,error){return w.Write(data);}
function input(w:*ValueWriter,data:byte[]):(*ValueWriter,byte[]){return w,data;}
function getters(a:Getter<int>,b:GetterView):(Getter<int>,GetterView){return a,b;}
function tagged(value:bstring):(Tag,int){return Tag{value:value},3;}
export function WrapTag(value:bstring):(fmt.Stringer,int){return tagged(value);}
export function Counts(data:byte[]):int[]{
 let value=ValueWriter{Counter:Counter{count:10}};
 let counter=Counter{count:20};
 const shared=SharedWriter{Counter:&counter};
 const [a,e]=run(input(&value,data));
 const w=io.Writer(shared);
 const [b,f]=w.Write(data);
 const [s,ok]=w as? SharedWriter;
 if(!ok){return [];}
 let selected=0;
 switch(w){
 case const writer as SharedWriter { selected=writer.count; }
 case const _ as *ValueWriter { selected=-1; }
 default { selected=-2; }
 }
 if(e!=nil && f!=nil){return [value.count,counter.count,selected,len(e.Error()),len(f.Error())];}
 return [a,b,value.count,counter.count,s.count,selected];
}
export function Generic(n:int):int{
 const value=Getter<int>{value:n};
 const a:contracts.Getter<int>=value;
 return a.Get()+contracts.Read(a);
}
export function GenericDirect(n:int):int[]{
 const v=Getter<int>{value:n};
 const view=GetterView{Getter:v};
 let pointer=PointerGetter<int>{value:n+1};
 const shared=SharedGetter{PointerGetter:&pointer};
 const own=Count{value:n+3};
 const ownGetter=Getter<Count>{value:own};
 const [count,t]=contracts.ReadPair(ownGetter);
 const all:contracts.Getter<int>[]=[v,view,shared];
 return [contracts.Read(v),contracts.Read<int>(view),contracts.Read(&pointer),contracts.Read(shared),
  contracts.Convert<int>(shared,(x)=>x*2),contracts.Sum(v,view,shared),contracts.Sum(all...),
  contracts.Take<contracts.Getter<int>>(v).Get(),contracts.Sum(getters(v,view)),
  count.value+t,contracts.Read(ownGetter).value,contracts.Combine(ownGetter,own).value];
}
export function Callback():int{const c:contracts.Caller=CallbackRunner{};return c.Call((n:int)=>n+1);}
`,
		"bad.km": `import go contracts from "struct-go-interfaces.test/contracts";
struct Wrong{ public function call(fn:(x:int)=>int):int{return fn(5);} }
function wrong():contracts.Caller{return Wrong{};}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "structgointerfaces")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	_, diagnostics, err = EmitGo([]string{filepath.Join(root, "bad.km")}, "structgointerfaces")
	if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "cannot use Wrong") {
		t.Fatalf("distinct callback type: %v %v", err, diagnostics)
	}
	for _, example := range []struct{ name, body, want string }{
		{"wrong-interface-type", `const x=contracts.Read<int64>(Getter<int>{value:2});`, "cannot apply explicit Go type arguments"},
		{"conflicting-inference", `const x=contracts.Combine(Getter<int>{value:2},int64(3));`, "cannot infer Go type arguments"},
		{"pointer-receiver", `const x=contracts.Read(PointerGetter<int>{value:2});`, "cannot infer Go type arguments"},
		{"incompatible-type-set", `const x=contracts.Sum(Getter<int64>{value:2});`, "cannot infer Go type arguments"},
		{"spread-preserves-slice-type", `const items:Getter<int>[]=[Getter<int>{value:2}];const x=contracts.Sum<int>(items...);`, "cannot apply explicit Go type arguments"},
		{"verified-signature", `const x=contracts.Read(Getter<string>{value:"x"});`, "cannot use Getter<string>"},
	} {
		t.Run(example.name, func(t *testing.T) {
			path := filepath.Join(root, example.name+".km")
			input := `import {Getter,PointerGetter} from "./models";import go contracts from "struct-go-interfaces.test/contracts";
function reject():void{` + example.body + `}`
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitGo([]string{path}, "structgointerfaces")
			if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, example.want) {
				t.Fatalf("generic argument diagnostics: %v %v", err, diagnostics)
			}
		})
	}
	reference := `package reference
import("errors";"fmt";"io")
type counter struct{count int}
func(c *counter)Write(p []byte)(int,error){if len(p)==0{return 0,errors.New("empty")};c.count+=len(p);return len(p),nil}
type valueWriter struct{counter}
type sharedWriter struct{*counter}
type tag string
func(t tag)String()string{return string(t)}
func WrapTag(s string)(fmt.Stringer,int){return tag(s),3}
func run(w io.Writer,data []byte)(int,error){return w.Write(data)}
func Counts(data []byte)[]int{
 v:=valueWriter{counter{10}};c:=counter{20};shared:=sharedWriter{&c}
 a,e:=run(&v,data);w:=io.Writer(shared);b,f:=w.Write(data)
 s,ok:=w.(sharedWriter);if !ok{return []int{}}
 selected:=0
 switch writer:=w.(type){case sharedWriter:selected=writer.count;case *valueWriter:selected=-1;default:selected=-2}
 if e!=nil&&f!=nil{return []int{v.count,c.count,selected,len(e.Error()),len(f.Error())}}
 return []int{a,b,v.count,c.count,s.count,selected}
}
type getter[T any] struct{ value T }
func(v getter[T])Get()T{return v.value}
type getterContract[T any] interface{Get() T}
func read[T any](v getterContract[T])T{return v.Get()}
func pair[T any](v getterContract[T])(T,int){return v.Get(),1}
func combine[T any](v getterContract[T], fallback T)T{_ = fallback;return v.Get()}
func Generic(n int)int{v:=getter[int]{n};var a getterContract[int]=v;return a.Get()+read[int](v)}
type pointerGetter[T any] struct{ value T }
func(v *pointerGetter[T])Get()T{return v.value}
type getterView struct{getter[int]}
type sharedGetter struct{*pointerGetter[int]}
type count struct{value int}
func convert[U,T any](v getterContract[T],f func(T)U)U{return f(v.Get())}
func sum[T ~int](values ...getterContract[T])T{var x T;for _,v:=range values{x+=v.Get()};return x}
func take[T any](v T)T{return v}
func GenericDirect(n int)[]int{
 v:=getter[int]{n};view:=getterView{v};p:=pointerGetter[int]{n+1};shared:=sharedGetter{&p}
 own:=count{n+3};ownGetter:=getter[count]{own};c,t:=pair(ownGetter)
 return []int{read(v),read[int](view),read(&p),read(shared),convert[int](shared,func(x int)int{return x*2}),sum(v,view,shared),sum([]getterContract[int]{v,view,shared}...),take[getterContract[int]](v).Get(),sum(v,view),c.value+t,read(ownGetter).value,combine(ownGetter,own).value}
}
type intCallback func(int)int
type caller interface{Call(intCallback)int}
type callbackRunner struct{}
func(callbackRunner)Call(fn intCallback)int{return fn(5)}
func Callback()int{var c caller=callbackRunner{};return c.Call(func(n int)int{return n+1})}
`
	comparison := `package structgointerfaces_test
import("reflect";"testing";g "struct-go-interfaces.test";r "struct-go-interfaces.test/reference")
func TestStructInterfaces(t *testing.T){
 for _,data:=range [][]byte{nil,{}, {1}, {3,4,5}}{
  if got,want:=g.Counts(data),r.Counts(data);!reflect.DeepEqual(got,want){t.Fatal(data,got,want)}
 }
 for _,label:=range []string{"","text","\xff"}{got,n:=g.WrapTag(label);want,m:=r.WrapTag(label);if got.String()!=want.String()||n!=m{t.Fatal(label,got,n,want,m)}}
 for _,n:=range []int{-17,0,9}{
  if g.Generic(n)!=r.Generic(n){t.Fatal(n)}
  if got,want:=g.GenericDirect(n),r.GenericDirect(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}
 }
 if g.Callback()!=r.Callback(){t.Fatal("named callback")}
}
`
	runGeneratedGoDifferentialTest(t, root, "struct-go-interfaces.test", generated, reference, comparison)
}
