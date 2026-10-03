package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExceptionFlowMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `class Item{constructor(public value:int){}}
export function Read(flag:boolean):int{
 let p:Item|null=new Item(1);let result=0;
 try{p=null;if(flag){throw new Exception("stop");}p=new Item(2);}
 catch(_:error){if(p!==null){result=p.value;}else{result=-1;}}
 finally{if(p!==null){result+=p.value;}else{result-=10;}}
 return result;
}
export function Early(flag:boolean):int{
 let p:Item|null=new Item(1);
 try{p=null;if(flag){return 1;}p=new Item(2);}
 finally{if(p===null){return -1;}return p.value;}
}
export function Restore(flag:boolean):int{
 let p:Item|null=new Item(1);
 try{p=null;if(flag){throw new Exception("stop");}p=new Item(2);}
 catch(_:error){}finally{p=new Item(3);}
 return p.value;
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "exceptionflow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "errors"
type item struct{value int}
func Read(flag bool)int{
 p:=&item{1};result:=0
 func(){
  defer func(){if p!=nil{result+=p.value}else{result-=10}}()
  func(){
   defer func(){if v:=recover();v!=nil{if _,ok:=v.(error);!ok{panic(v)};if p!=nil{result=p.value}else{result=-1}}}()
   p=nil;if flag{panic(errors.New("stop"))};p=&item{2}
  }()
 }()
 return result
}
func Early(flag bool)(result int){p:=&item{1};defer func(){if p==nil{result=-1}else{result=p.value}}();p=nil;if flag{return 1};p=&item{2};return}
func Restore(flag bool)int{p:=&item{1};func(){defer func(){p=&item{3}}();func(){defer func(){if v:=recover();v!=nil{if _,ok:=v.(error);!ok{panic(v)}}}();p=nil;if flag{panic(errors.New("stop"))};p=&item{2}}()}();return p.value}
`
	tests := `package exceptionflow_test
import("testing";g "exception-flow.test";r "exception-flow.test/reference")
func TestPaths(t *testing.T){for _,flag:=range []bool{false,true}{
 if got,want:=g.Read(flag),r.Read(flag);got!=want{t.Fatal(flag,got,want)}
 if got,want:=g.Early(flag),r.Early(flag);got!=want{t.Fatal(flag,got,want)}
 if got,want:=g.Restore(flag),r.Restore(flag);got!=want{t.Fatal(flag,got,want)}
}}`
	runGeneratedGoDifferentialTest(t, root, "exception-flow.test", generated, reference, tests)
}
