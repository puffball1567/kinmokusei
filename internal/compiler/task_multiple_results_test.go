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
function narrow(n:byte):byte{return n;}
function nullable(p:*int):int{if(p==nil){return 1;}return 0;}
function Normal():int{
 const taskFunction=5;const taskArgument0=6;const __taskCapture0=7;
 const a=go first(3,7);const b=go first<int>(4,8);
 const c=go cmp.Compare<int>(1,2);const d=go receiver().first(9,10);
 const e=go variadic<int>([11,12]...);const f=go narrow(255);
 const g=go nullable(nil);const h=go sum(taskArgument0,__taskCapture0);
 const i=go sum(taskFunction,2);
 return (await a)+(await b)+(await c)+(await d)+(await e)+int(await f)+(await g)+(await h)+(await i);
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
func Normal()int{return 3+4-1+9+11+255+1+13+7}
`
	comparison := `package tasks_test
import("testing";g "task-results.test";r "task-results.test/reference")
func TestTasks(t *testing.T){for i:=0;i<20;i++{if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %d want %d",got,want)}};if got,want:=g.CapturedFunction(),r.CapturedFunction();got!=want{t.Fatalf("callee captured after arguments: %d != %d",got,want)};if got,want:=g.Normal(),r.Normal();got!=want{t.Fatalf("ordinary arguments: %d != %d",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "task-results.test", generated, reference, comparison)
}
