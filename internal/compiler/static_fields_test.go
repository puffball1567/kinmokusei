package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaticFieldsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"model.km": `
export let trace=0;
function mark(n:int):int{trace=trace*10+n;return n;}
export class Initial{
 public static first:int=Initial.next+mark(2);
 public static next:int=mark(1);
 public static last:int=mark(3);
}
export class Item{constructor(public n:int){}}
export class State<T>{
 public static count:int=0;
 private static raw:int=4;
 protected static protectedCount:int=3;
 public static item:Item=new Item(5);
 public static maybe:Item|null=null;
 public static values:int[]=[1,2];
 public static array:[2]int=[3,4];
 public static lookup:Map<string,int>=makeMap<string,int>();
 public static callback:(n:int)=>int=(n:int):int=>n+1;
 public static narrow:byte=255;
 public static get value():int{return State.raw;}
 public static set value(v:int){State.raw=v;}
 constructor(public instance:T){State.count++;}
}
export class Child<U> extends State<U>{
 constructor(v:U){super(v);}
 public static function bump():int{Child.protectedCount++;return Child.protectedCount;}
}
export class Local{public count:int=7;}
export class Single{public static instance:Single=new Single();public n:int=9;}
alias V=int;
export class Lexical<V>{public static value:V=6;public static get copied():V{return Lexical.value;}}
`,
		"bridge.km": `export {Initial,State as Store,Child,Item,Local,Single,Lexical,trace} from "./model";`,
		"entry.km": `
import {Initial,Store,Child,Item,Local,Single,Lexical,trace} from "./bridge";
export {Store,Single} from "./bridge";
alias V=string;
export function Initialized():int[]{return [Initial.first,Initial.next,Initial.last,trace,Single.instance.n,Lexical.value,Lexical.copied];}
export function Sharing():int[]{Store.count=0;const a=new Store<int>(1);const b=new Child<string>("x");Child.count+=3;const before=Store.count;const p=&Child.count;*p+=2;Store.value=8;Child.value++;return [before,Store.count,Child.count,Store.value,Child.bump<int>(),a.instance,len(b.instance)];}
export function Objects():int[]{Store.maybe=null;Store.item.n=7;Child.maybe=Store.item;const item=Store.maybe;let n=0;if(item!==null){item.n=12;n=item.n;}Store.values[0]=8;Child.array[0]=9;Store.lookup["x"]=10;return [n,Store.item.n,Store.values[0],Store.array[0],Store.lookup["x"],Store.callback(3)];}
export function Width(n:uint):int{Store.narrow=255;Store.narrow++;Store.narrow+=1;Store.narrow<<=n;return int(Store.narrow);}
export function Shadow():int{const Store=new Local();Store.count++;return Store.count;}
export function CopyAndLengths():int[]{const copy=Store.array;const p=&Store.array;(*p)[1]=8;return [copy[1],Store.array[1],len(Store.array),cap(Store.array)];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "staticfields")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var trace int
func mark(n int)int{trace=trace*10+n;return n}
var first=next+mark(2)
var next=mark(1)
var last=mark(3)
type item struct{n int}
type single struct{n int}
var singleton=&single{9}
func Initialized()[]int{return []int{first,next,last,trace,singleton.n,6,6}}
var count int
var raw=4
var protectedCount=3
var stored=&item{5}
var maybe *item
var values=[]int{1,2}
var array=[2]int{3,4}
var lookup=make(map[string]int)
var callback=func(n int)int{return n+1}
var narrow byte=255
func get()int{return raw}
func set(v int){raw=v}
type state[T any] struct{instance T}
func newState[T any](v T)*state[T]{count++;return &state[T]{v}}
func Sharing()[]int{count=0;a:=newState(1);b:=newState("x");count+=3;before:=count;p:=&count;*p+=2;set(8);v:=get();v++;set(v);protectedCount++;return []int{before,count,count,get(),protectedCount,a.instance,len(b.instance)}}
func Objects()[]int{maybe=nil;stored.n=7;maybe=stored;it:=maybe;n:=0;if it!=nil{it.n=12;n=it.n};values[0]=8;array[0]=9;lookup["x"]=10;return []int{n,stored.n,values[0],array[0],lookup["x"],callback(3)}}
func Width(n uint)int{narrow=255;narrow++;narrow+=1;narrow<<=n;return int(narrow)}
func Shadow()int{local:=struct{count int}{7};local.count++;return local.count}
func CopyAndLengths()[]int{copy:=array;p:=&array;(*p)[1]=8;return []int{copy[1],array[1],len(array),cap(array)}}
`
	comparison := `package staticfields_test
import("testing";"reflect";"encoding/json";g "static-fields.test";r "static-fields.test/reference")
func TestFields(t *testing.T){for _,p:=range [][2][]int{{g.Initialized(),r.Initialized()},{g.Sharing(),r.Sharing()},{g.Objects(),r.Objects()},{g.CopyAndLengths(),r.CopyAndLengths()}}{if !reflect.DeepEqual(p[0],p[1]){t.Fatal(p)}};for _,n:=range []uint{0,7,8,32}{if g.Width(n)!=r.Width(n){t.Fatal("width")}};if g.Shadow()!=r.Shadow(){t.Fatal("local class name shadow")}}
func TestPublicGoAPI(t *testing.T){g.StateCount=20;if g.NewState[int](2).Instance!=2||g.StateCount!=21{t.Fatal("Go variable")};if g.SingleInstance.N!=9{t.Fatal("singleton")};value:=g.NewState[int](7);data,err:=json.Marshal(value);if err!=nil||string(data)!="{\"instance\":7}"{t.Fatal("static fields serialized",string(data),err)}}
`
	runGeneratedGoDifferentialTest(t, root, "static-fields.test", generated, reference, comparison)
}
