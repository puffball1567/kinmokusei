package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCallbackResultInferenceMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `let calls=0;
function pair():(int,string){calls++;return 7,"ok";}
function first<T,U>(f:()=>(T,U)):T{const [value,_]=f();return value;}
class Picker{public function first<T,U>(f:()=>(T,U)):T{const [value,_]=f();return value;}}
function map<T,U>(f:()=>(T,int),g:(v:T)=>U):U{const [value,_]=f();return g(value);}
class Box<T>{constructor(public value:T){}}
function boxed():(Box<int>,int){return new Box<int>(5),1;}
function unbox<T>(f:()=>(Box<T>,int)):T{const [box,_]=f();return box.value;}
interface Marker<T>{function size():int;}
class Tagged<T> implements Marker<T>{public function size():int{return 9;}}
function marked():(Marker<int>,int){return new Tagged<int>(),2;}
function measure<T>(f:()=>(Marker<T>,int)):int{const [m,n]=f();return m.size()+n;}
class Meter{public function measure<T>(f:()=>(Marker<T>,int)):int{const [m,n]=f();return m.size()+n;}}
type Pair<T,U>=distinct ()=>(T,U);
function namedFirst<T,U>(f:Pair<T,U>):T{const [v,_]=f();return v;}
function transform<T,U>(f:()=>(T,U),g:(v:T)=>T):T{const [v,_]=f();return g(v);}
function Named():int{calls=0;const f:Pair<int,string>=pair;const p=new Picker();const a=first(f);const b=namedFirst(pair);const c=namedFirst(()=>pair());const d=transform(f,(v)=>v+1);const e=p.first(f);return a+b+c+d+e+calls*100;}
function Run():int{calls=0;const p=new Picker();const a=first(pair);const b=first(()=>pair());const c=first<int>(pair);const d=p.first(pair);const e=map(()=>{return 4,1;},(n)=>n*2);const meter=new Meter();return a+b+c+d+e+unbox(boxed)+calls*100+measure(marked)+measure(()=>marked())+meter.measure(marked);}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "callbacks")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{"measure[int](marked)", "measure[int](func()", "MeterMeasure[int](meter, marked)"} {
		if !strings.Contains(string(generated), expected) {
			t.Fatalf("callback interface result must retain %q:\n%s", expected, generated)
		}
	}
	reference := `package reference
var calls int
func pair()(int,string){calls++;return 7,"ok"}
func first[T,U any](f func()(T,U))T{v,_:=f();return v}
func mapValue[T,U any](f func()(T,int),g func(T)U)U{v,_:=f();return g(v)}
type box[T any]struct{value T}
func boxed()(*box[int],int){return &box[int]{5},1}
func unbox[T any](f func()(*box[T],int))T{b,_:=f();return b.value}
type marker[T any]interface{size()int}
type tagged[T any]struct{}
func(tagged[T])size()int{return 9}
func marked()(marker[int],int){return tagged[int]{},2}
func measure[T any](f func()(marker[T],int))int{m,n:=f();return m.size()+n}
type Pair[T,U any]func()(T,U)
func namedFirst[T,U any](f Pair[T,U])T{v,_:=f();return v}
func Named()int{calls=0;var f Pair[int,string]=pair;a:=first(f);b:=namedFirst(pair);c:=namedFirst(func()(int,string){return pair()});d:=first(f)+1;e:=first(f);return a+b+c+d+e+calls*100}
func Run()int{calls=0;a:=first(pair);b:=first(func()(int,string){return pair()});c:=first[int](pair);d:=first(pair);e:=mapValue(func()(int,int){return 4,1},func(n int)int{return n*2});return a+b+c+d+e+unbox(boxed)+calls*100+measure[int](marked)+measure[int](func()(marker[int],int){return marked()})+measure[int](marked)}
`
	comparison := `package callbacks_test
import("testing";g "callback-results.test";r "callback-results.test/reference")
func TestCalls(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("callback inference/evaluation: %d != %d",got,want)}}
func TestNamed(t *testing.T){if got,want:=g.Named(),r.Named();got!=want{t.Fatalf("named callback inference/evaluation: %d != %d",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "callback-results.test", generated, reference, comparison)
}
