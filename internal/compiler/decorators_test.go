package compiler

import (
	"bytes"
	goast "go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/project"
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

func TestDecoratorTargetCarriesClassAndBaseIdentity(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "main.km")
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: `
function D(context:DecoratorContext):void{}
class Base{}
@D class Child extends Base{
 constructor(){super();}
 @D public function run():void{}
}`})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	if len(checked.Program.Decorators) != 2 {
		t.Fatalf("decorators=%d", len(checked.Program.Decorators))
	}
	for _, application := range checked.Program.Decorators {
		target := application.Target
		if target == nil || target.ClassIdentity != "type|Child" || target.BaseIdentity != "type|Base" {
			t.Fatalf("inheritance identity=%+v", target)
		}
	}
}

func TestDecoratorTargetCarriesOverrideChain(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	input := `
function D(context:DecoratorContext):void{}
class Base{
 @D public virtual function run(@D value:int):int{return value;}
 @D public virtual get item():int{return 1;}
}
class Middle extends Base{
 @D public override function run(@D renamed:int):int{return renamed;}
 @D public override get item():int{return 2;}
}
class Leaf extends Middle{
 @D public override function run(@D last:int):int{return last;}
 @D public override get item():int{return 3;}
}`
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	targets := map[string]*ast.DecoratorTarget{}
	for _, application := range checked.Program.Decorators {
		if application.Target != nil {
			targets[application.Target.Identity] = application.Target
		}
	}
	assertChain := func(identity string, want ...string) {
		t.Helper()
		target := targets[identity]
		if target == nil || strings.Join(target.OverrideChain, ",") != strings.Join(want, ",") {
			t.Fatalf("%s override chain=%v want=%v", identity, target, want)
		}
	}
	assertChain("type|Base|method|run")
	assertChain("type|Middle|method|run", "type|Base|method|run")
	assertChain("type|Leaf|method|run", "type|Middle|method|run", "type|Base|method|run")
	assertChain("type|Leaf|method|run|parameter|0", "type|Middle|method|run|parameter|0", "type|Base|method|run|parameter|0")
	assertChain("type|Leaf|get|item", "type|Middle|get|item", "type|Base|get|item")

	if err := os.WriteFile(entry, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "decorator_override")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, identity := range []string{"type|Middle|method|run", "type|Base|method|run", "type|Middle|get|item", "type|Base|get|item"} {
		if !strings.Contains(text, strconv.Quote(identity)) {
			t.Fatalf("generated Go does not retain override identity %q:\n%s", identity, generated)
		}
	}
}

func TestDecoratorOverrideChainSurvivesModuleLinking(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	library := filepath.Join(root, "library.km")
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		library: `export function D(context:DecoratorContext):void{}
export class Base{@D public virtual function run(@D value:int):int{return value;}}`,
		entry: `import {Base,D} from "./library";
class Child extends Base{@D public override function run(@D renamed:int):int{return renamed;}}`,
	})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	var baseMethod, baseParameter, childMethod, childParameter *ast.DecoratorTarget
	for _, application := range checked.Program.Decorators {
		target := application.Target
		if target == nil {
			continue
		}
		switch target.ClassName + ":" + target.Kind {
		case "Base:method":
			baseMethod = target
		case "Base:parameter":
			baseParameter = target
		case "Child:method":
			childMethod = target
		case "Child:parameter":
			childParameter = target
		}
	}
	if baseMethod == nil || childMethod == nil || len(childMethod.OverrideChain) != 1 || childMethod.OverrideChain[0] != baseMethod.Identity {
		t.Fatalf("linked method chain: base=%+v child=%+v", baseMethod, childMethod)
	}
	if baseParameter == nil || childParameter == nil || len(childParameter.OverrideChain) != 1 || childParameter.OverrideChain[0] != baseParameter.Identity {
		t.Fatalf("linked parameter chain: base=%+v child=%+v", baseParameter, childParameter)
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

func TestDecoratorTargetSpecificContexts(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "main.km")
	input := `
function ClassOnly(context:ClassDecoratorContext):void{}
function FieldOnly(context:FieldDecoratorContext):void{}
function ConstructorOnly(context:ConstructorDecoratorContext):void{}
function MethodOnly(context:MethodDecoratorContext):void{}
function GetterOnly(context:GetterDecoratorContext):void{}
function SetterOnly(context:SetterDecoratorContext):void{}
function ParameterOnly(context:ParameterDecoratorContext):void{}
@ClassOnly class Service {
 @FieldOnly public value:int=0;
 @ConstructorOnly constructor(@ParameterOnly dependency:int){}
 @MethodOnly public function run(@ParameterOnly argument:int):void{}
 @GetterOnly public get item():int{return this.value;}
 @SetterOnly public set item(value:int){this.value=value;}
}
function main():void{}
`
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	if err := os.WriteFile(entry, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "decorator_targets")
	if err != nil || len(diagnostics) != 0 || !strings.Contains(string(generated), "func init()") {
		t.Fatalf("emit err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, context := range []string{"ClassDecoratorContext", "FieldDecoratorContext", "ConstructorDecoratorContext", "MethodDecoratorContext", "GetterDecoratorContext", "SetterDecoratorContext", "ParameterDecoratorContext"} {
		if strings.Contains(string(generated), context) {
			t.Fatalf("source-only context %s leaked into generated Go:\n%s", context, generated)
		}
	}
	files := token.NewFileSet()
	file, parseErr := parser.ParseFile(files, "generated.go", generated, 0)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	config := types.Config{Importer: importer.Default()}
	if _, typeErr := config.Check("decorator_targets", files, []*goast.File{file}, nil); typeErr != nil {
		t.Fatalf("generated Go does not type-check: %v\n%s", typeErr, generated)
	}
}

func TestDecoratorTargetSpecificContextDiagnostics(t *testing.T) {
	for _, tc := range []struct{ context, target, input string }{
		{"ClassDecoratorContext", "method", `class C{@D public function run():void{}}`},
		{"FieldDecoratorContext", "class", `@D class C{}`},
		{"ConstructorDecoratorContext", "field", `class C{@D public value:int=0;}`},
		{"MethodDecoratorContext", "get", `class C{@D public get value():int{return 1;}}`},
		{"GetterDecoratorContext", "set", `class C{@D public set value(v:int){}}`},
		{"SetterDecoratorContext", "parameter", `class C{public function run(@D value:int):void{}}`},
		{"ParameterDecoratorContext", "constructor", `class C{@D constructor(){}}`},
	} {
		t.Run(tc.context+"_on_"+tc.target, func(t *testing.T) {
			entry := filepath.Join(t.TempDir(), "main.km")
			input := "function D(context:" + tc.context + "):void{}" + tc.input
			checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input})
			if err != nil {
				t.Fatal(err)
			}
			want := "using " + tc.context + " cannot be applied to " + tc.target + " target"
			for _, diagnostic := range checked.Diagnostics {
				if strings.Contains(diagnostic.Message, want) {
					return
				}
			}
			t.Fatalf("want %q, got %v", want, checked.Diagnostics)
		})
	}
}

func TestDecoratorTargetContextRestrictionsCannotBeErased(t *testing.T) {
	for _, input := range []string{
		`function ClassOnly(context:ClassDecoratorContext):void{} const D:(context:DecoratorContext)=>void=ClassOnly;`,
		`function Factory():(context:ClassDecoratorContext)=>void{return (context:DecoratorContext)=>{};}`,
	} {
		t.Run(input, func(t *testing.T) {
			entry := filepath.Join(t.TempDir(), "main.km")
			checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input})
			if err != nil {
				t.Fatal(err)
			}
			for _, diagnostic := range checked.Diagnostics {
				if strings.Contains(diagnostic.Message, "cannot use") || strings.Contains(diagnostic.Message, "does not match") {
					return
				}
			}
			t.Fatalf("target restriction was structurally erased: %v", checked.Diagnostics)
		})
	}
}

func TestImportedDecoratorRetainsTargetContext(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	library := filepath.Join(root, "library.km")
	files := map[string]string{
		library: `export function Controller(path:string):(context:ClassDecoratorContext)=>void{return (context)=>{};}`,
		entry: `import {Controller} from "./library";
@Controller("/valid") class Valid{}
class Invalid{@Controller("/invalid") public function run():void{}}`,
	}
	checked, err := CheckFilesWithOverlay([]string{entry}, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(checked.Diagnostics) != 1 || !strings.Contains(checked.Diagnostics[0].Message, "using ClassDecoratorContext cannot be applied to method target") {
		t.Fatalf("imported target restriction diagnostics=%v", checked.Diagnostics)
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
 return (context)=>{if(len(context.overrideChain)>0){trace+=context.overrideChain[0];}trace+="A"+label+":"+context.kind+":"+context.className+":"+context.memberName+":"+context.parameterName+":"+context.valueType+":"+context.valueIdentity+";";};
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

func TestDecoratorConstructorAdaptersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "container.km")
	entry := filepath.Join(root, "entry.km")
	librarySource := `alias ConstructorAdapter=(arguments:DecoratorValue[])=>Result<DecoratorValue>;
let constructors:ConstructorAdapter[]=[];
export function Injectable(): (context:ClassDecoratorContext)=>void {
 return (context)=>{constructors=append(constructors,context.construct);};
}
export function Construct(index:int,arguments:DecoratorValue[]):Result<DecoratorValue>{const adapter=constructors[index];return adapter(arguments);}
`
	source := `import {Injectable,Construct} from "./container";
let trace:string="";
@Injectable() class Dependency{
 constructor(){trace+="dependency;";}
}
@Injectable() class Service{
 constructor(public dependency:Dependency){trace+="service;";}
}
@Injectable() class Batch{
 constructor(...dependencies:Dependency[]){trace+="batch;";if(len(dependencies)==2){trace+="two;";}}
}
@Injectable() class Message{
 constructor(public text:string){trace+=text+";";}
}
export function Build():Result<string>{
 const dependency=Construct(0,[])?;
 const service=Construct(1,[dependency])?;
 const batch=Construct(2,[dependency,dependency])?;
 const text=decoratorValue("boxed");
 const message=Construct(3,[text])?;
 const unboxed=decoratorValueAs<string>(text)?;
 return ok(trace+dependency.typeIdentity+";"+service.typeIdentity+";"+batch.typeIdentity+";"+message.typeIdentity+";"+text.typeIdentity+";"+unboxed);
}
export function BadArity():Result<DecoratorValue>{return Construct(1,[]);}
export function BadVariadicType():Result<DecoratorValue>{const dependency=Construct(0,[])?;const service=Construct(1,[dependency])?;return Construct(2,[service]);}
export function BadUnbox():Result<int>{const text=decoratorValue("text");return decoratorValueAs<int>(text);}
`
	if err := os.WriteFile(library, []byte(librarySource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "decoratoradapters")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, fragment := range []string{"type __kinmokuseiDecoratorValue struct", "Constructible: true", "NewDependency()", "NewService(argument0)", "NewBatch(variadicArguments...)", "NewMessage(argument0)"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("missing %q in generated adapter:\n%s", fragment, generated)
		}
	}
	reference := `package reference
func Build()(string,error){return "dependency;service;batch;two;boxed;type|Dependency;type|Service;type|Batch;type|Message;string;boxed",nil}
func BadArity()error{return errorString("decorator constructor for Service expects 1 argument")}
func BadVariadicType()error{return errorString("decorator constructor for Batch variadic arguments expect Dependency")}
func BadUnbox()error{return errorString("decorator value does not contain int")}
type errorString string
func(e errorString)Error()string{return string(e)}
`
	comparison := `package decoratoradapters_test
import("testing";g "decorator-adapters.test";r "decorator-adapters.test/reference")
func TestAdapters(t *testing.T){
 got,err:=g.Build();want,werr:=r.Build();if err!=nil||werr!=nil||got!=want{t.Fatalf("Build=%q,%v want %q,%v",got,err,want,werr)}
 _,err=g.BadArity();wantErr:=r.BadArity();if err==nil||err.Error()!=wantErr.Error(){t.Fatalf("BadArity=%v want %v",err,wantErr)}
 _,err=g.BadVariadicType();wantErr=r.BadVariadicType();if err==nil||err.Error()!=wantErr.Error(){t.Fatalf("BadVariadicType=%v want %v",err,wantErr)}
 _,err=g.BadUnbox();wantErr=r.BadUnbox();if err==nil||err.Error()!=wantErr.Error(){t.Fatalf("BadUnbox=%v want %v",err,wantErr)}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-adapters.test", generated, reference, comparison)
}

func TestDecoratorConstructorAdapterAvailabilityAndOpacity(t *testing.T) {
	t.Parallel()
	check := func(source string) Result {
		path := filepath.Join(t.TempDir(), "main.km")
		checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: source})
		if err != nil {
			t.Fatal(err)
		}
		return checked
	}
	checked := check(`function D(context:ClassDecoratorContext):void{}
@D abstract class AbstractService{public abstract function run():void;}
@D class GenericService<T>{}
@D class VariadicService{constructor(...values:int[]){}}
`)
	if len(checked.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", checked.Diagnostics)
	}
	wantReasons := []string{
		"abstract classes cannot be constructed",
		"generic classes require concrete type arguments",
	}
	if len(checked.Program.Decorators) != len(wantReasons)+1 {
		t.Fatalf("decorators=%d", len(checked.Program.Decorators))
	}
	for index, application := range checked.Program.Decorators[:len(wantReasons)] {
		if application.Target == nil || application.Target.Constructible || application.Target.ConstructUnavailableReason != wantReasons[index] {
			t.Errorf("target %d=%+v", index, application.Target)
		}
	}
	variadic := checked.Program.Decorators[len(wantReasons)].Target
	if variadic == nil || !variadic.Constructible || !variadic.ConstructorVariadic {
		t.Errorf("variadic target=%+v", variadic)
	}

	for _, name := range []string{"DecoratorValue", "ClassDecoratorContext"} {
		forged := check("const value:" + name + `={typeIdentity:"forged"};`)
		if len(forged.Diagnostics) == 0 || !strings.Contains(forged.Diagnostics[0].Message, "compiler-owned") {
			t.Errorf("%s diagnostics=%v", name, forged.Diagnostics)
		}
	}
}

func TestDecoratorMethodAdaptersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "routes.km")
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		library: `alias Invoker=(receiver:DecoratorValue,arguments:DecoratorValue[])=>Result<DecoratorValue>;
let invokers:Invoker[]=[];
export function Route():(context:MethodDecoratorContext)=>void{return (context)=>{invokers=append(invokers,context.invoke);};}
export function Call(index:int,receiver:DecoratorValue,arguments:DecoratorValue[]):Result<DecoratorValue>{const invoke=invokers[index];return invoke(receiver,arguments);}`,
		entry: `import {Route,Call} from "./routes";
import go errors from "errors";
import go strconv from "strconv";
let trace:string="";
class Controller{
 constructor(public prefix:string){}
 @Route() public function greet(name:string):Result<string>{if(name=="blocked"){return fail(errors.New("blocked"));}return ok(this.prefix+name);}
 @Route() public function total(...numbers:int[]):int{let sum=0;for(const item of numbers){sum+=item;}return sum;}
 @Route() public function ping():void{trace+="ping;";}
 @Route() public function finish():Result<void>{trace+="finish;";return ok();}
}
class Base{
 @Route() public virtual function message():string{return "base";}
}
class Child extends Base{
 constructor(){super();}
 public override function message():string{return "child";}
}
export function Run():Result<string>{
 const controller=decoratorValue(new Controller("hello "));
 const greetingValue=Call(0,controller,[decoratorValue("world")])?;
 const greeting=decoratorValueAs<string>(greetingValue)?;
 const totalValue=Call(1,controller,[decoratorValue(2),decoratorValue(3)])?;
 const total=decoratorValueAs<int>(totalValue)?;
 const ping=Call(2,controller,[])?;
 const finished=Call(3,controller,[])?;
 const base:Base=new Child();
 const messageValue=Call(4,decoratorValue(base),[])?;
 const message=decoratorValueAs<string>(messageValue)?;
 return ok(greeting+";"+strconv.Itoa(total)+";"+trace+ping.typeIdentity+";"+finished.typeIdentity+";"+message);
}
export function WrongReceiver():Result<DecoratorValue>{return Call(0,decoratorValue("wrong"),[decoratorValue("name")]);}
export function WrongArity():Result<DecoratorValue>{return Call(0,decoratorValue(new Controller("x")),[]);}
export function WrongArgument():Result<DecoratorValue>{return Call(0,decoratorValue(new Controller("x")),[decoratorValue(1)]);}
export function PropagatedError():Result<DecoratorValue>{return Call(0,decoratorValue(new Controller("x")),[decoratorValue("blocked")]);}
`,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "decoratormethods")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, fragment := range []string{"Invocable: true", "typedReceiver.Greet(argument0)", "typedReceiver.Total(variadicArguments...)", "typedReceiver.Message()"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("missing %q in generated method adapter:\n%s", fragment, generated)
		}
	}
	reference := `package reference
func Run()(string,error){return "hello world;5;ping;finish;void;void;child",nil}
func WrongReceiver()error{return errorString("decorator method Controller.greet expects a non-null Controller receiver")}
func WrongArity()error{return errorString("decorator method Controller.greet expects 1 argument")}
func WrongArgument()error{return errorString("decorator method Controller.greet argument 0 expects string")}
func PropagatedError()error{return errorString("blocked")}
type errorString string
func(e errorString)Error()string{return string(e)}
`
	comparison := `package decoratormethods_test
import("testing";g "decorator-methods.test";r "decorator-methods.test/reference")
func TestMethods(t *testing.T){
 got,err:=g.Run();want,werr:=r.Run();if err!=nil||werr!=nil||got!=want{t.Fatalf("Run=%q,%v want %q,%v",got,err,want,werr)}
 _,err=g.WrongReceiver();if want:=r.WrongReceiver();err==nil||err.Error()!=want.Error(){t.Fatalf("WrongReceiver=%v want %v",err,want)}
 _,err=g.WrongArity();if want:=r.WrongArity();err==nil||err.Error()!=want.Error(){t.Fatalf("WrongArity=%v want %v",err,want)}
 _,err=g.WrongArgument();if want:=r.WrongArgument();err==nil||err.Error()!=want.Error(){t.Fatalf("WrongArgument=%v want %v",err,want)}
 _,err=g.PropagatedError();if want:=r.PropagatedError();err==nil||err.Error()!=want.Error(){t.Fatalf("PropagatedError=%v want %v",err,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-methods.test", generated, reference, comparison)
}

func TestDecoratorMethodAdapterAvailability(t *testing.T) {
	t.Parallel()
	entry := filepath.Join(t.TempDir(), "entry.km")
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: `
function D(context:MethodDecoratorContext):void{}
class Service{
 @D private function hidden():void{}
 @D public static function shared():void{}
 @D public function generic<T>(value:T):T{return value;}
 @D public function available():void{}
}
`})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	want := []string{"method is not public", "static method requires invokeStatic", "generic methods require concrete type arguments", ""}
	if len(checked.Program.Decorators) != len(want) {
		t.Fatalf("decorators=%d", len(checked.Program.Decorators))
	}
	for index, application := range checked.Program.Decorators {
		target := application.Target
		if target == nil || target.Invocable != (want[index] == "") || target.InvokeUnavailableReason != want[index] {
			t.Errorf("target %d=%+v, want reason %q", index, target, want[index])
		}
		if target != nil && (target.StaticInvocable != (index == 1) || (index == 1 && target.StaticInvokeUnavailableReason != "")) {
			t.Errorf("target %d static invocation=%+v", index, target)
		}
	}
}

func TestDecoratorStaticMethodAdaptersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "routes.km")
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		library: `alias Invoker=(arguments:DecoratorValue[])=>Result<DecoratorValue>;
alias InstanceInvoker=(receiver:DecoratorValue,arguments:DecoratorValue[])=>Result<DecoratorValue>;
let invokers:Invoker[]=[];
let instanceInvokers:InstanceInvoker[]=[];
export function Route():(context:MethodDecoratorContext)=>void{return (context)=>{invokers=append(invokers,context.invokeStatic);instanceInvokers=append(instanceInvokers,context.invoke);};}
export function Call(index:int,arguments:DecoratorValue[]):Result<DecoratorValue>{const invoke=invokers[index];return invoke(arguments);}
export function CallInstance(index:int,receiver:DecoratorValue,arguments:DecoratorValue[]):Result<DecoratorValue>{const invoke=instanceInvokers[index];return invoke(receiver,arguments);}`,
		entry: `import {Route,Call,CallInstance} from "./routes";
import go errors from "errors";
import go strconv from "strconv";
class Formatter{
 @Route() public static function greet(name:string):Result<string>{if(name=="blocked"){return fail(errors.New("blocked"));}return ok("hello "+name);}
 @Route() public static function sum(...values:int[]):int{let total=0;for(const value of values){total+=value;}return total;}
 @Route() public static function ping():void{}
 @Route() public static function finish():Result<void>{return ok();}
}
export function Run():Result<string>{
 const greetingValue=Call(0,[decoratorValue("world")])?;
 const greeting=decoratorValueAs<string>(greetingValue)?;
 const totalValue=Call(1,[decoratorValue(2),decoratorValue(3)])?;
 const total=decoratorValueAs<int>(totalValue)?;
 const ping=Call(2,[])?;
 const finish=Call(3,[])?;
 return ok(greeting+":"+strconv.Itoa(total)+":"+ping.typeIdentity+":"+finish.typeIdentity);
}
export function WrongArity():Result<DecoratorValue>{return Call(0,[]);}
export function WrongArgument():Result<DecoratorValue>{return Call(0,[decoratorValue(1)]);}
export function PropagatedError():Result<DecoratorValue>{return Call(0,[decoratorValue("blocked")]);}
export function WrongAdapter():Result<DecoratorValue>{return CallInstance(0,decoratorValue(1),[]);}
`,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "decoratorstaticmethods")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, fragment := range []string{"StaticInvocable: true", "FormatterGreet(argument0)", "FormatterSum(variadicArguments...)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("missing %q in generated static adapter:\n%s", fragment, generated)
		}
	}
	reference := `package reference
func Run()(string,error){return "hello world:5:void:void",nil}
func WrongArity()error{return errorString("decorator method Formatter.greet expects 1 argument")}
func WrongArgument()error{return errorString("decorator method Formatter.greet argument 0 expects string")}
func PropagatedError()error{return errorString("blocked")}
func WrongAdapter()error{return errorString("static method requires invokeStatic")}
type errorString string
func(e errorString)Error()string{return string(e)}
`
	comparison := `package decoratorstaticmethods_test
import("testing";g "decorator-static-methods.test";r "decorator-static-methods.test/reference")
func TestMethods(t *testing.T){
 got,err:=g.Run();want,werr:=r.Run();if err!=nil||werr!=nil||got!=want{t.Fatalf("Run=%q,%v want %q,%v",got,err,want,werr)}
 _,err=g.WrongArity();if want:=r.WrongArity();err==nil||err.Error()!=want.Error(){t.Fatalf("WrongArity=%v want %v",err,want)}
 _,err=g.WrongArgument();if want:=r.WrongArgument();err==nil||err.Error()!=want.Error(){t.Fatalf("WrongArgument=%v want %v",err,want)}
 _,err=g.PropagatedError();if want:=r.PropagatedError();err==nil||err.Error()!=want.Error(){t.Fatalf("PropagatedError=%v want %v",err,want)}
 _,err=g.WrongAdapter();if want:=r.WrongAdapter();err==nil||err.Error()!=want.Error(){t.Fatalf("WrongAdapter=%v want %v",err,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-static-methods.test", generated, reference, comparison)
}

func TestExternalDecoratorPackageIdentityAndInitialization(t *testing.T) {
	var previousGenerated []byte
	var previousIdentities []string
	for attempt := 0; attempt < 2; attempt++ {
		base := t.TempDir()
		root := filepath.Join(base, "app")
		files := map[string]string{
			"app/kinmokusei.toml": externalManifest("decorator-packages.test", false) + `[dependencies]
"pkg.test/library" = "v0.1.0"
[replace]
"pkg.test/library" = "../library"
"pkg.test/registry" = "../registry"
`,
			"registry/kinmokusei.toml": externalManifest("pkg.test/registry", true),
			"registry/index.km": `let trace:string="";
export function Mark(label:string):(context:ClassDecoratorContext)=>void{return (context)=>{trace+=label+";";};}
export function Trace():string{return trace;}
@Mark("registry") class Marker{}`,
			"library/kinmokusei.toml": externalManifest("pkg.test/library", true) + `[dependencies]
"pkg.test/registry" = "v0.1.0"
`,
			"library/index.km": `import {Mark,Trace} from "pkg.test/registry";
export {Mark,Trace};
@Mark("library") class Marker{}`,
			"app/main.km": `import {Mark,Trace} from "pkg.test/library";
@Mark("app") class Marker{}
export function Snapshot():string{return Trace();}`,
		}
		if attempt == 1 {
			files["app/kinmokusei.toml"] += "[imports]\n\"framework\" = \"pkg.test/library\"\n"
			files["app/main.km"] = strings.Replace(files["app/main.km"], `"pkg.test/library"`, `"framework"`, 1)
		}
		for name, contents := range files {
			path := filepath.Join(base, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := project.LockDependencies(root, true); err != nil {
			t.Fatal(err)
		}
		entry := filepath.Join(root, "main.km")
		checked, err := CheckFiles([]string{entry})
		if err != nil || len(checked.Diagnostics) != 0 {
			t.Fatalf("attempt %d err=%v diagnostics=%v", attempt, err, checked.Diagnostics)
		}
		identities := make([]string, 0, len(checked.Program.Decorators))
		for _, application := range checked.Program.Decorators {
			if application.Target == nil {
				t.Fatal("external decorator target was not checked")
			}
			identities = append(identities, application.Target.Identity)
		}
		if len(identities) != 3 || identities[0] == identities[1] || identities[1] == identities[2] || identities[0] == identities[2] {
			t.Fatalf("external decorator identities=%v", identities)
		}
		directory, diagnostics, err := WriteGeneratedModule([]string{entry}, "decoratorpackages")
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("attempt %d emit err=%v diagnostics=%v", attempt, err, diagnostics)
		}
		generated, err := os.ReadFile(filepath.Join(directory, "generated.go"))
		if err != nil {
			t.Fatal(err)
		}
		if attempt != 0 {
			if !bytes.Equal(generated, previousGenerated) {
				t.Fatal("decorator output depends on checkout path or import alias")
			}
			if strings.Join(identities, ",") != strings.Join(previousIdentities, ",") {
				t.Fatalf("decorator identities changed: first=%v second=%v", previousIdentities, identities)
			}
		}
		previousGenerated = append([]byte(nil), generated...)
		previousIdentities = append([]string(nil), identities...)
		reference := `package reference
func Snapshot()string{return "registry;library;app;"}
`
		comparison := `package decoratorpackages_test
import("testing";g "decorator-packages.test";r "decorator-packages.test/reference")
func TestRegistration(t *testing.T){if got,want:=g.Snapshot(),r.Snapshot();got!=want{t.Fatalf("got %q want %q",got,want)}}
`
		runGeneratedGoDifferentialTestInExistingModule(t, directory, "decorator-packages.test", generated, reference, comparison, []string{"test", "-mod=readonly", "./..."}, []string{"GOPROXY=off"})
	}
}
