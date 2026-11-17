package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNullCoalescingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `import go errors from "errors";
export interface Reader{function read():int;}
export class Box implements Reader{constructor(public n:int){}public function read():int{return this.n;}}
export class Child extends Box{constructor(n:int){super(n);}}
alias Action=(n:int)=>int;
alias DeferredAction=()=>void;
type Link=distinct *int;
type Chain<T>=distinct T[];
constraint PointerType=~*int;
export function Pointer(value:*int|null):int{let seed=9;const p:*int|null=value??&seed;return *p;}
export function Callback(value:Action|null):int{return (value??((n)=>n+3))(4);}
export function Slice(value:int[]|null):int[]{return value??[];}
export function MapValue(value:Map<string,int>|null):int{return len(value??makeMap<string,int>());}
export function Interface(value:Reader|null):int{return (value??new Box(9)).read();}
export function Upcast(value:Child|null):Reader{return value??new Box(9);}
export function Parent(value:Child|null):Box{return value??new Box(9);}
export function Nullable(value:Box|null):Box|null{return value??null;}
export function Error(value:error):bstring{return (value??errors.New("fallback")).Error();}
function defaultError():error{return errors.New("fallback");}
export function StaticError(value:error):bstring{return (value??defaultError()).Error();}
export function NamedPointer(value:Link|null,fallback:Link):Link{return value??fallback;}
export function NamedSlice(value:Chain<int>|null,fallback:Chain<int>):Chain<int>{return value??fallback;}
export function Generic<T extends PointerType>(value:T|null,fallback:T):T{return value??fallback;}
export function Shift(n:uint):byte{return null??(1<<n);}
export function Defaulted():int{return null??5;}
export function FalseValue():boolean{return null??false;}
export function Zero():int{return null??0;}
export function Empty():string{return null??"";}
export function NilPointer(value:*int):*int|null{return value??nil;}
export function FixedSize():int{let calls=0;const fallback=():*[3]int=>{calls++;let value:[3]int=[1,2,3];return &value;};const n=len(null??fallback());return calls*10+n;}
export function Ordered(present:boolean):int[]{
 let calls=0;let seed=7;
 const load=():*int|null=>{calls=calls*10+1;if(present){return &seed;}return null;};
 const fallback=():*int=>{calls=calls*10+2;seed=9;return &seed;};
 const value=load()??fallback();return [*value,calls];
}
export function Chained(a:*int|null,b:*int|null):int{let seed=5;return *(a??b??&seed);}
export function Channel(value:GoChannel<int>|null,fallback:GoChannel<int>):int{return <- (value??fallback);}
export function Awaited():int{const pending=go Defaulted();return null??await pending;}
export function Deferred(present:boolean):int[]{
 const events=makeSlice<int>(2);
 let index=0;
 const first=():void=>{events[index]=1;};
 const second=():void=>{events[index]=2;};
 let callback:DeferredAction|null=null;
 if(present){callback=first;}
 {defer (callback??second)();}
 events[index]=3;index++;return events;
}
export function NullableInterface(value:Box|null,other:Box|null):Reader|null{return value??other;}
`
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(entry, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "coalesce")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{entry}, "coalesce")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "errors"
type Reader interface{Read()int}
type Box struct{N int}
func(b *Box)Read()int{return b.N}
type Child struct{Box}
type Link *int
type Chain[T any][]T
func Pointer(v *int)int{if v==nil{seed:=9;v=&seed};return *v}
func Callback(v func(int)int)int{if v==nil{v=func(n int)int{return n+3}};return v(4)}
func Slice(v []int)[]int{if v==nil{return []int{}};return v}
func MapValue(v map[string]int)int{if v==nil{v=make(map[string]int)};return len(v)}
func Interface(v Reader)int{if v==nil{v=&Box{9}};return v.Read()}
func Upcast(v *Child)Reader{if v==nil{return &Box{9}};return v}
func Parent(v *Child)*Box{if v==nil{return &Box{9}};return &v.Box}
func Nullable(v *Box)*Box{return v}
func Error(v error)string{if v==nil{v=errors.New("fallback")};return v.Error()}
func StaticError(v error)string{return Error(v)}
func NamedPointer(v,f Link)Link{if v==nil{return f};return v}
func NamedSlice(v,f Chain[int])Chain[int]{if v==nil{return f};return v}
func Generic[T ~*int](v,f T)T{if v==nil{return f};return v}
func Shift(n uint)byte{return 1<<n}
func Defaulted()int{return 5}
func FalseValue()bool{return false}
func Zero()int{return 0}
func Empty()string{return ""}
func NilPointer(v *int)*int{return v}
func FixedSize()int{calls:=0;fallback:=func()*[3]int{calls++;return &[3]int{1,2,3}};n:=len(fallback());return calls*10+n}
func Ordered(p bool)[]int{calls:=0;seed:=7;load:=func()*int{calls=calls*10+1;if p{return &seed};return nil};fallback:=func()*int{calls=calls*10+2;seed=9;return &seed};v:=load();if v==nil{v=fallback()};return []int{*v,calls}}
func Chained(a,b *int)int{v:=a;if v==nil{v=b};if v==nil{seed:=5;v=&seed};return *v}
func Channel(v,f chan int)int{if v==nil{v=f};return <-v}
func Awaited()int{pending:=make(chan int,1);go func(){pending<-Defaulted()}();return <-pending}
func Deferred(p bool)[]int{events:=make([]int,2);index:=0;first:=func(){events[index]=1};second:=func(){events[index]=2};var callback func();if p{callback=first};if callback==nil{callback=second};defer callback();events[index]=3;index++;return events}
func NullableInterface(v,w *Box)Reader{if v==nil{v=w};if v==nil{return nil};return v}
`
	comparison := `package coalesce_test
import("errors";"reflect";"testing";g "coalesce.test";r "coalesce.test/reference")
type typedError struct{}
func(*typedError)Error()string{return "typed nil"}
func TestValues(t *testing.T){
 n:=11
 for _,v:=range []*int{nil,&n}{if got,want:=g.Pointer(v),r.Pointer(v);got!=want{t.Fatal(got,want)};if got,want:=g.NamedPointer(g.Link(v),g.Link(&n)),r.NamedPointer(r.Link(v),r.Link(&n));(*int)(got)!=(*int)(want){t.Fatal(got,want)};if got,want:=g.Generic[*int](v,&n),r.Generic[*int](v,&n);got!=want{t.Fatal(got,want)};if got,want:=g.NilPointer(v),r.NilPointer(v);got!=want{t.Fatal(got,want)};for _,w:=range []*int{nil,&n}{if got,want:=g.Chained(v,w),r.Chained(v,w);got!=want{t.Fatal(got,want)}}}
 for _,v:=range []func(int)int{nil,func(n int)int{return n*2}}{if got,want:=g.Callback(v),r.Callback(v);got!=want{t.Fatal(got,want)}}
 for _,v:=range [][]int{nil,{}, {1,2}}{if got,want:=g.Slice(v),r.Slice(v);!reflect.DeepEqual(got,want){t.Fatal(got,want)};if got,want:=g.NamedSlice(g.Chain[int](v),g.Chain[int]{9}),r.NamedSlice(r.Chain[int](v),r.Chain[int]{9});!reflect.DeepEqual([]int(got),[]int(want)){t.Fatal(got,want)}}
 for _,v:=range []map[string]int{nil,{}, {"x":1}}{if got,want:=g.MapValue(v),r.MapValue(v);got!=want{t.Fatal(got,want)}}
 var typedNil *typedError
 for _,v:=range []error{nil,typedNil,errors.New("provided")}{if got,want:=g.Error(v),r.Error(v);got!=want{t.Fatal(got,want)};if got,want:=g.StaticError(v),r.StaticError(v);got!=want{t.Fatal(got,want)}}
 for _,p:=range []bool{false,true}{var gv *g.Child;var rv *r.Child;var gi g.Reader;var ri r.Reader;if p{gv=g.NewChild(4);rv=&r.Child{Box:r.Box{N:4}};gi=gv;ri=rv};if got,want:=g.Upcast(gv).Read(),r.Upcast(rv).Read();got!=want{t.Fatal(got,want)};if got,want:=g.Parent(gv).N,r.Parent(rv).N;got!=want{t.Fatal(got,want)};if got,want:=g.Interface(gi),r.Interface(ri);got!=want{t.Fatal(got,want)};if got,want:=g.Ordered(p),r.Ordered(p);!reflect.DeepEqual(got,want){t.Fatal(got,want)};if got,want:=g.Deferred(p),r.Deferred(p);!reflect.DeepEqual(got,want){t.Fatal(got,want)}}
 for _,p:=range []bool{false,true}{for _,q:=range []bool{false,true}{var a,b *g.Box;var c,d *r.Box;if p{a=g.NewBox(3);c=&r.Box{N:3}};if q{b=g.NewBox(7);d=&r.Box{N:7}};got,want:=g.NullableInterface(a,b),r.NullableInterface(c,d);if(got==nil)!=(want==nil){t.Fatal(got,want)};if got!=nil&&got.Read()!=want.Read(){t.Fatal(got,want)}}}
 for _,n:=range []uint{0,7,8,63,64}{if got,want:=g.Shift(n),r.Shift(n);got!=want{t.Fatal(got,want)}}
 if got,want:=g.Defaulted(),r.Defaulted();got!=want{t.Fatal(got,want)};if got,want:=g.FalseValue(),r.FalseValue();got!=want{t.Fatal(got,want)};if got,want:=g.Zero(),r.Zero();got!=want{t.Fatal(got,want)};if got,want:=g.Empty(),r.Empty();got!=want{t.Fatal(got,want)};if got,want:=g.Awaited(),r.Awaited();got!=want{t.Fatal(got,want)}
 if got,want:=g.FixedSize(),r.FixedSize();got!=want{t.Fatal(got,want)}
 for _,p:=range []bool{false,true}{gc,rc:=make(chan int,1),make(chan int,1);gc<-9;rc<-9;var gv,rv chan int;if p{gv,rv=gc,rc};if got,want:=g.Channel(gv,gc),r.Channel(rv,rc);got!=want{t.Fatal(got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "coalesce.test", generated, reference, comparison)
}
