package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectionMultipleResultsMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `let calls=0;
function appendInputs():(int[],int,int){calls++;return [1],2,3;}
function copyInputs(dst:int[]):(int[],int[]){calls++;return dst,[4,5];}
function textInputs(dst:byte[]):(byte[],string){calls++;return dst,"ok";}
function deleteInputs(m:Map<string,int>):(Map<string,int>,string){calls++;return m,"key";}
function Run():int{
 calls=0;const a=append(appendInputs());const b:int[]=[0,0];
 const n=copy(copyInputs(b));const text:byte[]=[0,0];const size=copy(textInputs(text));
 const m=makeMap<string,int>();m["key"]=1;delete(deleteInputs(m));
 return a[2]+b[1]+n+size+len(m)+int(text[0])+calls*1000;
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "collections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var calls int
func appendInputs()([]int,int,int){calls++;return []int{1},2,3}
func copyInputs(dst []int)([]int,[]int){calls++;return dst,[]int{4,5}}
func textInputs(dst []byte)([]byte,string){calls++;return dst,"ok"}
func deleteInputs(m map[string]int)(map[string]int,string){calls++;return m,"key"}
func Run()int{calls=0;a:=append(appendInputs());b:=[]int{0,0};n:=copy(copyInputs(b));text:=[]byte{0,0};size:=copy(textInputs(text));m:=map[string]int{"key":1};delete(deleteInputs(m));return a[2]+b[1]+n+size+len(m)+int(text[0])+calls*1000}
`
	comparison := `package collections_test
import("testing";g "collection-results.test";r "collection-results.test/reference")
func TestRun(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %d want %d",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "collection-results.test", generated, reference, comparison)
}
