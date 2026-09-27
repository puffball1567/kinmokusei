package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaticPropertiesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"model.km": `
let raw=5;
export let trace=0;
export function mark(n:int):void{trace=trace*10+n;}
export function reset():void{raw=5;trace=0;}
export function plain():int{return raw;}
export class Counter{
 public static get value():int{mark(2);return raw;}
 public static set value(v:int){mark(4);raw=v;}
 private static get secret():int{return raw;}
 protected static set input(v:int){raw=v;}
 public static function peek():int{return Counter.secret;}
 public function read():int{return Counter.value;}
}
export class Child extends Counter{
 public static function write(v:int):void{Child.input=v;}
}
export const Startup=Counter.value;
export class Namespaces{public static get value():int{return 1;}public function getValue():int{return 2;}}
let shared=0;
export class Shared<T>{
 constructor(public item:T){}
 public static get value():int{return shared;}
 public static set value(v:int){shared=v;}
 public function id(v:T):T{return v;}
 public static function echo(v:T):T{return v;}
}
export class Derived<U> extends Shared<U>{constructor(v:U){super(v);}}
export class Item{constructor(public n:int){}}
let stored:Item|null=null;
let items:int[]=[1,2];
let narrow:byte=255;
export class Values{
 public static get item():Item|null{return stored;}
 public static set item(v:Item|null){stored=v;}
 public static get slice():int[]{return items;}
 public static get run():(n:int)=>int{mark(2);return (n:int):int=>n+1;}
 public static get array():[2]int{mark(2);return [1,2];}
 public static get width():byte{return narrow;}
 public static set width(v:byte){narrow=v;}
}
`,
		"bridge.km": `export {Counter as Settings,Child,Shared,Derived,Item,Values,trace,mark,reset,plain,Startup,Namespaces} from "./model";`,
		"entry.km": `
export {Startup,Namespaces} from "./bridge";
import {Settings,Child,Shared,Derived,Item,Values,trace,mark,reset,plain} from "./bridge";
export function Order():int[]{reset();const rhs=():int=>{mark(3);return 7;};Child.value+=rhs();return [trace,plain()];}
export function Assignment():int[]{reset();const rhs=():int=>{mark(3);return 7;};Settings.value=rhs();return [trace,plain()];}
export function Updates():int[]{reset();Settings.value++;Child.value--;for(let i=0;i<2;Child.value++){i++;}return [trace,plain()];}
export function Failure():int[]{reset();try{const rhs=():int=>{mark(3);throw new Exception("bad");};Settings.value+=rhs();}catch(e:Exception){}return [trace,plain()];}
export function Inheritance():int[]{Child.write(8);const c=new Child();return [Settings.peek(),Child.peek(),c.read()];}
export function Generic():int[]{Shared.value=1;Derived.value+=2;const a=new Shared<int>(7);const b=new Shared<string>("hi");return [Shared.value,Derived.value,a.id(3),len(b.id("ok")),Shared.echo<int>(6)];}
export function Objects():int[]{Values.item=null;const before=Values.item;let empty=0;if(before===null){empty=1;}const item=new Item(7);Values.item=item;const result=Values.item;let n=0;if(result!==null){result.n=12;n=result.n;}Values.slice[0]=8;return [empty,n,item.n,Values.slice[0]];}
export function LengthsAndCalls():int[]{reset();const n=len(Values.array);const m=cap(Values.array);const arg=():int=>{mark(3);return 4;};const result=Values.run(arg());return [n,m,result,trace];}
export function Width(n:uint):int{Values.width=255;Values.width++;Values.width+=1;Values.width<<=n;return int(Values.width);}
export function Division(n:int):int{reset();Settings.value/=n;return plain();}
export function Hygiene():int{Shared.value=2;const __propertyValue=3;Shared.value+=__propertyValue;return Shared.value;}
const initialized:int=initialize();
function initialize():int{Initialization.value=1;return 1;}
class Initialization{public static get value():int{return initialized;}public static set value(v:int){}}
export function InitializationValue():int{return initialized;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "staticproperties")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var raw=5
var trace int
func mark(n int){trace=trace*10+n}
func reset(){raw=5;trace=0}
func get()int{mark(2);return raw}
func set(v int){mark(4);raw=v}
func Order()[]int{reset();rhs:=func()int{mark(3);return 7};v:=get();v+=rhs();set(v);return []int{trace,raw}}
func Assignment()[]int{reset();rhs:=func()int{mark(3);return 7};set(rhs());return []int{trace,raw}}
func Updates()[]int{reset();v:=get();v++;set(v);v=get();v--;set(v);for i:=0;i<2;i++{v=get();v++;set(v)};return []int{trace,raw}}
func Failure()[]int{reset();func(){defer func(){recover()}();rhs:=func()int{mark(3);panic("bad")};v:=get();v+=rhs();set(v)}();return []int{trace,raw}}
func Inheritance()[]int{raw=8;return []int{raw,raw,get()}}
var shared int
func getShared()int{return shared}
func setShared(v int){shared=v}
func id[T any](v T)T{return v}
func Generic()[]int{setShared(1);v:=getShared();v+=2;setShared(v);return []int{getShared(),getShared(),id(3),len(id("ok")),id(6)}}
type item struct{n int}
var stored *item
var items=[]int{1,2}
func getItem()*item{return stored}
func setItem(v *item){stored=v}
func getSlice()[]int{return items}
func Objects()[]int{setItem(nil);before:=getItem();empty:=0;if before==nil{empty=1};it:=&item{7};setItem(it);result:=getItem();n:=0;if result!=nil{result.n=12;n=result.n};getSlice()[0]=8;return []int{empty,n,it.n,getSlice()[0]}}
func array()[2]int{mark(2);return [2]int{1,2}}
func callback()func(int)int{mark(2);return func(n int)int{return n+1}}
func LengthsAndCalls()[]int{reset();n:=len(array());m:=cap(array());arg:=func()int{mark(3);return 4};result:=callback()(arg());return []int{n,m,result,trace}}
var narrow byte
func getWidth()byte{return narrow}
func setWidth(v byte){narrow=v}
func Width(n uint)int{setWidth(255);v:=getWidth();v++;setWidth(v);v=getWidth();v+=1;setWidth(v);v=getWidth();v<<=n;setWidth(v);return int(getWidth())}
func Division(n int)int{reset();v:=get();v/=n;set(v);return raw}
func Hygiene()int{setShared(2);rhs:=3;v:=getShared();v+=rhs;setShared(v);return getShared()}
var Startup=get()
var initialized=initialize()
func initialize()int{initializationSet(1);return 1}
func initializationGet()int{return initialized}
func initializationSet(v int){}
func InitializationValue()int{return initialized}
`
	comparison := `package staticproperties_test
import("testing";"reflect";g "static-properties.test";r "static-properties.test/reference")
func TestProperties(t *testing.T){for _,p:=range [][2][]int{{g.Order(),r.Order()},{g.Assignment(),r.Assignment()},{g.Updates(),r.Updates()},{g.Failure(),r.Failure()},{g.Inheritance(),r.Inheritance()},{g.Generic(),r.Generic()},{g.Objects(),r.Objects()},{g.LengthsAndCalls(),r.LengthsAndCalls()}}{if !reflect.DeepEqual(p[0],p[1]){t.Fatal(p)}};for _,n:=range []uint{0,7,8,32}{if g.Width(n)!=r.Width(n){t.Fatal("width",n)}};if g.Hygiene()!=r.Hygiene(){t.Fatal("hygiene")}}
func panics(f func())(p bool){defer func(){p=recover()!=nil}();f();return}
func TestPanic(t *testing.T){for _,n:=range []int{0,2}{var a,b int;ap:=panics(func(){a=g.Division(n)});bp:=panics(func(){b=r.Division(n)});if ap!=bp||a!=b{t.Fatal(n,a,b,ap,bp)}}}
func TestGoAPI(t *testing.T){g.CounterSetValue(9);if g.CounterGetValue()!=9{t.Fatal("public API")};g.SharedSetValue(4);if g.SharedGetValue()!=4{t.Fatal("generic-independent API")};var instance any=g.NewCounter();if _,ok:=instance.(interface{GetValue()int});ok{t.Fatal("static accessor leaked into instance method set")};if g.NamespacesGetValue()!=1||g.NewNamespaces().GetValue()!=2{t.Fatal("separate namespaces")};if g.Startup!=r.Startup||g.InitializationValue()!=r.InitializationValue(){t.Fatal("initialization")}}
`
	runGeneratedGoDifferentialTest(t, root, "static-properties.test", generated, reference, comparison)
}
