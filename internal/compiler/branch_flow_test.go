package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBranchFlowMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `function value(n:int):int{return n;}
export function Run(stop:int):int{
 let sum=0;
 outer:for(let row=0;row<4;row++){
  const task=go value(row);const n=await task;
  switch(row){default{
   for(let col=0;col<4;col++){
    if(col==2){continue outer;}
    sum+=n+col;
    if(row==stop){break outer;}
   }
  }}
 }
 return sum;
}
export function Fill(stop:boolean):int{
 let p:*int|null=null;let n=9;
 outer:while(p===null){
  select{default{while(stop){p=&n;break outer;}p=&n;}}
 }
 return *p;
}
export function Inner():int{
 const task=go value(7);
 switch(1){default{while(true){break;}}}
 return await task;
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "branches")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Run(stop int)int{sum:=0;outer:for row:=0;row<4;row++{ch:=make(chan int,1);go func(n int){ch<-n}(row);n:=<-ch;switch row{default:for col:=0;col<4;col++{if col==2{continue outer};sum+=n+col;if row==stop{break outer}}}};return sum}
func Fill(stop bool)int{var p *int;n:=9;outer:for p==nil{select{default:for stop{p=&n;break outer};p=&n}};return *p}
func Inner()int{ch:=make(chan int,1);go func(){ch<-7}();switch 1{default:for{break}};return <-ch}
`
	tests := `package branches_test
import("testing";g "branch-flow.test";r "branch-flow.test/reference")
func TestBranches(t *testing.T){for _,stop:=range []int{-1,0,1,3,5}{if got,want:=g.Run(stop),r.Run(stop);got!=want{t.Fatal(stop,got,want)}};for _,stop:=range []bool{false,true}{if got,want:=g.Fill(stop),r.Fill(stop);got!=want{t.Fatal(stop,got,want)}};if got,want:=g.Inner(),r.Inner();got!=want{t.Fatal(got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "branch-flow.test", generated, reference, tests)
}
