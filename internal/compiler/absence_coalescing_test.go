package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAbsenceCoalescingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `export class Item{constructor(public n:int){}}
export struct Row{public item:Item;}
export alias Pair=[2]Item;
`,
		"entry.km": `import {Item,Row,Pair} from "./models";
alias Rows=Map<int,Row>;
type Numbers=distinct Map<int,int>;
export function Lookup(present:boolean,n:int,visit:(step:int)=>void):int{
 const m=Numbers(makeMap<int,int>());if(present){m[0]=n;}
 const object=():Map<int,int>=>{visit(1);return m;};const key=():int=>{visit(2);return 0;};const fallback=():int=>{visit(3);return 9;};
 return object()[key()]??fallback();
}
export function Record(present:boolean):int{const m:Rows=makeMap<int,Row>();if(present){m[0]=Row{item:new Item(7)};}return (m[0]??Row{item:new Item(9)}).item.n;}
export function Array(present:boolean):int{const m=makeMap<int,Pair>();if(present){const pair:Pair=[new Item(1),new Item(2)];m[0]=pair;}const pair=m[0]??[new Item(3),new Item(4)];return pair[0].n+pair[1].n;}
export function Receive(present:boolean,n:int,visit:(step:int)=>void):int{
 const ch=goChannel<int>(1);if(present){ch<-n;}closeGoChannel(ch);
 const channel=():GoChannel<int>=>{visit(1);return ch;};const fallback=():int=>{visit(2);return 9;};return (<-channel())??fallback();
}
export function RecordReceive(present:boolean):int{const ch=goChannel<Row>(1);if(present){ch<-Row{item:new Item(5)};}closeGoChannel(ch);return ((<-ch)??Row{item:new Item(9)}).item.n;}
constraint Number=~int|~int64;
export function Generic<T extends Number>(m:Map<int,T>,fallback:T):T{return m[0]??fallback;}
export function GenericReceive<T extends Number>(ch:GoReceiveChannel<T>,fallback:T):T{return (<-ch)??fallback;}
export function Snapshot():int{let m=makeMap<int,int>();m[0]=4;const other=makeMap<int,int>();other[0]=5;const key=():int=>{m=other;return 0;};return m[key()]??9;}
export function NilMap():int{const m:Map<int,int>=nil;return m[0]??9;}
export function Scalars():boolean{const flags=makeMap<int,boolean>();flags[0]=false;const texts=makeMap<int,string>();texts[0]="";return !(flags[0]??true)&&(texts[0]??"fallback")==="";}
export function Shift(present:boolean,n:uint):byte{const m=makeMap<int,byte>();if(present){m[0]=0;}return m[0]??(1<<n);}
export function Finally(present:boolean):int[]{let calls=0;const m=makeMap<int,int>();if(present){m[0]=0;}const fallback=():int=>{calls++;throw new Exception("missing");};try{const n=m[0]??fallback();return [n,calls];}catch(_:Exception){return [-1,calls];}finally{calls++;}}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "absencefallback")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "absencefallback")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type Item struct{N int}
type Row struct{Item *Item}
type Pair [2]*Item
func Lookup(p bool,n int,visit func(int))int{m:=map[int]int{};if p{m[0]=n};object:=func()map[int]int{visit(1);return m};key:=func()int{visit(2);return 0};fallback:=func()int{visit(3);return 9};table:=object();index:=key();v,ok:=table[index];if !ok{return fallback()};return v}
func Record(p bool)int{m:=map[int]Row{};if p{m[0]=Row{&Item{7}}};row,ok:=m[0];if !ok{row=Row{&Item{9}}};return row.Item.N}
func Array(p bool)int{m:=map[int]Pair{};if p{m[0]=Pair{&Item{1},&Item{2}}};pair,ok:=m[0];if !ok{pair=Pair{&Item{3},&Item{4}}};return pair[0].N+pair[1].N}
func Receive(p bool,n int,visit func(int))int{ch:=make(chan int,1);if p{ch<-n};close(ch);channel:=func()chan int{visit(1);return ch};fallback:=func()int{visit(2);return 9};v,open:=<-channel();if !open{return fallback()};return v}
func RecordReceive(p bool)int{ch:=make(chan Row,1);if p{ch<-Row{&Item{5}}};close(ch);row,open:=<-ch;if !open{row=Row{&Item{9}}};return row.Item.N}
func Generic[T ~int|~int64](m map[int]T,fallback T)T{v,ok:=m[0];if !ok{return fallback};return v}
func GenericReceive[T ~int|~int64](ch <-chan T,fallback T)T{v,open:=<-ch;if !open{return fallback};return v}
func Snapshot()int{m:=map[int]int{0:4};other:=map[int]int{0:5};key:=func()int{m=other;return 0};table:=m;index:=key();v,ok:=table[index];if !ok{return 9};return v}
func NilMap()int{var m map[int]int;v,ok:=m[0];if !ok{return 9};return v}
func Scalars()bool{flags:=map[int]bool{0:false};texts:=map[int]string{0:""};v,ok:=flags[0];if !ok{v=true};text,found:=texts[0];if !found{text="fallback"};return !v&&text==""}
func Shift(p bool,n uint)byte{m:=map[int]byte{};if p{m[0]=0};v,ok:=m[0];if !ok{return 1<<n};return v}
func Finally(p bool)(result []int){calls:=0;defer func(){if recover()!=nil{result=[]int{-1,calls}};calls++}();m:=map[int]int{};if p{m[0]=0};fallback:=func()int{calls++;panic("missing")};v,ok:=m[0];if !ok{v=fallback()};return []int{v,calls}}
`
	comparison := `package absencefallback_test
import("reflect";"testing";g "absence-fallback.test";r "absence-fallback.test/reference")
func TestValues(t *testing.T){for _,p:=range []bool{false,true}{for _,n:=range []int{0,5}{
 for _,pair:=range [][2]func(bool,int,func(int))int{{g.Lookup,r.Lookup},{g.Receive,r.Receive}}{var gs,rs []int;got:=pair[0](p,n,func(v int){gs=append(gs,v)});want:=pair[1](p,n,func(v int){rs=append(rs,v)});if got!=want||!reflect.DeepEqual(gs,rs){t.Fatal(p,n,got,want,gs,rs)}}
 }
 for _,pair:=range [][2]func(bool)int{{g.Record,r.Record},{g.Array,r.Array},{g.RecordReceive,r.RecordReceive}}{if got,want:=pair[0](p),pair[1](p);got!=want{t.Fatal(p,got,want)}}
 if got,want:=g.Finally(p),r.Finally(p);!reflect.DeepEqual(got,want){t.Fatal(got,want)}
 for _,n:=range []uint{0,7,8}{if got,want:=g.Shift(p,n),r.Shift(p,n);got!=want{t.Fatal(got,want)}}
 m:=map[int]int64{};if p{m[0]=0};if got,want:=g.Generic(m,9),r.Generic(m,9);got!=want{t.Fatal(got,want)}
 gc,rc:=make(chan int64,1),make(chan int64,1);if p{gc<-0;rc<-0};close(gc);close(rc);if got,want:=g.GenericReceive(gc,9),r.GenericReceive(rc,9);got!=want{t.Fatal(got,want)}
 }
 if got,want:=g.Snapshot(),r.Snapshot();got!=want{t.Fatal(got,want)}
 if got,want:=g.NilMap(),r.NilMap();got!=want{t.Fatal(got,want)}
 if got,want:=g.Scalars(),r.Scalars();got!=want{t.Fatal(got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "absence-fallback.test", generated, reference, comparison)
}
