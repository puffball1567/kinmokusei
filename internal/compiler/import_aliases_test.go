package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestImportAliasesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"other.km": `export class Box{constructor(public value:string){}}`,
		"library.km": `export let count:int=0;
export class Box<T>{constructor(public value:T){}}
export function make<T>(value:T):Box<T>{return new Box<T>(value);}
export function bump():int{count++;return count;}
export const load=():Result<int>=>{return ok(count);};
`,
		"bridge.km": `import {count as shared,Box as Container,make as create,bump as add} from "./library";
export {shared,Container as Crate,create,add};
`,
		"entry.km": `import {shared as first,shared as second,Crate as LocalBox,create as build,add as increment} from "./bridge";
import {Box as OtherBox} from "./other";
import {load as fetch} from "./library";
import go {Sprint as render,Sprint as text,Sprint as len} from "fmt";
import go {Compare as compare,Ordered as Order} from "cmp";
import go {Duration as Span,Second as secondUnit} from "time";
import go {Buffer as Bytes} from "bytes";
import go {MaxUint8 as maximum} from "math";
import go {Args as arguments} from "os";
import go {Join as join} from "strings";
function compareValues<T extends Order>(a:T,b:T):int{return compare(a,b);}
function restore(saved:string[]):void{arguments=saved;}
function identity<Span>(value:Span):Span{return value;}
export function Run(n:int):Result<string>{
 const saved=arguments;defer restore(saved);
 arguments=["one"];const pointer=&arguments;arguments=append(arguments,"two");(*pointer)[0]="changed";
 const a=&first;const b=&second;first+=n;const next=increment();
 const value:LocalBox<int>=build(next);const read=fetch;const loaded=read()?;
 const duration:Span=Span(identity(n))*secondUnit;let buffer:Bytes=Bytes{};
 const [written,err]=buffer.WriteString(render(duration));
 const narrow:byte=maximum;
 const render=():string=>new OtherBox("local").value;
 return ok(text(a==b,";",value.value,";",loaded,";",compareValues(n,0),";",narrow,";",buffer.String(),";",join(arguments,","),";",render(),";",len(42)));
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "aliases")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("fmt";"cmp";"time";"bytes";"math";"os";"strings")
var count int
type box[T any]struct{value T}
func makeBox[T any](v T)box[T]{return box[T]{v}}
func Run(n int)(string,error){saved:=os.Args;defer func(){os.Args=saved}();os.Args=[]string{"one"};p:=&os.Args;os.Args=append(os.Args,"two");(*p)[0]="changed";a,b:=&count,&count;count+=n;count++;v:=makeBox(count);loaded:=count;duration:=time.Duration(n)*time.Second;var buffer bytes.Buffer;buffer.WriteString(fmt.Sprint(duration));var narrow byte=math.MaxUint8;render:=func()string{return "local"};return fmt.Sprint(a==b,";",v.value,";",loaded,";",cmp.Compare(n,0),";",narrow,";",buffer.String(),";",strings.Join(os.Args,","),";",render(),";",fmt.Sprint(42)),nil}
`
	comparison := `package aliases_test
import("testing";g "import-aliases.test";r "import-aliases.test/reference")
func TestAliases(t *testing.T){for _,n:=range []int{0,2,-5,7}{got,err:=g.Run(n);want,werr:=r.Run(n);if got!=want||err!=nil||werr!=nil{t.Fatalf("n=%d got=%q,%v want=%q,%v",n,got,err,want,werr)}}}
`
	runGeneratedGoDifferentialTest(t, root, "import-aliases.test", generated, reference, comparison)
}

func TestImportAliasesUnsafe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":          "module import-unsafe-alias.test\n\ngo 1.23\n",
		"kinmokusei.toml": "[project]\nname = \"import-alias\"\nversion = \"0.1.0\"\ngo-module = \"import-unsafe-alias.test\"\ngo-version = \"1.23\"\n[go.interop]\nunsafe = \"allow\"\n",
		"entry.km": `import go {SliceData as data,Slice as view,Pointer as Address,Add as offset} from "unsafe";
alias IntPointer=*int;
export function Run():int{const values:int[]=[40,2];const p=data(values);const address=Address(p);const back=IntPointer(offset(address,0));const xs=view(back,2);xs[0]+=xs[1];return values[0];}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "aliases")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "unsafe"
func Run()int{values:=[]int{40,2};p:=unsafe.SliceData(values);back:=(*int)(unsafe.Add(unsafe.Pointer(p),0));xs:=unsafe.Slice(back,2);xs[0]+=xs[1];return values[0]}`
	comparison := `package aliases_test
import("testing";g "import-unsafe-alias.test";r "import-unsafe-alias.test/reference")
func TestUnsafe(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatal(got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "import-unsafe-alias.test", generated, reference, comparison)
}

func TestImportAliasDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, message string }{
		{`import {value as local} from "./lib";function f():int{return value();}`, "undefined"},
		{`import {value as local} from "./lib";function local():void{}`, "conflicts with a declaration"},
		{`import {value as local,other as local} from "./lib";`, "duplicate"},
		{`import {hidden as local} from "./lib";`, "does not export"},
		{`import {missing as local} from "./lib";`, "does not declare"},
		{`import {value as int} from "./lib";`, "built-in"},
		{`import {value as complex64} from "./lib";`, "built-in"},
		{`import {value as goChannel} from "./lib";`, "built-in"},
		{`import {value as GoChannel} from "./lib";`, "built-in"},
		{`import {value as __kinmokusei_private} from "./lib";`, "built-in"},
		{`import {value as _} from "./lib";`, "cannot be '_'"},
		{`import {value as} from "./lib";`, "local name"},
		{`import {value as local} from "./lib";export {value};`, "not a local"},
		{`import go {Sprint as local} from "fmt";function f():string{return Sprint(1);}`, "undefined"},
		{`import go {Sprint as local} from "fmt";import {value as local} from "./lib";`, "duplicate"},
		{`import go {Sprint as local,Sprintln as local} from "fmt";`, "duplicate"},
		{`import go {Sprint as int} from "fmt";`, "built-in"},
		{`import go {Sprint as Map} from "fmt";`, "built-in"},
		{`import go {Sprint as GoReceiveChannel} from "fmt";`, "built-in"},
		{`import go {Ordered as comparable} from "cmp";`, "built-in"},
		{`import go {Pointer as Address} from "unsafe";`, "unsafe"},
		{`import go {missing as local} from "fmt";`, "no exported member"},
		{`import go {Pi as value} from "math";function f():void{value=1;}`, "cannot assign"},
	} {
		t.Run(test.input, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "entry.km")
			result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: test.input, filepath.Join(root, "lib.km"): `export function value():int{return 1;}export function other():int{return 2;}function hidden():int{return 3;}`})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, diagnostic := range result.Diagnostics {
				if strings.Contains(diagnostic.Message, test.message) {
					found = true
				}
				if strings.Contains(diagnostic.Message, "generated Go") {
					t.Errorf("expected source diagnostic: %v", diagnostic)
				}
			}
			if !found {
				t.Fatalf("expected %q: %v", test.message, result.Diagnostics)
			}
		})
	}
}
