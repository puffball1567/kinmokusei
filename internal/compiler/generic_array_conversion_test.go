package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericArrayConversionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-array-conversion.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Slice[E any] interface{~[]E}
type Numbers []int
type Pair [2]int
`,
		"bounds.km": `import go contracts from "generic-array-conversion.test/contracts";
export constraint Slice<E>=contracts.Slice<E>;
export function copyPair<E,S extends Slice<E>>(values:S):[2]E{return copyArray[[2]E](values);}
export function viewPair<E,S extends Slice<E>>(values:S):*[2]E{return viewArray[[2]E](values);}
`,
		"entry.km": `import {Slice,copyPair,viewPair} from "./bounds";
import go contracts from "generic-array-conversion.test/contracts";
type Numbers=distinct int[];
type Other=distinct int[];
type Pair=distinct [2]int;
constraint Named=Numbers|Other;
function named<S extends Named>(values:S):Pair{return copyArray[Pair](values);}
function zeroCopy<E,S extends Slice<E>>(values:S):[0]E{return copyArray[[0]E](values);}
function zeroView<E,S extends Slice<E>>(values:S):*[0]E{return viewArray[[0]E](values);}
function length<S extends Slice<int>>(values:S):int{const n=len(copyArray[[2]int](values));const m=cap(viewArray[[2]int](values));return n+m;}
class Converter<E>{public function copy<S extends Slice<E>>(values:S):[2]E{return copyArray[[2]E](values);}public function view<S extends Slice<E>>(values:S):*[2]E{return viewArray[[2]E](values);}}
class Holder<E,S extends Slice<E>>{constructor(private values:S){}public function view():*[2]E{return viewArray[[2]E](this.values);}}
interface Reader{function read():int;}
class Base implements Reader{constructor(public value:int){}public virtual function read():int{return this.value;}}
class Child extends Base{constructor(value:int){super(value);}public override function read():int{return this.value*2;}}
alias Maybe=Base|null;
export function Scalars():int[]{const values=Numbers([1,2,3]);let copied=copyPair(values);const viewed=viewPair(values);copied[0]=9;viewed[1]=8;const slice=viewed[:];slice[0]=7;return [values[0],values[1],copied[0],copied[1],len(slice),cap(slice),viewed[0]];}
export function NamedArrays():int[]{const values=Numbers([1,2,3]);const copy=named(values);const other=named(Other([4,5]));const view=viewArray[Pair](values);view[0]=9;const external=contracts.Numbers([6,7]);const externalCopy=copyArray[contracts.Pair](external);const externalView=viewArray[contracts.Pair](external);externalView[1]=8;return [copy[0],other[0],values[0],externalCopy[1],external[1]];}
export function Zero():int[]{const nilSlice:Numbers=nil;const empty=Numbers(makeSlice<int>(0));const nilView=zeroView(nilSlice);const emptyView=zeroView(empty);let flags=0;if(nilView===nil){flags+=1;}if(emptyView!==nil){flags+=2;}return [len(zeroCopy(nilSlice)),len(zeroCopy(empty)),flags,length(nilSlice)];}
export function Objects():int[]{const values:Base[]=[new Child(3),new Base(4)];let copied=copyPair(values);const viewed=new Holder<Base,Base[]>(values).view();copied[0]=new Base(9);copied[1].value=11;const slice=viewed[:];slice[0].value=5;const before=values[0].read();viewed[0]=new Child(7);return [copied[0].read(),values[1].read(),before,values[0].read(),viewed[0].read(),copied[1].read()];}
export function Nullable():int[]{const values:Maybe[]=[new Child(4),null];const converter=new Converter<Maybe>();const copied=converter.copy(values);const viewed=converter.view(values);viewed[0]=null;const first=copied[0];let result=0;if(first!==null){result=first.read();}const slice=copied[:];const second=slice[1];let flags=0;if(second===null){flags+=1;}if(values[0]===null){flags+=2;}return [result,flags];}
export function Interfaces():int{const values:Reader[]=[new Child(3),new Base(4)];const view=viewPair(values);const copied=copyPair(values);return view[0].read()+copied[1].read();}
export function ObjectLengths():int[]{const values:Base[]=[new Child(3),new Base(4)];const view=viewPair(values);const n=len(view);const m=cap(view);return [n,m,len(viewPair(values)),cap(viewPair(values))];}
export function Nested():int[]{const first=[1,2];const second=[3,4];const values:int[][]=[first,second];let copied=copyPair(values);copied[0][0]=7;copied[1]=[9];const viewed=viewPair(values);const replacement=[8];viewed[1]=replacement;return [first[0],second[0],copied[1][0],values[1][0]];}
export function Evaluation():int[]{let calls=0;const next=():Numbers=>{calls++;return Numbers([1,2]);};const copied=copyArray[[2]int](next());const viewed=viewArray[[2]int](next());const n=len(copyArray[[2]int](next()));return [calls,copied[0],viewed[1],n];}
export function ShortCopy(values:int[]):[2]int{return copyPair(values);}
export function ShortView(values:int[]):*[2]int{return viewPair(values);}
export function CalledLength(values:int[]):int{return len(copyPair(values));}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericarrays")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"[2]E(values)", "(*[2]E)(values)", "const n = len([2]int(values))", "const m = cap((*[2]int)(values))"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
type numbers []int
type other []int
type pair [2]int
type externalNumbers []int
type externalPair [2]int
func copyPair[E any,S ~[]E](values S)[2]E{return [2]E(values)}
func viewPair[E any,S ~[]E](values S)*[2]E{return (*[2]E)(values)}
func named[S interface{numbers|other}](values S)pair{return pair(values)}
func zeroCopy[E any,S ~[]E](values S)[0]E{return [0]E(values)}
func zeroView[E any,S ~[]E](values S)*[0]E{return (*[0]E)(values)}
func length[S ~[]int](values S)int{const n=len([2]int(values));const m=cap((*[2]int)(values));return n+m}
func Scalars()[]int{values:=numbers{1,2,3};copied:=copyPair(values);viewed:=viewPair(values);copied[0]=9;viewed[1]=8;slice:=viewed[:];slice[0]=7;return []int{values[0],values[1],copied[0],copied[1],len(slice),cap(slice),viewed[0]}}
func NamedArrays()[]int{values:=numbers{1,2,3};copied:=named(values);other:=named(other{4,5});view:=(*pair)(values);view[0]=9;external:=externalNumbers{6,7};externalCopy:=externalPair(external);externalView:=(*externalPair)(external);externalView[1]=8;return []int{copied[0],other[0],values[0],externalCopy[1],external[1]}}
func Zero()[]int{var nilSlice numbers;empty:=numbers(make([]int,0));nilView:=zeroView(nilSlice);emptyView:=zeroView(empty);flags:=0;if nilView==nil{flags++};if emptyView!=nil{flags+=2};return []int{len(zeroCopy(nilSlice)),len(zeroCopy(empty)),flags,length(nilSlice)}}
type reader interface{read()int}
type base struct{value int;twice bool}
func (b *base)read()int{if b.twice{return b.value*2};return b.value}
func Objects()[]int{values:=[]*base{{3,true},{4,false}};copied:=copyPair(values);viewed:=viewPair(values);copied[0]=&base{9,false};copied[1].value=11;slice:=viewed[:];slice[0].value=5;before:=values[0].read();viewed[0]=&base{7,true};return []int{copied[0].read(),values[1].read(),before,values[0].read(),viewed[0].read(),copied[1].read()}}
func Nullable()[]int{values:=[]*base{{4,true},nil};copied:=copyPair(values);viewed:=viewPair(values);viewed[0]=nil;first:=copied[0];result:=0;if first!=nil{result=first.read()};slice:=copied[:];second:=slice[1];flags:=0;if second==nil{flags++};if values[0]==nil{flags+=2};return []int{result,flags}}
func Interfaces()int{values:=[]reader{&base{3,true},&base{4,false}};view:=viewPair(values);copied:=copyPair(values);return view[0].read()+copied[1].read()}
func ObjectLengths()[]int{values:=[]*base{{3,true},{4,false}};view:=viewPair(values);const n=len(view);const m=cap(view);return []int{n,m,len(viewPair(values)),cap(viewPair(values))}}
func Nested()[]int{first:=[]int{1,2};second:=[]int{3,4};values:=[][]int{first,second};copied:=copyPair(values);copied[0][0]=7;copied[1]=[]int{9};viewed:=viewPair(values);viewed[1]=[]int{8};return []int{first[0],second[0],copied[1][0],values[1][0]}}
func Evaluation()[]int{calls:=0;next:=func()numbers{calls++;return numbers{1,2}};copied:=[2]int(next());viewed:=(*[2]int)(next());n:=len([2]int(next()));return []int{calls,copied[0],viewed[1],n}}
func ShortCopy(values []int)[2]int{return copyPair(values)}
func ShortView(values []int)*[2]int{return viewPair(values)}
func CalledLength(values []int)int{return len(copyPair(values))}
`
	comparison := `package genericarrays_test
import("testing";"reflect";g "generic-array-conversion.test";r "generic-array-conversion.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestObjectLengths(t *testing.T){if got,want:=g.ObjectLengths(),r.ObjectLengths();!reflect.DeepEqual(got,want){t.Fatalf("got %v want %v",got,want)}}
func TestConversions(t *testing.T){for _,pair:=range [][2][]int{{g.Scalars(),r.Scalars()},{g.NamedArrays(),r.NamedArrays()},{g.Zero(),r.Zero()},{g.Objects(),r.Objects()},{g.Nullable(),r.Nullable()},{g.Nested(),r.Nested()},{g.Evaluation(),r.Evaluation()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatalf("got %v want %v",pair[0],pair[1])}};if g.Interfaces()!=r.Interfaces(){t.Fatal("interfaces")};for _,values:=range [][]int{nil,make([]int,0,8),make([]int,1,8),{1,2},{1,2,3}}{var gc,rc [2]int;gp:=panics(func(){gc=g.ShortCopy(values)});rp:=panics(func(){rc=r.ShortCopy(values)});if gp!=rp||gp!=(len(values)<2)||gc!=rc{t.Fatal("copy length")};gp=panics(func(){g.ShortView(values)});rp=panics(func(){r.ShortView(values)});if gp!=rp||gp!=(len(values)<2){t.Fatal("view length")};gp=panics(func(){g.CalledLength(values)});rp=panics(func(){r.CalledLength(values)});if gp!=rp||gp!=(len(values)<2){t.Fatal("called length")}}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-array-conversion.test", generated, reference, comparison)
}
