package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedGoStructEmbeddingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module imported-struct-embedding.test\n\ngo 1.23\n",
		"models/models.go": `package models
type Cell[T any] struct{ Item T }
func(c Cell[T])Read()T{return c.Item}
func(c *Cell[T])Put(v T){c.Item=v}
type IntCell = Cell[int]
type Getter[T any] interface{Read()T}
func Read[T any](c Getter[T])T{return c.Read()}
type hidden struct{ Number int }
type Nested struct{ hidden }
func New(n int)Nested{return Nested{hidden{n}}}
type Holder struct{Getter[int]}
`,
		"storage.km": `import go bytes from "bytes";
export struct Output{bytes.Buffer;}
export struct Linked{*bytes.Buffer;}
`,
		"entry.km": `import {Output,Linked} from "./storage";
import go bytes from "bytes";
import go {Buffer as Raw} from "bytes";
import go io from "io";
import go m from "imported-struct-embedding.test/models";
struct Layer{Output;}
struct Renamed{Raw;}
struct Box{m.Cell<int>;}
struct AliasBox{m.IntCell;}
constraint Integer=~int;
alias CellAlias<T extends Integer>=m.Cell<T>;
struct SourceAliasBox{CellAlias<int>;}
struct Envelope{m.Nested;}
struct Delegate{m.Holder;}
function appendText(w:io.Writer,s:bstring):int{
 const [n,e]=io.WriteString(w,s);
 if(e!=nil){return -1;}
 return n;
}
export function Buffers(data:bstring):int[]{
 let value=Layer{Output:Output{Buffer:bytes.Buffer{}}};
 let raw=bytes.Buffer{};
 const linked=Linked{Buffer:&raw};
 let named=Renamed{Raw:bytes.Buffer{}};
 const lenOf=(*Layer).Len;
 const bound=value.Len;
 const a=appendText(&value,data);
 const b=appendText(linked,data);
 const c=appendText(&named,data);
 return [a,b,c,lenOf(&value),bound(),value.Buffer.Len(),raw.Len(),linked.Len(),named.Raw.Len(),named.Len()];
}
export function Records(n:int):int[]{
 let box=Box{Cell:m.Cell<int>{Item:n}};
 const value=box;
 box.Item+=1;
 const put=(*Box).Put;
 put(&box,n+3);
 let e=Envelope{Nested:m.New(n)};
 e.Number+=2;
 const d=Delegate{Holder:m.Holder{Getter:box.Cell}};
 const a=AliasBox{IntCell:box.Cell};
 const s=SourceAliasBox{CellAlias:box.Cell};
 return [box.Read(),value.Read(),m.Read(box),e.Number,d.Read(),a.Read(),s.Read()];
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "importedembedding")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	invalid := filepath.Join(root, "invalid.km")
	if err := os.WriteFile(invalid, []byte(`import go m from "imported-struct-embedding.test/models"; struct Invalid { m.Cell; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err = EmitGo([]string{invalid}, "importedembedding")
	if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "requires 1 type arguments") {
		t.Fatalf("uninstantiated Go struct: %v %v", err, diagnostics)
	}
	reference := `package reference
import("bytes";"io")
type output struct{bytes.Buffer}
type layer struct{output}
type linked struct{*bytes.Buffer}
type renamed struct{bytes.Buffer}
func appendText(w io.Writer,s string)int{n,e:=io.WriteString(w,s);if e!=nil{return -1};return n}
func Buffers(data string)[]int{
 var value layer;var raw bytes.Buffer;l:=linked{&raw};var named renamed
 lenOf,bound:=(*layer).Len,value.Len
 a,b,c:=appendText(&value,data),appendText(l,data),appendText(&named,data)
 return []int{a,b,c,lenOf(&value),bound(),value.Buffer.Len(),raw.Len(),l.Len(),named.Buffer.Len(),named.Len()}
}
type cell[T any] struct{ Item T }
func(c cell[T])Read()T{return c.Item}
func(c *cell[T])Put(v T){c.Item=v}
type getter[T any] interface{Read()T}
func read[T any](c getter[T])T{return c.Read()}
type box struct{cell[int]}
type intCell=cell[int]
type aliasBox struct{intCell}
type sourceAliasBox struct{cell[int]}
type hidden struct{Number int}
type nested struct{hidden}
type envelope struct{nested}
type holder struct{getter[int]}
type delegate struct{holder}
func Records(n int)[]int{
 b:=box{cell[int]{n}};v:=b;b.Item++
 put:=(*box).Put;put(&b,n+3)
 e:=envelope{nested{hidden{n}}};e.Number+=2
 d:=delegate{holder{b.cell}}
 a,s:=aliasBox{b.cell},sourceAliasBox{b.cell}
 return []int{b.Read(),v.Read(),read(b),e.Number,d.Read(),a.Read(),s.Read()}
}
`
	comparison := `package importedembedding_test
import("reflect";"testing";g "imported-struct-embedding.test";r "imported-struct-embedding.test/reference")
func TestEmbedding(t *testing.T){
 for _,s:=range []string{"","abc","\xff"}{if got,want:=g.Buffers(s),r.Buffers(s);!reflect.DeepEqual(got,want){t.Fatal(s,got,want)}}
 for _,n:=range []int{-8,0,7}{if got,want:=g.Records(n),r.Records(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "imported-struct-embedding.test", generated, reference, comparison)
}
