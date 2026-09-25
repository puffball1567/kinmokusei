package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecoratorTypeIdentitiesUseResolvedContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := `function D(ctx:DecoratorContext):void{}
class Service{}
alias Alias=Service;
alias Again=Alias;
interface Reader<T>{function read():T;}
alias IntReader=Reader<int>;
class Box<T>{constructor(public value:T){}}
alias Wrapped<T>=Box<T>;
struct Record<T>{value:T;}
alias IntRecord=Record<int>;
class Consumer{
 @D private direct:Service=new Service();
 @D private alias:Again=new Service();
 @D private optional:Alias|null=null;
 @D private integer:Box<int>=new Box<int>(1);
 @D private aliasInteger:Wrapped<int>=new Box<int>(1);
 @D private text:Box<string>=new Box<string>("x");
 @D private nested:Box<Service>=new Box<Service>(new Service());
 @D private nestedNullable:Box<Service|null>=new Box<Service|null>(null);
 constructor(@D a:Service,@D b:Again,@D c:Reader<int>,@D d:IntReader,@D e:Reader<string>,@D f:Record<int>,@D g:IntRecord,@D h:Record<string>){}
 public function use(@D value:Again,@D optional:Service|null,@D list:Service[],@D ...rest:Service[]):void{}
 public set property(@D value:Again){}
}
class Generic<T>{
 @D private value:Box<T>;
 constructor(@D value:Box<T>){this.value=value;}
 public function method<U>(@D value:Box<U>):void{}
}
`
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(entry, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	checked, err := CheckFiles([]string{entry})
	if err != nil || len(checked.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
	}
	identities := map[string]string{}
	for _, application := range checked.Program.Decorators {
		target := application.Target
		identities[target.ClassName+"."+target.MemberName+"."+target.ParameterName] = target.ValueIdentity
	}
	for _, pair := range [][2]string{
		{"Consumer.direct.", "Consumer.alias."},
		{"Consumer.direct.", "Consumer.optional."},
		{"Consumer.integer.", "Consumer.aliasInteger."},
		{"Consumer.constructor.a", "Consumer.constructor.b"},
		{"Consumer.constructor.c", "Consumer.constructor.d"},
		{"Consumer.constructor.f", "Consumer.constructor.g"},
		{"Consumer.direct.", "Consumer.use.value"},
		{"Consumer.direct.", "Consumer.use.optional"},
		{"Consumer.direct.", "Consumer.property.value"},
	} {
		if a, b := identities[pair[0]], identities[pair[1]]; a == "" || a != b {
			t.Errorf("equivalent identities %s=%q %s=%q", pair[0], a, pair[1], b)
		}
	}
	for _, pair := range [][2]string{
		{"Consumer.integer.", "Consumer.text."},
		{"Consumer.constructor.c", "Consumer.constructor.e"},
		{"Consumer.constructor.f", "Consumer.constructor.h"},
		{"Consumer.nested.", "Consumer.nestedNullable."},
	} {
		if a, b := identities[pair[0]], identities[pair[1]]; a == "" || b == "" || a == b {
			t.Errorf("distinct identities %s=%q %s=%q", pair[0], a, pair[1], b)
		}
	}
	for _, key := range []string{"Generic.value.", "Generic.constructor.value", "Generic.method.value", "Consumer.use.list", "Consumer.use.rest"} {
		if got, exists := identities[key]; !exists || got != "" {
			t.Errorf("non-concrete/non-nominal identity %s=%q exists=%v", key, got, exists)
		}
	}
}

func TestDecoratorLinkedTypeIdentitiesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, source := range map[string]string{
		"registry.km": `const ids=makeMap<string,string>();
export function D(ctx:DecoratorContext):void{ids[ctx.memberName+"."+ctx.parameterName]=ctx.valueIdentity;}
export function ID(key:string):string{return ids[key];}
`,
		"one.km": `export class Service{}
export alias Alias=Service;
export class Box<T>{constructor(public value:T){}}
export alias Wrapped<T>=Box<T>;
`,
		"two.km": `export class Service{}
export class Box<T>{constructor(public value:T){}}
`,
		"bridge.km": `export {Alias as Provider,Wrapped as WrappedBox} from "./one";
export {Service as OtherService,Box as OtherBox} from "./two";`,
		"entry.km": `import {D,ID} from "./registry";
import {Service,Box} from "./one";
import {Provider,WrappedBox,OtherService,OtherBox} from "./bridge";
class Consumer{
 @D private first:Service=new Service();
 @D private same:Provider=new Service();
 @D private other:OtherService=new OtherService();
 @D private integer:Box<int>=new Box<int>(1);
 @D private aliasInteger:WrappedBox<int>=new Box<int>(1);
 @D private text:Box<string>=new Box<string>("x");
 @D private otherInteger:OtherBox<int>=new OtherBox<int>(1);
}
export function Snapshot():boolean[]{return [ID("first.")!="",ID("first.")==ID("same."),ID("first.")!=ID("other."),ID("integer.")!="",ID("integer.")==ID("aliasInteger."),ID("integer.")!=ID("text."),ID("integer.")!=ID("otherInteger.")];}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "identities")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "reflect"
type service struct{}
type provider=service
type otherService struct{}
type box[T any]struct{value T}
type integerBox=box[int]
type otherBox[T any]struct{value T}
func Snapshot()[]bool{return []bool{reflect.TypeFor[service]() != nil,reflect.TypeFor[service]()==reflect.TypeFor[provider](),reflect.TypeFor[service]()!=reflect.TypeFor[otherService](),reflect.TypeFor[box[int]]()!=nil,reflect.TypeFor[box[int]]()==reflect.TypeFor[integerBox](),reflect.TypeFor[box[int]]()!=reflect.TypeFor[box[string]](),reflect.TypeFor[box[int]]()!=reflect.TypeFor[otherBox[int]]()}}
`
	comparison := `package identities_test
import("testing";"reflect";g "decorator-type-identities.test";r "decorator-type-identities.test/reference")
func TestIdentities(t *testing.T){if got,want:=g.Snapshot(),r.Snapshot();!reflect.DeepEqual(got,want){t.Fatalf("got=%v want=%v",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-type-identities.test", generated, reference, comparison)
}
