package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskMultipleArgumentsMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `import go cmp from "cmp";
let calls=0;
function pair():(int,int){calls++;return 3,7;}
function sum(a:int,b:int):int{return a+b;}
function first<T>(a:T,b:T):T{return a;}
function variadic<T>(...values:T[]):T{return values[0];}
function task<T>(a:T,b:T):T{return b;}
class C{public function first<T>(a:T,b:T):T{return a;}}
function receiver():C{calls++;return new C();}
function Run():int{
 calls=0;
 const a=go sum(pair());const b=go first(pair());
 const c=go cmp.Compare(pair());const d=go receiver().first(pair());
 const e=go variadic<int>(pair());const f=go task(pair());
 const captured=calls;
 return (await a)+(await b)+(await c)+(await d)+(await e)+(await f)+captured*100;
}
function CapturedFunction():int{
 let f:(a:int,b:int)=>int=sum;
 const inputs=():(int,int)=>{f=(a:int,b:int)=>99;return 3,7;};
 const task=go f(inputs());return await task;
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module task-results.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "tasks")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "cmp"
var calls int
func pair()(int,int){calls++;return 3,7}
func sum(a,b int)int{return a+b}
func first[T any](a,b T)T{return a}
func variadic[T any](values ...T)T{return values[0]}
type C struct{}
func receiver()*C{calls++;return &C{}}
func method[T any](c *C,a,b T)T{return a}
func start(f func()int)<-chan int{done:=make(chan int,1);go func(){done<-f()}();return done}
func Run()int{
 calls=0
 a,b:=pair();one:=start(func()int{return sum(a,b)})
 c,d:=pair();two:=start(func()int{return first(c,d)})
 e,f:=pair();three:=start(func()int{return cmp.Compare(e,f)})
 r:=receiver();g,h:=pair();four:=start(func()int{return method(r,g,h)})
 i,j:=pair();five:=start(func()int{return variadic[int](i,j)})
 _,k:=pair();six:=start(func()int{return k})
 captured:=calls
 return <-one + <-two + <-three + <-four + <-five + <-six + captured*100
}
func CapturedFunction()int{f:=sum;inputs:=func()(int,int){f=func(a,b int)int{return 99};return 3,7};saved:=f;a,b:=inputs();return <-start(func()int{return saved(a,b)})}
`
	comparison := `package tasks_test
import("testing";g "task-results.test";r "task-results.test/reference")
func TestTasks(t *testing.T){for i:=0;i<20;i++{if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %d want %d",got,want)}};if got,want:=g.CapturedFunction(),r.CapturedFunction();got!=want{t.Fatalf("callee captured after arguments: %d != %d",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "task-results.test", generated, reference, comparison)
}
