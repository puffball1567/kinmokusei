package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstraintMethodsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module constraint-methods.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
import "strconv"
type Getter[E any] interface{Get()E}
type Setter[E any] interface{Set(E)}
type Cell[E any] struct{Value E}
func(c *Cell[E])Get()E{return c.Value}
func(c *Cell[E])Set(value E){c.Value=value}
type Numbers []int
func(n Numbers)String()string{return strconv.Itoa(len(n))}
`,
		"bounds.km": `import go fmt from "fmt";
import go c from "constraint-methods.test/contracts";
constraint Integer=~int|~int64;
constraint Printable=Integer&fmt.Stringer;
constraint Both<E>=c.Getter<E>&c.Setter<E>;
constraint Getter<E>=c.Getter<E>;
constraint Storage<E>=~E[];
constraint NamedSlice<E>=Storage<E>&fmt.Stringer;
export {Printable as Printed, Both as Access, Getter, NamedSlice};`,
		"bridge.km": `export {Printed,Access,Getter,NamedSlice} from "./bounds";`,
		"entry.km": `import {Printed,Access,Getter,NamedSlice} from "./bridge";
import go c from "constraint-methods.test/contracts";
import go io from "io";
import go strings from "strings";
import go strconv from "strconv";
constraint Public=Printed&~int;
constraint Stream=io.Reader&io.Closer;
type Score=distinct int;
public function string(this:Score):string{return strconv.Itoa(int(this));}
function show<T extends Public>(value:T):string{return (value+value).String();}
function access<E,T extends Access<E>>(cell:T,value:E):E{const set=cell.Set;set(value);const get=cell.Get;return get();}
function read<T extends Getter<E>,E>(cell:T):E{return cell.Get();}
function collect<E,S extends NamedSlice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}
class Holder<E,T extends Access<E>>{constructor(private cell:T){}public function update(value:E):E{return access(this.cell,value);}}
function Format(value:int):string{return show(Score(value));}
function Update(value:int):int{let storage=c.Cell<int>{};const cell=&storage;return new Holder<int,*c.Cell<int>>(cell).update(value)+read(cell);}
function Text(value:string):string{let storage=c.Cell<string>{};const cell=&storage;return access(cell,value);}
function Collected(values:int[]):int[]{return collect(c.Numbers(values));}
function readStream<T extends Stream>(stream:T):Result<string>{const bytes=io.ReadAll(stream)?;const problem=stream.Close();if(problem!==nil){return fail(problem);}return ok(string(bytes));}
function Read(text:string):Result<string>{return readStream(io.NopCloser(strings.NewReader(text)));}
function ReadOnce<T extends Stream>(stream:T,bytes:byte[]):Result<int>{const [count,problem]=stream.Read(bytes);if(problem!==nil){return fail(problem);}return ok(count);}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "methodconstraints")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"fmt.Stringer", "io.Reader", "io.Closer", "c.Getter[E]", "c.Setter[E]"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import("fmt";"io";"strconv";"strings")
type Printed interface{~int|~int64;fmt.Stringer}
type Public interface{Printed;~int}
type Score int
func(s Score)String()string{return strconv.Itoa(int(s))}
func show[T Public](value T)string{return (value+value).String()}
type Getter[E any]interface{Get()E}
type Setter[E any]interface{Set(E)}
type Access[E any]interface{Getter[E];Setter[E]}
type Cell[E any]struct{Value E}
func(c *Cell[E])Get()E{return c.Value}
func(c *Cell[E])Set(value E){c.Value=value}
func access[E any,T Access[E]](cell T,value E)E{set:=cell.Set;set(value);get:=cell.Get;return get()}
func read[T Getter[E],E any](cell T)E{return cell.Get()}
type Holder[E any,T Access[E]]struct{cell T}
func(h *Holder[E,T])update(value E)E{return access(h.cell,value)}
type Numbers []int
func(n Numbers)String()string{return strconv.Itoa(len(n))}
type NamedSlice[E any]interface{~[]E;fmt.Stringer}
func collect[E any,S NamedSlice[E]](values S)[]E{result:=[]E{};for _,value:=range values{result=append(result,value)};return result}
func Format(value int)string{return show(Score(value))}
func Update(value int)int{cell:=new(Cell[int]);return (&Holder[int,*Cell[int]]{cell}).update(value)+read(cell)}
func Text(value string)string{return access(new(Cell[string]),value)}
func Collected(values []int)[]int{return collect(Numbers(values))}
func readStream[T interface{io.Reader;io.Closer}](stream T)(string,error){data,err:=io.ReadAll(stream);if err!=nil{return "",err};if err=stream.Close();err!=nil{return "",err};return string(data),nil}
func Read(text string)(string,error){return readStream(io.NopCloser(strings.NewReader(text)))}
func ReadOnce[T interface{io.Reader;io.Closer}](stream T,bytes []byte)(int,error){count,err:=stream.Read(bytes);if err!=nil{return 0,err};return count,nil}
`
	comparison := `package methodconstraints_test
import("reflect";"strconv";"io";"strings";"testing";generated "constraint-methods.test";reference "constraint-methods.test/reference")
type Named int
func(n Named)String()string{return strconv.Itoa(int(n))}
func external[T generated.Public](x T)string{return (x+x).String()}
func TestBehavior(t *testing.T){
 for _,x:=range []int{-17,0,19}{if got,want:=generated.Format(x),reference.Format(x);got!=want{t.Errorf("format=%q want=%q",got,want)};if got,want:=generated.Update(x),reference.Update(x);got!=want{t.Errorf("update=%d want=%d",got,want)};if got,want:=external(Named(x)),reference.Format(x);got!=want{t.Errorf("external=%q want=%q",got,want)}}
 for _,x:=range []string{"","hello","金木犀"}{if got,want:=generated.Text(x),reference.Text(x);got!=want{t.Errorf("text=%q want=%q",got,want)};got,ge:=generated.Read(x);want,we:=reference.Read(x);if got!=want||(ge==nil)!=(we==nil){t.Errorf("read=(%q,%v) want=(%q,%v)",got,ge,want,we)}}
 for _,xs:=range [][]int{nil,{}, {1,-2,3}}{if got,want:=generated.Collected(xs),reference.Collected(xs);!reflect.DeepEqual(got,want){t.Errorf("collected=%v want=%v",got,want)}}
 for _,text:=range []string{"","hello","金木犀"}{for _,size:=range []int{0,1,8}{a,b:=make([]byte,size),make([]byte,size);got,ge:=generated.ReadOnce(io.NopCloser(strings.NewReader(text)),a);want,we:=reference.ReadOnce(io.NopCloser(strings.NewReader(text)),b);if got!=want||(ge==nil)!=(we==nil)||!reflect.DeepEqual(a,b){t.Errorf("read once=(%d,%v,%v) want=(%d,%v,%v)",got,ge,a,want,we,b)}}}
}
`
	runGeneratedGoDifferentialTest(t, root, "constraint-methods.test", generated, reference, comparison)
}
