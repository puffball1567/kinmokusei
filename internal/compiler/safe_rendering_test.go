package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeferredFormattingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "format.km")
	input := `import go fmt from "fmt";
import go {Sprintf as format} from "fmt";
export function Render<T>(value:T):bstring{return format("%#v",value);}
class Counter{public value:int=42;}
class Mutate implements fmt.GoStringer{
 constructor(private counter:Counter){}
 public function GoString():bstring{this.counter.value=99;return b"changed-value";}
}
class Argument implements fmt.GoStringer{
 constructor(private render:()=>bstring){}
 public function GoString():bstring{return this.render();}
}
function message<A,B>(expected:A,actual:B):bstring{
 const first=new Argument(():bstring=>Render(expected));
 const second=new Argument(():bstring=>Render(actual));
 return fmt.Sprintf("expected %#v, got %#v",first,second);
}
export function Deferred():(bstring,int){const counter=new Counter();const text=message(new Mutate(counter),counter.value);return text,counter.value;}
export function Native():bstring{return fmt.Sprintf("%s:%s:%s",["a","b"],"雪",b"\xff");}
export function Concurrent():bstring{const task=go fmt.Sprintf("%s",["a","b"]);return await task;}
export function Later(value:string[],write:(text:bstring)=>void):void{defer write(fmt.Sprintf("%s",value));}
function args<T>(value:T):(bstring,T){return b"%#v",value;}
export function Multi<T>(value:T):bstring{return fmt.Sprintf(args(value));}
export function Pair<T>(value:T):(bstring,T){return fmt.Sprintf("%#v",value),value;}
export function DirectDefer(value:string[]):void{defer fmt.Sprintf("%#v",value);}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "rendering")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "safe-rendering.test", generated,
		`package reference
import "fmt"
func Render[T any](value T)string{return fmt.Sprintf("%#v",value)}
type counter struct{value int}
type mutate struct{counter *counter}
func(m mutate)GoString()string{m.counter.value=99;return "changed-value"}
type argument struct{render func()string}
func(a argument)GoString()string{return a.render()}
func message[A,B any](expected A,actual B)string{return fmt.Sprintf("expected %#v, got %#v",argument{func()string{return Render(expected)}},argument{func()string{return Render(actual)}})}
func Deferred()(string,int){c:=&counter{42};text:=message(mutate{c},c.value);return text,c.value}
func Native()string{return fmt.Sprintf("%s:%s:%s",[]string{"a","b"},"雪","\xff")}
func Concurrent()string{done:=make(chan string,1);go func(){done<-fmt.Sprintf("%s",[]string{"a","b"})}();return <-done}`,
		`package rendering_test
import("fmt";"math";"testing";g "safe-rendering.test";r "safe-rendering.test/reference")
type formatter int
func(formatter)Format(state fmt.State,verb rune){fmt.Fprintf(state,"format:%c:%v",verb,state.Flag('#'))}
type panicStringer struct{}
func(panicStringer)GoString()string{panic("formatter failed")}
func TestBehavior(t *testing.T){
 a,b:=g.Deferred();c,d:=r.Deferred();if a!=c||b!=d||a!="expected changed-value, got 42"||b!=99{t.Fatal("deferred capture/mutation",a,b,c,d)}
 if g.Native()!=r.Native()||g.Concurrent()!=r.Concurrent(){t.Fatal("native containers/task")}
 for _,v:=range []any{42,"雪","\xff",[]string{"a","b"},formatter(1),panicStringer{},math.NaN(),math.Copysign(0,-1)}{if got,want:=g.Render(v),r.Render(v);got!=want{t.Fatalf("format %T: %q vs %q",v,got,want)}}
 called:=0;g.Later([]string{"safe"},func(text string){called++;if text!="[safe]"{t.Fatal(text)}});if called!=1{t.Fatal("defer",called)}
 a,n:=g.Pair(42);if a!="42"||n!=42||g.Multi([]string{"safe"})!=r.Render([]string{"safe"}){t.Fatal("multiple result",a,n)};g.DirectDefer([]string{"safe"})
}`)
}

func TestJSONSnapshotMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "snapshot.km")
	input := `export function Snapshot<T>(value:T):Result<byte[]>{return jsonSnapshot(value,"","  ");}
export function Compact<T>(value:T):Result<byte[]>{return jsonSnapshot(value);}
export function Async<T>(value:T):Result<byte[]>{const task=go Compact(value);return await task;}
struct Record{public name:string;public tags:string[];}
export function Native():Result<byte[]>{return jsonSnapshot(Record{name:"雪",tags:["a","b"]},"","  ");}
let evaluations=0;
function value():string[]{evaluations++;return ["safe"];}
function prefix():bstring{evaluations=evaluations*10+2;return b"";}
function indent():bstring{evaluations=evaluations*10+3;return b"  ";}
export function Count():Result<int>{evaluations=0;const bytes=jsonSnapshot(value(),prefix(),indent())?;return ok(evaluations);}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "snapshot")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "safe-json-snapshot.test", generated,
		`package reference
import "encoding/json"
func Snapshot[T any](v T)([]byte,error){return json.MarshalIndent(v,"","  ")}
func Compact[T any](v T)([]byte,error){return json.Marshal(v)}
func Native()([]byte,error){return Snapshot(map[string]any{"name":"雪","tags":[]string{"a","b"}})}`,
		`package snapshot_test
import("bytes";"encoding/json";"errors";"fmt";"math";"testing";g "safe-json-snapshot.test";r "safe-json-snapshot.test/reference")
type cycle struct{Name string;Next *cycle}
type customError struct{Value *cycle}
func(e *customError)Error()string{return "custom failure"}
type marshaler struct{Value *cycle}
func(m marshaler)MarshalJSON()([]byte,error){return nil,&customError{m.Value}}
type validMarshaler struct{}
func(validMarshaler)MarshalJSON()([]byte,error){return []byte("{\"safe\":true}"),nil}
type countingError struct{calls *int}
func(e countingError)Error()string{*e.calls++;return "counted failure"}
type countingMarshaler struct{calls *int}
func(m countingMarshaler)MarshalJSON()([]byte,error){return nil,countingError{m.calls}}
func TestBehavior(t *testing.T){
 c:=&cycle{Name:"safe"};c.Next=c
 for _,v:=range []any{nil,42,"雪","\xff",[]string{"a","雪"},map[string][]string{"safe":{"a"}},math.NaN(),math.Inf(1),c,marshaler{c},validMarshaler{}}{
  a,e:=g.Snapshot(v);b,f:=r.Snapshot(v);if !bytes.Equal(a,b)||fmt.Sprint(e)!=fmt.Sprint(f){t.Fatalf("snapshot %T: %s/%v vs %s/%v",v,a,e,b,f)}
  if e!=nil{var unsupported *json.UnsupportedValueError;var custom *customError;var wrapped *json.MarshalerError;if errors.As(e,&unsupported)||errors.As(e,&custom)||errors.As(e,&wrapped)||errors.Unwrap(e)!=nil{t.Fatalf("returned writable alias: %T",e)}}
 }
 a,e:=g.Native();b,f:=r.Native();if !bytes.Equal(a,b)||e!=nil||f!=nil{t.Fatal("verified native DTO",string(a),e,string(b),f)}
 a,e=g.Compact([]string{"safe"});b,f=r.Compact([]string{"safe"});if !bytes.Equal(a,b)||e!=nil||f!=nil{t.Fatal("compact")}
 n,e:=g.Count();if n!=123||e!=nil{t.Fatal("evaluation order/count",n,e)}
 a,e=g.Async([]string{"safe"});if string(a)!="[\"safe\"]"||e!=nil{t.Fatal("snapshot task",string(a),e)}
 calls:=0;_,e=g.Compact(countingMarshaler{&calls});if e==nil||calls!=1{t.Fatal("error rendering",e,calls)};_=e.Error();_=e.Error();if calls!=1{t.Fatal("error callback was retained",calls)}
}`)
}
