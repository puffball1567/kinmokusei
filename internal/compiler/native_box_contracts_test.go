package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeGenericBoxRetainsSourceContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "boxes.km")
	input := `import go errors from "errors";
class Base{}
class Box<T> extends Base{constructor(public value:T){}}
function box<T>(value:T):Base{return new Box<T>(value);}
function match<T>(value:Base):boolean{const [_,matched]=value as? Box<T>;return matched;}
function unbox<T>(value:Base):Result<T>{const [typed,matched]=value as? Box<T>;if(!matched){return fail(errors.New("contract mismatch"));}return ok(typed.value);}
function forward<T>(value:T):Base{const create=box[T];return create(value);}
function wrap<T>(value:T):Base{return box<T[]>([value]);}
function first<T>(value:T,_:T):Base{return box(value);}
function pair():(bstring,bstring){return b"\xff",b"raw";}
function variadic<T>(...values:T[]):Base{return box(values[0]);}
class Factory<T>{
 constructor(public value:T){}
 public function store():Base{return forward(this.value);}
 public function constant():int{return 7;}
 public function other<U>(value:U):Base{return box(value);}
 public static function create(value:T):Base{return box(value);}
}
class Child<U> extends Box<U>{constructor(value:U){super(value);}}
struct Holder<T>{public value:T;
 private function map():Base{return box(this.value);}
 public function store():Base{return this.map();}
 public function other<U>(value:U):Base{return box(value);}
 public pointer function update(value:T):Base{this.value=value;return box(this.value);}
}
struct External<T>{public value:T;}
public function store<U>(this:External<U>):Base{return box(this.value);}
type Values<T>=distinct T[];
public function store<U>(this:Values<U>):Base{return box(this[0]);}
class Item{public text:string="safe";}
export function Scalar():boolean[]{
 const verified=box<string>("日本語");const raw=box<bstring>(b"\xff");
 return [match<string>(verified),match<bstring>(raw),match<string>(raw),match<bstring>(verified)];
}
export function Collections():boolean[]{
 const verified=box<string[]>(["safe"]);const raw=box<bstring[]>([b"\xff"]);
 const nested=box<string[][]>([["safe"]]);
 return [match<string[]>(verified),match<bstring[]>(raw),match<string[]>(raw),match<bstring[]>(verified),match<bstring[][]>(nested)];
}
export function Nullable():boolean[]{
 const present=box<Item>(new Item());const missing=box<Item|null>(null);
 return [match<Item>(present),match<Item|null>(missing),match<Item|null>(present),match<Item>(missing)];
}
export function Nominal():boolean[]{const value=box<int>(42);return [match<int>(value),match<int32>(value)];}
export function HasVerified(value:Base):boolean{return match<string>(value);}
export function HasRaw(value:Base):boolean{return match<bstring>(value);}
export function Results():boolean{
 const [text,textError]=unbox<string>(box<string>("日本語"));
 if(textError!=nil||text!="日本語"){return false;}
 const stored=box<bstring>(b"\xff");
 const [raw,rawError]=unbox<bstring>(stored);
 if(rawError!=nil||raw!=b"\xff"){return false;}
 const [_,contractError]=unbox<string>(stored);
 return contractError!=nil;
}
export function Paths():boolean[]{
 const factory=new Factory<bstring>(b"\xff");
 const holder=Holder<bstring>{value:b"\xff"};
 const method=holder.store;
 const expression=Holder<bstring>.store;
 const external=External<bstring>{value:b"\xff"};
 const renamed=external.store;
 const values=Values<bstring>([b"\xff"]);
 const child:Base=new Child<bstring>(b"\xff");
 return [match<bstring>(factory.store()),match<string>(factory.store()),
 match<bstring>(factory.other(b"\xff")),match<string>(Factory.create<bstring>(b"\xff")),
 match<bstring>(method()),match<string>(method()),match<bstring>(expression(holder)),
 match<bstring>(holder.other(b"\xff")),match<string>(holder.other(b"\xff")),
 match<bstring>(holder.update(b"\xfe")),match<bstring>(renamed()),match<string>(renamed()),
 match<bstring>(values.store()),match<string>(values.store()),
 match<bstring>(first(pair())),match<string>(first(pair())),
 match<bstring>(variadic<bstring>([b"\xff"]...)),match<string>(variadic<bstring>([b"\xff"]...)),
 match<bstring>(child),match<string>(child),
 match<bstring[]>(wrap(b"\xff")),match<string[]>(wrap(b"\xff"))];
}
export function Forced():string{return (box<bstring>(b"\xff") as! Box<string>).value;}
export function RoundTrip():string{const [value,matched]=box<string>("safe") as? Box<string>;if(!matched){return "mismatch";}return value.value;}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "nativeboxes")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "nativeboxes")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("mapped emission changed source contracts: %v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "native-box-contracts.test", generated,
		`package reference
// Distinct phantom contract arguments model source distinctions independently
// of the generated Go projection interfaces.
type verified struct{}
type raw struct{}
type base struct{root any}
type box[T,W any]struct{value T}
func store[T,W any](v T)base{return base{&box[T,W]{v}}}
func match[T,W any](v base)bool{_,ok:=v.root.(*box[T,W]);return ok}
func Scalar()[]bool{a:=store[string,verified]("日本語");b:=store[string,raw]("\xff");return []bool{match[string,verified](a),match[string,raw](b),match[string,verified](b),match[string,raw](a)}}
func Collections()[]bool{a:=store[[]string,[]verified]([]string{"safe"});b:=store[[]string,[]raw]([]string{"\xff"});n:=store[[][]string,[][]verified]([][]string{{"safe"}});return []bool{match[[]string,[]verified](a),match[[]string,[]raw](b),match[[]string,[]verified](b),match[[]string,[]raw](a),match[[][]string,[][]raw](n)}}
func Nullable()[]bool{return []bool{true,true,false,false}}
func Nominal()[]bool{v:=store[int,int](42);return []bool{match[int,int](v),match[int32,int32](v)}}
func Results()bool{text:=store[string,verified]("日本語");stored:=store[string,raw]("\xff");return match[string,verified](text)&&match[string,raw](stored)&&!match[string,verified](stored)}
func Paths()[]bool{v:=store[string,raw]("\xff");same,different:=match[string,raw](v),match[string,verified](v);return []bool{same,different,same,different,same,different,same,same,different,same,same,different,same,different,same,different,same,different,same,different,same,different}}`,
		`package nativeboxes_test
import("reflect";"testing";g "native-box-contracts.test";r "native-box-contracts.test/reference")
func TestScalar(t *testing.T){a,b:=g.Scalar(),r.Scalar();if !reflect.DeepEqual(a,b){t.Fatalf("scalar: got %v want %v",a,b)};if g.RoundTrip()!="safe"{t.Fatal("same-contract round trip")}}
func TestCollections(t *testing.T){a,b:=g.Collections(),r.Collections();if !reflect.DeepEqual(a,b){t.Fatalf("collections: got %v want %v",a,b)}}
func TestNullable(t *testing.T){a,b:=g.Nullable(),r.Nullable();if !reflect.DeepEqual(a,b){t.Fatalf("nullable: got %v want %v",a,b)}}
func TestNominal(t *testing.T){a,b:=g.Nominal(),r.Nominal();if !reflect.DeepEqual(a,b){t.Fatalf("nominal: got %v want %v",a,b)}}
func TestPaths(t *testing.T){a,b:=g.Paths(),r.Paths();if !reflect.DeepEqual(a,b){t.Fatalf("forwarding: got %v want %v",a,b)}}
func TestResults(t *testing.T){a,b:=g.Results(),r.Results();if a!=b{t.Fatalf("Result boxing: got %v want %v",a,b)}}
func TestUnknownGoProof(t *testing.T){
 var receiver *g.Factory[string]
 if receiver.Constant()!=7{t.Fatal("contract bookkeeping changed nil receiver behavior")}
 for _,value:=range []string{"valid","\xff"}{
  created:=g.NewBox(value);base:=g.UpcastBoxToBase(created)
  if g.HasVerified(base)||g.HasRaw(base){t.Fatal("Go storage acquired an unproven native contract")}
  restored,ok:=g.DowncastBaseToBox[string](base)
  if !ok||restored!=created{t.Fatal("public Go projection API changed")}
 }
}
func TestForced(t *testing.T){defer func(){if recover()==nil{t.Error("forced raw-to-verified cast did not panic")}}();_=g.Forced()}`)
}
