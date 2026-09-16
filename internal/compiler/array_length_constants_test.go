package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArrayLengthConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module array-length-constants.test\n\ngo 1.23\n",
		"values.km": `export const Values:[3]int=[1,2,3];export const Width=len(Values);
export constraint Sized=~[3]int|~int[]|~string|~Map<string,int>|~GoChannel<int>;
export constraint Capacity=~[3]int|~int[]|~GoChannel<int>;`,
		"bridge.km": `export {Width as Size,Sized,Capacity} from "./values";`,
		"entry.km": `import {Size,Sized,Capacity} from "./bridge";
import go crc32 from "hash/crc32";
type Array=distinct [3]int;
alias Callback=()=>int;
struct Holder{public values:[3]int;}
class Box<T>{constructor(public values:[3]T){}public function size():int{const n=len(this.values);return n;}}
function length<T extends Sized>(value:T):int{const n=len(value);const address=&n;return *address;}
function capacity<T extends Capacity>(value:T):int{const n=cap(value);const address=&n;return *address;}
function genericElement<T>(value:[3]T):int{const n=len(value);return n;}
export function Sizes(a:[3]int):int[]{const n=len(a);const alias=n;const m=cap(&a);const named=Array(a);const count=len(named);const xs=makeSlice<int>(alias,m);const box=new Box<int>(a);return [n,m,count,len(xs),Size,box.size(),genericElement(a)];}
export function Ignored(index:int):int[]{const pointer:*[3]int=nil;const holder:*Holder=nil;const matrix:[1][3]int=[[1,2,3]];const empty:int[]=[];const table:*crc32.Table=nil;const n=len(pointer);const m=cap(*pointer);const out=len(matrix[index]);const field=cap(holder.values);const converted=len(copyArray[[3]int](empty));const viewed=cap(viewArray[[3]int](empty));const imported=len(table);return [n,m,out,field,converted,viewed,imported];}
export function Runtime():int{let calls=0;const get=():[3]int=>{calls++;return [1,2,3];};const first=len(get());const alias=first;const address=&alias;const second=cap(get());return calls*100+*address+second;}
export function Receive():int{const channel=goChannel<[3]int>(2);channel<-[1,2,3];channel<-[4,5,6];const first=len(<-channel);const second=cap(<-channel);const address=&first;return *address+second+len(channel)*100;}
export function Generic():int[]{const array:[3]int=[1,2,3];const slice=makeSlice<int>(2,5);const map=makeMap<string,int>();map["x"]=1;const channel=goChannel<int>(4);return [length(array),length(slice),length("温泉"),length(map),length(channel),capacity(array),capacity(slice),capacity(channel)];}
export function ClosureBody():int{let calls=0;const next=():int=>{calls++;return 0;};const n=len(copyArray[[1]Callback]([()=>next()]));return n+calls*100;}
export function ConstantIndex():int{const matrix:[1][3]int=[[1,2,3]];const n=len(matrix[len("x")-1]);return n;}
export function Panics():int{const get=():int[]=>[];return len(copyArray[[3]int](get()));}
export function IndexCall(index:int):int{const matrix:[1][3]int=[[1,2,3]];const get=():int=>index;return cap(matrix[get()]);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "arraylengthconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const n = len(a)", "const alias = n", "const m = cap(&a)", "const out = len(matrix[index])", "var first = len(get())", "var n = len(value)", "var n = cap(value)"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import "hash/crc32"
type array [3]int
type holder struct{values [3]int}
type sized interface{~[3]int|~[]int|~string|~map[string]int|~chan int}
type capacityType interface{~[3]int|~[]int|~chan int}
func length[T sized](value T)int{n:=len(value);address:=&n;return *address}
func capacity[T capacityType](value T)int{n:=cap(value);address:=&n;return *address}
func Sizes(a [3]int)[]int{const n=len(a);const alias=n;const m=cap(&a);named:=array(a);const count=len(named);xs:=make([]int,alias,m);return []int{n,m,count,len(xs),3,3,3}}
func Ignored(index int)[]int{var pointer *[3]int;var h *holder;matrix:=[1][3]int{{1,2,3}};empty:=[]int{};var table *crc32.Table;const n=len(pointer);const m=cap(*pointer);const out=len(matrix[index]);const field=cap(h.values);const converted=len([3]int(empty));const viewed=cap((*[3]int)(empty));const imported=len(table);return []int{n,m,out,field,converted,viewed,imported}}
func Runtime()int{calls:=0;get:=func()[3]int{calls++;return [3]int{1,2,3}};first:=len(get());alias:=first;address:=&alias;second:=cap(get());return calls*100+*address+second}
func Receive()int{channel:=make(chan [3]int,2);channel<-[3]int{1,2,3};channel<-[3]int{4,5,6};first:=len(<-channel);second:=cap(<-channel);address:=&first;return *address+second+len(channel)*100}
func Generic()[]int{a:=[3]int{1,2,3};s:=make([]int,2,5);m:=map[string]int{"x":1};c:=make(chan int,4);return []int{length(a),length(s),length("温泉"),length(m),length(c),capacity(a),capacity(s),capacity(c)}}
func ClosureBody()int{calls:=0;next:=func()int{calls++;return 0};const n=len([1]func()int([]func()int{func()int{return next()}}));return n+calls*100}
func ConstantIndex()int{matrix:=[1][3]int{{1,2,3}};const n=len(matrix[len("x")-1]);return n}
func Panics()int{get:=func()[]int{return []int{}};return len([3]int(get()))}
func IndexCall(index int)int{matrix:=[1][3]int{{1,2,3}};get:=func()int{return index};return cap(matrix[get()])}
`
	comparison := `package arraylengthconstants_test
import("testing";"reflect";g "array-length-constants.test";r "array-length-constants.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestLengths(t *testing.T){if !reflect.DeepEqual(g.Sizes([3]int{7,8,9}),r.Sizes([3]int{7,8,9}))||!reflect.DeepEqual(g.Generic(),r.Generic()){t.Fatal("sizes/generics")};for _,i:=range []int{-1,0,999}{if !reflect.DeepEqual(g.Ignored(i),r.Ignored(i)){t.Fatal("unevaluated")};gp:=panics(func(){g.IndexCall(i)});rp:=panics(func(){r.IndexCall(i)});if gp!=rp{t.Fatal("index call panic")}};if g.Runtime()!=r.Runtime()||g.Receive()!=r.Receive()||g.ClosureBody()!=r.ClosureBody()||g.ConstantIndex()!=r.ConstantIndex(){t.Fatal("evaluation")};gp:=panics(func(){g.Panics()});rp:=panics(func(){r.Panics()});if gp!=rp{t.Fatal("conversion call panic")}}
`
	runGeneratedGoDifferentialTest(t, root, "array-length-constants.test", generated, reference, comparison)
}
