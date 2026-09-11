package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedGoImportPipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go { Println } from "fmt"; function main():void { Println("hello"); }`,
		`import go { Compare, Ordered } from "cmp"; function compare<T extends Ordered>(a:T,b:T):int{return Compare(a,b);}`,
		`import go { Duration, Second } from "time"; function f():Duration{return Duration(2)*Second;}`,
		`import go { Buffer } from "bytes"; function f():int { let b:Buffer=Buffer{}; return b.Len(); }`,
		`import go { MaxUint8, Pi } from "math"; function f():byte{return MaxUint8;} function g():float32{return Pi;}`,
		`import go { DefaultClient, Client } from "net/http"; function f(value:*Client):void { DefaultClient=value; const pointer=&DefaultClient; }`,
		`import go { Sprint } from "fmt"; function f(Sprint:(v:int)=>int):int{return Sprint(1);}`,
		`import go { Sprint } from "fmt"; function f():string { const Sprint=(v:int):string=>{return "local";}; return Sprint(1); }`,
		`import go { Sprint } from "fmt"; import go fmt from "fmt"; function f():string{return Sprint(1)+fmt.Sprint(2);}`,
		`import go fmt from "fmt"; import go { Sprint } from "fmt"; function f():string{return Sprint(1)+fmt.Sprint(2);}`,
		`import go { Sprint } from "fmt"; import go { Sprintln } from "fmt"; function f():string{return Sprint(1)+Sprintln(2);}`,
		`import go { Duration } from "time"; function f<Duration>(v:Duration):Duration{return v;}`,
		`import go fmt from "fmt"; import go { Sprint } from "fmt"; function f(fmt:int):string{return Sprint(fmt);} function g():string{return fmt.Sprint(2);}`,
		`import go { Sprint } from "fmt"; import go fmt from "fmt"; function f(fmt:int):string{return Sprint(fmt);} function g():string{return fmt.Sprint(2);}`,
		`import go time from "time"; import go { Second } from "time"; function f():time.Duration{return Second;}`,
		`import go { Pi } from "math"; const value=Pi; function f():float32{return value;}`,
		`import go { Reader } from "io"; interface Input extends Reader {}`,
		`import go { Pointer } from "sync/atomic"; function f():void { let p:Pointer<int>=Pointer<int>{}; p.Store(nil); }`,
		`import go { EOF } from "io"; function pair():Result<int>{return ok(1);} function f():void {let n=0; [n,EOF]=pair();}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				t.Fatalf("accepted=%v err=%v", accepted, err)
			}
		})
	}
}

func TestNamedGoImportDiagnostics(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go {} from "fmt";`,
		`import go { missing } from "fmt";`,
		`import go { Println, Println } from "fmt";`,
		`import go { Println } from "fmt"; function Println():void {}`,
		`import go { Println } from "fmt"; import go Println from "strings";`,
		`import go { Pi } from "math"; function f():void { Pi=1; }`,
		`import go { Println } from "fmt"; function f():void { Println=1; }`,
		`import go { MaxUint16 } from "math"; function f():byte { return MaxUint16; }`,
		`import go { Pi } from "math"; function f():int { return Pi; }`,
		`import go { Println } from "fmt"; function f(value:Println):void {}`,
		`import go { Compare } from "cmp"; function f():int { return Compare(1,"x"); }`,
		`import go { Pointer } from "unsafe";`,
	} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
			result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
			if err != nil || len(result.Diagnostics) == 0 {
				t.Fatalf("err=%v result=%+v", err, result)
			}
			for _, d := range result.Diagnostics {
				if d.Span.Path != path || strings.Contains(d.Message, "generated Go") {
					t.Fatal(d)
				}
			}
		})
	}
}

func TestNamedGoImportsStayFileLocal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	helper := filepath.Join(root, "helper.km")
	main := filepath.Join(root, "main.km")
	for _, input := range []string{
		`import { helper } from "./helper"; function f():string{return Sprint(1);}`,
		`import { helper } from "./helper"; import go { Sprint } from "fmt"; function Sprint():void{}`,
		`import { Sprint } from "./helper"; import go { Sprint } from "fmt";`,
	} {
		helperSource := `import go { Sprint } from "fmt"; function helper():string{return Sprint(1);}`
		if strings.Contains(input, "import { Sprint }") {
			helperSource = `function Sprint():string{return "source";}`
		}
		result, err := CheckFilesWithOverlay([]string{main}, map[string]string{
			main:   input,
			helper: helperSource,
		})
		if err != nil || len(result.Diagnostics) == 0 {
			t.Fatalf("input=%s err=%v result=%+v", input, err, result)
		}
	}
}

func TestNamedGoImportsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"helper.km": `import go { Sprint } from "fmt"
function helper(value:int):string{return Sprint(value)}`,
		"main.km": `import { helper } from "./helper"
import go { Sprint } from "fmt"
import go { Compare } from "cmp"
import go { Duration, Second } from "time"
import go { Pi, MaxUint8 } from "math"
import go { Buffer } from "bytes"
import go { Args } from "os"
import go { Join } from "strings"
function restoreArgs(saved:string[]):void { Args=saved }
function MutateArgs():string {
  const saved=Args
  defer restoreArgs(saved)
  Args=["left"]
  const pointer=&Args
  Args=append(Args,"right");
  (*pointer)[0]="updated"
  return Join(Args,",")
}
function Run(value:int):string {
  const render = Sprint
  const duration:Duration = Duration(value)*Second
  let b:Buffer = Buffer{}
  const [written, failure] = b.WriteString(render(duration))
  const sign = Compare(value, 0)
  const narrow:byte = MaxUint8
  return helper(value)+":"+b.String()+":"+Sprint(sign, narrow, Pi, written, failure)
}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "namedimports")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import ("fmt"; "cmp"; "time"; "math"; "bytes"; "os"; "strings")
func MutateArgs() string {
  saved:=os.Args
  defer func(){os.Args=saved}()
  os.Args=[]string{"left"}
  pointer:=&os.Args
  os.Args=append(os.Args,"right")
  (*pointer)[0]="updated"
  return strings.Join(os.Args,",")
}
func Run(value int) string {
  duration := time.Duration(value)*time.Second
  var b bytes.Buffer
  written, failure := b.WriteString(fmt.Sprint(duration))
  var narrow byte = math.MaxUint8
  return fmt.Sprint(value)+":"+b.String()+":"+fmt.Sprint(cmp.Compare(value,0),narrow,math.Pi,written,failure)
}`
	comparison := `package namedimports
import ("testing"; reference "named-go-imports.test/reference")
func TestBehavior(t *testing.T) {
  if got,want:=MutateArgs(),reference.MutateArgs();got!=want { t.Errorf("mutation=%q want %q",got,want) }
  for _, value := range []int{-10,0,1,100} {
    if got,want:=Run(value),reference.Run(value);got!=want { t.Errorf("Run(%d)=%q want %q",value,got,want) }
  }
}`
	runGeneratedGoDifferentialTest(t, root, "named-go-imports.test", generated, reference, comparison)
}
