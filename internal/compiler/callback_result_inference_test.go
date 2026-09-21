package compiler

import (
	"os"
	"path/filepath"
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
function Run():int{calls=0;const p=new Picker();const a=first(pair);const b=first(()=>pair());const c=first<int>(pair);const d=p.first(pair);const e=map(()=>{return 4,1;},(n)=>n*2);return a+b+c+d+e+unbox(boxed)+calls*100;}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "callbacks")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var calls int
func pair()(int,string){calls++;return 7,"ok"}
func first[T,U any](f func()(T,U))T{v,_:=f();return v}
func mapValue[T,U any](f func()(T,int),g func(T)U)U{v,_:=f();return g(v)}
type box[T any]struct{value T}
func boxed()(*box[int],int){return &box[int]{5},1}
func unbox[T any](f func()(*box[T],int))T{b,_:=f();return b.value}
func Run()int{calls=0;a:=first(pair);b:=first(func()(int,string){return pair()});c:=first[int](pair);d:=first(pair);e:=mapValue(func()(int,int){return 4,1},func(n int)int{return n*2});return a+b+c+d+e+unbox(boxed)+calls*100}
`
	comparison := `package callbacks_test
import("testing";g "callback-results.test";r "callback-results.test/reference")
func TestCalls(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("callback inference/evaluation: %d != %d",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "callback-results.test", generated, reference, comparison)
}
