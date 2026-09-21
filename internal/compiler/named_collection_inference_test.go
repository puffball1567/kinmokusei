package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamedCollectionInferenceMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"go.mod": "module named-collection-inference.test\n\ngo 1.23\n",
		"api/api.go": `package api
type Values []int
func Numbers()Values{return Values{11,12}}`,
		"entry.km": `import go api from "named-collection-inference.test/api";
type Values<T>=distinct T[];
type Lookup=distinct Map<string,int>;
type Pair=distinct [2]int;
type Ref=distinct *int;
type Channel=distinct GoChannel<int>;
function first<T>(v:T[]):T{return v[0];}
function named<T>(v:Values<T>):T{return v[0];}
function read<K extends comparable,V>(v:Map<K,V>,key:K):V{return v[key];}
function array<T>(v:[2]T):T{return v[1];}
function deref<T>(v:*T):T{return *v;}
function receive<T>(v:GoChannel<T>):T{return <-v;}
function transform<T>(v:T[],f:(v:T)=>T):T{return f(v[0]);}
class Reader{public function first<T>(v:T[]):T{return v[0];}}
export function Run():int{
  const values=Values<int>([3,4]);const m=makeMap<string,int>();m["x"]=5;
  const pair:[2]int=[6,7];let n=8;const ch=goChannel<int>(1);ch<-9;
  const reader=new Reader();
  return first(values)+named([4,5])+read(Lookup(m),"x")+array(Pair(pair))+deref(Ref(&n))+receive(Channel(ch))+transform(values,(v)=>v*2)+reader.first(values)+first(api.Numbers());
}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "collections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type ApiValues []int
func numbers()ApiValues{return ApiValues{11,12}}
type Values[T any][]T
type Lookup map[string]int
type Pair [2]int
type Ref *int
type Channel chan int
func first[T any](v []T)T{return v[0]}
func named[T any](v Values[T])T{return v[0]}
func read[K comparable,V any](v map[K]V,key K)V{return v[key]}
func array[T any](v [2]T)T{return v[1]}
func deref[T any](v *T)T{return *v}
func receive[T any](v chan T)T{return <-v}
func transform[T any](v []T,f func(T)T)T{return f(v[0])}
func Run()int{values:=Values[int]{3,4};m:=Lookup{"x":5};pair:=Pair{6,7};n:=8;ch:=make(Channel,1);ch<-9;return first(values)+named([]int{4,5})+read(m,"x")+array(pair)+deref(Ref(&n))+receive(ch)+transform(values,func(v int)int{return v*2})+first(values)+first(numbers())}
`
	comparison := `package collections_test
import("testing";g "named-collection-inference.test";r "named-collection-inference.test/reference")
func TestInference(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %d want %d",got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "named-collection-inference.test", generated, reference, comparison)
}
