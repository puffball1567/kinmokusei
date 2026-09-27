package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskCallableBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"worker.km": `import go strconv from "strconv";
export function value(n:int):int{return n;}
export function load(text:string):Result<int>{const n=strconv.Atoi(text)?;return ok(n);}`,
		"entry.km": `import {value,load} from "./worker";
export function Run(text:string):Result<int>{
 const outer=go value(7);
 const callback=():Result<int>=>{
  const inner=go value(2);
  const nested=()=>{return 3;};
  const base=await inner;
  const n=load(text)?;
  return ok(base+n+nested());
 };
 const [n,err]=callback();
 const base=await outer;
 if(err!==nil){return fail(err);}
 return ok(base+n);
}
export function Awaited(text:string):Result<int>{
 const task=go load(text);const n=await task?;return ok(n);
}
export function Parallel(text:string):Result<int>{
 const first=go load(text);const second=go load("2");
 const [a,e]=await first;const [b,f]=await second;
 if(e!==nil){return fail(e);}if(f!==nil){return fail(f);}return ok(a+b);
}
export function Mutual():int{
 const outer=go value(7);
 const a=(n:int):int=>{if(n==0){return 1;}return b(n-1);};
 const b=(n:int):int=>{return a(n);};
 return await outer+a(3);
}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "boundaries")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "strconv"
func start(n int)<-chan int{ch:=make(chan int,1);go func(){ch<-n}();return ch}
func Run(text string)(int,error){
 outer:=start(7)
 callback:=func()(int,error){inner:=start(2);nested:=func()int{return 3};base:=<-inner;n,err:=strconv.Atoi(text);if err!=nil{return 0,err};return base+n+nested(),nil}
 n,err:=callback();base:=<-outer;if err!=nil{return 0,err};return base+n,nil
}
func Awaited(text string)(int,error){var n int;var err error;done:=make(chan struct{});go func(){n,err=strconv.Atoi(text);close(done)}();<-done;return n,err}
func Parallel(text string)(int,error){type result struct{n int;err error};start:=func(s string)<-chan result{ch:=make(chan result,1);go func(){n,err:=strconv.Atoi(s);ch<-result{n,err}}();return ch};first,second:=start(text),start("2");a,b:=<-first,<-second;if a.err!=nil{return 0,a.err};if b.err!=nil{return 0,b.err};return a.n+b.n,nil}
func Mutual()int{outer:=start(7);var a,b func(int)int;a=func(n int)int{if n==0{return 1};return b(n-1)};b=func(n int)int{return a(n)};return <-outer+a(3)}
`
	tests := `package boundaries_test
import("fmt";"testing";g "task-boundaries.test";r "task-boundaries.test/reference")
func TestBoundaries(t *testing.T){for _,text:=range []string{"3","-5","invalid"}{
 for _,pair:=range [][2]func(string)(int,error){{g.Run,r.Run},{g.Awaited,r.Awaited},{g.Parallel,r.Parallel}}{
  got,err:=pair[0](text);want,refErr:=pair[1](text);if got!=want||fmt.Sprint(err)!=fmt.Sprint(refErr){t.Fatal(text,got,err,want,refErr)}
 }};if got,want:=g.Mutual(),r.Mutual();got!=want{t.Fatal(got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "task-boundaries.test", generated, reference, tests)
}
