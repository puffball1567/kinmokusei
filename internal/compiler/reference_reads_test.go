package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReferenceReadsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "reads.km")
	input := `import go reflect from "reflect";
export class Item{constructor(public n:int){}}
export interface View{function read():int;}
export alias Callback=()=>int;
export class Node{constructor(public item:Item,public view:View,public callback:Callback){}public next:Node|null=null;}
export struct Row{public item:Item;public view:View;public callback:Callback;}
let state=0;
export function Reset():void{state=0;}
export function State():int{return state;}
export let Supply:()=>Item=()=>new Item(7);
export let FunctionSupply:()=>Callback=()=>()=>7;
export let Current:Item=new Item(7);
export function Field(node:Node):int{return node.item.n;}
export function InterfaceField(node:Node):int{return node.view.read();}
export function CallbackField(node:Node):int{return node.callback();}
export function Copy(row:Row):int{const item=row.item;state++;return item.n;}
export function Slice(items:Item[]):int{const item=items[0];state++;return item.n;}
export function Nested(rows:Row[]):int{return rows[0].item.n;}
export function Range(items:Item[]):int{for(const item of items){return item.n;}return 0;}
export function RangeCallback(items:Callback[]):int{for(const item of items){return item();}return 0;}
export function Unused():void{const item=Supply();state++;}
export function Inferred():int{const item=Supply();state++;return item.n;}
export function InferredFunction():int{const callback=FunctionSupply();state++;return callback();}
export function For():void{for(const item=Supply();false;){state++;}}
export function Shadow():int{const Item=3;const item=Supply();state++;return item.n+Item;}
export function Global():int{return Current.n;}
export function Escape(replace:boolean):int{let item=new Item(7);const storage=reflect.ValueOf(&item).Elem();if(replace){storage.Set(reflect.Zero(storage.Type()));}return item.n;}
export function Address(row:Row):boolean{const pointer=&row.item;return *pointer===row.item;}
export function Cycle(root:Node):int{const next=root.next;if(next===null){return 0;}return next.item.n;}
export function Optional(node:Node|null):int{return node?.next?.item.n??0;}
export function Discard():void{_=Supply();state++;}
export function PairDiscard():void{const [_,err]=source();if(err!==nil){return;}state++;}
function source():Result<Item>{return ok(Supply());}
export class Arrays{public items:[2]int=[1,2];}
export let ArraySource:()=>Arrays=()=>new Arrays();
export function Length():int{const n=len(ArraySource().items);state++;return n;}
export function Capacity():int{const n=cap(ArraySource().items);state++;return n;}
export function Switched(value:interface{}):int{switch(value){case const callback as Callback{return callback();}default{return 0;}}}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, ds, err := EmitGo([]string{path}, "reads")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	mapped, ds, err := EmitGoWithSourceMap([]string{path}, "reads")
	if err != nil || len(ds) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("map: %v %v", err, ds)
	}
	reference := `package reference
type Item struct{N int}
func Read(value *Item)int{if value==nil{panic("non-null reference read")};return value.N}
func Callback(fn func()int)int{if fn==nil{panic("non-null reference read")};return fn()}
`
	comparison := `package reads_test
import("fmt";"strings";"testing";g "reference-reads.test";r "reference-reads.test/reference")
type view struct{}
func(*view)Read()int{return 7}
func capture(call func())(failure any){defer func(){failure=recover()}();call();return nil}
func TestReads(t *testing.T){
 good:=g.NewItem(7);goodFn:=func()int{return 7};goodView:=(*view)(nil);node:=g.NewNode(good,goodView,goodFn)
 tests:=map[string]func(){
 "field":func(){n:=*node;n.Item=nil;g.Field(&n)},"interface field":func(){n:=*node;n.View=nil;g.InterfaceField(&n)},"callback field":func(){n:=*node;n.Callback=nil;g.CallbackField(&n)},
 "copy":func(){g.Copy(g.Row{Item:nil,View:goodView,Callback:goodFn})},"slice":func(){g.Slice([]*g.Item{nil})},"nested":func(){g.Nested([]g.Row{{Item:nil}})},"range":func(){g.Range([]*g.Item{nil})},"range callback":func(){g.RangeCallback([]func()int{nil})},
 }
 g.Supply=func()*g.Item{return nil};g.FunctionSupply=func()func()int{return nil};g.Current=nil
 g.ArraySource=func()*g.Arrays{return nil};tests["fixed array len"]=func(){g.Length()};tests["fixed array cap"]=func(){g.Capacity()}
 tests["typed nil switch"]=func(){g.Switched((func()int)(nil))}
 for name,call:=range map[string]func(){"inferred":func(){g.Inferred()},"unused":g.Unused,"function":func(){g.InferredFunction()},"for":g.For,"shadow":func(){g.Shadow()},"global":func(){g.Global()},"escape":func(){g.Escape(true)}}{tests[name]=call}
 for name,call:=range tests{t.Run(name,func(t *testing.T){g.Reset();got:=capture(call);want:=capture(func(){r.Read(nil)});if(got==nil)!=(want==nil)||!strings.Contains(fmt.Sprint(got),"non-null reference read failed at reads.km:")||g.State()!=0{t.Fatal(got,want,g.State())}})}
 g.Reset();g.Discard();if g.State()!=1{t.Fatal("explicit discard")}
 g.Supply=func()*g.Item{return good};g.FunctionSupply=func()func()int{return goodFn};g.Current=good
 g.ArraySource=func()*g.Arrays{return g.NewArrays()};if g.Length()!=2||g.Capacity()!=2{t.Fatal("array length")}
 if g.Switched(goodFn)!=7{t.Fatal("typed callback switch")}
 node.Next=node
 if g.Field(node)!=r.Read(&r.Item{N:7})||g.InterfaceField(node)!=7||g.CallbackField(node)!=r.Callback(goodFn)||g.Cycle(node)!=7||g.Optional(node)!=7||g.Optional(nil)!=0{t.Fatal("valid graph or typed nil interface")}
 row:=g.Row{Item:good,View:goodView,Callback:goodFn};if g.Copy(row)!=7||g.Slice([]*g.Item{good})!=7||g.Nested([]g.Row{row})!=7||g.Range([]*g.Item{good})!=7||g.RangeCallback([]func()int{goodFn})!=7{t.Fatal("valid collections")}
 if !g.Address(row){t.Fatal("addressable reference storage")}
 if g.Inferred()!=7||g.InferredFunction()!=7||g.Shadow()!=10||g.Global()!=7||g.Escape(false)!=7{t.Fatal("valid reads")}
}
`
	runGeneratedGoDifferentialTest(t, root, "reference-reads.test", generated, reference, comparison)
}
