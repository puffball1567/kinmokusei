package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMixedGenericCollectionsMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module mixed-generic-collections.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Sequence[E any] interface{~[]E|~[2]E|~*[3]E}
type Text interface{~string|~[]byte}
type Bytes []byte
type Label string
`,
		"helpers.km": `import go contracts from "mixed-generic-collections.test/contracts";
export constraint Sequence<E>=contracts.Sequence<E>;
export function first<E,S extends Sequence<E>>(values:S):E{return values[0];}
export function setFirst<E,S extends Sequence<E>>(values:S,value:E):E{const pointer=&values[0];*pointer=value;return values[0];}
export function tail<T extends contracts.Text>(value:T,lo:int,hi:int):T{return value[lo:hi];}
export function byteAt<T extends contracts.Text>(value:T,i:int):byte{return value[i];}
`,
		"entry.km": `import {Sequence,first,setFirst,tail,byteAt} from "./helpers";
import go contracts from "mixed-generic-collections.test/contracts";
type Pair=distinct [2]int;
type Numbers=distinct int[];
constraint Lengths=~[2]int|~[3]int;
constraint Mutable=~int[]|~*[3]int;
function last<T extends Lengths>(value:T):int{return value[1];}
function evaluate<T extends Mutable>(get:()=>T,index:()=>int,rhs:()=>int):int{get()[index()]=rhs();return get()[0];}
class Access<E>{public function read<S extends Sequence<E>>(values:S):E{return values[0];}}
class Store<E,S extends Sequence<E>>{constructor(private values:S){}public function read():E{return this.values[0];}}
interface Reader{function read():int;}
class Base implements Reader{constructor(public value:int){}public virtual function read():int{return this.value;}}
class Child extends Base{constructor(value:int){super(value);}public override function read():int{return this.value*2;}}
constraint Objects=~Base[]|~[2]Base;
function replace<T extends Objects>(values:T):void{values[0]=new Child(7);}
alias Maybe=Base|null;
export const Global=():int=>first<int,int[]>([6]);
export function Values():int[]{const values=Numbers([1,2]);const a=setFirst<int,Numbers>(values,4);let raw:[2]int=[2,3];const pair=Pair(raw);const b=setFirst<int,Pair>(pair,8);let long:[3]int=[5,6,7];const c=setFirst<int,*[3]int>(&long,9);return [a,values[0],b,pair[0],c,long[0],last(raw),last(long),Global()];}
export function ObjectsCase():int[]{const values:Base[]=[new Child(3)];const access=new Access<Base>();const store=new Store<Base,Base[]>(values);const before=access.read<Base[]>(values).read();replace(values);const after=store.read().read();const items:Reader[]=[new Child(5)];const maybe:Maybe[]=[null];const optional=new Access<Maybe>().read<Maybe[]>(maybe);if(optional!==null){return [optional.read()];}return [before,after,first<Reader,Reader[]>(items).read()];}
export function Evaluation():int[]{let order=0;const values=[1,2];const get=():int[]=>{order=order*10+1;return values;};const index=():int=>{order=order*10+2;return 0;};const rhs=():int=>{order=order*10+3;return 8;};const value=evaluate(get,index,rhs);return [order,value,values[0]];}
export function Aliasing():int[]{const raw:byte[]=[1,2,3];const values=contracts.Bytes(raw);const rest:contracts.Bytes=tail(values,1,3);rest[0]=8;return [int(byteAt(values,1)),len(rest),cap(rest)];}
export function Text(value:string,lo:int,hi:int):string{const result:contracts.Label=tail(contracts.Label(value),lo,hi);return string(result);}
export function Bytes(value:byte[],lo:int,hi:int):byte[]{return tail(value,lo,hi);}
export function Byte(value:string,i:int):byte{return byteAt(value,i);}
export function ByteSlice(value:byte[],i:int):byte{return byteAt(value,i);}
function arrayIndex<T extends Lengths>(value:T,i:int):int{return value[i];}
export function ArrayIndex(value:[2]int,i:int):int{return arrayIndex(value,i);}
export function NilPointer():int{const value:*[3]int=nil;return first<int,*[3]int>(value);}
`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "mixedcollections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type sequence[E any] interface{~[]E|~[2]E|~*[3]E}
type text interface{~string|~[]byte}
type pair [2]int
type numbers []int
type bytes []byte
type label string
func first[E any,S sequence[E]](v S)E{return v[0]}
func setFirst[E any,S sequence[E]](v S,e E)E{p:=&v[0];*p=e;return v[0]}
func tail[T text](v T,lo,hi int)T{return v[lo:hi]}
func byteAt[T text](v T,i int)byte{return v[i]}
func last[T interface{~[2]int|~[3]int}](v T)int{return v[1]}
func evaluate[T interface{~[]int|~*[3]int}](get func()T,index,rhs func()int)int{get()[index()]=rhs();return get()[0]}
var Global=func()int{return first[int]([]int{6})}
func Values()[]int{v:=numbers{1,2};a:=setFirst[int](v,4);raw:=[2]int{2,3};p:=pair(raw);b:=setFirst[int](p,8);long:=[3]int{5,6,7};c:=setFirst[int](&long,9);return []int{a,v[0],b,p[0],c,long[0],last(raw),last(long),Global()}}
type reader interface{read()int}
type base struct{value int;twice bool}
func (b *base)read()int{if b.twice{return b.value*2};return b.value}
func ObjectsCase()[]int{v:=[]*base{{3,true}};before:=first[*base](v).read();v[0]=&base{7,true};after:=first[*base](v).read();items:=[]reader{&base{5,true}};maybe:=[]*base{nil};if p:=first[*base](maybe);p!=nil{return []int{p.read()}};return []int{before,after,first[reader](items).read()}}
func Evaluation()[]int{order:=0;v:=[]int{1,2};get:=func()[]int{order=order*10+1;return v};index:=func()int{order=order*10+2;return 0};rhs:=func()int{order=order*10+3;return 8};value:=evaluate(get,index,rhs);return []int{order,value,v[0]}}
func Aliasing()[]int{v:=bytes{1,2,3};rest:=tail(v,1,3);rest[0]=8;return []int{int(byteAt(v,1)),len(rest),cap(rest)}}
func Text(v string,lo,hi int)string{return string(tail(label(v),lo,hi))}
func Bytes(v []byte,lo,hi int)[]byte{return tail(v,lo,hi)}
func Byte(v string,i int)byte{return byteAt(v,i)}
func ByteSlice(v []byte,i int)byte{return byteAt(v,i)}
func arrayIndex[T interface{~[2]int|~[3]int}](v T,i int)int{return v[i]}
func ArrayIndex(v [2]int,i int)int{return arrayIndex(v,i)}
func NilPointer()int{var v *[3]int;return first[int](v)}
`
	comparison := `package mixedcollections_test
import("testing";"reflect";g "mixed-generic-collections.test";r "mixed-generic-collections.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestContracts(t *testing.T){for _,p:=range [][2][]int{{g.Values(),r.Values()},{g.ObjectsCase(),r.ObjectsCase()},{g.Evaluation(),r.Evaluation()},{g.Aliasing(),r.Aliasing()}}{if !reflect.DeepEqual(p[0],p[1]){t.Fatalf("got %v want %v",p[0],p[1])}};if !panics(func(){g.NilPointer()})||!panics(func(){r.NilPointer()}){t.Fatal("nil pointer")}}
func TestBounds(t *testing.T){for _,s:=range []string{"","abc","温"}{for lo:=-1;lo<=4;lo++{var a,b byte;pa:=panics(func(){a=g.Byte(s,lo)});pb:=panics(func(){b=r.Byte(s,lo)});if pa!=pb||a!=b{t.Fatal("text index")};for hi:=-1;hi<=4;hi++{var x,y string;px:=panics(func(){x=g.Text(s,lo,hi)});py:=panics(func(){y=r.Text(s,lo,hi)});if px!=py||x!=y{t.Fatal("text slice")}}}};for _,v:=range [][]byte{nil,{},make([]byte,1,3),{1,2,3}}{for lo:=-1;lo<=4;lo++{var a,b byte;pa:=panics(func(){a=g.ByteSlice(v,lo)});pb:=panics(func(){b=r.ByteSlice(v,lo)});if pa!=pb||a!=b{t.Fatal("byte index")};for hi:=-1;hi<=4;hi++{var x,y []byte;px:=panics(func(){x=g.Bytes(v,lo,hi)});py:=panics(func(){y=r.Bytes(v,lo,hi)});if px!=py||!reflect.DeepEqual(x,y)||cap(x)!=cap(y){t.Fatal("byte slice")}}}};for i:=-1;i<=3;i++{var a,b int;pa:=panics(func(){a=g.ArrayIndex([2]int{1,2},i)});pb:=panics(func(){b=r.ArrayIndex([2]int{1,2},i)});if pa!=pb||a!=b{t.Fatal("array index")}}}
`
	runGeneratedGoDifferentialTest(t, root, "mixed-generic-collections.test", generated, reference, comparison)
}
