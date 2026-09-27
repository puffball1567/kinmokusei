package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderedClassFieldDefaultsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	source := `let trace:int=0;
function mark(value:int):int{trace=trace*10+value;return value;}
class Pair{
 public first:int=mark(2);
 public second:int=mark(this.first+3);
 public third:int=this.second*2;
}
class Generic<T>{
 public values:T[]=[];
 public count:int=len(this.values);
}
export function Run():int[]{
 const first=new Pair();
 const second=new Pair();
 const generic=new Generic<int>();
 first.first=9;
 return [first.first,first.second,first.third,second.second,trace,generic.count];
}`
	if err := os.WriteFile(entry, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "orderedfields")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, fragment := range []string{"this.Second = mark(this.First + 3)", "this.Third = this.Second * 2", "this.Count = len(this.Values)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("missing %q in generated defaults:\n%s", fragment, generated)
		}
	}
	reference := `package reference
var trace int
func mark(value int) int { trace=trace*10+value;return value }
type Pair struct{ First,Second,Third int }
func newPair() *Pair { value:=&Pair{};value.First=mark(2);value.Second=mark(value.First+3);value.Third=value.Second*2;return value }
type Generic[T any] struct{ Values []T;Count int }
func newGeneric[T any]() *Generic[T] { value:=&Generic[T]{};value.Values=[]T{};value.Count=len(value.Values);return value }
func Run() []int { first:=newPair();second:=newPair();generic:=newGeneric[int]();first.First=9;return []int{first.First,first.Second,first.Third,second.Second,trace,generic.Count} }
`
	comparison := `package orderedfields_test
import("reflect";"testing";g "ordered-fields.test";r "ordered-fields.test/reference")
func TestOrderedDefaults(t *testing.T){got,want:=g.Run(),r.Run();if !reflect.DeepEqual(got,want){t.Fatalf("got %v want %v",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "ordered-fields.test", generated, reference, comparison)
}

func TestInheritedClassFieldDefaultsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "base.km")
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		base: `export class Base<T>{
 protected value:T;
 public count:int=4;
 constructor(value:T){this.value=value;}
}
export class Middle<U> extends Base<U>{
 public extra:int=this.count+1;
 constructor(value:U){super(value);}
}`,
		entry: `import {Middle} from "./base";
class Leaf extends Middle<int>{
 public result:int=this.value+this.extra;
 constructor(value:int){super(value);}
}
export function Run():int[]{const leaf=new Leaf(7);return [leaf.count,leaf.extra,leaf.result];}`,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "inheritedfields")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, fragment := range []string{"this.Extra = this.Count + 1", "this.Result = this.value + this.Extra"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("missing %q in generated defaults:\n%s", fragment, generated)
		}
	}
	reference := `package reference
type Base[T any] struct{ value T;Count int }
func initBase[T any](base *Base[T],value T){base.Count=4;base.value=value}
type Middle[U any] struct{ Base[U];Extra int }
func initMiddle[U any](middle *Middle[U],value U){initBase(&middle.Base,value);middle.Extra=middle.Count+1}
type Leaf struct{ Middle[int];Result int }
func newLeaf(value int)*Leaf{leaf:=&Leaf{};initMiddle(&leaf.Middle,value);leaf.Result=leaf.value+leaf.Extra;return leaf}
func Run()[]int{leaf:=newLeaf(7);return []int{leaf.Count,leaf.Extra,leaf.Result}}
`
	comparison := `package inheritedfields_test
import("reflect";"testing";g "inherited-fields.test";r "inherited-fields.test/reference")
func TestInheritedDefaults(t *testing.T){got,want:=g.Run(),r.Run();if !reflect.DeepEqual(got,want){t.Fatalf("got %v want %v",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "inherited-fields.test", generated, reference, comparison)
}
