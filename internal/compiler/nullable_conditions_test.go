package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNullableConditionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `export function Run(present:boolean,flag:boolean):int[]{
 let n=7;let p:*int|null=null;if(present){p=&n;}
 let calls=0;const hit=():boolean=>{calls++;return flag;};
 let score=0;
 if(p!==null && hit() && *p>0){score+=*p;}
 if(p===null || hit()){score+=10;}else{score+=*p;}
 if(!(p===null || !flag)){score+=*p;}
 const exists=p!==null && *p>0;
 const absent=p===null || *p<0;
 if(exists){score+=20;}if(absent){score+=30;}
 while(p!==null && *p>0){score+=*p;p=null;}
 p=&n;
 for(;!(p===null) && *p>0;){score+=*p;p=null;}
 return [score,calls];
}
export function Tasks(flag:boolean):boolean{
 const compute=():boolean=>true;
 const task=go compute();return await task && flag;
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "conditions")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Run(present,flag bool)[]int{
 n:=7;var p *int;if present{p=&n};calls:=0;hit:=func()bool{calls++;return flag};score:=0
 if p!=nil && hit() && *p>0{score+=*p}
 if p==nil || hit(){score+=10}else{score+=*p}
 if !(p==nil || !flag){score+=*p}
 exists:=p!=nil && *p>0;absent:=p==nil || *p<0
 if exists{score+=20};if absent{score+=30}
 for p!=nil && *p>0{score+=*p;p=nil}
 p=&n;for !(p==nil) && *p>0{score+=*p;p=nil}
 return []int{score,calls}
}`
	tests := `package conditions_test
import("reflect";"testing";g "nullable-conditions.test";r "nullable-conditions.test/reference")
func TestConditions(t *testing.T){for _,present:=range []bool{false,true}{for _,flag:=range []bool{false,true}{
 if got,want:=g.Run(present,flag),r.Run(present,flag);!reflect.DeepEqual(got,want){t.Fatal(present,flag,got,want)}
 if got:=g.Tasks(flag);got!=flag{t.Fatal(got,flag)}
}}}`
	runGeneratedGoDifferentialTest(t, root, "nullable-conditions.test", generated, reference, tests)
}
