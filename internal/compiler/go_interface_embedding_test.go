package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoInterfaceEmbeddingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module go-interface-embedding.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
import "io"
type Getter[T any] interface{Get()T}
type number int
func(n number)Get()int{return int(n)}
func NewGetter(n int)Getter[int]{return number(n)}
func Get[T any](v Getter[T])T{return v.Get()}
type Sealed interface{seal()int}
func(n number)seal()int{return int(n)}
func NewSealed(n int)Sealed{return number(n)}
func ReadSealed(s Sealed)int{return s.seal()}
type nilWriter struct{}
func(w *nilWriter)Write(p []byte)(int,error){if w==nil{return len(p)+70,nil};return len(p),nil}
func TypedNilWriter()io.Writer{return (*nilWriter)(nil)}
`,
		"entry.km": `import go bytes from "bytes";
import go io from "io";
import go model from "go-interface-embedding.test/contracts";
struct Forward{io.Writer;}
alias WriterAlias=io.Writer;
struct AliasForward{WriterAlias;}
struct Wrapped{Forward;}
struct Traced{
 io.Writer;
 public calls:int;
 public pointer function write(data:byte[]):(int,error){
  this.calls++;
  return this.Writer.Write(data);
 }
}
struct Number<T>{model.Getter<T>;}
struct Seal{model.Sealed;}
struct Failure{error;}
function send(w:io.Writer,p:byte[]):int{
 const [n,err]=w.Write(p);
 if(err!=nil){return -1;}
 return n;
}
export function Run(p:byte[]):int[]{
 let first=bytes.Buffer{};
 let second=bytes.Buffer{};
 let f=Forward{Writer:&first};
 const bound=f.Write;
 const write=Forward.Write;
 f.Writer=&second;
 const [a,_]=bound(p);
 const [b,_]=write(f,p);
 const c=send(Wrapped{Forward:f},p);
 let traced=Traced{Writer:&first,calls:0};
 const d=send(&traced,p);
 const e=send(AliasForward{WriterAlias:&second},p);
 return [a,b,c,d,e,traced.calls,first.Len(),second.Len()];
}
export function Values(n:int):int[]{
 const getter=Number<int>{Getter:model.NewGetter(n)};
 const sealed:model.Sealed=Seal{Sealed:model.NewSealed(n+1)};
 return [getter.Get(),model.Get(getter),model.ReadSealed(sealed)];
}
export function ForwardError(e:error):error{return Failure{error:e};}
export function NilCall():int{return send(Forward{Writer:nil},[1]);}
export function TypedNil(p:byte[]):int {
 const f=Forward{Writer:model.TypedNilWriter()};
 return send(f,p);
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "interfaceembedding")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	for _, failure := range []struct{ name, definition, want string }{
		{"open-generic", `struct Invalid{model.Getter;}`, "requires 1 type arguments"},
		{"unrelated-private", `struct Invalid{private function seal():int{return 1;}} function use(v:Invalid):model.Sealed{return v;}`, "cannot use Invalid"},
		{"conflicting-private", `struct A{model.Sealed;}struct B{model.Sealed;}struct Invalid{A;B;}function use(v:Invalid):model.Sealed{return v;}`, "cannot use Invalid"},
		{"private-method-access", `struct Wrapped{model.Sealed;}function use(w:Wrapped):int{return w.seal();}`, "has no field"},
	} {
		t.Run(failure.name, func(t *testing.T) {
			path := filepath.Join(root, failure.name+".km")
			input := `import go model from "go-interface-embedding.test/contracts";` + failure.definition
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitGo([]string{path}, "interfaceembedding")
			if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, failure.want) {
				t.Fatalf("diagnostics: %v %v", err, diagnostics)
			}
		})
	}
	reference := `package reference
import("bytes";"io")
type forward struct{io.Writer}
type writerAlias=io.Writer
type aliasForward struct{writerAlias}
type wrapped struct{forward}
type traced struct{io.Writer;calls int}
func(t *traced)Write(p []byte)(int,error){t.calls++;return t.Writer.Write(p)}
func send(w io.Writer,p []byte)int{n,err:=w.Write(p);if err!=nil{return -1};return n}
func Run(p []byte)[]int{
 var first,second bytes.Buffer
 f:=forward{&first};bound,write:=f.Write,forward.Write;f.Writer=&second
 a,_:=bound(p);b,_:=write(f,p);c:=send(wrapped{f},p);t:=traced{Writer:&first}
 d:=send(&t,p);e:=send(aliasForward{&second},p)
 return []int{a,b,c,d,e,t.calls,first.Len(),second.Len()}
}
type getter[T any]interface{Get()T}
type number int
func(n number)Get()int{return int(n)}
func(n number)seal()int{return int(n)}
type viewed[T any]struct{getter[T]}
type sealed interface{seal()int}
type seal struct{sealed}
func get[T any](v getter[T])T{return v.Get()}
func readSeal(s sealed)int{return s.seal()}
func Values(n int)[]int{g:=viewed[int]{number(n)};var s sealed=seal{number(n+1)};return []int{g.Get(),get(g),readSeal(s)}}
type failure struct{error}
func ForwardError(e error)error{return failure{e}}
func NilCall()int{return send(forward{},[]byte{1})}
type nilWriter struct{}
func(w *nilWriter)Write(p []byte)(int,error){if w==nil{return len(p)+70,nil};return len(p),nil}
func TypedNil(p []byte)int{return send(forward{(*nilWriter)(nil)},p)}
`
	comparison := `package interfaceembedding_test
import("errors";"reflect";"testing";g "go-interface-embedding.test";r "go-interface-embedding.test/reference")
func panics(f func()int)(did bool){defer func(){did=recover()!=nil}();f();return}
func TestInterfaceEmbedding(t *testing.T){
 for _,p:=range [][]byte{nil,{}, {2}, {1,2,3}}{
  if got,want:=g.Run(p),r.Run(p);!reflect.DeepEqual(got,want){t.Fatal(p,got,want)}
  if g.TypedNil(p)!=r.TypedNil(p){t.Fatal("typed nil",p)}
 }
 for _,n:=range []int{-8,0,11}{if got,want:=g.Values(n),r.Values(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}}
 e:=errors.New("delegated");if g.ForwardError(e).Error()!=r.ForwardError(e).Error(){t.Fatal("error embedding")}
 if !panics(g.NilCall)||!panics(r.NilCall){t.Fatal("nil interface dispatch")}
}
`
	runGeneratedGoDifferentialTest(t, root, "go-interface-embedding.test", generated, reference, comparison)
}
