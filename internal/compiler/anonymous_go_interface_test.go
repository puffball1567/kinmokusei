package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnonymousGoInterfaceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module anonymous-go-interface.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
import "fmt"
type Token int
type Reader = interface{Read()Token}
type Wrapper[T any] interface{Get()interface{Read()T}}
type Value struct{ N int }
func(v Value)Read()Token{return Token(v.N)}
func(v Value)Pair()(Token,string){return Token(v.N),fmt.Sprint(v.N)}
func(v Value)All(values ...Token)[]Token{return append([]Token{Token(v.N)},values...)}
func(v Value)String()string{return fmt.Sprint(v.N)}
func New(n int)interface{Read()Token;Pair()(Token,string);All(...Token)[]Token}{return Value{n}}
func Named(n int)interface{fmt.Stringer}{return Value{n}}
func Use(v interface{Read()Token})int{return int(v.Read())}
func Identity[T any](v T)T{return v}
func Values(n int)[]interface{Read()Token}{return []interface{Read()Token}{Value{n}}}
func Object(n int)struct{Value interface{Read()Token}}{return struct{Value interface{Read()Token}}{Value{n}}}
func Apply(n int, cb func(interface{Read()Token})int)int{return cb(Value{n})}
func Any(v interface{})interface{}{return v}
`,
		"base.km": `import go c from "anonymous-go-interface.test/contracts";
import go fmt from "fmt";
interface Named extends fmt.Stringer{}
interface W<T> extends c.Wrapper<T>{}
function wrapped<T>(v:W<T>):T{const r=v.Get();return r.Read();}
class Label implements Named{constructor(private n:int){}public function string():string{return fmt.Sprint(this.n);}}
function label(n:int):Named{return new Label(n);}
function empty(n:int):boolean{const v=c.Any(n);return v!==nil;}
`,
		"entry.km": `import {label,empty,W,wrapped} from "./base";
import go api from "anonymous-go-interface.test/contracts";
import go f from "fmt";
function Read(n:int):int{const v=api.New(n);const read=v.Read;return int(read());}
function Pair(n:int):string{const v=api.New(n);const pair=v.Pair;const [a,b]=pair();return f.Sprint(a)+":"+b;}
function All(n:int):int{const v=api.New(n);const all=v.All;return len(all(api.Token(1),api.Token(2)));}
function Pass(n:int):int{const v=api.Identity(api.New(n));return api.Use(v);}
function Collection(n:int):int{const xs=api.Values(n);const obj=api.Object(n);return int(xs[0].Read())+int(obj.Value.Read());}
function Callback(n:int):int{return api.Apply(n,(v:api.Reader):int=>int(v.Read())+1);}
function Named(n:int):string{const v=api.Named(n);const named:f.Stringer=v;return named.String();}
function Empty(n:int):boolean{const v=api.Any(label(n));return v!==nil&&empty(n);}
class Leaf{constructor(public value:int){}}
function Wrapped(v:W<int>):int{return wrapped(v);}
function ReadLeaf(v:W<Leaf|null>):int{const r=v.Get();const leaf=r.Read();if(leaf===null){return -1;}return leaf.value;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "anonymous")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "fmt"
func Read(n int)int{return n}
func Pair(n int)string{return fmt.Sprint(n)+":"+fmt.Sprint(n)}
func All(n int)int{return len([]int{n,1,2})}
func Pass(n int)int{return n}
func Collection(n int)int{return n+n}
func Callback(n int)int{return n+1}
func Named(n int)string{return fmt.Sprint(n)}
func Empty(n int)bool{return interface{}(n)!=nil}
func Wrapped(n int)int{return n}
func ReadLeaf(present bool,n int)int{if !present{return -1};return n}
`
	comparison := `package anonymous_test
import("testing";g "anonymous-go-interface.test";r "anonymous-go-interface.test/reference")
type reader[T any]struct{value T}
func(v reader[T])Read()T{return v.value}
type wrapper[T any]struct{value T}
func(v wrapper[T])Get()interface{Read()T}{return reader[T]{v.value}}
func TestContracts(t *testing.T){for _,n:=range []int{-8,0,17,1024}{
if g.Read(n)!=r.Read(n)||g.Pass(n)!=r.Pass(n)||g.Collection(n)!=r.Collection(n)||g.Callback(n)!=r.Callback(n){t.Fatal("read/parameter/collection/callback",n)}
if g.Pair(n)!=r.Pair(n)||g.All(n)!=r.All(n)||g.Named(n)!=r.Named(n)||g.Empty(n)!=r.Empty(n){t.Fatal("method values/results/variadic/identity",n)}
if g.Wrapped(wrapper[int]{n})!=r.Wrapped(n){t.Fatal("generic nested interface")}
for _,present:=range []bool{false,true}{var leaf *g.Leaf;if present{leaf=g.NewLeaf(n)};if g.ReadLeaf(wrapper[*g.Leaf]{leaf})!=r.ReadLeaf(present,n){t.Fatal("nullable class result")}}
}}
`
	runGeneratedGoDifferentialTest(t, root, "anonymous-go-interface.test", generated, reference, comparison)
}
