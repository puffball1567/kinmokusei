package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualPropertiesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"model.km": `
export let trace=0;
export function mark(n:int):void{trace=trace*10+n;}
export function reset():void{trace=0;}
export class Base{
 constructor(protected raw:int){}
 public virtual get value():int{mark(2);return this.raw;}
 public virtual set value(v:int){mark(4);this.raw=v;}
 public function plain():int{return this.raw;}
}
export class Child extends Base{
 constructor(n:int){super(n);}
 public override get value():int{mark(7);return super.value*2;}
 public override set value(v:int){mark(8);super.value=v+1;}
 public function baseIncrement():void{super.value++;}
}
export class GetterOnly extends Child{
 constructor(n:int){super(n);}
 public final override get value():int{return super.value+10;}
}
export abstract class Repository<T>{
 public abstract get value():T;
 public abstract set value(v:T);
}
export class Store<T> extends Repository<T>{
 constructor(private stored:T){}
 public override get value():T{return this.stored;}
 public override set value(v:T){this.stored=v;}
}
export abstract class Pending<U> extends Store<U>{
 constructor(v:U){super(v);}
 public abstract override get value():U;
}
export class Ready extends Pending<int>{
 constructor(v:int){super(v);}
 public override get value():int{return 42;}
}
export class Item{constructor(public n:int){}}
export class PhaseBase{
 protected raw:int=0;
 public baseSeen:int=0;
 constructor(){this.value=4;this.baseSeen=this.value;}
 public virtual get value():int{return this.raw;}
 public virtual set value(v:int){this.raw=v;}
}
export class PhaseChild extends PhaseBase{
 private extra:int=20;
 public childSeen:int=0;
 constructor(){super();this.value=5;this.childSeen=this.value;}
 public override get value():int{return super.value+this.extra;}
 public override set value(v:int){super.value=v*2;}
}
abstract class Building{
 constructor(){this.read();}
 private function read():void{const n=this.value;}
 public abstract get value():int;
}
class Built extends Building{public override get value():int{return 7;}}
export function BadConstruction():void{const value=new Built();}
`,
		"bridge.km": `export {Base,Child,GetterOnly,Repository as Contract,Store,Ready,Item,PhaseChild,BadConstruction,trace,mark,reset} from "./model";`,
		"entry.km": `
import {Base,Child,GetterOnly,Contract,Store,Ready,Item,PhaseChild,BadConstruction,trace,mark,reset} from "./bridge";
export function Dispatch():int[]{const child=new Child(5);const base:Base=child;reset();const receiver=():Base=>{mark(1);return base;};const rhs=():int=>{mark(3);return 3;};receiver().value+=rhs();const order=trace;const got=base.value;child.baseIncrement();return [order,got,child.plain(),base.value];}
export function Partial():int[]{const child=new GetterOnly(5);const base:Base=child;base.value=7;return [base.value,child.value,child.plain()];}
function inject<T>(repo:Contract<T>,v:T):T{repo.value=v;return repo.value;}
export function DI():int[]{const repo=new Store<int>(1);const contract:Contract<int>=repo;const first=inject(contract,7);const ready=new Ready(0);const base:Contract<int>=ready;const again=inject(base,9);const owner=new Store<Item|null>(null);const item=new Item(8);const result=inject<Item|null>(owner,item);let read=0;if(result!==null){result.n=12;read=result.n;}const [same,ok]=contract as? Store<int>;let identity=0;if(ok&&same===repo){identity=1;}return [first,again,read,item.n,identity];}
export function Phases():int[]{const child=new PhaseChild();return [child.baseSeen,child.childSeen,child.value];}
export function Build():void{BadConstruction();}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "virtualproperties")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, name := range []string{"Repository", "Pending"} {
		if strings.Contains(string(generated), "func New"+name+"[") {
			t.Fatalf("abstract factory for %s", name)
		}
	}
	reference := `package reference
var trace int
func mark(n int){trace=trace*10+n}
type contract interface{GetValue()int;SetValue(int)}
type base struct{raw int}
func(b *base)GetValue()int{mark(2);return b.raw}
func(b *base)SetValue(v int){mark(4);b.raw=v}
type child struct{base}
func(c *child)GetValue()int{mark(7);return c.base.GetValue()*2}
func(c *child)SetValue(v int){mark(8);c.base.SetValue(v+1)}
func(c *child)baseIncrement(){v:=c.base.GetValue();v++;c.base.SetValue(v)}
func Dispatch()[]int{c:=&child{base{5}};var b contract=c;trace=0;receiver:=func()contract{mark(1);return b};rhs:=func()int{mark(3);return 3};r:=receiver();v:=r.GetValue();v+=rhs();r.SetValue(v);order:=trace;got:=b.GetValue();c.baseIncrement();return []int{order,got,c.raw,b.GetValue()}}
type getterOnly struct{child}
func(c *getterOnly)GetValue()int{return c.child.GetValue()+10}
func Partial()[]int{c:=&getterOnly{child{base{5}}};var b contract=c;b.SetValue(7);return []int{b.GetValue(),c.GetValue(),c.raw}}
type repository[T any] interface{GetValue()T;SetValue(T)}
type store[T any] struct{value T}
func(s *store[T])GetValue()T{return s.value}
func(s *store[T])SetValue(v T){s.value=v}
func inject[T any](r repository[T],v T)T{r.SetValue(v);return r.GetValue()}
type ready struct{store[int]}
func(r *ready)GetValue()int{return 42}
type item struct{n int}
func DI()[]int{s:=&store[int]{1};var r repository[int]=s;first:=inject(r,7);a:=&ready{};again:=inject[int](a,9);owner:=&store[*item]{};it:=&item{8};result:=inject[*item](owner,it);read:=0;if result!=nil{result.n=12;read=result.n};same,ok:=r.(*store[int]);identity:=0;if ok&&same==s{identity=1};return []int{first,again,read,it.n,identity}}
type phaseBase struct{self contract;raw,baseSeen int}
func(b *phaseBase)GetValue()int{return b.raw}
func(b *phaseBase)SetValue(v int){b.raw=v}
type phaseChild struct{phaseBase;extra,childSeen int}
func(c *phaseChild)GetValue()int{return c.phaseBase.GetValue()+c.extra}
func(c *phaseChild)SetValue(v int){c.phaseBase.SetValue(v*2)}
func Phases()[]int{c:=&phaseChild{};b:=&c.phaseBase;b.self=b;b.self.SetValue(4);b.baseSeen=b.self.GetValue();c.extra=20;b.self=c;b.self.SetValue(5);c.childSeen=b.self.GetValue();return []int{b.baseSeen,c.childSeen,b.self.GetValue()}}
func Build(){panic("unimplemented construction phase")}
`
	comparison := `package virtualproperties_test
import("testing";"reflect";g "virtual-properties.test";r "virtual-properties.test/reference")
func TestDispatch(t *testing.T){for _,pair:=range [][2][]int{{g.Dispatch(),r.Dispatch()},{g.Partial(),r.Partial()},{g.DI(),r.DI()},{g.Phases(),r.Phases()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatal(pair)}}}
func panics(f func())(p bool){defer func(){p=recover()!=nil}();f();return}
func TestPhaseFailure(t *testing.T){if panics(g.Build)!=panics(r.Build){t.Fatal("construction")};if !panics(func(){new(g.Repository[int]).GetValue()})||!panics(func(){new(g.Repository[int]).SetValue(1)}){t.Fatal("abstract zero value")}}
func TestPublicGoAPI(t *testing.T){child:=g.NewChild(5);base:=g.UpcastChildToBase(child);get,set:=base.GetValue,base.SetValue;set(7);if get()!=16{t.Fatal("bound accessor dispatch")};if again,ok:=g.DowncastBaseToChild(base);!ok||again!=child{t.Fatal("identity")};zero:=new(g.Base);zero.SetValue(3);if zero.GetValue()!=3{t.Fatal("zero concrete fallback")}}
`
	runGeneratedGoDifferentialTest(t, root, "virtual-properties.test", generated, reference, comparison)
}
