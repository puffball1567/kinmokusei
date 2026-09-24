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
