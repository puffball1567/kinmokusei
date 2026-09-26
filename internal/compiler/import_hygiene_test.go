package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportAliasesAvoidLocalCapture(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"library.km": `export let count=4;export function pair(n:int):int{return n+10;}export class Box<T>{constructor(public value:T){}}`,
		"bridge.km":  `export {count as shared,pair as read,Box as Crate} from "./library";`,
		"entry.km": `import {shared as total,read as load,Crate as Container} from "./bridge";
export function Run():int{const pair=(n:int):int=>n+100;let count=30;total++;const p=&total;*p+=2;return load(2)+pair(3)+count+total;}
export function Parameter(pair:int,count:int):int{return load(pair)+total+count;}
export function Generic<Box>(value:Box):Box{const item:Container<Box>=new Container<Box>(value);return item.value;}
export function Nested():int{let result=0;for(const pair of [1,2]){result+=load(pair);}for(let count=0;count<2;count++){result+=total+count;}const wrap=(pair:int):int=>load(pair);return result+wrap(3);}
export function Shadow():int{const load=(n:int):int=>n+1000;return load(1);}
export class Reader<Box>{constructor(public value:Box){}public function read(pair:int):int{const wrapped:Container<Box>=new Container<Box>(this.value);return load(pair);}}
export function OOP():int{return new Reader<string>("ok").read(2);}
export function Selected():int{const channel=goChannel<int>(1);channel<-2;select{case const pair=<-channel{return load(pair);}default{return -1;}}}
export function Destructured():int{const produce=():(int,int)=>{return 2,3;};const [pair,count]=produce();return load(pair)+total+count;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "hygiene")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var total=4
func Run()int{total++;p:=&total;*p+=2;return 12+103+30+total}
func Parameter(pair,count int)int{return pair+10+total+count}
func Generic[T any](value T)T{return value}
func Nested()int{return 11+12+total+(total+1)+13}
func Shadow()int{return 1001}
func OOP()int{return 12}
func Selected()int{ch:=make(chan int,1);ch<-2;select{case n:=<-ch:return n+10;default:return -1}}
func Destructured()int{return 12+total+3}
`
	comparison := `package hygiene_test
import("testing";g "import-hygiene.test";r "import-hygiene.test/reference")
func TestHygiene(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatal("run",got,want)};if got,want:=g.Parameter(2,3),r.Parameter(2,3);got!=want{t.Fatal("parameters",got,want)};if got,want:=g.Generic("ok"),r.Generic("ok");got!=want{t.Fatal("generic",got,want)};if got,want:=g.Nested(),r.Nested();got!=want{t.Fatal("nested",got,want)};if got,want:=g.Shadow(),r.Shadow();got!=want{t.Fatal("shadow",got,want)};if got,want:=g.OOP(),r.OOP();got!=want{t.Fatal("OOP",got,want)};if got,want:=g.Selected(),r.Selected();got!=want{t.Fatal("select",got,want)};if got,want:=g.Destructured(),r.Destructured();got!=want{t.Fatal("destructure",got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "import-hygiene.test", generated, reference, comparison)
}

func TestImportLinkNamesAvoidGeneratedNameCollisions(t *testing.T) {
	t.Parallel()
	var previous []byte
	for i := 0; i < 2; i++ {
		root := t.TempDir()
		lib := filepath.Join(root, "lib.km")
		entry := filepath.Join(root, "entry.km")
		loader := &moduleLoader{linkBase: root}
		candidate := loader.linkedModuleName(lib, "value")
		goCandidate := linkedGoAlias("fmt", "fmt")
		files := map[string]string{
			"lib.km": `export function value(n:int):int{return n+1;}`,
			"entry.km": fmt.Sprintf(`import {value as load} from "./lib";import go fmt from "fmt";
export function Run(value:int,%s:int,%s_2:int,fmt:int,%s:int):string{return fmtValue(load(value),%s,%s_2,fmt,%s);}
function fmtValue(a:int,b:int,c:int,d:int,e:int):string{return fmt.Sprint(a,b,c,d,e);}`, candidate, candidate, goCandidate, candidate, candidate, goCandidate),
		}
		for name, input := range files {
			if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		generated, diagnostics, err := EmitGo([]string{entry}, "hygiene")
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
		}
		if i != 0 && !bytes.Equal(generated, previous) {
			t.Fatal("hygienic output depends on checkout location")
		}
		previous = generated
	}
}

func TestRootImportCaptureIsDiagnosed(t *testing.T) {
	t.Parallel()
	for _, entryText := range []string{
		`import {value as load} from "./api";function f(value:int):int{return load();}`,
		`import {Box as Crate} from "./api";function f<Box>():Crate{return new Crate();}`,
	} {
		root := t.TempDir()
		api := filepath.Join(root, "api.km")
		entry := filepath.Join(root, "entry.km")
		result, err := CheckFilesWithOverlay([]string{api, entry}, map[string]string{
			api:   `export function value():int{return 42;}export class Box{}`,
			entry: entryText,
		})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			if strings.Contains(d.Message, "would be captured") && d.Span.Path == entry {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing source capture diagnostic: %v", result.Diagnostics)
		}
	}
}

func TestGoAliasesAvoidLocalCapture(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// The alphabetically first module establishes a different canonical alias.
	files := map[string]string{
		"a.km": `import go fmt from "fmt";export function first():string{return fmt.Sprint("a");}`,
		"entry.km": `import {first} from "./a";import go text from "fmt";
export function Run(fmt:int):string{return first()+text.Sprint(fmt);}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "hygiene")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "go-import-hygiene.test", generated,
		`package reference;import "fmt";func Run(n int)string{return "a"+fmt.Sprint(n)}`,
		`package hygiene_test;import("testing";g "go-import-hygiene.test";r "go-import-hygiene.test/reference");func TestRun(t *testing.T){if got,want:=g.Run(42),r.Run(42);got!=want{t.Fatal(got,want)}}`)
}
