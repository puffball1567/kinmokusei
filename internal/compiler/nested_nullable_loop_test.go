package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestedNullableLoopsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `class Item { public value:int=3; }
export function Run(n:int):int {
 let p:Item|null=new Item();let total=0;
 for(let i=0;i<n;i++) {
   p=new Item();
   for(let j=0;j<2;j++) {
     if(p!==null){const current=p;total+=current.value;}
     p=null;
   }
 }
 if(p!==null){return total+p.value;}
 return total;
}
export function Checked(n:int):int {
 let value=7;let p:*int|null=&value;let total=0;
 for(let i=0;i<n;i++) {
   p=&value;
   for(let j=0;j<2;j++) {
     if(p!==null){total+=*p;}
     p=null;
   }
 }
 if(p!==null){return total+*p;}
 return total;
}`
	path := filepath.Join(root, "loops.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "loops")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Item struct{value int}
func Run(n int)int{
 p:=&Item{3};total:=0
 for i:=0;i<n;i++{p=&Item{3};for j:=0;j<2;j++{if p!=nil{current:=p;total+=current.value};p=nil}}
 if p!=nil{return total+p.value};return total
}
func Checked(n int)int{
 value:=7;p:=&value;total:=0
 for i:=0;i<n;i++{p=&value;for j:=0;j<2;j++{if p!=nil{total+=*p};p=nil}}
 if p!=nil{return total+*p};return total
}`
	testSource := `package loops_test
import("testing";g "nested-loops.test";r "nested-loops.test/reference")
func TestLoops(t *testing.T){for _,n:=range []int{-1,0,1,2,8}{
 if got,want:=g.Run(n),r.Run(n);got!=want{t.Fatalf("class n=%d: %d != %d",n,got,want)}
 if got,want:=g.Checked(n),r.Checked(n);got!=want{t.Fatalf("pointer n=%d: %d != %d",n,got,want)}
}}`
	runGeneratedGoDifferentialTest(t, root, "nested-loops.test", generated, reference, testSource)
}
