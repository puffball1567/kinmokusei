package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForPostFlowMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `class Item{constructor(public value:int){}}
export function Run(limit:int,stop:int,skip:boolean):int[]{
 let n=0;let sum=0;let posts=0;
 const next=():Item=>{posts++;return new Item(n+1);};
 outer:for(let p:Item|null=new Item(1);n<limit;p=next()){
  sum+=p.value;n++;
  if(n==stop){break;}
  while(skip){p=null;continue outer;}
  p=null;
 }
 return [sum,posts];
}
export function Tasks(limit:int):int{
 let total=0;
 const bump=():void=>{total++;};
 for(let n=0;n<limit;await go bump()){n++;continue;}
 return total;
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "postflow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type item struct{value int}
func Run(limit,stop int,skip bool)[]int{n,sum,posts:=0,0,0;next:=func()*item{posts++;return &item{n+1}};outer:for p:=(&item{1});n<limit;p=next(){sum+=p.value;n++;if n==stop{break};for skip{p=nil;continue outer};p=nil};return []int{sum,posts}}
func Tasks(limit int)int{total:=0;post:=func(){done:=make(chan struct{});go func(){total++;close(done)}();<-done};for n:=0;n<limit;post(){n++;continue};return total}
`
	tests := `package postflow_test
import("reflect";"testing";g "for-post-flow.test";r "for-post-flow.test/reference")
func TestPost(t *testing.T){for _,limit:=range []int{0,1,4}{for _,stop:=range []int{-1,1,3}{for _,skip:=range []bool{false,true}{if got,want:=g.Run(limit,stop,skip),r.Run(limit,stop,skip);!reflect.DeepEqual(got,want){t.Fatal(limit,stop,skip,got,want)}}};if got,want:=g.Tasks(limit),r.Tasks(limit);got!=want{t.Fatal(got,want)}}}`
	runGeneratedGoDifferentialTest(t, root, "for-post-flow.test", generated, reference, tests)
}
