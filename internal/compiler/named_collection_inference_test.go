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
class Item{constructor(public value:int){}}
constraint Items=~Item[];
constraint ItemMap=~Map<string,Item>;
constraint ItemPair=~[2]Item;
constraint ItemChannel=~GoChannel<Item>|~GoReceiveChannel<Item>;
function classSlice<S extends Items>(v:S):Item[]{return v;}
function classMap<M extends ItemMap>(v:M):Map<string,Item>{return v;}
function classArray<A extends ItemPair>(v:A):[2]Item{return v;}
function classChannel<C extends ItemChannel>(v:C):GoReceiveChannel<Item>{return v;}
function classFirst<S extends Items>(v:S):Item{return first(v);}
export function Classes():int{
  const item=new Item(21);const values:Item[]=[item];const m=makeMap<string,Item>();m["x"]=item;
  const pair:[2]Item=[item,item];const ch=goChannel<Item>(2);ch<-item;ch<-item;
  const fromChannel=<-classChannel(ch);classSlice(values)[0].value=22;
  const [checked,ok]=<-classChannel(ch);if(!ok){return -1;}
  return classFirst(values).value+classMap(m)["x"].value+classArray(pair)[1].value+fromChannel.value+checked.value;
}
constraint Slice<E>=~E[];
constraint Mapping<K extends comparable,V>=~Map<K,V>;
constraint ArrayPair<E>=~[2]E;
constraint Receiving<E>=~GoChannel<E>|~GoReceiveChannel<E>;
function forwardSlice<E,S extends Slice<E>>(v:S):E{return first(v);}
function forwardMap<K extends comparable,V,M extends Mapping<K,V>>(v:M,key:K):V{return read(v,key);}
function forwardArray<E,A extends ArrayPair<E>>(v:A):E{return array(v);}
function readOnly<T>(v:GoReceiveChannel<T>):T{return <-v;}
function forwardChannel<E,C extends Receiving<E>>(v:C):E{return readOnly(v);}
function forwardCallback<E,S extends Slice<E>>(v:S):E{return transform(v,(x)=>x);}
export function Forward():int{
  const values=Values<int>([13,14]);const m=makeMap<string,int>();m["x"]=15;
  const pair:[2]int=[16,17];const ch=goChannel<int>(1);ch<-18;
  return forwardSlice(values)+forwardMap(Lookup(m),"x")+forwardArray(Pair(pair))+forwardChannel(Channel(ch))+forwardCallback(values);
}
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
type item struct{value int}
func classSlice[S ~[]*item](v S)[]*item{return v}
func classMap[M ~map[string]*item](v M)map[string]*item{return v}
func classArray[A ~[2]*item](v A)[2]*item{return v}
func classChannel[C interface{~chan *item|~<-chan *item}](v C)<-chan *item{return v}
func classFirst[S ~[]*item](v S)*item{return first(v)}
func Classes()int{x:=&item{21};values:=[]*item{x};m:=map[string]*item{"x":x};pair:=[2]*item{x,x};ch:=make(chan *item,2);ch<-x;ch<-x;fromChannel:=<-classChannel(ch);classSlice(values)[0].value=22;checked,ok:=<-classChannel(ch);if !ok{return -1};return classFirst(values).value+classMap(m)["x"].value+classArray(pair)[1].value+fromChannel.value+checked.value}
func first[T any](v []T)T{return v[0]}
func named[T any](v Values[T])T{return v[0]}
func read[K comparable,V any](v map[K]V,key K)V{return v[key]}
func array[T any](v [2]T)T{return v[1]}
func deref[T any](v *T)T{return *v}
func receive[T any](v chan T)T{return <-v}
func transform[T any](v []T,f func(T)T)T{return f(v[0])}
func forwardSlice[E any,S ~[]E](v S)E{return first(v)}
func forwardMap[K comparable,V any,M ~map[K]V](v M,key K)V{return read(v,key)}
func forwardArray[E any,A ~[2]E](v A)E{return array(v)}
func readOnly[T any](v <-chan T)T{return <-v}
func forwardChannel[E any,C interface{~chan E|~<-chan E}](v C)E{return readOnly(v)}
func forwardCallback[E any,S ~[]E](v S)E{return transform(v,func(x E)E{return x})}
func Forward()int{values:=Values[int]{13,14};m:=Lookup{"x":15};pair:=Pair{16,17};ch:=make(Channel,1);ch<-18;return forwardSlice(values)+forwardMap(m,"x")+forwardArray(pair)+forwardChannel(ch)+forwardCallback(values)}
func Run()int{values:=Values[int]{3,4};m:=Lookup{"x":5};pair:=Pair{6,7};n:=8;ch:=make(Channel,1);ch<-9;return first(values)+named([]int{4,5})+read(m,"x")+array(pair)+deref(Ref(&n))+receive(ch)+transform(values,func(v int)int{return v*2})+first(values)+first(numbers())}
`
	comparison := `package collections_test
import("testing";g "named-collection-inference.test";r "named-collection-inference.test/reference")
func TestInference(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %d want %d",got,want)}}
func TestForward(t *testing.T){if got,want:=g.Forward(),r.Forward();got!=want{t.Fatalf("forward got %d want %d",got,want)}}
func TestClasses(t *testing.T){if got,want:=g.Classes(),r.Classes();got!=want{t.Fatalf("classes got %d want %d",got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "named-collection-inference.test", generated, reference, comparison)
}
