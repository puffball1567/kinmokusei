package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkedMethodContractDiagnostics(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"result", `class Bad implements Reader<Leaf>{public function read():Maybe{return null;}}`},
		{"parameter", `class Bad implements Writer<Maybe>{public function write(value:Leaf):void{}}`},
		{"override", `class Bad extends Base{public override function read():Maybe{return null;}}`},
		{"diamond", `interface A extends Reader<Leaf>{} interface B extends Reader<Maybe>{} interface Bad extends A,B{}`},
		{"nested callback", `interface Apply{function run(callback:(value:Maybe)=>void):void;} class Bad implements Apply{public function run(callback:(value:Leaf)=>void):void{}}`},
		{"generic implementation", `class Bad implements Reader<Leaf>{public function read<T>():Leaf{return new Leaf(1);}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			files := map[string]string{
				"types.km":  `class Leaf{constructor(public value:int){}} alias Maybe=Leaf|null; interface Reader<T>{function read():T;} interface Writer<T>{function write(value:T):void;} class Base{public virtual function read():Leaf{return new Leaf(1);}}`,
				"bridge.km": `export {Leaf,Maybe,Reader,Writer,Base} from "./types";`,
				"entry.km":  `import {Leaf,Maybe,Reader,Writer,Base} from "./bridge";` + test.source,
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "contracts")
			found := false
			for _, diagnostic := range diagnostics {
				found = found || strings.Contains(diagnostic.Message, "incompatible signature")
			}
			if err != nil || !found || len(generated) != 0 {
				t.Fatalf("err=%v diagnostics=%v generated=%s", err, diagnostics, generated)
			}
		})
	}
}

func TestNullableMethodContractsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"go.mod":                 "module method-contracts.test\n\ngo 1.23\n",
		"contracts/contracts.go": "package contracts\ntype Loader[E any] interface{Load()(E,error)}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	input := `import go fmt from "fmt";
import go c from "method-contracts.test/contracts";
class Leaf{constructor(public value:int){}}
alias Maybe=Leaf|null;
interface Reader<T>{function read():T;}
interface Writer<T>{function write(value:T):void;}
interface Left extends Reader<Maybe>{}
interface Right extends Reader<Maybe>{}
interface Store extends Left,Right,Writer<Maybe>{function snapshot():{value:Maybe};function visit(callback:(value:Maybe)=>void):void;function load():Result<Maybe>;}
interface GenericLoader<E> extends c.Loader<E>{}
interface NativeLoader extends GenericLoader<Maybe>{}
class Base implements Store{constructor(protected value:Maybe){}public virtual function read():Maybe{return this.value;}public function write(value:Maybe):void{this.value=value;}public function snapshot():{value:Maybe}{return {value:this.read()};}public function visit(callback:(value:Maybe)=>void):void{callback(this.read());}public function load():Result<Maybe>{return ok(this.read());}}
class Derived extends Base implements NativeLoader{constructor(value:Maybe){super(value);}public override function read():Maybe{return super.read();}}
function Use(value:int,missing:boolean):Result<int>{let input:Maybe=new Leaf(value);if(missing){input=null;}const store:Store=new Derived(input);const read=store.read;const first=read();let total=0;if(first!==null){total+=first.value;}store.visit((leaf:Maybe):void=>{if(leaf!==null){total+=leaf.value;}});const snapshot=store.snapshot();const current=snapshot.value;if(current!==null){total+=current.value;}const loaded=store.load()?;if(loaded!==null){total+=loaded.value;}store.write(null);if(store.read()===null){total+=1;}return ok(total);}
interface Printed extends fmt.Stringer{function string():string;}
class Label implements Printed{public function string():string{return "label";}}
function Names():string{const value:Printed=new Label();return value.string()+value.String();}
function Loaded(value:int,missing:boolean):Result<int>{let input:Maybe=new Leaf(value);if(missing){input=null;}const loader:NativeLoader=new Derived(input);const [loaded,problem]=loader.Load();if(problem!==nil){return fail(problem);}if(loaded===null){return ok(-1);}return ok(loaded.value);}
`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "methodcontracts")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Leaf struct{value int}
type Reader[T any]interface{Read()T}
type Writer[T any]interface{Write(T)}
type Left interface{Reader[*Leaf]}
type Right interface{Reader[*Leaf]}
type Snapshot struct{value *Leaf}
type Store interface{Left;Right;Writer[*Leaf];Snapshot()Snapshot;Visit(func(*Leaf));Load()(*Leaf,error)}
type Base struct{value *Leaf}
func(b *Base)Read()*Leaf{return b.value}
func(b *Base)Write(value *Leaf){b.value=value}
func(b *Base)Snapshot()Snapshot{return Snapshot{b.Read()}}
func(b *Base)Visit(callback func(*Leaf)){callback(b.Read())}
func(b *Base)Load()(*Leaf,error){return b.Read(),nil}
type Derived struct{Base}
func(d *Derived)Read()*Leaf{return d.Base.Read()}
func Use(value int,missing bool)(int,error){var input *Leaf=&Leaf{value};if missing{input=nil};var store Store=&Derived{Base{input}};read:=store.Read;first:=read();total:=0;if first!=nil{total+=first.value};store.Visit(func(leaf *Leaf){if leaf!=nil{total+=leaf.value}});snapshot:=store.Snapshot();current:=snapshot.value;if current!=nil{total+=current.value};loaded,err:=store.Load();if err!=nil{return 0,err};if loaded!=nil{total+=loaded.value};store.Write(nil);if store.Read()==nil{total++};return total,nil}
type Printed interface{String()string}
type Label struct{}
func(Label)String()string{return "label"}
func Names()string{var value Printed=Label{};return value.String()+value.String()}
type NativeLoader interface{Load()(*Leaf,error)}
func Loaded(value int,missing bool)(int,error){var input *Leaf=&Leaf{value};if missing{input=nil};var loader NativeLoader=&Derived{Base{input}};loaded,err:=loader.Load();if err!=nil{return 0,err};if loaded==nil{return -1,nil};return loaded.value,nil}
`
	comparison := `package methodcontracts_test
import("testing";generated "method-contracts.test";reference "method-contracts.test/reference")
func TestBehavior(t *testing.T){for _,value:=range []int{-7,0,12}{for _,missing:=range []bool{false,true}{got,ge:=generated.Use(value,missing);want,we:=reference.Use(value,missing);if got!=want||(ge==nil)!=(we==nil){t.Errorf("value=%d missing=%v got=(%d,%v) want=(%d,%v)",value,missing,got,ge,want,we)}}};if got,want:=generated.Names(),reference.Names();got!=want{t.Errorf("names=%q want=%q",got,want)}}
func TestLoader(t *testing.T){for _,value:=range []int{-7,0,12}{for _,missing:=range []bool{false,true}{got,ge:=generated.Loaded(value,missing);want,we:=reference.Loaded(value,missing);if got!=want||(ge==nil)!=(we==nil){t.Errorf("loaded=(%d,%v) want=(%d,%v)",got,ge,want,we)}}}}
`
	runGeneratedGoDifferentialTest(t, root, "method-contracts.test", generated, reference, comparison)
}
