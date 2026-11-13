package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParameterBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module parameter-boundaries.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type Raw func()int
func Invoke(callback func(func()int)int)int{return callback(nil)}
`,
		"models.km": `export class Item{constructor(public n:int){}}
export class Box<T>{constructor(public value:T){}}
export alias Callback=()=>int;
export type Named=distinct ()=>int;
export alias Identity<T>=(value:T)=>T;
export interface View{function read():int;}
`,
		"entry.km": `import go native from "example.com/native";
import {Item,Box,Callback,Named,Identity,View} from "./models";
let state=0;
function mark():int{state+=1;return state;}
export function Reset():void{state=0;}
export function State():int{return state;}
export function Entry(value:Item):int{mark();return 7;}
export function Fn(value:Callback):int{mark();return 7;}
export function Defined(value:Named):int{mark();return 7;}
export function GenericAlias(value:Identity<int>):int{mark();return 7;}
export function Interface(value:View):int{mark();return 7;}
export function Anonymous(value:interface{read():int;}):int{mark();return 7;}
export function Nullable(value:Item|null):boolean{mark();return value===null;}
export function Raw(value:*int,callback:native.Raw,failure:error,anything:interface{}):int{mark();return 7;}
export function Rest(...value:Item[]):int{mark();return len(value);}
export function RestFn(...value:Callback[]):int{mark();return len(value);}
alias NullableItem=Item|null;
export function NullableRest(...value:NullableItem[]):int{mark();return len(value);}
export function Blank(_:Callback):int{mark();return 7;}
export function Escaped(panic:Callback):int{mark();return 7;}
export function Arrow():(value:Item)=>int{return (value:Item):int=>mark();}
export function BlockArrow():(value:Item)=>int{return (value:Item):int=>{mark();return 7;};}
export function InferredArrow():(value:Item)=>int{return (value)=>{mark();return 7;};}
export function RestArrow():(...value:Item[])=>int{return (...value:Item[]):int=>{mark();return len(value);};}
export function Invoked():int{return native.Invoke((value:Callback):int=>{mark();return 7;});}
export function RawInvoked():int{return native.Invoke((value)=>{mark();if(value===nil){return 7;}return value();});}
constraint Functions=Callback;
export function Bound<T extends Functions>(value:T):int{mark();return 7;}
constraint References=Item|Callback;
export function UnionBound<T extends References>(value:T):int{mark();return 7;}
constraint Mixed=Callback|*int;
export function MixedBound<T extends Mixed>(value:T):int{mark();return 7;}
constraint ScalarMixed=Callback|int;
export function ScalarBound<T extends ScalarMixed>(value:T):int{mark();return 7;}
export function GenericClass<T>(value:Box<T>):int{mark();return 7;}
export function GenericArrow<T>():(value:Box<T>)=>int{return (value)=>{mark();return 7;};}
export class Host{
 public function entry(value:Item):int{mark();return 7;}
 public static function staticEntry(value:Item):int{mark();return 7;}
 public function generic<T>(value:Box<T>):int{mark();return 7;}
 public set item(value:Item){mark();}
}
export struct Record{public function entry(value:Item):int{mark();return 7;}}
export enum Status{Ready}
public function entry(this:Status,value:Item):int{mark();return 7;}
export type Number=distinct int;
public function entry(this:Number,value:Item):int{mark();return 7;}
export class Construct{public initial:int=mark();constructor(public value:Item){mark();}}
export class Base{constructor(){mark();}}
export class Derived extends Base{constructor(value:Item){super();mark();}}
export function Finally(value:Item):void{try{mark();}finally{mark();}}
function pair(first:Callback,later:int):void{mark();}
export function PanicOrder():void{const values:int[]=[];const raw:native.Raw=nil;pair(raw,values[0]);}
export function ArgumentOrder(visit:()=>int):void{const raw:native.Raw=nil;pair(raw,visit());}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "parameterboundaries")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "parameterboundaries")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
var state int
func Reset(){state=0}
func State()int{return state}
func Accept(present bool)int{if !present{panic("nil parameter")};state++;return 7}
func Rest(present []bool)int{for _,p:=range present{if !p{panic("nil parameter")}};state++;return len(present)}
func Raw()int{state++;return 7}
`
	comparison := `package parameterboundaries_test
import("fmt";"strings";"testing";g "parameter-boundaries.test";r "parameter-boundaries.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
type view struct{}
func(*view)Read()int{return 7}
func TestRejectBeforeBody(t *testing.T){host:=g.NewHost();record:=g.Record{};status:=g.StatusReady;number:=g.Number(0)
 tests:=map[string]func(){
 "function":func(){g.Entry(nil)},"function signature":func(){g.Fn(nil)},"defined function":func(){g.Defined(nil)},"generic alias":func(){g.GenericAlias(nil)},"interface":func(){g.Interface(nil)},"anonymous":func(){g.Anonymous(nil)},
 "arrow":func(){g.Arrow()(nil)},"block arrow":func(){g.BlockArrow()(nil)},"contextual arrow":func(){g.InferredArrow()(nil)},"Go invokes callback":func(){g.Invoked()},
 "bound":func(){g.Bound[func()int](nil)},"class union bound":func(){g.UnionBound[*g.Item](nil)},"function union bound":func(){g.UnionBound[func()int](nil)},"mixed function bound":func(){g.MixedBound[func()int](nil)},"scalar mixed function bound":func(){g.ScalarBound[func()int](nil)},"generic class":func(){g.GenericClass[int](nil)},"generic arrow":func(){g.GenericArrow[int]()(nil)},
 "method":func(){host.Entry(nil)},"static":func(){g.HostStaticEntry(nil)},"generic method":func(){g.HostGeneric[int](host,nil)},"struct":func(){record.Entry(nil)},"enum":func(){status.Entry(nil)},"defined method":func(){number.Entry(nil)},
 "constructor":func(){g.NewConstruct(nil)},"derived super":func(){g.NewDerived(nil)},"escaped builtin":func(){g.Escaped(nil)},"setter":func(){host.SetItem(nil)},"finally":func(){g.Finally(nil)},
 }
 for name,call:=range tests{t.Run(name,func(t *testing.T){g.Reset();r.Reset();gp:=capture(call);rp:=capture(func(){r.Accept(false)});if(gp==nil)!=(rp==nil)||g.State()!=r.State(){t.Fatal(gp,rp,g.State(),r.State())};if !strings.Contains(fmt.Sprint(gp),"non-null parameter")||!strings.Contains(fmt.Sprint(gp),"entry.km:"){t.Fatal(gp)}})}
 if got:=fmt.Sprint(capture(func(){g.Entry(nil)}));got!="non-null parameter \"value\" failed at entry.km:7:23"{t.Fatal(got)}
}
func TestAccept(t *testing.T){item:=g.NewItem(1);callback:=func()int{return 7};view:=(*view)(nil)
 for name,call:=range map[string]func()int{"function":func()int{return g.Entry(item)},"signature":func()int{return g.Fn(callback)},"defined":func()int{return g.Defined(callback)},"alias":func()int{return g.GenericAlias(func(n int)int{return n})},"typed nil interface":func()int{return g.Interface(view)},"anonymous typed nil":func()int{return g.Anonymous(view)},"bound":func()int{return g.Bound[func()int](callback)},"generic class":func()int{return g.GenericClass(g.NewBox(1))}}{t.Run(name,func(t *testing.T){g.Reset();r.Reset();if got,want:=call(),r.Accept(true);got!=want||g.State()!=r.State(){t.Fatal(got,want,g.State(),r.State())}})}
}
func TestRawAndNullable(t *testing.T){for name,call:=range map[string]func()int{"raw":func()int{return g.Raw(nil,nil,nil,nil)},"nullable":func()int{if !g.Nullable(nil){t.Fatal("lost null")};return 7},"raw context":g.RawInvoked,"mixed pointer bound":func()int{return g.MixedBound[*int](nil)},"scalar mixed bound":func()int{return g.ScalarBound[int](0)},"blank":func()int{return g.Blank(nil)}}{t.Run(name,func(t *testing.T){g.Reset();r.Reset();if got,want:=call(),r.Raw();got!=want||g.State()!=r.State(){t.Fatal(got,want,g.State(),r.State())}})}}
func TestArgumentOrder(t *testing.T){
 g.Reset();failure:=capture(g.PanicOrder);if !strings.Contains(fmt.Sprint(failure),"index out of range")||g.State()!=0{t.Fatal(failure,g.State())}
 count:=0;failure=capture(func(){g.ArgumentOrder(func()int{count++;return 7})});if !strings.Contains(fmt.Sprint(failure),"non-null parameter")||count!=1||g.State()!=0{t.Fatal(failure,count,g.State())}
}
func TestRest(t *testing.T){item:=g.NewItem(1);for _,values:=range [][]*g.Item{nil,{}, {item},{item,nil},{nil,item}}{g.Reset();r.Reset();present:=make([]bool,len(values));for i,v:=range values{present[i]=v!=nil};var got,want int;gp:=capture(func(){got=g.Rest(values...)});rp:=capture(func(){want=r.Rest(present)});if(gp==nil)!=(rp==nil)||got!=want||g.State()!=r.State(){t.Fatal(gp,rp,got,want,g.State(),r.State())}}
 for _,call:=range []func(){func(){g.RestArrow()(nil)},func(){g.RestFn(func()int{return 7},nil)}}{g.Reset();if capture(call)==nil||g.State()!=0{t.Fatal("rest body executed")}}
 if got:=g.NullableRest(nil);got!=1{t.Fatal(got)}
}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "parameter-boundaries.test", generated, reference, comparison, nil, nil)
}
