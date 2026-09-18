package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericIndexSliceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-index-slice.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Slice[E any] interface{~[]E}
type Numbers []int
type Text string
`,
		"helpers.km": `import go contracts from "generic-index-slice.test/contracts";
export constraint Slice<E>=contracts.Slice<E>;
export function first<E,S extends Slice<E>>(values:S):E{return values[0];}
export function tail<E,S extends Slice<E>>(values:S):S{return values[1:];}
export function limit<E,S extends Slice<E>>(values:S):S{return values[:2:2];}
`,
		"entry.km": `import {Slice,first,tail,limit} from "./helpers";
import go contracts from "generic-index-slice.test/contracts";
type Numbers=distinct int[];
type Other=distinct int[];
type Pair=distinct [2]int;
type Scores=distinct Map<string,int>;
constraint Named=Numbers|Other;
constraint Array<E>=~[2]E;
constraint Pointer<E>=~*[2]E;
constraint Mapping<E>=~Map<string,E>;
constraint Text=~string;
function named<S extends Named>(values:S):S{values[0]+=2;return values[:];}
function arraySlice<E,A extends Array<E>>(values:A):E[]{values[0]=values[1];return values[:];}
function pointerSlice<E,A extends Pointer<E>>(values:A):E[]{const p=&values[0];*p=values[1];return values[:];}
function readMap<E,M extends Mapping<E>>(values:M,fallback:E):E{const [value,ok]=values["key"];if(ok){return value;}values["key"]=fallback;return fallback;}
function textSlice<T extends Text>(value:T):T{return value[1:];}
function textByte<T extends Text>(value:T):byte{return value[0];}
function evaluate<S extends Slice<int>>(get:()=>S,idx:()=>int,high:()=>int,max:()=>int,rhs:()=>int):S{const a=get()[idx():high():max()];get()[idx()]=rhs();return a;}
class Access<E>{public function read<S extends Slice<E>>(values:S):E{return values[0];}public function view<S extends Slice<E>>(values:S):S{return values[:];}}
class Store<E,S extends Slice<E>>{constructor(private values:S){}public function read():E{return this.values[0];}public function tail():S{return this.values[1:];}}
interface Reader{function read():int;}
class Base implements Reader{constructor(public value:int){}public virtual function read():int{return this.value;}}
class Child extends Base{constructor(value:int){super(value);}public override function read():int{return this.value*2;}}
alias Maybe=Base|null;
constraint Objects=~Base[];
function replace<S extends Objects>(values:S):void{values[0]=new Child(7);}
export function Slices():int[]{const values=Numbers([1,2,3]);const rest:Numbers=tail(values);rest[0]=8;const bounded:Numbers=limit(values);const grown=append(bounded,9);grown[0]=7;const p=&values[0];*p=4;const a:Numbers=named(values);const b:Other=named(Other([5]));const external=contracts.Numbers([6,7]);const externalTail:contracts.Numbers=tail(external);return [first(values),rest[0],len(rest),cap(rest),cap(bounded),grown[0],a[0],b[0],externalTail[0]];}
export function Arrays():int[]{let values:[2]int=[1,2];const copied=arraySlice(Pair(values));const pointer=&values;const view=pointerSlice<int>(pointer);view[0]=9;return [values[0],values[1],copied[0],copied[1],cap(view)];}
export function Maps():int[]{const values=Scores(makeMap<string,int>());const a=readMap(values,7);const b=readMap(values,9);return [a,b,values["key"]];}
export function Texts():string{const value=contracts.Text("A温");const rest:contracts.Text=textSlice(value);return string(rest);}
export function Byte():byte{return textByte(contracts.Text("温"));}
export function ObjectsCase():int[]{const values:Base[]=[new Child(3),new Base(4)];const access=new Access<Base>();const store=new Store<Base,Base[]>(values);const view=access.view(values);view[0].value=5;const before=access.read(values).read();replace(view);const rest=store.tail();rest[0].value=8;return [before,store.read().read(),values[1].value];}
export function Nullable():int{const values:Maybe[]=[null,new Child(3)];const access=new Access<Maybe>();const item=access.read(access.view(values));if(item!==null){return item.read();}return 0;}
export function Interfaces():int{const values:Reader[]=[new Child(3)];return first(values).read();}
export function Nil():boolean{const values:Numbers=nil;return new Access<int>().view(values)===nil;}
export function Evaluation():int[]{let order=0;const values=[1,2,3];const get=():Numbers=>{order=order*10+1;return Numbers(values);};const idx=():int=>{order=order*10+2;return 0;};const high=():int=>{order=order*10+3;return 2;};const max=():int=>{order=order*10+4;return 3;};const rhs=():int=>{order=order*10+5;return 9;};const a=evaluate(get,idx,high,max,rhs);return [order,a[0],values[0]];}
function index<S extends Slice<int>>(values:S,i:int):int{return values[i];}
function slice<S extends Slice<int>>(values:S,lo:int,hi:int,max:int):S{return values[lo:hi:max];}
export function Index(values:int[],i:int):int{return index(values,i);}
export function Slicing(values:int[],lo:int,hi:int,max:int):int[]{return slice(values,lo,hi,max);}
export function NilMap():int{const values:Scores=nil;return readMap(values,1);}
`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericindex")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type numbers []int
type other []int
type pair [2]int
type scores map[string]int
type external []int
type text string
func first[E any,S ~[]E](v S)E{return v[0]}
func tail[E any,S ~[]E](v S)S{return v[1:]}
func limit[E any,S ~[]E](v S)S{return v[:2:2]}
func named[S interface{numbers|other}](v S)S{v[0]+=2;return v[:]}
func arraySlice[E any,A ~[2]E](v A)[]E{v[0]=v[1];return v[:]}
func pointerSlice[E any,A ~*[2]E](v A)[]E{p:=&v[0];*p=v[1];return v[:]}
func readMap[E any,M ~map[string]E](v M,fallback E)E{value,ok:=v["key"];if ok{return value};v["key"]=fallback;return fallback}
func textSlice[T ~string](v T)T{return v[1:]}
func textByte[T ~string](v T)byte{return v[0]}
func evaluate[S ~[]int](get func()S,idx,high,max,rhs func()int)S{a:=get()[idx():high():max()];get()[idx()]=rhs();return a}
func Slices()[]int{v:=numbers{1,2,3};rest:=tail(v);rest[0]=8;bounded:=limit(v);grown:=append(bounded,9);grown[0]=7;p:=&v[0];*p=4;a:=named(v);b:=named(other{5});externalTail:=tail(external{6,7});return []int{first(v),rest[0],len(rest),cap(rest),cap(bounded),grown[0],a[0],b[0],externalTail[0]}}
func Arrays()[]int{v:=[2]int{1,2};copied:=arraySlice(pair(v));view:=pointerSlice(&v);view[0]=9;return []int{v[0],v[1],copied[0],copied[1],cap(view)}}
func Maps()[]int{v:=scores{};a:=readMap(v,7);b:=readMap(v,9);return []int{a,b,v["key"]}}
func Texts()string{return string(textSlice(text("A温")))}
func Byte()byte{return textByte(text("温"))}
type reader interface{read()int}
type base struct{value int;twice bool}
func (b *base)read()int{if b.twice{return b.value*2};return b.value}
func ObjectsCase()[]int{v:=[]*base{{3,true},{4,false}};view:=v[:];view[0].value=5;before:=v[0].read();view[0]=&base{7,true};rest:=v[1:];rest[0].value=8;return []int{before,v[0].read(),v[1].value}}
func Nullable()int{v:=[]*base{nil,{3,true}};item:=first(v[:]);if item!=nil{return item.read()};return 0}
func Interfaces()int{v:=[]reader{&base{3,true}};return first(v).read()}
func Nil()bool{var v numbers;return v[:]==nil}
func Evaluation()[]int{order:=0;v:=[]int{1,2,3};get:=func()numbers{order=order*10+1;return numbers(v)};idx:=func()int{order=order*10+2;return 0};high:=func()int{order=order*10+3;return 2};max:=func()int{order=order*10+4;return 3};rhs:=func()int{order=order*10+5;return 9};a:=evaluate(get,idx,high,max,rhs);return []int{order,a[0],v[0]}}
func index[S ~[]int](v S,i int)int{return v[i]}
func slice[S ~[]int](v S,lo,hi,max int)S{return v[lo:hi:max]}
func Index(v []int,i int)int{return index(v,i)}
func Slicing(v []int,lo,hi,max int)[]int{return slice(v,lo,hi,max)}
func NilMap()int{var v scores;return readMap(v,1)}
`
	comparison := `package genericindex_test
import("testing";"reflect";g "generic-index-slice.test";r "generic-index-slice.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestContracts(t *testing.T){for _,p:=range [][2][]int{{g.Slices(),r.Slices()},{g.Arrays(),r.Arrays()},{g.Maps(),r.Maps()},{g.ObjectsCase(),r.ObjectsCase()},{g.Evaluation(),r.Evaluation()}}{if !reflect.DeepEqual(p[0],p[1]){t.Fatalf("got %v want %v",p[0],p[1])}};if g.Texts()!=r.Texts()||g.Byte()!=r.Byte()||g.Nullable()!=r.Nullable()||g.Interfaces()!=r.Interfaces()||g.Nil()!=r.Nil(){t.Fatal("scalar contracts")};if !panics(func(){g.NilMap()})||!panics(func(){r.NilMap()}){t.Fatal("nil map write")}}
func TestBounds(t *testing.T){for _,v:=range [][]int{nil,{},make([]int,1,4),{1,2,3}}{for i:=-1;i<=4;i++{var a,b int;pa:=panics(func(){a=g.Index(v,i)});pb:=panics(func(){b=r.Index(v,i)});if pa!=pb||a!=b{t.Fatal("index")};for hi:=-1;hi<=4;hi++{for max:=-1;max<=4;max++{var x,y []int;px:=panics(func(){x=g.Slicing(v,i,hi,max)});py:=panics(func(){y=r.Slicing(v,i,hi,max)});if px!=py||!reflect.DeepEqual(x,y)||cap(x)!=cap(y){t.Fatalf("slice %d:%d:%d",i,hi,max)}}}}}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-index-slice.test", generated, reference, comparison)
}
