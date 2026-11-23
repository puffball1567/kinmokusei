package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeStructEmbeddingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"base.km": `export struct Cell<T> {
 public value:T;
 public function read():T {return this.value;}
 public pointer function put(value:T):void {this.value=value;}
 public pointer function present():boolean {return this!=nil;}
 public function choose<V>(other:V):V {return other;}
}
export alias View<T>=Cell<T>;
`,
		"entry.km": `import {Cell,View} from "./base";
export struct Layer {View<int>;}
export struct Envelope {Layer; public code:int;}
export struct Linked { *Cell<int>; }
export struct Shadow {Layer; public value:int;}

function replace<T extends interface{put(v:int):void;}>(dst:T,v:int):void {dst.put(v);}
export function Values(n:int):int[]{
 let cell=Cell<int>{value:n};
 let outer=Envelope{Layer:Layer{View:cell},code:5};
 outer.value+=1;
 const snapshot=outer.read;
 const reader=Envelope.read;
 const writer=(*Envelope).put;
 writer(&outer,n+4);
 const link=Linked{Cell:&cell};
 replace(link,n+7);
 const shadow=Shadow{Layer:outer.Layer,value:71};
 return [snapshot(),reader(outer),link.read(),cell.value,link.choose(11),shadow.value,shadow.Layer.value];
}
export function HasCell():boolean {
 const link=Linked{Cell:nil};
 return link.present();
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "structembedding")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type Cell[T any] struct{Value T ` + "`json:\"value\"`" + `}
func(c Cell[T])Read()T{return c.Value}
func(c *Cell[T])Put(v T){c.Value=v}
func(c *Cell[T])Present()bool{return c!=nil}
func choose[V any](v V)V{return v}
type Layer struct{Cell[int]}
type Envelope struct{Layer;Code int ` + "`json:\"code\"`" + `}
type Linked struct{*Cell[int]}
type Shadow struct{Layer;Value int}
func replace[T interface{Put(int)}](dst T,v int){dst.Put(v)}
func Values(n int)[]int{
 cell:=Cell[int]{Value:n};outer:=Envelope{Layer:Layer{Cell:cell},Code:5}
 outer.Value++
 snapshot,reader,writer:=outer.Read,Envelope.Read,(*Envelope).Put
 writer(&outer,n+4)
 link:=Linked{Cell:&cell};replace(link,n+7)
 shadow:=Shadow{Layer:outer.Layer,Value:71}
 return []int{snapshot(),reader(outer),link.Read(),cell.Value,choose(11),shadow.Value,shadow.Layer.Value}
}
func HasCell()bool{return (Linked{}).Present()}
`
	comparison := `package structembedding_test
import("encoding/json";"reflect";"testing";g "struct-embedding.test";r "struct-embedding.test/reference")
func TestEmbedding(t *testing.T){
 for _,n:=range []int{-17,0,8}{if got,want:=g.Values(n),r.Values(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}}
 if g.HasCell()!=r.HasCell(){t.Fatal("nil embedded pointer")}
 var native interface{Read()int;Put(int)}=&g.Envelope{}
 native.Put(23)
 if native.Read()!=23{t.Fatal("Go method set")}
 a,e:=json.Marshal(g.Envelope{Code:9});b,f:=json.Marshal(r.Envelope{Code:9})
 if e!=nil||f!=nil||string(a)!=string(b){t.Fatal(string(a),e,string(b),f)}
}`
	runGeneratedGoDifferentialTest(t, root, "struct-embedding.test", generated, reference, comparison)
}
