package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
)

func TestCheckRejectsExpandingGenericInstantiations(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"function":               "function f<T>(x:T):void {\n  f<T[]>([x]);\n}",
		"class":                  "class C<T> {\n  public x:C<T[]>|null=null;\n}",
		"inferred call":          "function f<T>(x:T):void {\n  f([x]);\n}",
		"mutual functions":       "function f<T>(x:T):void{g<T[]>([x]);}\nfunction g<U>(x:U):void{f<U>(x);}",
		"function value":         "function f<T>():void{const next=f<T[]>;next();}",
		"local main binding":     `function f<T>(x:T):void{const main=1;f<T[]>([x]);}`,
		"generic method":         `class Box{public function f<T>(x:T):void{this.f<T[]>([x]);}}`,
		"method return":          "class C<T>{\n public function next():C<T[]> {return new C<T[]>();}\n}",
		"generic struct method":  `struct Box{public value:int;public function f<T>(x:T):void{this.f<T[]>([x]);}}`,
		"recursive struct":       `struct C<T>{public x:*C<T[]>;}`,
		"recursive interface":    "interface C<T> {\n function next():C<T[]>;\n}",
		"recursive defined type": `type C<T>=distinct C<T[]>[];`,
		"mutual classes":         `class A<T>{public b:B<T[]>|null=null;}class B<U>{public a:A<U>|null=null;}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cycle.km")
			result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, diagnostic := range result.Diagnostics {
				if strings.Contains(diagnostic.Message, "instantiation cycle") && diagnostic.Span.Path == path && diagnostic.Span.Start.Line > 0 {
					if (name == "function" || name == "class" || name == "inferred call" || name == "recursive interface" || name == "method return") && diagnostic.Span.Start.Line != 2 {
						t.Fatalf("expected recursive use on line 2: %v", diagnostic)
					}
					found = true
				}
			}
			if !found {
				t.Fatalf("missing source diagnostic for expanding instantiation: %v", result.Diagnostics)
			}
		})
	}
}

func TestCheckAllowsFiniteGenericInstantiations(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"same argument":            `function f<T>(v:T,n:int):void{if(n>0){f<T>(v,n-1);}}`,
		"permutation":              `function f<A,B>(a:A,b:B,n:int):void{if(n>0){f<B,A>(b,a,n-1);}}`,
		"constant reset":           `function f<T>(v:T,n:int):void{if(n>0){g<T[]>([v],n-1);}}function g<U>(v:U,n:int):void{if(n>0){f<int>(1,n-1);}}`,
		"erased phantom alias":     `alias Drop<T>=int;class C<T>{public x:Drop<C<T[]>>=0;}`,
		"erased function argument": `alias Drop<T>=int;function f<T>(n:int):void{if(n>0){f<Drop<T[]>>(n-1);}}`,
		"recursive class":          `class C<T>{public next:C<T>|null=null;}`,
		"recursive interface":      `interface C<T>{function next():C<T>;}`,
		"generic static helpers":   `class C<T>{public static function f(v:T):void{C.g<T[],T[]>([v]);}public static function g<U>(v:U):void{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "finite.km")
			result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
			if err != nil || len(result.Diagnostics) != 0 {
				t.Fatalf("finite instantiation rejected: %v %v", err, result.Diagnostics)
			}
			if _, err := codegen.GenerateWithTarget(result.Program, "fixture", result.goImporter, result.goSizes); err != nil {
				t.Fatalf("finite instantiation generated invalid Go: %v", err)
			}
		})
	}
}

func TestCheckReportsGenericCycleInImportedModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, library := filepath.Join(root, "main.km"), filepath.Join(root, "library.km")
	result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		entry:   `import {repeat} from "./library";function main():void{repeat<int>(1);}`,
		library: "export function repeat<T>(x:T):void {\n repeat<T[]>([x]);\n}",
	})
	if err != nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Span.Path != library || result.Diagnostics[0].Span.Start.Line != 2 || !strings.Contains(result.Diagnostics[0].Message, "instantiation cycle") {
		t.Fatalf("cycle did not retain imported source position: %v %v", err, result.Diagnostics)
	}
}
