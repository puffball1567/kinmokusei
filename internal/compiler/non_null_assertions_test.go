package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNonNullAssertionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	models := `export class Box<T>{constructor(public value:T){}}
export interface Reader{function read():int;}
export class Counter implements Reader{constructor(public value:int){} public function read():int{return this.value;}}
export alias MaybeBox=Box<int>|null;
`
	input := `import {Box,Counter,Reader,MaybeBox} from "./models";
import go flag from "flag";
import go errors from "errors";
alias Action=()=>boolean;
type Link=distinct *int;
type Chain<T>=distinct T[];
constraint PointerType=~*int;
export function ClassValue(present:boolean):int{let value:MaybeBox=null;if(present){value=new Box<int>(7);}return value!.value;}
export function Pointer(value:*int|null):int{return *value!;}
export function Slice(value:int[]|null):int{return len(value!);}
export function MapSize(value:Map<string,int>|null):int{return len(value!);}
export function Callback(value:Action|null):boolean{return !value!();}
export function Interface(value:Reader|null):int{return value!.read();}
export function ErrorText(value:error|null):bstring{return value!.Error();}
export function RawGo(value:*flag.Flag):bstring{return value!.Name;}
export function FixedLen(value:*[3]int|null):int{return len(value!);}
export function FixedCap(value:*[3]int|null):int{return cap(value!);}
export function NamedPointer(value:Link|null):int{return *value!;}
export function NamedSlice(value:Chain<int>|null):int{return len(value!);}
export function Generic<T extends PointerType>(value:T|null):T{return value!;}
export function Receive(value:GoChannel<int>|null):int{return <-value!;}
export function Ordered(present:boolean,visit:(step:int)=>void):int{
 let seed=7;
 const load=():*int|null=>{visit(1);if(present){return &seed;}return null;};
 const next=():int=>{visit(2);return 3;};
 return *load()!+next();
}
export function Shadow(value:*int|null,panic:()=>void):int{return *value!;}
export function Snapshot(value:*int|null):int{const checked:*int|null=value!;return *checked;}
export function Finally(value:*int|null,visit:()=>void):int{try{return *value!;}finally{visit();}}
function obtain(present:boolean,failed:boolean):Result<*int|null>{
 if(failed){return fail(errors.New("failure"));}
 let seed=4;if(present){return ok(&seed);}return ok(null);
}
export function WithResult(present:boolean,failed:boolean):Result<int>{const value=obtain(present,failed)?;return ok(*value!);}
export function NamedTypeParameter<value extends PointerType>(p:value|null):value{return p!;}
`
	for name, contents := range map[string]string{"models.km": models, "entry.km": input} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "nonnull")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "nonnull")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte(root)) {
		t.Fatal("assertion messages leaked an absolute build path")
	}
	reference := `package reference
import("flag";"errors")
type Box[T any]struct{Value T}
type Reader interface{Read()int}
type Counter struct{Value int}
func(c *Counter)Read()int{return c.Value}
type Link *int
type Chain[T any][]T
func ClassValue(present bool)int{var value *Box[int];if present{value=&Box[int]{7}};if value==nil{panic("null")};return value.Value}
func Pointer(value *int)int{if value==nil{panic("null")};return *value}
func Slice(value []int)int{if value==nil{panic("null")};return len(value)}
func MapSize(value map[string]int)int{if value==nil{panic("null")};return len(value)}
func Callback(value func()bool)bool{if value==nil{panic("null")};return !value()}
func Interface(value Reader)int{if value==nil{panic("null")};return value.Read()}
func ErrorText(value error)string{if value==nil{panic("null")};return value.Error()}
func RawGo(value *flag.Flag)string{if value==nil{panic("null")};return value.Name}
func FixedLen(value *[3]int)int{if value==nil{panic("null")};return len(value)}
func FixedCap(value *[3]int)int{if value==nil{panic("null")};return cap(value)}
func NamedPointer(value Link)int{if value==nil{panic("null")};return *value}
func NamedSlice(value Chain[int])int{if value==nil{panic("null")};return len(value)}
func Generic[T ~*int](value T)T{if value==nil{panic("null")};return value}
func Receive(value chan int)int{if value==nil{panic("null")};return <-value}
func Ordered(present bool,visit func(int))int{
 number:=7;load:=func()*int{visit(1);if present{return &number};return nil};next:=func()int{visit(2);return 3}
 checked:=func()*int{v:=load();if v==nil{panic("null")};return v}()
 return *checked+next()
}
func Shadow(value *int,panicArg func())int{if value==nil{panic("null")};return *value}
func Snapshot(value *int)int{if value==nil{panic("null")};return *value}
func Finally(value *int,visit func())int{defer visit();if value==nil{panic("null")};return *value}
func WithResult(present,failed bool)(int,error){if failed{return 0,errors.New("failure")};var value *int;if present{seed:=4;value=&seed};if value==nil{panic("null")};return *value,nil}
func NamedTypeParameter[value ~*int](p value)value{if p==nil{panic("null")};return p}
`
	tests := `package nonnull_test
import("errors";"flag";"fmt";"reflect";"strings";"testing";g "non-null.test";r "non-null.test/reference")
func capture(call func())(failure any){defer func(){failure=recover()}();call();return nil}
func compare(t *testing.T,got,want func()){
 t.Helper();gp,rp:=capture(got),capture(want)
 if (gp==nil)!=(rp==nil){t.Fatalf("panic differs: %v / %v",gp,rp)}
 if gp!=nil&&!strings.Contains(fmt.Sprint(gp),"non-null assertion failed at entry.km:"){t.Fatalf("bad assertion location: %v",gp)}
}
type typedError struct{}
func (*typedError)Error()string{return "typed nil accepted as non-nil interface"}
func TestAssertions(t *testing.T){
 for _,present:=range []bool{false,true}{compare(t,func(){if got:=g.ClassValue(present);got!=7{t.Fatal(got)}},func(){if want:=r.ClassValue(present);want!=7{t.Fatal(want)}})}
 n:=9
 for _,value:=range []*int{nil,&n}{compare(t,func(){g.Pointer(value)},func(){r.Pointer(value)});compare(t,func(){g.Snapshot(value)},func(){r.Snapshot(value)});compare(t,func(){g.Shadow(value,func(){t.Fatal("shadowed panic called")})},func(){r.Shadow(value,nil)});compare(t,func(){g.NamedPointer(g.Link(value))},func(){r.NamedPointer(r.Link(value))});compare(t,func(){g.Generic[*int](value)},func(){r.Generic[*int](value)})
  if value!=nil{if got,want:=g.Pointer(value),r.Pointer(value);got!=want{t.Fatal(got,want)};if got,want:=g.Generic[*int](value),r.Generic[*int](value);got!=want{t.Fatal(got,want)}}
  gf,rf:=0,0;compare(t,func(){g.Finally(value,func(){gf++})},func(){r.Finally(value,func(){rf++})});if gf!=1||gf!=rf{t.Fatal("finally",gf,rf)}
  compare(t,func(){g.NamedTypeParameter[*int](value)},func(){r.NamedTypeParameter[*int](value)})
 }
 for _,value:=range [][]int{nil,{}, {1,2}}{compare(t,func(){g.Slice(value)},func(){r.Slice(value)});compare(t,func(){g.NamedSlice(g.Chain[int](value))},func(){r.NamedSlice(r.Chain[int](value))});if value!=nil{if got,want:=g.Slice(value),r.Slice(value);got!=want{t.Fatal(got,want)}}}
 for _,value:=range []map[string]int{nil,{}, {"x":1}}{compare(t,func(){g.MapSize(value)},func(){r.MapSize(value)});if value!=nil{if got,want:=g.MapSize(value),r.MapSize(value);got!=want{t.Fatal(got,want)}}}
 for _,value:=range []func()bool{nil,func()bool{return true},func()bool{return false}}{compare(t,func(){g.Callback(value)},func(){r.Callback(value)});if value!=nil{if got,want:=g.Callback(value),r.Callback(value);got!=want{t.Fatal(got,want)}}}
 compare(t,func(){g.Interface(nil)},func(){r.Interface(nil)});if got,want:=g.Interface(g.NewCounter(11)),r.Interface(&r.Counter{11});got!=want{t.Fatal(got,want)}
 var typedNil *typedError
 for _,value:=range []error{nil,errors.New("error"),typedNil}{compare(t,func(){g.ErrorText(value)},func(){r.ErrorText(value)});if value!=nil{if got,want:=g.ErrorText(value),r.ErrorText(value);got!=want{t.Fatal(got,want)}}}
 for _,value:=range []*flag.Flag{nil,{Name:"known"}}{compare(t,func(){g.RawGo(value)},func(){r.RawGo(value)});if value!=nil{if got,want:=g.RawGo(value),r.RawGo(value);got!=want{t.Fatal(got,want)}}}
 for _,value:=range []*[3]int{nil,{1,2,3}}{compare(t,func(){g.FixedLen(value)},func(){r.FixedLen(value)});compare(t,func(){g.FixedCap(value)},func(){r.FixedCap(value)});if value!=nil{if got,want:=g.FixedLen(value),r.FixedLen(value);got!=want{t.Fatal(got,want)}}}
 compare(t,func(){g.Receive(nil)},func(){r.Receive(nil)})
 gc,rc:=make(chan int,2),make(chan int,2);gc<-7;gc<-8;rc<-7;rc<-8;if got,want:=g.Receive(gc),r.Receive(rc);got!=want||len(gc)!=1||len(rc)!=1{t.Fatal(got,want,len(gc),len(rc))}
 for _,present:=range []bool{false,true}{var ge,re []int;compare(t,func(){g.Ordered(present,func(v int){ge=append(ge,v)})},func(){r.Ordered(present,func(v int){re=append(re,v)})});if !reflect.DeepEqual(ge,re){t.Fatal(ge,re)}}
 for _,present:=range []bool{false,true}{for _,failed:=range []bool{false,true}{compare(t,func(){g.WithResult(present,failed)},func(){r.WithResult(present,failed)});if present||failed{gv,ge:=g.WithResult(present,failed);rv,re:=r.WithResult(present,failed);if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Fatal(gv,ge,rv,re)}}}}
 if message:=fmt.Sprint(capture(func(){g.Pointer(nil)}));message!="non-null assertion failed at entry.km:9:59"{t.Fatal(message)}
}
`
	runGeneratedGoDifferentialTest(t, root, "non-null.test", generated, reference, tests)
}
