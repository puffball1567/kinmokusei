package compiler

import (
	goast "go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalArrowGroupsPipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function run():int{const f=():int=>g();const g=():int=>42;return f();}`,
		`function run():int{const f=()=>g();const g:()=>int=()=>42;return f();}`,
		`function run():boolean{const even=(n:int):boolean=>{if(n==0){return true;}return odd(n-1);};const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};return even(8);}`,
		`function run():int{const a=():int=>b();const b=():int=>c();const c=():int=>42;return a();}`,
		`function run():int{const a=():int=>{const nested=():int=>b();return nested();};const b=():int=>42;return a();}`,
		`function run():int{const a=():int=>b();let b=():int=>1;b=():int=>2;return a();}`,
		`function run():int{const a=():int=>{b=():int=>7;return b();};let b=():int=>1;return a();}`,
		`function run():int{const a=():int=>{const p=&b;return (*p)();};let b=():int=>42;return a();}`,
		`function run():int{const a=(b:int):int=>b;const b=():int=>a(42);return b();}`,
		`function run():int{const a=():int=>{const b=():int=>7;return b();};const b=():int=>a();return b();}`,
		`function run<T>(value:T):T{const a=():T=>b();const b=():T=>value;return a();}`,
		`alias Callback=(n:int)=>int;function run():int{const a:Callback=(n)=>b(n);const b:Callback=(n)=>n+1;return a(41);}`,
		`import go http from "net/http";function run():void{const a:http.HandlerFunc=(w,r)=>b(w,r);const b:http.HandlerFunc=(w,r)=>{w.Header().Set("X-Method",r.Method);};}`,
		`function run():int{try{const a=():int=>b();const b=():int=>42;return a();}finally{}}`,
		`function run():int{for(let n=0;n<1;n++){const a=():int=>b();const b=():int=>n;return a();}return -1;}`,
		`function run():int{switch(1){case 1{const a=():int=>b();const b=():int=>42;return a();}default{return 0;}}}`,
		`class Box{public value:int=42;public run:()=>int;constructor(){const a=():int=>b();const b=():int=>this.value;this.run=a;}}`,
	} {
		t.Run(input, func(t *testing.T) {
			generated, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "group.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
			files := token.NewFileSet()
			file, err := parser.ParseFile(files, "generated.go", generated, 0)
			if err != nil {
				t.Fatal(err)
			}
			config := types.Config{Importer: importer.Default()}
			if _, err := config.Check("localgroups", files, []*goast.File{file}, nil); err != nil {
				t.Fatalf("generated Go does not type-check: %v\n%s", err, generated)
			}
		})
	}
}

func TestLocalArrowGroupsDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`class Box{}function run():Box{const first=():Box=>{Box();return later();};const later=():Box=>new Box();const Box=():int=>1;return first();}`, "conflicts with a type"},
		{`function run<T>(value:T):T{const first=():T=>{T();return value;};const T=():int=>1;return first();}`, "conflicts with a type parameter"},
		{`import go http from "net/http";function run():void{const first=():void=>http();const http=():void=>{};first();}`, "conflicts with a type or Go package"},
		{`function run():int{const a=()=>b();const b=()=>42;return a();}`, "needs an explicit return type"},
		{`function run():int{const a=()=>b();const b=()=>a();return a();}`, "needs an explicit return type"},
		{`function run():int{return a();const a=():int=>42;}`, "undefined"},
		{`function run():int{const a=():int=>b();a();const b=():int=>42;return b();}`, "undefined"},
		{`function run():int{const a=():int=>b();const x=1;const b=():int=>x;return a();}`, "undefined"},
		{`function run():int{const a=():int=>value;const b=():int=>42;const value=1;return a();}`, "undefined"},
		{`function run():int{const a=():int=>{b=():int=>7;return b();};const b=():int=>1;return a();}`, "cannot assign to const"},
		{`function run():int{const a=():int=>b("bad");const b=(n:int):int=>n;return a();}`, "cannot use string"},
		{`function run():int{const a=():int=>b();const b=():string=>"bad";return a();}`, "cannot use string"},
		{`function run():int{const a=():int=>b();const b=():int=>42;const b=():int=>7;return a();}`, "duplicate local name"},
		{`class Box{public value:int=1;}function run(x:Box|null):int{if(x===null){return 0;}const a=():void=>b();const b=():void=>{x=null;};a();return x.value;}`, "nullable"},
	} {
		t.Run(test.input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
			checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range checked.Diagnostics {
				if strings.Contains(d.Message, test.want) {
					return
				}
			}
			t.Fatalf("diagnostics=%v, want %s", checked.Diagnostics, test.want)
		})
	}
}

func TestLocalArrowGroupsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"library.km": `export const later=():int=>999;alias Callback=()=>int;
export function Even(n:int):boolean{const even=(n:int):boolean=>{if(n==0){return true;}return odd(n-1);};const odd=(n:int):boolean=>{if(n==0){return false;}return even(n-1);};return even(n);}
export function Shadow():int{const first=():int=>later();const later=():int=>42;return first();}
export function Mutable():int{let first=():int=>second();let second=():int=>1;const saved=first;second=():int=>7;return saved()*10+second();}
export function Escape():()=>int{let calls=0;const first=():int=>second();const second=():int=>{calls++;return calls;};return first;}
export function Iterations():int{let callbacks:Callback[]=[];for(const n of [1,2,3]){const first=():int=>second();const second=():int=>n;callbacks=append(callbacks,first);}return callbacks[0]()*100+callbacks[1]()*10+callbacks[2]();}`,
		"main.km": `import { Even, Shadow, Mutable, Escape, Iterations } from "./library";
export function Run(n:int):boolean{return Even(n);}
export function Values():int{const escaped=Escape();return Shadow()*10000+Mutable()*100+escaped()*10+escaped();}
export function Loops():int{return Iterations();}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "localgroups")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "local-arrow-groups.test", generated,
		`package reference
func Run(n int)bool{var even,odd func(int)bool;even=func(n int)bool{if n==0{return true};return odd(n-1)};odd=func(n int)bool{if n==0{return false};return even(n-1)};return even(n)}
func Values()int{var second func()int;first:=func()int{return second()};second=func()int{return 1};saved:=first;second=func()int{return 7};calls:=0;next:=func()int{calls++;return calls};return 42*10000+(saved()*10+second())*100+next()*10+next()}
func Loops()int{var callbacks []func()int;for _,n:=range []int{1,2,3}{var second func()int;first:=func()int{return second()};second=func()int{return n};callbacks=append(callbacks,first)};return callbacks[0]()*100+callbacks[1]()*10+callbacks[2]()}`,
		`package localgroups
import("testing";reference "local-arrow-groups.test/reference")
func TestBehavior(t *testing.T){for _,n:=range []int{0,1,2,5,8}{if Run(n)!=reference.Run(n){t.Fatal("mutual recursion",n)}};if Values()!=reference.Values(){t.Fatal("capture and mutable storage",Values(),reference.Values())};if Loops()!=reference.Loops(){t.Fatal("iteration capture")}}`)
}
