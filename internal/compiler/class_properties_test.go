package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassPropertiesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"model.km": `
export let trace=0;
export function reset():void{trace=0;}
export function mark(n:int):void{trace=trace*10+n;}
export class Counter{
  constructor(private raw:int){}
  public get value():int{mark(2);return this.raw;}
  public set value(v:int){mark(4);this.raw=v;}
  public function rawValue():int{return this.raw;}
}
export class Box<T>{constructor(private raw:T){}public get value():T{return this.raw;}protected set value(v:T){this.raw=v;}}
export class Numbers extends Box<int>{constructor(){super(1);}public function change():int{super.value+=2;this.value++;return super.value;}}
export class Item{constructor(public n:int){}}
export class Owner{constructor(private raw:Item|null){}public get value():Item|null{return this.raw;}public set value(v:Item|null){this.raw=v;}}
export class Callback{public get run():(n:int)=>int{return (n:int):int=>n+1;}}
export class SliceBox{private raw:int[]=[1,2];public get value():int[]{return this.raw;}}
export class ReadWrite{private raw:int=0;private get secret():int{return this.raw;}public set input(v:int){this.raw=v;}public get output():int{return this.secret;}}
export class Narrow{private raw:byte=255;public get value():byte{return this.raw;}public set value(v:byte){this.raw=v;}}
export class Arrays{public calls:int=0;public get value():[2]int{this.calls++;return [1,2];}}
`,
		"bridge.km": `export {Counter,Box,Numbers,Item,Owner,Callback,SliceBox,ReadWrite,Narrow,Arrays,trace,reset,mark} from "./model";`,
		"entry.km": `
import {Counter,Box,Numbers,Item,Owner,Callback,SliceBox,ReadWrite,Narrow,Arrays,trace,reset,mark} from "./bridge";
export function Order():int[]{reset();const c=new Counter(5);const receiver=():Counter=>{mark(1);return c;};const rhs=():int=>{mark(3);return 7;};receiver().value+=rhs();return [trace,c.rawValue()];}
export function Assignment():int[]{reset();const c=new Counter(5);const receiver=():Counter=>{mark(1);return c;};const rhs=():int=>{mark(3);return 7;};receiver().value=rhs();return [trace,c.rawValue()];}
export function Updates():int[]{reset();const c=new Counter(5);c.value++;c.value--;for(let i=0;i<2;c.value++){i++;}return [trace,c.rawValue()];}
export function Objects():int[]{const n=new Numbers();const owner=new Owner(null);const before=owner.value;let empty=0;if(before===null){empty=1;}const item=new Item(9);owner.value=item;const v=owner.value;let result=0;if(v!==null){v.n=12;result=v.n;}const box=new Box<Item>(item);const rw=new ReadWrite();rw.input=4;return [n.change(),empty,result,item.n,box.value.n,rw.output,new Callback().run(3)];}
export function Aliasing():int{const b=new SliceBox();b.value[0]=8;return b.value[0];}
export function Width(n:uint):int[]{const c=new Narrow();c.value++;c.value+=1;c.value<<=n;return [int(c.value)];}
export function Hygiene():int{const __propertyReceiver=new Counter(2);const __propertyValue=3;__propertyReceiver.value+=__propertyValue;return __propertyReceiver.rawValue();}
export function Failure():int[]{reset();const c=new Counter(5);try{const fail=():int=>{mark(3);throw new Exception("bad");};c.value+=fail();}catch(e:Exception){}return [trace,c.rawValue()];}
export function Division(n:int):int{const c=new Counter(5);c.value/=n;return c.rawValue();}
export function Rebind():int[]{const original=new Counter(5);let current=original;const rhs=():int=>{current=new Counter(99);return 2;};current.value+=rhs();return [original.rawValue(),current.rawValue()];}
export function Lengths():int[]{const a=new Arrays();const n=len(a.value);const m=cap(a.value);return [n,m,a.calls];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "properties")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var trace int
func mark(n int){trace=trace*10+n}
type counter struct{raw int}
func(c *counter)get()int{mark(2);return c.raw}
func(c *counter)set(v int){mark(4);c.raw=v}
func Order()[]int{trace=0;c:=&counter{5};receiver:=func()*counter{mark(1);return c};rhs:=func()int{mark(3);return 7};r:=receiver();v:=r.get();v+=rhs();r.set(v);return []int{trace,c.raw}}
func Assignment()[]int{trace=0;c:=&counter{5};receiver:=func()*counter{mark(1);return c};rhs:=func()int{mark(3);return 7};receiver().set(rhs());return []int{trace,c.raw}}
func Updates()[]int{trace=0;c:=&counter{5};v:=c.get();v++;c.set(v);v=c.get();v--;c.set(v);for i:=0;i<2;i++{v=c.get();v++;c.set(v)};return []int{trace,c.raw}}
type box[T any] struct{raw T}
func(b *box[T])get()T{return b.raw}
func(b *box[T])set(v T){b.raw=v}
type numbers struct{box[int]}
func(n *numbers)change()int{v:=n.box.get();v+=2;n.box.set(v);v=n.get();v++;n.set(v);return n.box.get()}
type item struct{n int}
type rw struct{raw int}
func(r *rw)secret()int{return r.raw}
func(r *rw)input(v int){r.raw=v}
func(r *rw)output()int{return r.secret()}
func callback()func(int)int{return func(n int)int{return n+1}}
func Objects()[]int{n:=&numbers{box[int]{1}};owner:=&box[*item]{nil};before:=owner.get();empty:=0;if before==nil{empty=1};it:=&item{9};owner.set(it);v:=owner.get();result:=0;if v!=nil{v.n=12;result=v.n};b:=&box[*item]{it};r:=&rw{};r.input(4);return []int{n.change(),empty,result,it.n,b.get().n,r.output(),callback()(3)}}
func Aliasing()int{b:=&box[[]int]{[]int{1,2}};b.get()[0]=8;return b.get()[0]}
func Width(n uint)[]int{b:=&box[byte]{255};v:=b.get();v++;b.set(v);v=b.get();v+=1;b.set(v);v=b.get();v<<=n;b.set(v);return []int{int(b.get())}}
func Hygiene()int{c:=&counter{2};rhs:=3;v:=c.get();v+=rhs;c.set(v);return c.raw}
func Failure()[]int{trace=0;c:=&counter{5};func(){defer func(){recover()}();fail:=func()int{mark(3);panic("bad")};v:=c.get();v+=fail();c.set(v)}();return []int{trace,c.raw}}
func Division(n int)int{c:=&counter{5};v:=c.get();v/=n;c.set(v);return c.raw}
func Rebind()[]int{original:=&counter{5};current:=original;rhs:=func()int{current=&counter{99};return 2};receiver:=current;v:=receiver.get();v+=rhs();receiver.set(v);return []int{original.raw,current.raw}}
type arrays struct{calls int}
func(a *arrays)get()[2]int{a.calls++;return [2]int{1,2}}
func Lengths()[]int{a:=&arrays{};n:=len(a.get());m:=cap(a.get());return []int{n,m,a.calls}}
`
	comparison := `package properties_test
import("testing";"reflect";g "class-properties.test";r "class-properties.test/reference")
func TestProperties(t *testing.T){for _,pair:=range [][2][]int{{g.Order(),r.Order()},{g.Assignment(),r.Assignment()},{g.Updates(),r.Updates()},{g.Objects(),r.Objects()},{g.Failure(),r.Failure()},{g.Width(0),r.Width(0)},{g.Width(7),r.Width(7)},{g.Width(8),r.Width(8)}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatal(pair)}};if g.Aliasing()!=r.Aliasing()||g.Hygiene()!=r.Hygiene(){t.Fatal("alias/hygiene")}}
func panics(f func())(p bool){defer func(){p=recover()!=nil}();f();return}
func TestFailure(t *testing.T){for _,n:=range []int{0,2}{var gv,rv int;gp:=panics(func(){gv=g.Division(n)});rp:=panics(func(){rv=r.Division(n)});if gp!=rp||gv!=rv{t.Fatal(n,gv,rv,gp,rp)}}}
func TestRebindAndGoAPI(t *testing.T){if !reflect.DeepEqual(g.Rebind(),r.Rebind()){t.Fatal("receiver rebound")};c:=g.NewCounter(3);var api interface{GetValue()int;SetValue(int)}=c;api.SetValue(8);if api.GetValue()!=8{t.Fatal("public accessors")};b:=g.NewBox[string]("text");if b.GetValue()!="text"{t.Fatal("generic public getter")}}
func TestArrayLengths(t *testing.T){if !reflect.DeepEqual(g.Lengths(),r.Lengths()){t.Fatal("getter len/cap evaluation",g.Lengths(),r.Lengths())}}
`
	runGeneratedGoDifferentialTest(t, root, "class-properties.test", generated, reference, comparison)
}
