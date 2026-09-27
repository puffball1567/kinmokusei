package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskGotoMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `function value(n:int):int{return n;}
export function Forward(flag:boolean):int{
 const task=go value(7);let extra=0;
 if(flag){goto done;}extra++;
 done:return extra+await task;
}
export function Fresh(n:int):int{
 let sum=0;
 again:if(n<=0){return sum;}
 const task=go value(n);sum+=await task;n--;
 if(n>0){goto again;}return sum;
}
export function Pending(limit:int):int{
 const task=go value(7);let n=0;
 again:n++;if(n<limit){goto again;}
 return n+await task;
}
export function Joined(flag:boolean):int{
 const task=go value(7);let sum=0;
 if(flag){sum=await task;goto done;}
 sum=await task;
 done:return sum;
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "jumps")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func start(n int)<-chan int{ch:=make(chan int,1);go func(){ch<-n}();return ch}
func Forward(flag bool)int{task:=start(7);extra:=0;if flag{goto done};extra++;done:return extra+<-task}
func Fresh(n int)int{sum:=0;again:if n<=0{return sum};task:=start(n);sum+=<-task;n--;if n>0{goto again};return sum}
func Pending(limit int)int{task:=start(7);n:=0;again:n++;if n<limit{goto again};return n+<-task}
func Joined(flag bool)int{task:=start(7);sum:=0;if flag{sum=<-task;goto done};sum=<-task;done:return sum}
`
	tests := `package jumps_test
import("testing";g "task-goto.test";r "task-goto.test/reference")
func TestJumps(t *testing.T){for _,flag:=range []bool{false,true}{if got,want:=g.Forward(flag),r.Forward(flag);got!=want{t.Fatal(got,want)};if got,want:=g.Joined(flag),r.Joined(flag);got!=want{t.Fatal(got,want)}};for _,n:=range []int{-1,0,1,5}{if got,want:=g.Fresh(n),r.Fresh(n);got!=want{t.Fatal(got,want)};if got,want:=g.Pending(n),r.Pending(n);got!=want{t.Fatal(got,want)}}}`
	runGeneratedGoDifferentialTest(t, root, "task-goto.test", generated, reference, tests)
}
