package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestDecoratorsCannotBeSilentlyDiscarded(t *testing.T) {
	for _, input := range []string{`@D export class C{}`, `export class C{@D public value:int=1;}`, `export class C{constructor(@D value:int){}}`, `export class C{public function f(@D value:int):void{}}`, `export class C{public function f():void{const a=(@D x:int)=>x;}}`} {
		t.Run(input, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "main.km")
			dependency := filepath.Join(root, "lib.km")
			checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: `import { C } from "./lib"; function main():void{}`, dependency: input})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range checked.Diagnostics {
				if strings.Contains(d.Message, "decorator execution and metadata generation are not implemented yet") && d.Span.Path == dependency {
					return
				}
			}
			t.Fatalf("missing dependency decorator diagnostic: %v", checked.Diagnostics)
		})
	}
}

func TestDecoratorFactoryLinkingAndTargetMetadata(t *testing.T) {
	root := t.TempDir()
	entry, library, barrel := filepath.Join(root, "main.km"), filepath.Join(root, "lib.km"), filepath.Join(root, "barrel.km")
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		library: `export function Build(path:string):(ctx:string)=>void{return (ctx)=>{};}`,
		barrel:  `export {Build as Route} from "./lib";`,
		entry: `import {Route} from "./barrel";
function Build():int{return 1;}
@Route("class") class C{
 @Route("field") public static value:int=1;
 constructor(@Route("inject") private dependency:int){}
 @Route("method") public function run(@Route("param") Route:string):string{return Route;}
}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(checked.Diagnostics) != 5 {
		t.Fatalf("unexpected diagnostics: %v", checked.Diagnostics)
	}
	for _, d := range checked.Diagnostics {
		if !strings.Contains(d.Message, "decorator execution and metadata generation are not implemented yet") {
			t.Fatal(d)
		}
	}
	seen := map[string]*ast.DecoratorTarget{}
	for _, application := range checked.Program.Decorators {
		if application.Target == nil {
			t.Fatal("missing checked target")
		}
		call := application.Expression.(*ast.CallExpr)
		if call.Callee.(*ast.IdentifierExpr).Name == "Route" || call.Callee.(*ast.IdentifierExpr).Name == "Build" {
			t.Fatal("factory not linked to dependency declaration")
		}
		seen[application.Target.Kind+":"+application.Target.Name] = application.Target
	}
	field := seen["field:value"]
	if field == nil || !field.Static || field.Visibility != ast.Public || field.ValueType.Name != "int" {
		t.Fatalf("field metadata: %+v", field)
	}
	method := seen["method:run"]
	parameter := seen["parameter:Route"]
	if method == nil || parameter == nil || parameter.ParameterIndex != 0 || parameter.ValueType.Name != "string" || parameter.Owner != method.Declaration || method.Owner != seen["class:C"].Declaration || method.ValueType.Return.Name != "string" || len(method.ValueType.Parameters) != 1 {
		t.Fatal("method/parameter identity or signature lost")
	}
}

func TestDecoratorFactoryDiagnostics(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`@Unknown class C{}`, "undefined name"},
		{`function Factory(n:int):(ctx:string)=>void{return (ctx)=>{};} @Factory("wrong") class C{}`, "cannot use"},
		{`function Factory(n:int):(ctx:string)=>void{return (ctx)=>{};} @Factory() class C{}`, "expects"},
		{`function Factory():int{return 1;} @Factory() class C{}`, "must resolve to a function"},
		{`const D=1; @D class C{}`, "must resolve to a function"},
		{`function D(ctx:string):void{} function f(@D n:int):void{}`, "decorator target must be"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			entry := filepath.Join(t.TempDir(), "main.km")
			checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: tc.input})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range checked.Diagnostics {
				if strings.Contains(d.Message, tc.want) {
					return
				}
			}
			t.Fatalf("want %q, got %v", tc.want, checked.Diagnostics)
		})
	}
}
