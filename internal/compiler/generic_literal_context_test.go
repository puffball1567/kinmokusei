package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericFreshLiteralContext(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function ordinary(v:bstring[]):void{}function f<T>(v:T):void{}function main():void{ordinary(["a"]);f<bstring[]>(["a"]);}`,
		`function f<T>(v:T[]):void{}function main():void{f<bstring>(["a"]);}`,
		`function f<T>(a:T,b:T):void{}function main():void{const a:bstring[]=["a"];f(a,["a"]);}`,
		`function f<T,U>(a:T,b:U[]):void{}function main():void{f<int,bstring>(1,["a"]);}`,
		`function f<T>(v:T):void{}function main():void{f<bstring[][]>([["a"],["b"]]);f<bstring[]>([]);f<[2]bstring>(["a","b"]);}`,
		`function f<T>(v:{data:T[]}):void{}function main():void{f<bstring>({data:["a"]});}`,
		`function f<T>(...v:T[]):void{}function main():void{f<bstring[]>(["a"],["b"]);f<bstring>(["a","b"]...);}`,
		`function f<T>(v:T):void{}function main():void{const a:bstring[]=["a"];f<bstring[]>(a);f<bstring[]>([b"a"]);f<bstring>("a");}`,
		`class C{public function f<T>(v:T):void{}}function main():void{new C().f<bstring[]>(["a"]);}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "input.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestGenericFreshLiteralsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"library.km": `export function identity<T>(value:T):T{return value;}
export function second<T>(a:T,b:T):T{return b;}
export function object<T>(value:{data:T[]}):T[]{return value.data;}
export function rest<T>(...values:T[]):T{return values[0];}`,
		"main.km": `import { identity, second, object, rest } from "./library";
let trace=0;
function mark(n:int):string{trace=trace*10+n;return "safe";}
export function Run():int{
 trace=0;
 const array=identity<bstring[]>([mark(1),mark(2)]);
 const nested=object<bstring>({data:[mark(3)]});
 const variadic=rest<bstring[]>([mark(4),mark(5)],[mark(6)]);
 const expanded=rest<bstring>([mark(7),mark(8)]...);
 const inferred=second(array,[mark(9)]);
 return trace;
}
export function Copy():boolean{
 const verified:string="safe";
 const raw=identity<bstring[]>([verified]);
 raw[0]=b"\xff";
 return verified=="safe" && raw[0]==b"\xff";
}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := []string{filepath.Join(root, "main.km")}
	generated, diagnostics, err := EmitGo(entry, "genericliteralcontext")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(entry, "genericliteralcontext")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "generic-literal-context.test", generated,
		`package reference
func identity[T any](value T)T{return value}
func second[T any](a,b T)T{return b}
func object[T any](value struct{data []T})[]T{return value.data}
func rest[T any](values ...T)T{return values[0]}
func Run()int{
 trace:=0
 mark:=func(n int)string{trace=trace*10+n;return "safe"}
 array:=identity[[]string]([]string{mark(1),mark(2)})
 _=object[string](struct{data []string}{[]string{mark(3)}})
 _=rest[[]string]([]string{mark(4),mark(5)},[]string{mark(6)})
 _=rest[string]([]string{mark(7),mark(8)}...)
 _=second(array,[]string{mark(9)})
 return trace
}
func Copy()bool{verified:="safe";raw:=identity[[]string]([]string{verified});raw[0]="\xff";return verified=="safe"&&raw[0]=="\xff"}`,
		`package genericliteralcontext
import("testing";reference "generic-literal-context.test/reference")
func TestBehavior(t *testing.T){if got,want:=Run(),reference.Run();got!=want||got!=123456789{t.Fatal("evaluation order/count",got,want)};if !Copy()||Copy()!=reference.Copy(){t.Fatal("fresh storage")}}`)
}

func TestGenericFreshLiteralRetainsStorageContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`function f<T>(v:T):void{}function main():void{const a:string[]=["a"];f<bstring[]>(a);}`, "cannot use string[] as bstring[]"},
		{`function f<T>(a:T,b:T):void{}function main():void{const a:bstring[]=["a"];const b:string[]=["b"];f(a,b);}`, "cannot use string[] as bstring[]"},
		{`function f<T>(v:T):void{}function main():void{const a:string[]=["a"];f<bstring[][]>([a]);}`, "cannot use string[] as bstring[]"},
		{`function f<T>(v:T):void{}function main():void{f<string[]>([b"\xff"]);}`, "cannot use bstring as string"},
		{`function f<T>(v:T):void{}function main():void{f<[2]bstring>(["a"]);}`, "fixed array literal has 1 elements"},
		{`function f<T>(v:T):void{}function outer<U>():void{f<U[]>(["a"]);}`, "cannot use"},
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
			t.Fatalf("diagnostics=%v want=%s", checked.Diagnostics, test.want)
		})
	}
}
