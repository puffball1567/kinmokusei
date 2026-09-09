package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoInterfaceInheritanceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module go-interface-inheritance.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Reader[T any] interface { Read() T }
type Batch[T any] interface { All(...T) []T }
`,
		"base.km": `import go fmt from "fmt";
import go io from "io";
import go api from "go-interface-inheritance.test/contracts";
interface Named extends fmt.Stringer {}
interface Left extends Named {}
interface Right extends Named {}
interface Combined extends Left,Right {function count():int;}
interface Reader<T> extends api.Reader<T>,api.Batch<T> {}
interface Bytes extends io.Reader {}
class Label implements Combined {constructor(protected value:int){}public virtual function string():string{return fmt.Sprint(this.value);}public function count():int{return this.value;}}
class Double extends Label {constructor(value:int){super(value);}public override function string():string{return fmt.Sprint(this.value*2);}}
class Cell<T> implements Reader<T>{constructor(private value:T){}public function read():T{return this.value;}public function all(...values:T[]):T[]{return values;}}
class ByteReader implements Bytes {constructor(private count:int){}public function read(buffer:byte[]):Result<int>{return ok(this.count+len(buffer));}}
class Leaf{constructor(public value:int){}}
alias Maybe=Leaf|null;
function nullable(present:boolean,value:int):int{let leaf:Maybe=null;if(present){leaf=new Leaf(value);}const reader:Reader<Maybe>=new Cell<Maybe>(leaf);const result=reader.Read();if(result===null){return -1;}return result.value;}
`,
		"entry.km": `import {Named,Combined,Reader,Bytes,Double,Cell,ByteReader,nullable} from "./base";
import go f from "fmt";
import go io from "io";
import go c from "go-interface-inheritance.test/contracts";
function Make(value:int):Combined{return new Double(value);}
function Upcast(value:Combined):f.Stringer{return value;}
function Direct(value:int):f.Stringer{return new Double(value);}
function Use(value:int):string{const named=Make(value);const bound=named.String;return bound()+":"+f.Sprint(named.count());}
function Nullable(value:Combined|null):f.Stringer|null{return value;}
function Generic(value:int):int{const reader:Reader<int>=new Cell<int>(value);const bound=reader.Read;return bound()+len(reader.All(1,2,3));}
function GenericUpcast(value:int):c.Reader<int>{return new Cell<int>(value);}
function ByteResult(value:int):int{const source:Bytes=new ByteReader(value);const [n,err]=source.Read([1,2]);return n;}
function ByteUpcast(value:int):io.Reader{return new ByteReader(value);}
function Objects(present:boolean,value:int):int{return nullable(present,value);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "gointerfaces")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("fmt";"io")
type Combined interface{fmt.Stringer;Count()int}
type Value struct{value int}
func(v *Value)String()string{return fmt.Sprint(v.value*2)}
func(v *Value)Count()int{return v.value}
func Make(value int)Combined{return &Value{value}}
func Upcast(value Combined)fmt.Stringer{return value}
func Direct(value int)fmt.Stringer{return &Value{value}}
func Use(value int)string{named:=Make(value);bound:=named.String;return bound()+":"+fmt.Sprint(named.Count())}
func Nullable(value Combined)fmt.Stringer{return value}
type Reader[T any] interface{Read()T;All(...T)[]T}
type Cell[T any]struct{value T}
func(c *Cell[T])Read()T{return c.value}
func(c *Cell[T])All(values ...T)[]T{return values}
func Generic(value int)int{var reader Reader[int]=&Cell[int]{value};bound:=reader.Read;return bound()+len(reader.All(1,2,3))}
func GenericUpcast(value int)interface{Read()int}{return &Cell[int]{value}}
type ByteReader struct{count int}
func(r *ByteReader)Read(buffer []byte)(int,error){return r.count+len(buffer),nil}
func ByteResult(value int)int{var source io.Reader=&ByteReader{value};n,_:=source.Read([]byte{1,2});return n}
func ByteUpcast(value int)io.Reader{return &ByteReader{value}}
type Leaf struct{value int}
func Objects(present bool,value int)int{var leaf *Leaf;if present{leaf=&Leaf{value}};var reader Reader[*Leaf]=&Cell[*Leaf]{leaf};result:=reader.Read();if result==nil{return -1};return result.value}
`
	comparison := `package gointerfaces_test
import("testing";"fmt";"io";generated "go-interface-inheritance.test";reference "go-interface-inheritance.test/reference")
func TestContracts(t *testing.T){
 for _,n:=range []int{-5,0,7,100}{
  if generated.Use(n)!=reference.Use(n){t.Error("dispatch and bound method")}
  g,r:=generated.Make(n),reference.Make(n)
  var _ fmt.Stringer=g
  if generated.Upcast(g).String()!=reference.Upcast(r).String(){t.Error("interface upcast")}
  if generated.Upcast(g)!=generated.Upcast(g){t.Error("identity")}
  if generated.Direct(n).String()!=reference.Direct(n).String(){t.Error("class upcast")}
  if generated.Generic(n)!=reference.Generic(n){t.Error("generic/variadic")}
  if generated.GenericUpcast(n).Read()!=reference.GenericUpcast(n).Read(){t.Error("generic upcast")}
  if generated.ByteResult(n)!=reference.ByteResult(n){t.Error("multiple results")}
  var gb io.Reader=generated.ByteUpcast(n);rb:=reference.ByteUpcast(n)
  gn,ge:=gb.Read(nil);rn,re:=rb.Read(nil);if gn!=rn||(ge==nil)!=(re==nil){t.Error("Result implementation")}
  for _,present:=range []bool{false,true}{if generated.Objects(present,n)!=reference.Objects(present,n){t.Error("nullable generic class identity")}}
 }
 if generated.Nullable(nil)!=nil||reference.Nullable(nil)!=nil{t.Error("nil upcast")}
}
`
	runGeneratedGoDifferentialTest(t, root, "go-interface-inheritance.test", generated, reference, comparison)
}
