package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstraintIntersectionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"bounds.km": `
export { Narrow as Integer, Exact, Slice, Score, Leaf, Maybe };
constraint Narrow = Number & Scalar;
constraint Number = ~int | ~int8;
constraint Scalar = ~int | ~string;
type Score = distinct int;
constraint Exact = Narrow & Score;
constraint Storage<E> = ~E[];
constraint Slice<E> = Storage<E> & ~E[];
class Leaf { constructor(public value:int){} }
alias Maybe = Leaf | null;
`,
		"bridge.km": `export { Integer as Both, Exact, Slice, Score, Leaf, Maybe } from "./bounds";`,
		"entry.km": `
import { Both, Exact, Slice, Score, Leaf, Maybe } from "./bridge";
constraint Public = Both & ~int;
constraint Reused = Exact | ~int8;
constraint Nullable = Slice<Maybe> & ~Maybe[];
function twice<T extends Public>(x:T):T{return x*2;}
function keep<T extends Reused>(x:T):T{return x;}
function exact<T extends Exact>(x:T):T{return x+1;}
function copy<S extends Slice<E>,E>(xs:S):E[]{let result:E[]=[];for(const x of xs){result=append(result,x);}return result;}
class Box<T extends Public>{constructor(public value:T){} public function doubled():T{return twice(this.value);}}
struct Pair<T extends Public>{public a:T;public b:T;public function total():T{return this.a+this.b;}}
function IntValue(x:int):int{return new Box<int>(x).doubled()+Pair<int>{a:x,b:x}.total();}
function ScoreValue(x:int):int{return int(exact(keep(Score(x))));}
function Int8Value(x:int8):int8{return keep(x);}
function Copy(xs:int[]):int[]{return copy(xs);}
function Text(xs:string[]):string[]{return copy(xs);}
function total<S extends Nullable>(xs:S):int{let result=0;for(const x of xs){if(x!==null){result+=x.value;}}return result;}
function Objects(x:int):int{const leaf=new Leaf(x);const xs:Maybe[]=[null,leaf];const result=copy(xs);let identity=0;if(result[1]===leaf){identity=1;}return total(result)+identity;}
`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "intersections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "type Public interface {\n\tNarrow\n\t~int\n}") {
		t.Fatalf("intersection must retain linked embedded operands:\n%s", generated)
	}
	reference := `package reference
type Number interface{~int|~int8}
type Scalar interface{~int|~string}
type Public interface{Number;Scalar;~int}
type Score int
type Exact interface{Public;Score}
type Reused interface{Exact|~int8}
type Storage[E any] interface{~[]E}
type Slice[E any] interface{Storage[E];~[]E}
type Leaf struct{value int}
type Nullable interface{Slice[*Leaf];~[]*Leaf}
func twice[T Public](x T)T{return x*2}
func keep[T Reused](x T)T{return x}
func exact[T Exact](x T)T{return x+1}
func copy[S Slice[E],E any](xs S)[]E{result:=[]E{};for _,x:=range xs{result=append(result,x)};return result}
type Box[T Public]struct{value T}
func(b *Box[T])doubled()T{return twice(b.value)}
type Pair[T Public]struct{a,b T}
func(p Pair[T])total()T{return p.a+p.b}
func IntValue(x int)int{return (&Box[int]{x}).doubled()+(Pair[int]{x,x}).total()}
func ScoreValue(x int)int{return int(exact(keep(Score(x))))}
func Int8Value(x int8)int8{return keep(x)}
func Copy(xs []int)[]int{return copy(xs)}
func Text(xs []string)[]string{return copy(xs)}
func total[S Nullable](xs S)int{result:=0;for _,x:=range xs{if x!=nil{result+=x.value}};return result}
func Objects(x int)int{leaf:=&Leaf{x};xs:=[]*Leaf{nil,leaf};result:=copy(xs);identity:=0;if result[1]==leaf{identity=1};return total(result)+identity}
`
	comparison := `package intersections_test
import("reflect";"testing";generated "constraint-intersections.test";reference "constraint-intersections.test/reference")
type Named int
func external[T generated.Public](x T)T{return x+x}
func TestBehavior(t *testing.T){
 for _,x:=range []int{-127,-1,0,1,126}{
  for _,pair:=range [][2]int{{generated.IntValue(x),reference.IntValue(x)},{generated.ScoreValue(x),reference.ScoreValue(x)},{generated.Objects(x),reference.Objects(x)},{int(external(Named(x))),x+x}}{if pair[0]!=pair[1]{t.Errorf("x=%d got=%d want=%d",x,pair[0],pair[1])}}
  if got,want:=generated.Int8Value(int8(x)),reference.Int8Value(int8(x));got!=want{t.Errorf("int8=%d want=%d",got,want)}
 }
 for _,xs:=range [][]int{nil,{}, {1,-3,4}}{if got,want:=generated.Copy(xs),reference.Copy(xs);!reflect.DeepEqual(got,want){t.Errorf("copy=%v want=%v",got,want)}}
 for _,xs:=range [][]string{nil,{}, {"金木犀","hello"}}{if got,want:=generated.Text(xs),reference.Text(xs);!reflect.DeepEqual(got,want){t.Errorf("text=%v want=%v",got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "constraint-intersections.test", generated, reference, comparison)
}
