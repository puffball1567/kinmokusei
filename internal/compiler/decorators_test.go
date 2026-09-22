package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestDependencyDecoratorsAreChecked(t *testing.T) {
	definition := `function D(context:DecoratorContext):void{}`
	for _, input := range []string{`@D export class C{}`, `export class C{@D public value:int=1;}`, `export class C{constructor(@D value:int){}}`, `export class C{public function f(@D value:int):void{}}`} {
		t.Run(input, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "main.km")
			dependency := filepath.Join(root, "lib.km")
			checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: `import { C } from "./lib"; function main():void{}`, dependency: definition + input})
			if err != nil {
				t.Fatal(err)
			}
			if len(checked.Diagnostics) != 0 {
				t.Fatalf("dependency decorator diagnostics: %v", checked.Diagnostics)
			}
			if len(checked.Program.Decorators) != 1 || checked.Program.Decorators[0].Target == nil || checked.Program.Decorators[0].Span.Path != dependency {
				t.Fatalf("dependency decorator was not retained: %#v", checked.Program.Decorators)
			}
		})
	}
}

func TestDecoratorFactoryLinkingAndTargetMetadata(t *testing.T) {
	root := t.TempDir()
	entry, library, barrel := filepath.Join(root, "main.km"), filepath.Join(root, "lib.km"), filepath.Join(root, "barrel.km")
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		library: `export function Build(path:string):(ctx:DecoratorContext)=>void{return (ctx)=>{};}`,
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
	if len(checked.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", checked.Diagnostics)
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

func TestDecoratorTargetIdentitySurvivesSameNamedDependencyClasses(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	left := filepath.Join(root, "left.km")
	right := filepath.Join(root, "right.km")
	leftBarrel := filepath.Join(root, "left_barrel.km")
	rightBarrel := filepath.Join(root, "right_barrel.km")
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		left:        `function D(context:DecoratorContext):void{} @D export class Service{}`,
		right:       `function D(context:DecoratorContext):void{} @D export class Service{}`,
		leftBarrel:  `export {Service as Left} from "./left";`,
		rightBarrel: `export {Service as Right} from "./right";`,
		entry:       `import {Left} from "./left_barrel"; import {Right} from "./right_barrel"; function use(left:Left,right:Right):void{}`,
	})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	if len(checked.Program.Decorators) != 2 {
		t.Fatalf("decorators=%d", len(checked.Program.Decorators))
	}
	leftTarget := checked.Program.Decorators[0].Target
	rightTarget := checked.Program.Decorators[1].Target
	if leftTarget == nil || rightTarget == nil || leftTarget.ClassName != "Service" || rightTarget.ClassName != "Service" || leftTarget.Identity == rightTarget.Identity {
		t.Fatalf("target identities are not distinct: left=%+v right=%+v", leftTarget, rightTarget)
	}
}

func TestDecoratorFactoryDiagnostics(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`@Unknown class C{}`, "undefined name"},
		{`function Factory(n:int):(ctx:DecoratorContext)=>void{return (ctx)=>{};} @Factory("wrong") class C{}`, "cannot use"},
		{`function Factory(n:int):(ctx:DecoratorContext)=>void{return (ctx)=>{};} @Factory() class C{}`, "expects"},
		{`function Factory():int{return 1;} @Factory() class C{}`, "must resolve to a function"},
		{`const D=1; @D class C{}`, "must resolve to a function"},
		{`function D(ctx:string):void{} @D class C{}`, "decorator callback must have type"},
		{`function D(ctx:DecoratorContext):void{} function f(@D n:int):void{}`, "decorator target must be"},
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

func TestDecoratorRuntimeRegistrationMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "decorators.km")
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		library: `let trace:string="";
export function Mark(label:string):(context:DecoratorContext)=>void{
 trace+="F"+label;
 return (context)=>{trace+="A"+label+":"+context.kind+":"+context.className+":"+context.memberName+":"+context.parameterName+":"+context.valueType+":"+context.valueIdentity+";";};
}
export function Trace():string{return trace;}
@Mark("library-class") export class Library{}`,
		entry: `import {Library,Mark,Trace} from "./decorators";
class Dependency{}
@Mark("class-first") @Mark("class-second")
export class Service{
 @Mark("field") public static count:int=0;
 constructor(@Mark("constructor-parameter") dependency:Dependency,private name:string){}
 @Mark("method") public function run(@Mark("method-parameter") value:int):string{return this.name;}
}
export function Snapshot():string{return Trace();}`,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "decorators")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	generatedAgain, repeatedDiagnostics, repeatedErr := EmitGo([]string{entry}, "decorators")
	if repeatedErr != nil || len(repeatedDiagnostics) != 0 || !bytes.Equal(generated, generatedAgain) {
		t.Fatalf("decorator output is not repeatable: err=%v diagnostics=%v equal=%v", repeatedErr, repeatedDiagnostics, bytes.Equal(generated, generatedAgain))
	}
	text := string(generated)
	if !strings.Contains(text, "type __kinmokuseiDecoratorContext struct") || !strings.Contains(text, "func init()") {
		t.Fatalf("missing decorator runtime lowering:\n%s", generated)
	}
	reference := `package reference
	var trace="Flibrary-classAlibrary-class:class:Library:::Library:type|Library;Fclass-firstFclass-secondAclass-second:class:Service:::Service:type|Service;Aclass-first:class:Service:::Service:type|Service;FfieldAfield:field:Service:count::int:;Fconstructor-parameterAconstructor-parameter:parameter:Service:constructor:dependency:Dependency:type|Dependency;FmethodAmethod:method:Service:run::(int) => string:;Fmethod-parameterAmethod-parameter:parameter:Service:run:value:int:;"
func Snapshot()string{return trace}
`
	comparison := `package decorators_test
import("testing";g "decorator-runtime.test";r "decorator-runtime.test/reference")
func TestRegistration(t *testing.T){if got,want:=g.Snapshot(),r.Snapshot();got!=want{t.Fatalf("trace=%q want %q",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-runtime.test", generated, reference, comparison)
}
