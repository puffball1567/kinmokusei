package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstrainedRangesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constraints"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module constrained-range.test\n\ngo 1.23\n",
		"constraints/constraints.go": `package constraints
type Slice[E any] interface { ~[]E }
type Maps interface { ~map[string]int }
type Array interface { ~[3]int }
type Pointer interface { ~*[3]int }
type Receive interface { ~chan int | ~<-chan int }
type Iterator interface { ~func(func(int) bool) }
type Narrowed interface { ~[]int | ~string; ~[]int }
func Closed() chan int { values:=make(chan int,3); values<-1; values<-2; values<-3; close(values); return values }
func ReadOnly() <-chan int { return Closed() }
func Iterate(yield func(int) bool) { for _,value:=range []int{1,2,3,4} { if !yield(value) { return } } }
`,
		"ranges.km": `
import go c from "constrained-range.test/constraints";
constraint Text = ~string;
type Numbers = distinct int[];
function weighted<S extends c.Slice<int>>(values: S): int { let total=0; for (const [index,value] of values) { total += (index+1)*value; } return total; }
function copyValues<S extends c.Slice<string>>(values: S): string[] { let result:string[]=[]; for (const value of values) { result=append(result,value); } return result; }
function array<S extends c.Array>(values: S): int { let total=0; for (const value of values) { total+=value; } return total; }
function pointer<S extends c.Pointer>(values: S): int { let total=0; for (const value of values) { total+=value; } return total; }
function indexes<S extends c.Pointer>(values: S): int { let total=0; for (const [index,_] of values) { total+=index; } return total; }
function maps<S extends c.Maps>(values: S): int { let total=0; for (const [key,value] of values) { total+=len(key)+value; } return total; }
function text<S extends Text>(value: S): int { let total=0; for (const [index,rune] of value) { total+=index+int(rune); } return total; }
function drain<S extends c.Receive>(channel: S): int { let total=0; for (const value of channel) { total+=value; } return total; }
function iterator<S extends c.Iterator>(values: S): int { let total=0; for (const value of values) { if(value==3) { break; } total+=value; } return total; }
function narrowed<S extends c.Narrowed>(values: S): int { let total=0; for (const value of values) { total+=value; } return total; }
alias Callback = () => int;
function closures<S extends c.Slice<int>>(values: S): int { let callbacks:Callback[]=[]; for (const value of values) { callbacks=append(callbacks,():int=>value); } let total=0; for(const callback of callbacks) { total=total*10+callback(); } return total; }
function observed<S extends c.Slice<int>>(calls:*int, values:S): S { *calls+=1; return values; }
function once<S extends c.Slice<int>>(values:S): int { let calls=0; let total=0; for(const value of observed(&calls,values)) { if(value<0) { continue; } total+=value; } return calls*1000+total; }
class Leaf { constructor(public value:int) {} public virtual function read():int { return this.value; } }
class Doubled extends Leaf { constructor(value:int) { super(value); } public override function read():int { return this.value*2; } }
constraint Leaves = ~Leaf[];
constraint PairLeaves = ~[2]Leaf;
function pairTotal<T extends PairLeaves>(values:T):int { let total=0; for(const leaf of values){total+=leaf.read();} return total; }
class Collection<S extends Leaves> {
  constructor(private values:S) {}
  public function total():int { let total=0; for(const leaf of this.values) { total+=leaf.read(); } return total; }
}
class Holder<S extends c.Array> { public leaf:Leaf; constructor(values:S) { for(const value of values) { this.leaf=new Leaf(value); } } }
`,
		"entry.km": `
import { Numbers, weighted, copyValues, array, pointer, indexes, maps, text, drain, iterator, narrowed, closures, once, Leaf, Doubled, Collection, Holder, pairTotal } from "./ranges";
import go c from "constrained-range.test/constraints";
function Slices(values:int[]):int[] { return [weighted(values), weighted(Numbers(values)), narrowed(values), closures(values), once(values)]; }
function Copy(values:string[]):string[] { return copyValues(values); }
function Maps(values:Map<string,int>):int { return maps(values); }
function Text(value:string):int { return text(value); }
function Arrays(values:[3]int):int[] { return [array(values), pointer(&values), indexes(&values), new Holder<[3]int>(values).leaf.value]; }
function PointerValues(values:*[3]int):int { return pointer(values); }
function PointerIndexes(values:*[3]int):int { return indexes(values); }
function Streams():int[] { return [drain(c.Closed()),drain(c.ReadOnly()),iterator(c.Iterate)]; }
function Objects(value:int):int { const leaf:Leaf=new Doubled(value); const values:Leaf[]=[leaf,new Leaf(value+1)]; const pair:[2]Leaf=[leaf,new Leaf(value+1)]; let total=0; for(const item of values){total+=item.read();} return total+new Collection<Leaf[]>(values).total()+pairTotal(pair); }
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "constrainedrange")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Numbers []int
func weighted[S ~[]int](values S) int { total:=0; for index,value:=range values { total+=(index+1)*value }; return total }
func copyValues[E any,S ~[]E](values S) []E { result:=[]E{}; for _,value:=range values { result=append(result,value) }; return result }
func array[S ~[3]int](values S) int { total:=0; for _,value:=range values { total+=value }; return total }
func pointer[S ~*[3]int](values S) int { total:=0; for _,value:=range values { total+=value }; return total }
func indexes[S ~*[3]int](values S) int { total:=0; for index:=range values { total+=index }; return total }
func maps[S ~map[string]int](values S) int { total:=0; for key,value:=range values { total+=len(key)+value }; return total }
func text[S ~string](value S) int { total:=0; for index,rune:=range value { total+=index+int(rune) }; return total }
func drain[S interface{~chan int|~<-chan int}](values S) int { total:=0; for value:=range values { total+=value }; return total }
func iterator[S ~func(func(int)bool)](values S) int { total:=0; for value:=range values { if value==3 { break }; total+=value }; return total }
func narrowed[S interface{~[]int|~string;~[]int}](values S) int { total:=0; for _,value:=range values { total+=value }; return total }
func closures[S ~[]int](values S) int { var callbacks []func()int; for _,value:=range values { callbacks=append(callbacks,func()int{return value}) }; total:=0; for _,callback:=range callbacks { total=total*10+callback() }; return total }
func observed[S ~[]int](calls *int,values S) S { *calls++; return values }
func once[S ~[]int](values S) int { calls,total:=0,0; for _,value:=range observed(&calls,values) { if value<0 { continue }; total+=value }; return calls*1000+total }
func Slices(values []int) []int { return []int{weighted(values),weighted(Numbers(values)),narrowed(values),closures(values),once(values)} }
func Copy(values []string) []string { return copyValues[string](values) }
func Maps(values map[string]int) int { return maps(values) }
func Text(value string) int { return text(value) }
type Leaf struct{Value int}
func (leaf *Leaf) Read() int { return leaf.Value }
type Doubled struct{Leaf}
func (leaf *Doubled) Read() int { return leaf.Value*2 }
type Collection[S ~[]interface{Read()int}] struct{values S}
func (collection *Collection[S]) Total() int { total:=0;for _,leaf:=range collection.values { total+=leaf.Read() };return total }
func pairTotal[T ~[2]interface{Read()int}](values T) int {total:=0;for _,leaf:=range values{total+=leaf.Read()};return total}
func Objects(value int) int { leaf:=&Doubled{Leaf{value}};leaves:=[]interface{Read()int}{leaf,&Leaf{value+1}};pair:=[2]interface{Read()int}{leaf,&Leaf{value+1}};total:=0;for _,item:=range leaves{total+=item.Read()};return total+(&Collection[[]interface{Read()int}]{leaves}).Total()+pairTotal(pair) }
func Arrays(values [3]int) []int { var leaf *Leaf;for _,value:=range values { leaf=&Leaf{value} };return []int{array(values),pointer(&values),indexes(&values),leaf.Value} }
func PointerValues(values *[3]int) int { return pointer(values) }
func PointerIndexes(values *[3]int) int { return indexes(values) }
func closed() chan int { c:=make(chan int,3);c<-1;c<-2;c<-3;close(c);return c }
func iterate(yield func(int)bool) { for _,value:=range []int{1,2,3,4} { if !yield(value) { return } } }
func Streams() []int { var receive <-chan int=closed();return []int{drain(closed()),drain(receive),iterator(iterate)} }
`
	comparison := `package constrainedrange_test
import (
 "reflect"
 "testing"
 generated "constrained-range.test"
 reference "constrained-range.test/reference"
)
func panics(call func()) (result bool) { defer func(){result=recover()!=nil}();call();return }
func TestRanges(t *testing.T) {
 for _,values:=range [][]int{nil,{}, {1}, {1,2,3}, {-1,0,2}} { if got,want:=generated.Slices(values),reference.Slices(values);!reflect.DeepEqual(got,want){t.Errorf("slices=%v want=%v",got,want)} }
 for _,values:=range [][]string{nil,{}, {"hello","温泉"}} { if got,want:=generated.Copy(values),reference.Copy(values);!reflect.DeepEqual(got,want){t.Errorf("copy=%v want=%v",got,want)} }
 for _,values:=range []map[string]int{nil,{}, {"one":1,"two":2}} { if got,want:=generated.Maps(values),reference.Maps(values);got!=want{t.Errorf("map=%v want=%v",got,want)} }
 for _,value:=range []string{"","hello","温泉","\xff"} { if got,want:=generated.Text(value),reference.Text(value);got!=want{t.Errorf("text=%v want=%v",got,want)} }
 for _,values:=range [][3]int{{},{1,2,3},{-3,4,0}} { if got,want:=generated.Arrays(values),reference.Arrays(values);!reflect.DeepEqual(got,want){t.Errorf("array=%v want=%v",got,want)} }
 if got,want:=generated.Streams(),reference.Streams();!reflect.DeepEqual(got,want){t.Errorf("streams=%v want=%v",got,want)}
 for _,value:=range []int{-1,0,7} { if got,want:=generated.Objects(value),reference.Objects(value);got!=want{t.Errorf("objects=%v want=%v",got,want)} }
 if got,want:=generated.PointerIndexes(nil),reference.PointerIndexes(nil);got!=want{t.Errorf("nil index=%d want=%d",got,want)}
 if got,want:=panics(func(){generated.PointerValues(nil)}),panics(func(){reference.PointerValues(nil)});got!=want{t.Errorf("nil pointer panic=%t want=%t",got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "constrained-range.test", generated, reference, comparison)
}
