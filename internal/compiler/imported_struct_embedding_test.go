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
type Count int
func (n Count) Read() int { return int(n) }
func (n *Count) Add(step int) { *n += Count(step) }
type CountRef = *Count
type CountPointer *Count
type GetterPointer = *Getter[int]
type Incrementer interface { Add(int) }
type Samples[T any] []T
func (s Samples[T]) First() T { return s[0] }
func NewSamples[T any](value T) Samples[T] { return Samples[T]{value} }
type Callback func(int) int
func (c Callback) Apply(n int) int { return c(n) }
`,
		"value_types.km": `import go fmt from "fmt";
import go time from "time";
import go m from "imported-struct-embedding.test/models";
export struct Interval { time.Duration; }
export struct Meter { m.Count; }
export struct PointerMeter { *m.Count; }
export struct AliasMeter { m.CountRef; }
struct Series { m.Samples<int>; }
struct CallbackBox { m.Callback; }
export function Durations(n:int64):bstring[]{
 const interval = Interval{Duration: time.Duration(n)};
 const stringOf = Interval.String;
 const bound = interval.String;
 const stringer:fmt.Stringer = interval;
 return [interval.String(), stringOf(interval), bound(), stringer.String()];
}
export function Values(n:int):int[]{
 let meter = Meter{Count:m.Count(n)};
 const copy = meter;
 const add = (*Meter).Add;
 add(&meter, 1);
 const shared = PointerMeter{Count:&meter.Count};
 const aliased = AliasMeter{CountRef:&meter.Count};
 PointerMeter.Add(shared, 2);
 AliasMeter.Add(aliased, 3);
 const incrementer:m.Incrementer = aliased;
 incrementer.Add(4);
 const series = Series{Samples:m.NewSamples(n)};
 const callback:m.Callback = (v:int):int => v + 5;
 const c = CallbackBox{Callback:callback};
 return [copy.Read(), shared.Read(), aliased.Read(), meter.Read(), series.First(), c.Apply(n), c.Callback(n)];
}
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
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km"), filepath.Join(root, "value_types.km")}, "importedembedding")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	for _, tc := range []struct{ field, want string }{
		{"m.Cell", "requires 1 type arguments"},
		{"m.Samples", "requires 1 type arguments"},
		{"m.CountPointer", "cannot embed a Go defined pointer type"},
		{"*m.CountRef", "cannot embed a Go defined pointer type or a pointer to a pointer"},
		{"m.GetterPointer", "cannot embed a pointer to a Go interface"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			invalid := filepath.Join(root, "invalid.km")
			input := `import go m from "imported-struct-embedding.test/models"; struct Invalid { ` + tc.field + `; }`
			checked, err := CheckFilesWithOverlay([]string{invalid}, map[string]string{invalid: input})
			if err != nil || len(checked.Diagnostics) != 1 || !strings.Contains(checked.Diagnostics[0].Message, tc.want) {
				t.Fatalf("got %v, %v; want a diagnostic containing %q", checked.Diagnostics, err, tc.want)
			}
		})
	}
	reference := `package reference
import (
 "bytes"
 "fmt"
 "io"
 "time"
)
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
type interval struct{time.Duration}
func Durations(n int64) []string {
 v := interval{time.Duration(n)}
 stringOf, bound := interval.String, v.String
 var stringer fmt.Stringer = v
 return []string{v.String(), stringOf(v), bound(), stringer.String()}
}
type count int
func (n count) Read() int { return int(n) }
func (n *count) Add(step int) { *n += count(step) }
type countRef = *count
type meter struct{ count }
type pointerMeter struct{ *count }
type aliasMeter struct{ countRef }
type samples[T any] []T
func (s samples[T]) First() T { return s[0] }
type series struct{ samples[int] }
type callback func(int) int
func (c callback) Apply(n int) int { return c(n) }
type callbackBox struct{ callback }
func Values(n int) []int {
 v := meter{count(n)}
 copy := v
 add := (*meter).Add
 add(&v, 1)
 shared := pointerMeter{&v.count}
 aliased := aliasMeter{&v.count}
 pointerMeter.Add(shared, 2)
 aliasMeter.Add(aliased, 3)
 var incrementer interface{ Add(int) } = aliased
 incrementer.Add(4)
 s := series{samples[int]{n}}
 c := callbackBox{func(v int) int { return v + 5 }}
 return []int{copy.Read(), shared.Read(), aliased.Read(), v.Read(), s.First(), c.Apply(n), c.callback(n)}
}
`
	comparison := `package importedembedding_test
import (
 "fmt"
 "reflect"
 "testing"
 "time"
 g "imported-struct-embedding.test"
 m "imported-struct-embedding.test/models"
 r "imported-struct-embedding.test/reference"
)
func TestEmbedding(t *testing.T){
 for _,s:=range []string{"","abc","\xff"}{if got,want:=g.Buffers(s),r.Buffers(s);!reflect.DeepEqual(got,want){t.Fatal(s,got,want)}}
 for _,n:=range []int{-8,0,7}{if got,want:=g.Records(n),r.Records(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}}
}
func TestValueTypeEmbedding(t *testing.T) {
 for _, n := range []int{-8, 0, 7} {
  if got, want := g.Values(n), r.Values(n); !reflect.DeepEqual(got, want) { t.Fatal(n, got, want) }
 }
 for _, d := range []time.Duration{-time.Second, 0, time.Millisecond} {
  if got, want := g.Durations(int64(d)), r.Durations(int64(d)); !reflect.DeepEqual(got, want) { t.Fatal(d, got, want) }
  var interval fmt.Stringer = g.Interval{Duration:d}
  if interval.String() != d.String() { t.Fatal(interval) }
 }
 count := m.Count(7)
 var meter m.Incrementer = g.AliasMeter{CountRef:&count}
 meter.Add(3)
 if count != 10 || count.Read() != 10 { t.Fatal(count) }
}
`
	runGeneratedGoDifferentialTest(t, root, "imported-struct-embedding.test", generated, reference, comparison)
}
