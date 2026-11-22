package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDistinctTypeEmbeddingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"value.km": `export type Count=distinct int;
public function read(this:Count):int{return int(this);}
public function add(this:*Count,delta:int):void{*this=Count(int(*this)+delta);}
public function present(this:*Count):boolean{return this!=nil;}
export type Values<T>=distinct T[];
public function first<T>(this:Values<T>):T{return this[0];}
export struct Leaf{public amount:int;public function twice():int{return this.amount*2;}}
export struct Original<T>{Leaf;public metadata:T;public function onlyOriginal():int{return 11;}}
export type Defined<T>=distinct Original<T>;
public function meta<T>(this:Defined<T>):T{return this.metadata;}
export alias CountAlias=Count;
`,
		"entry.km": `import {Count,CountAlias,Values,Leaf,Defined} from "./value";
struct Value{CountAlias;}
struct Pointer{*Count;}
struct Collection{Values<int>;}
struct Envelope{Defined<int>;}
function readLike<T extends interface{read():int;}>(v:T):int{return v.read();}
export function Run(n:int):int[]{
 let v=Value{CountAlias:Count(n)};
 const snapshot=v.read;
 const read=Value.read;
 const add=(*Value).add;
 add(&v,2);
 const p=Pointer{Count:&v.CountAlias};
 const bump=Pointer.add;
 bump(p,3);
 const record=Envelope{Defined:Defined<int>{Leaf:Leaf{amount:n+1},metadata:n+3}};
 const meta=Envelope.meta;
 const xs=Collection{Values:Values<int>([n+4])};
 const first=xs.first;
 return [snapshot(),read(v),p.read(),readLike(v),record.amount,record.twice(),record.Defined.twice(),meta(record),first()];
}
export function HasValue():boolean{return Pointer{Count:nil}.present();}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "distinctembedding")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type count int
func(v count)Read()int{return int(v)}
func(v *count)Add(delta int){*v+=count(delta)}
func(v *count)Present()bool{return v!=nil}
type countAlias=count
type value struct{countAlias}
type pointer struct{*count}
type values[T any][]T
func(v values[T])First()T{return v[0]}
type collection struct{values[int]}
type leaf struct{amount int}
func(v leaf)Twice()int{return v.amount*2}
type original[T any]struct{leaf;metadata T}
func(original[T])OnlyOriginal()int{return 11}
type defined[T any]original[T]
func(v defined[T])Meta()T{return v.metadata}
type envelope struct{defined[int]}
func readLike[T interface{Read()int}](v T)int{return v.Read()}
func Run(n int)[]int{
 v:=value{count(n)};snapshot,read,add:=v.Read,value.Read,(*value).Add
 add(&v,2);p:=pointer{&v.countAlias};bump:=pointer.Add;bump(p,3)
 r:=envelope{defined[int]{leaf{n+1},n+3}};meta:=envelope.Meta
 xs:=collection{values[int]{n+4}};first:=xs.First
 return []int{snapshot(),read(v),p.Read(),readLike(v),r.amount,r.Twice(),r.defined.Twice(),meta(r),first()}
}
func HasValue()bool{return (pointer{}).Present()}
`
	comparison := `package distinctembedding_test
import("reflect";"testing";g "distinct-type-embedding.test";r "distinct-type-embedding.test/reference")
func TestEmbedding(t *testing.T){
 for _,n:=range []int{-8,0,9}{
  if got,want:=g.Run(n),r.Run(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}
 }
 if g.HasValue()!=r.HasValue(){t.Fatal("nil pointer dispatch")}
}
`
	runGeneratedGoDifferentialTest(t, root, "distinct-type-embedding.test", generated, reference, comparison)
}
