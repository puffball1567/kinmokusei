package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceEvaluationOrderMatchesExplicitGoSnapshots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `let x=1;
function bump():int{x=10;return 5;}
function pair(a:int,b:int):int{return a*100+b;}
export function Calls():int{x=1;return pair(x,bump());}
export function Binary():int{x=1;return x+bump();}
export function Assign():int{x=1;x=x+bump();return x;}
export function SliceTarget():int[]{let i=0;const xs=[1,2];const rhs=():int=>{i=1;return 9;};xs[i]=rhs();return xs;}
export function MapTarget():int[]{let i=0;const xs=makeMap<int,int>();xs[0]=1;xs[1]=2;const rhs=():int=>{i=1;return 9;};xs[i]=rhs();return [xs[0],xs[1]];}
export function DereferenceTarget():int[]{let a=1;let b=2;let p=&a;const rhs=():int=>{p=&b;return 9;};*p=rhs();return [a,b];}
class Box{constructor(public value:int){}}
export function FieldTarget():int[]{let box=new Box(1);const old=box;const rhs=():int=>{box=new Box(2);return 9;};box.value=rhs();return [old.value,box.value];}
export function CompoundTarget():int[]{let i=0;const xs=[1,2];const rhs=():int=>{i=1;xs[0]=10;return 5;};xs[i]+=rhs();return xs;}
struct Point{public a:int;public b:int;}
export function Literal():int{x=1;const p=Point{a:x,b:bump()};return p.a*100+p.b;}
class Pair{constructor(public a:int,public b:int){}}
export function Constructor():int{x=1;const p=new Pair(x,bump());return p.a*100+p.b;}
function spread(a:int,...rest:int[]):int{return a*100+rest[0];}
export function Spread():int{x=1;const tail=():int[]=>{x=10;return [5];};return spread(x,tail()...);}
export function ShortCircuit():int{let calls=0;const rhs=():boolean=>{calls++;return true;};const a=false&&rhs();const b=true||rhs();return calls;}
function narrow(a:byte,b:int):int{return int(a)*100+b;}
export function Shift():int{let n=0;const rhs=():int=>{n=8;return 5;};return narrow(1<<n,rhs());}
export function Callee():int{x=1;let fn=pair;const rhs=():int=>{fn=(a:int,b:int):int=>a*1000+b;return 5;};return fn(x,rhs());}
class Target{constructor(public id:int){}public function plain(value:int):int{return this.id*100+value;}public function generic<T>(value:T):int{return this.id*100+5;}}
export function Receiver():int{let target=new Target(1);const rhs=():int=>{target=new Target(2);return 5;};return target.plain(rhs());}
export function GenericReceiver():int{let target=new Target(1);const rhs=():int=>{target=new Target(2);return 5;};return target.generic(rhs());}
export function IndexObject():int{let xs=[1];const key=():int=>{xs=[9];return 0;};return xs[key()];}
export function ArrayStorage():int{let xs:[1]int=[1];const key=():int=>{xs=[9];return 0;};return xs[key()];}
export function Tuple():(int,int){x=1;return x,bump();}
export function TupleFinally():(int,int){x=1;try{return x,bump();}finally{x=20;}}
export function SliceBounds():int[]{let xs=[1,2,3];let low=0;const high=():int=>{xs=[9,8,7];low=1;return 2;};return xs[low:high()];}
export function ArrayBounds():int[]{let xs:[3]int=[1,2,3];let low=0;const high=():int=>{xs=[9,8,7];low=1;return 2;};return xs[low:high()];}
export function SendChannel():int[]{const first=goChannel<int>(1);const second=goChannel<int>(1);let current=first;const rhs=():int=>{current=second;return 7;};current<-rhs();return [len(first),len(second)];}
export function NegativeShift(log:*int):int{let n=-1;const rhs=():int=>{*log=1;return 5;};return narrow(1<<n,rhs());}
export function Division(log:*int):int{let zero=0;const rhs=():int=>{*log=1;return 5;};return pair(1/zero,rhs());}
class PropertyBox{constructor(public stored:int){}public get value():int{return this.stored;}public set value(value:int){this.stored=value;}}
export function PropertyTarget():int[]{let box=new PropertyBox(1);const old=box;const rhs=():int=>{box=new PropertyBox(2);return 9;};box.value=rhs();return [old.stored,box.stored];}
export function CallShiftCount():int{const count=():int=>0;return narrow(1<<count(),bump());}
`
	path := filepath.Join(root, "order.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "ordered")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Calls()int{x:=1;a:=x;x=10;_=x;return a*100+5}
func Binary()int{x:=1;a:=x;x=10;_=x;return a+5}
func Assign()int{x:=1;a:=x;x=10;x=a+5;return x}
func SliceTarget()[]int{xs:=[]int{1,2};i:=0;target:=&xs[i];i=1;_=i;*target=9;return xs}
func MapTarget()[]int{xs:=map[int]int{0:1,1:2};i:=0;key:=i;i=1;_=i;xs[key]=9;return []int{xs[0],xs[1]}}
func DereferenceTarget()[]int{a,b:=1,2;p:=&a;target:=p;p=&b;_=p;*target=9;return []int{a,b}}
type box struct{value int}
func FieldTarget()[]int{current:=&box{1};old:=current;target:=&current.value;current=&box{2};*target=9;return []int{old.value,current.value}}
func CompoundTarget()[]int{xs:=[]int{1,2};i:=0;target:=&xs[i];old:=*target;i=1;_=i;xs[0]=10;*target=old+5;return xs}
func Literal()int{x:=1;a:=x;x=10;_=x;return a*100+5}
func Constructor()int{return Literal()}
func Spread()int{x:=1;a:=x;x=10;_=x;return a*100+5}
func ShortCircuit()int{return 0}
func Shift()int{n:=0;var a byte=1<<n;n=8;_=n;return int(a)*100+5}
func Callee()int{return 105}
func Receiver()int{return 105}
func GenericReceiver()int{return 105}
func IndexObject()int{return 1}
func ArrayStorage()int{xs:=[1]int{1};base:=&xs;key:=func()int{xs=[1]int{9};return 0}();return base[key]}
func Tuple()(int,int){x:=1;a:=x;x=10;_=x;return a,5}
func TupleFinally()(int,int){return Tuple()}
func SliceBounds()[]int{return []int{1,2}}
func ArrayBounds()[]int{return []int{9,8}}
func SendChannel()[]int{return []int{1,0}}
func NegativeShift(log *int)int{n:=-1;var a byte=1<<n;*log=1;return int(a)*100+5}
func Division(log *int)int{zero:=0;a:=1/zero;*log=1;return a*100+5}
func PropertyTarget()[]int{return []int{9,2}}
func CallShiftCount()int{return 105}
`
	comparison := `package ordered_test
import("reflect";"testing";g "source-order.test";r "source-order.test/reference")
func TestValues(t *testing.T){for name,calls:=range map[string][2]func()int{"call":{g.Calls,r.Calls},"binary":{g.Binary,r.Binary},"assign":{g.Assign,r.Assign},"literal":{g.Literal,r.Literal},"constructor":{g.Constructor,r.Constructor},"spread":{g.Spread,r.Spread},"short circuit":{g.ShortCircuit,r.ShortCircuit},"shift":{g.Shift,r.Shift},"call shift":{g.CallShiftCount,r.CallShiftCount},"callee":{g.Callee,r.Callee},"receiver":{g.Receiver,r.Receiver},"generic receiver":{g.GenericReceiver,r.GenericReceiver},"slice base":{g.IndexObject,r.IndexObject},"array storage":{g.ArrayStorage,r.ArrayStorage}}{if got,want:=calls[0](),calls[1]();got!=want{t.Errorf("%s: %d != %d",name,got,want)}};if g.Calls()!=r.Calls(){t.Fatal("call order")}}
func TestReturns(t *testing.T){a,b:=g.Tuple();c,d:=r.Tuple();if a!=c||b!=d{t.Fatal("tuple",a,b,c,d)};a,b=g.TupleFinally();c,d=r.TupleFinally();if a!=c||b!=d{t.Fatal("finally tuple",a,b,c,d)}}
func TestTargets(t *testing.T){for name,calls:=range map[string][2]func()[]int{"slice":{g.SliceTarget,r.SliceTarget},"map":{g.MapTarget,r.MapTarget},"dereference":{g.DereferenceTarget,r.DereferenceTarget},"field":{g.FieldTarget,r.FieldTarget},"property":{g.PropertyTarget,r.PropertyTarget},"compound":{g.CompoundTarget,r.CompoundTarget},"slice bounds":{g.SliceBounds,r.SliceBounds},"array bounds":{g.ArrayBounds,r.ArrayBounds},"send":{g.SendChannel,r.SendChannel}}{if got,want:=calls[0](),calls[1]();!reflect.DeepEqual(got,want){t.Errorf("%s: %v != %v",name,got,want)}}}
func capture(call func(*int)int)(log int,panicked bool){defer func(){panicked=recover()!=nil}();call(&log);return}
func TestPanics(t *testing.T){for name,calls:=range map[string][2]func(*int)int{"shift":{g.NegativeShift,r.NegativeShift},"division":{g.Division,r.Division}}{gl,gp:=capture(calls[0]);rl,rp:=capture(calls[1]);if gl!=rl||gp!=rp||!gp||gl!=0{t.Fatalf("%s panic: generated=%v,%v reference=%v,%v",name,gl,gp,rl,rp)}}}
`
	runGeneratedGoDifferentialTest(t, root, "source-order.test", generated, reference, comparison)
}
