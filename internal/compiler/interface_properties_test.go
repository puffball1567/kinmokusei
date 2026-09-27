package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInterfacePropertiesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module interface-properties.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Property[T any] interface{GetValue()T;SetValue(T)}
`,
		"model.km": `
import go api from "interface-properties.test/contracts";
interface Foreign<T> extends api.Property<T>{get value():T;set value(v:T);}
class Hybrid<T> implements Foreign<T>{constructor(private raw:T){}public get value():T{return this.raw;}public set value(v:T){this.raw=v;}}
export function HybridUse(n:int):int{const c:Foreign<int>=new Hybrid<int>(n);c.value++;const read=c.GetValue;return read()+c.value;}
export function ForeignAPI(n:int):api.Property<int>{return new Hybrid<int>(n);}
export interface Read<T>{get value():T;}
export interface Write<T>{set value(v:T);}
interface Left<T> extends Read<T>{}
interface Right<T> extends Read<T>{}
export interface Cell<T> extends Left<T>,Right<T>,Write<T>{}
export abstract class Repository<T> implements Cell<T>{public abstract get value():T;public abstract set value(v:T);}
export class Store<T> extends Repository<T>{
 constructor(private raw:T){}
 public override get value():T{return this.raw;}
 public override set value(v:T){this.raw=v;}
}
export let trace=0;
export function mark(n:int):void{trace=trace*10+n;}
export function reset():void{trace=0;}
class Base{
 constructor(private raw:int){}
 public get value():int{mark(2);return this.raw;}
 public set value(v:int){mark(4);this.raw=v;}
 public function plain():int{return this.raw;}
}
export class Counter extends Base implements Cell<int>{constructor(v:int){super(v);}}
export class Item{constructor(public n:int){}}
export interface ArrayView{get values():[2]int;}
export class Arrays implements ArrayView{public calls:int=0;public get values():[2]int{this.calls++;return [1,2];}}
export interface Callable{get run():(n:int)=>int;}
export class Callback implements Callable{public get run():(n:int)=>int{return (n:int):int=>n+1;}}
`,
		"bridge.km": `export {Read,Write,Cell as Contract,Store,Counter,Item,Arrays,ArrayView,Callable,Callback,trace,mark,reset,HybridUse,ForeignAPI} from "./model";`,
		"entry.km": `
export {HybridUse,ForeignAPI} from "./bridge";
import {Read,Write,Contract,Store,Counter,Item,Arrays,ArrayView,Callable,Callback,trace,mark,reset} from "./bridge";
export function ReadInt(c:Read<int>):int{return c.value;}
export function WriteInt(c:Write<int>,v:int):void{c.value=v;}
export function Increment(c:Contract<int>):int{c.value++;return c.value;}
function inject<T>(c:Contract<T>,v:T):T{c.value=v;return c.value;}
export function DI():int[]{const store=new Store<int>(1);const c:Contract<int>=store;const first=inject(c,5);const read:Read<int>=c;const write:Write<int>=c;write.value=7;const next=read.value;const owner=new Store<Item|null>(null);const item=new Item(9);const result=inject<Item|null>(owner,item);let identity=0;if(result===item){identity=1;}let n=0;if(result!==null){result.n=12;n=result.n;}const callback:Callable=new Callback();return [first,next,identity,n,item.n,callback.run(3)];}
export function Order():int[]{const counter=new Counter(5);const c:Contract<int>=counter;reset();const receiver=():Contract<int>=>{mark(1);return c;};const rhs=():int=>{mark(3);return 7;};receiver().value+=rhs();return [trace,counter.plain()];}
export function Assignment():int[]{const counter=new Counter(5);const c:Write<int>=counter;reset();const receiver=():Write<int>=>{mark(1);return c;};const rhs=():int=>{mark(3);return 7;};receiver().value=rhs();return [trace,counter.plain()];}
export function Rebind():int[]{const old=new Store<int>(5);let current:Contract<int>=old;const rhs=():int=>{current=new Store<int>(99);return 2;};current.value+=rhs();return [old.value,current.value];}
export function Failure():int[]{const counter=new Counter(5);const c:Contract<int>=counter;reset();try{const rhs=():int=>{mark(3);throw new Exception("bad");};c.value+=rhs();}catch(e:Exception){}return [trace,counter.plain()];}
export function Lengths():int[]{const a=new Arrays();const view:ArrayView=a;const n=len(view.values);const m=cap(view.values);return [n,m,a.calls];}
export function Aliasing():int{const s=new Store<int[]>([1]);const c:Contract<int[]>=s;c.value[0]=8;return s.value[0];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "interfaceproperties")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type read[T any] interface{GetValue()T}
type write[T any] interface{SetValue(T)}
type cell[T any] interface{read[T];write[T]}
type store[T any] struct{raw T}
func(s *store[T])GetValue()T{return s.raw}
func(s *store[T])SetValue(v T){s.raw=v}
func inject[T any](c cell[T],v T)T{c.SetValue(v);return c.GetValue()}
type item struct{n int}
func DI()[]int{s:=&store[int]{1};var c cell[int]=s;first:=inject(c,5);var r read[int]=c;var w write[int]=c;w.SetValue(7);next:=r.GetValue();owner:=&store[*item]{};it:=&item{9};result:=inject[*item](owner,it);identity:=0;if result==it{identity=1};n:=0;if result!=nil{result.n=12;n=result.n};callback:=func(n int)int{return n+1};return []int{first,next,identity,n,it.n,callback(3)}}
var trace int
func mark(n int){trace=trace*10+n}
type counter struct{raw int}
func(c *counter)GetValue()int{mark(2);return c.raw}
func(c *counter)SetValue(v int){mark(4);c.raw=v}
func Order()[]int{counter:=&counter{5};var c cell[int]=counter;trace=0;receiver:=func()cell[int]{mark(1);return c};rhs:=func()int{mark(3);return 7};r:=receiver();v:=r.GetValue();v+=rhs();r.SetValue(v);return []int{trace,counter.raw}}
func Assignment()[]int{counter:=&counter{5};var c write[int]=counter;trace=0;receiver:=func()write[int]{mark(1);return c};rhs:=func()int{mark(3);return 7};receiver().SetValue(rhs());return []int{trace,counter.raw}}
func Rebind()[]int{old:=&store[int]{5};var current cell[int]=old;rhs:=func()int{current=&store[int]{99};return 2};r:=current;v:=r.GetValue();v+=rhs();r.SetValue(v);return []int{old.GetValue(),current.GetValue()}}
func Failure()[]int{counter:=&counter{5};var c cell[int]=counter;trace=0;func(){defer func(){recover()}();rhs:=func()int{mark(3);panic("bad")};v:=c.GetValue();v+=rhs();c.SetValue(v)}();return []int{trace,counter.raw}}
type arrayView interface{GetValues()[2]int}
type arrays struct{calls int}
func(a *arrays)GetValues()[2]int{a.calls++;return [2]int{1,2}}
func Lengths()[]int{a:=&arrays{};var v arrayView=a;n:=len(v.GetValues());m:=cap(v.GetValues());return []int{n,m,a.calls}}
func Aliasing()int{s:=&store[[]int]{[]int{1}};var c cell[[]int]=s;c.GetValue()[0]=8;return s.raw[0]}
func ReadNil()int{var c read[int];return c.GetValue()}
func WriteNil(){var c write[int];c.SetValue(1)}
func UpdateNil(){var c cell[int];v:=c.GetValue();v++;c.SetValue(v)}
func HybridUse(n int)int{var c cell[int]=&store[int]{n};v:=c.GetValue();v++;c.SetValue(v);read:=c.GetValue;return read()+c.GetValue()}
func ForeignAPI(n int)interface{GetValue()int;SetValue(int)}{return &store[int]{n}}
`
	comparison := `package interfaceproperties_test
import("testing";"reflect";g "interface-properties.test";r "interface-properties.test/reference")
func TestProperties(t *testing.T){for _,pair:=range [][2][]int{{g.DI(),r.DI()},{g.Order(),r.Order()},{g.Assignment(),r.Assignment()},{g.Rebind(),r.Rebind()},{g.Failure(),r.Failure()},{g.Lengths(),r.Lengths()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatal(pair)}};if g.Aliasing()!=r.Aliasing(){t.Fatal("alias")}}
type host struct{value int}
func(h *host)GetValue()int{return h.value}
func(h *host)SetValue(v int){h.value=v}
func TestGoImplementation(t *testing.T){h:=&host{2};g.WriteInt(h,5);if g.ReadInt(h)!=5||g.Increment(h)!=6||h.value!=6{t.Fatal("Go implementation")};c:=g.NewCounter(2);g.WriteInt(c,8);if g.ReadInt(c)!=8{t.Fatal("inherited Go implementation")}}
func panics(f func())(p bool){defer func(){p=recover()!=nil}();f();return}
func TestNil(t *testing.T){for _,pair:=range [][2]func(){{func(){g.ReadInt(nil)},func(){r.ReadNil()}},{func(){g.WriteInt(nil,1)},r.WriteNil},{func(){g.Increment(nil)},r.UpdateNil}}{if panics(pair[0])!=panics(pair[1]){t.Fatal("nil contract")}}}
func TestGoAncestor(t *testing.T){for _,n:=range []int{-1,0,7}{if g.HybridUse(n)!=r.HybridUse(n){t.Fatal("Go property ancestor")};a,b:=g.ForeignAPI(n),r.ForeignAPI(n);a.SetValue(n+3);b.SetValue(n+3);if a.GetValue()!=b.GetValue(){t.Fatal("Go property upcast")}}}
`
	runGeneratedGoDifferentialTest(t, root, "interface-properties.test", generated, reference, comparison)
}
