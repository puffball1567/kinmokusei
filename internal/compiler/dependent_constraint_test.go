package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependentConstraintsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constraints"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module dependent-bounds.test\n\ngo 1.23\n",
		"alias.km": `import go c from "dependent-bounds.test/constraints";
alias Values<E extends Integer,S extends c.Slice<E>> = S;
constraint Integer=~int|~int32;
function size(values:Values<int,int[]>):int{return len(values);}
`,
		"constraints/constraints.go": `package constraints
type Slice[E any] interface { ~[]E }
type Map[K comparable,V any] interface { ~map[K]V }
type Array[E any] interface { ~[2]E }
type Sequence[E any] interface { ~func(func(E)bool) }
`,
		"bounds.km": `import go c from "dependent-bounds.test/constraints";
type Numbers = distinct int[];
function elements<E,S extends c.Slice<E>>(values:S):E[] {let result:E[]=[];for(const value of values){result=append(result,value);}return result;}
function reversed<S extends c.Slice<E>,E>(values:S):E[] {return elements(values);}
function flatten<E,S extends c.Slice<E>,SS extends c.Slice<S>>(values:SS):E[] {let result:E[]=[];for(const items of values){for(const value of items){result=append(result,value);}}return result;}
function mapped<M extends c.Map<K,V>,K extends comparable,V>(values:M):V[] {let result:V[]=[];for(const [key,value] of values){result=append(result,value);}return result;}
function pairs<E,A extends c.Array<E>>(values:A):E[] {let result:E[]=[];for(const value of values){result=append(result,value);}return result;}
function sequence<E,S extends c.Sequence<E>>(iterator:S):E[] {let result:E[]=[];for(const value of iterator){result=append(result,value);break;}return result;}
function observed<S>(calls:*int,values:S):S {*calls+=1;return values;}
class Collector<E> {
 public function collect<S extends c.Slice<E>>(values:S):E[] {return elements(values);}
 public function combine<U,S extends c.Slice<U>>(values:S,fallback:U):U[] {return append(elements(values),fallback);}
}
class IntCollector extends Collector<int> {}
struct ValueCollector<E> {public function collect<S extends c.Slice<E>>(values:S):E[] {return elements(values);}}
class Stored<E,S extends c.Slice<E>> {constructor(private values:S){} public function read():E[]{return elements(this.values);}}
interface Readable<E,S extends c.Slice<E>> {function read():S;}
class ReadInts implements Readable<int,int[]> {constructor(private values:int[]){} public function read():int[]{return this.values;}}
function throughInterface<E,S extends c.Slice<E>>(source:Readable<E,S>):E[]{return elements(source.read());}
alias Values<E,S extends c.Slice<E>> = S;
type Rows<E,S extends c.Slice<E>> = distinct S[];
class Leaf {constructor(public value:int){} public virtual function read():int{return this.value;}}
class DoubleLeaf extends Leaf {constructor(value:int){super(value);}public override function read():int{return this.value*2;}}
function objects(value:int):int {const leaf:Leaf=new DoubleLeaf(value); const source:Leaf[]=[leaf,new Leaf(value+1)];const copied=elements(source);const typed=new Collector<Leaf>().collect(source);let identity=0;if(copied[0]===source[0]){identity=1;}return copied[0].read()+typed[1].read()+identity;}
`,
		"entry.km": `import {Numbers,elements,reversed,flatten,mapped,pairs,sequence,observed,Collector,IntCollector,ValueCollector,Stored,ReadInts,throughInterface,Values,Rows,objects} from "./bounds";
function Slices(values:int[]):int[][] {let calls=0;const copied=elements(observed(&calls,values));return [copied,elements<int>(values),reversed(Numbers(values)),new Collector<int>().collect(values),new IntCollector().collect(values),ValueCollector<int>{}.collect(values),new Stored<int,int[]>(values).read(),throughInterface(new ReadInts(values)),[calls]];}
function Strings(values:string[]):string[][] {return [new Collector<string>().collect(values),new Collector<int>().combine(values,"end")];}
function Flatten(values:int[][]):int[]{return flatten(values);}
function Mapped(values:Map<string,int>):int[]{return mapped(values);}
function Pairs(values:[2]int):int[]{return pairs(values);}
function Stream(values:int[]):int[]{return sequence((yield:(value:int)=>boolean):void=>{for(const value of values){if(!yield(value)){return;}}});}
function Aliased(values:Values<int,int[]>):int[]{return elements(values);}
function Defined(values:int[][]):int[]{return flatten(Rows<int,int[]>(values));}
function Objects(value:int):int{return objects(value);}
function CloneIsolation(values:int[]):int[]{const first=new Collector<int>();const second=new Collector<string>();const other=second.collect(["x"]);return first.collect(values);}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	aliasGenerated, diagnostics, err := EmitGo([]string{filepath.Join(root, "alias.km")}, "dependentbounds")
	if err != nil || len(diagnostics) != 0 || strings.Contains(string(aliasGenerated), "dependent-bounds.test/constraints") {
		t.Fatalf("erased alias retained a constraint-only import: err=%v diagnostics=%v\n%s", err, diagnostics, aliasGenerated)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "dependentbounds")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func elements[E any,S ~[]E](values S) []E {result:=[]E{};for _,value:=range values{result=append(result,value)};return result}
func reversed[S ~[]E,E any](values S) []E{return elements(values)}
func flatten[E any,S ~[]E,SS ~[]S](values SS) []E {result:=[]E{};for _,items:=range values{for _,value:=range items{result=append(result,value)}};return result}
type Numbers []int
type Rows[E any,S ~[]E] []S
type Collector[E any] struct{}
func collect[E any,S ~[]E](_ *Collector[E],values S) []E{return elements(values)}
func combine[E,U any,S ~[]U](_ *Collector[E],values S,fallback U) []U{return append(elements(values),fallback)}
type Stored[E any,S ~[]E] struct{values S}
func (s *Stored[E,S]) read() []E{return elements(s.values)}
type Readable[E any,S ~[]E] interface{read() S}
type ReadInts struct{values []int}
func (r *ReadInts) read() []int{return r.values}
func throughInterface[E any,S ~[]E](source Readable[E,S]) []E{return elements(source.read())}
func observed[S any](calls *int,values S)S{*calls++;return values}
func Slices(values []int) [][]int{calls:=0;copied:=elements(observed(&calls,values));return [][]int{copied,elements[int](values),reversed(Numbers(values)),collect(&Collector[int]{},values),collect(&Collector[int]{},values),elements(values),(&Stored[int,[]int]{values}).read(),throughInterface[int,[]int](&ReadInts{values}),{calls}}}
func Strings(values []string) [][]string{return [][]string{collect(&Collector[string]{},values),combine(&Collector[int]{},values,"end")}}
func Flatten(values [][]int) []int{return flatten(values)}
func mapped[M ~map[K]V,K comparable,V any](values M) []V{result:=[]V{};for _,value:=range values{result=append(result,value)};return result}
func Mapped(values map[string]int) []int{return mapped(values)}
func pairs[E any,A ~[2]E](values A) []E{result:=[]E{};for _,value:=range values{result=append(result,value)};return result}
func Pairs(values [2]int) []int{return pairs(values)}
func sequence[E any,S ~func(func(E)bool)](iterator S) []E{result:=[]E{};for value:=range iterator{result=append(result,value);break};return result}
func Stream(values []int) []int{return sequence(func(yield func(int)bool){for _,value:=range values{if !yield(value){return}}})}
func Aliased(values []int) []int{return elements(values)}
func Defined(values [][]int) []int{return flatten(Rows[int,[]int](values))}
type Leaf struct{value int}
func (leaf *Leaf) read()int{return leaf.value}
type DoubleLeaf struct{Leaf}
func (leaf *DoubleLeaf)read()int{return leaf.value*2}
func Objects(value int)int{leaf:=&DoubleLeaf{Leaf{value}};source:=[]interface{read()int}{leaf,&Leaf{value+1}};copied:=elements(source);typed:=collect(&Collector[interface{read()int}]{},source);identity:=0;if copied[0]==source[0]{identity=1};return copied[0].read()+typed[1].read()+identity}
func CloneIsolation(values []int) []int{first:=&Collector[int]{};second:=&Collector[string]{};_ = collect(second,[]string{"x"});return collect(first,values)}
`
	comparison := `package dependentbounds_test
import (
 "reflect"
 "sort"
 "testing"
 generated "dependent-bounds.test"
 reference "dependent-bounds.test/reference"
)
func TestBehavior(t *testing.T){
 for _,values:=range [][]int{nil,{}, {0}, {-3,4,5}}{
  if got,want:=generated.Slices(values),reference.Slices(values);!reflect.DeepEqual(got,want){t.Errorf("slices=%v want=%v",got,want)}
  for _,pair:=range [][2][]int{{generated.Stream(values),reference.Stream(values)},{generated.Aliased(values),reference.Aliased(values)},{generated.CloneIsolation(values),reference.CloneIsolation(values)}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Errorf("got=%v want=%v",pair[0],pair[1])}}
 }
 for _,values:=range [][]string{nil,{}, {"hello","温泉"}}{if got,want:=generated.Strings(values),reference.Strings(values);!reflect.DeepEqual(got,want){t.Errorf("strings=%v want=%v",got,want)}}
 for _,values:=range [][][]int{nil,{}, {nil,{}}, {{1,2},{3}}}{for _,pair:=range [][2][]int{{generated.Flatten(values),reference.Flatten(values)},{generated.Defined(values),reference.Defined(values)}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Errorf("flatten=%v want=%v",pair[0],pair[1])}}}
 for _,values:=range []map[string]int{nil,{}, {"a":1,"b":2}}{got,want:=generated.Mapped(values),reference.Mapped(values);sort.Ints(got);sort.Ints(want);if !reflect.DeepEqual(got,want){t.Errorf("map=%v want=%v",got,want)}}
 for _,values:=range [][2]int{{},{1,2},{-3,4}}{if got,want:=generated.Pairs(values),reference.Pairs(values);!reflect.DeepEqual(got,want){t.Errorf("pairs=%v want=%v",got,want)}}
 for _,value:=range []int{-3,0,7}{if got,want:=generated.Objects(value),reference.Objects(value);got!=want{t.Errorf("objects=%v want=%v",got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "dependent-bounds.test", generated, reference, comparison)
}
