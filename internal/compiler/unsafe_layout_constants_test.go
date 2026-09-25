package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestUnsafeLayoutConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	module := "module unsafe-layout-constants.test\n\ngo 1.23\n\nrequire example.com/layout v0.0.0\n\nreplace example.com/layout => ./layout\n"
	files := map[string]string{
		"go.mod":          module,
		"kinmokusei.toml": "[project]\nname=\"unsafe-layout-constants\"\nversion=\"0.1.0\"\ngo-module=\"unsafe-layout-constants.test\"\ngo-version=\"1.23\"\n[go.interop]\nunsafe=\"allow\"\n",
		"layout/go.mod":   "module example.com/layout\n\ngo 1.23\n",
		"layout/layout.go": `package layout
type Inner struct{Pad byte; Value int64}
type Outer struct{Lead byte; Inner; Tail byte}
type ViaPointer struct{*Inner}
type Generic[T any] struct{Pad byte; Value T}
`,
		"sizes.km": `import go {Sizeof as size} from "unsafe";
export const Width=int(size(int(0)));
export const Header=int(size(""));`,
		"bridge.km": `export {Width as Word,Header} from "./sizes";`,
		"entry.km": `import {Word,Header} from "./bridge";
import go u from "unsafe";
import go m from "example.com/layout";
let packet:m.Outer=m.Outer{};
export const Promoted=int(u.Offsetof(packet.Value));
export const Alignment=int(u.Alignof(packet.Value));
struct Box<T>{public value:T;}
class Item{constructor(public value:int){}}
class Limits{public static const word:int=Word;}
function measure<T>(x:T):int{const size=u.Sizeof(x);const p=&size;return int(*p);}
function genericStruct<T>(x:m.Generic<T>):int{const offset=u.Offsetof(x.Value);const p=&offset;return int(*p);}
function genericArray<T>(x:[2]T):int{const size=u.Sizeof(x);const p=&size;return int(*p);}
function genericSlice<T>(x:T[]):int{const size=u.Sizeof(x);return int(size);}
export function Layout():int[]{return [Word,Header,Promoted,Alignment,int(u.Sizeof(packet)),Limits.word];}
export function Ignored():int[]{let calls=0;const get=():*m.Outer=>{calls++;return nil;};const x=u.Sizeof(get().Value);const y=u.Alignof(get().Value);const z=u.Offsetof(get().Value);const channel=goChannel<int>();const received=u.Sizeof(<-channel);const array:[1][2]int=[[1,2]];let index=0;const n=len(array[index+int(u.Sizeof(get().Value))-8]);const p=&n;return [calls,int(x),int(y),int(z),int(received),*p];}
export function Generics():int[]{const box:Box<int64>=Box<int64>{value:1};const native:m.Generic<int64>=m.Generic<int64>{Pad:1,Value:2};const xs:int64[]=[1,2];const array:[2]int64=[1,2];return [measure(int64(1)),measure(box),measure(new Item(3)),genericStruct(native),genericArray(array),genericSlice(xs)];}
export function Mutable():int{let n=u.Sizeof(int64(0));const copy=n;const p=&copy;return int(*p);}
`,
	}
	for name, input := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.AddDependency(root, "example.com/layout", "v0.0.0", "./layout", true); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "unsafelayout")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const Promoted", "const Alignment", "const size = ", "var size = ", "var offset = "} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import("unsafe";m "example.com/layout")
var packet m.Outer
const Word=int(unsafe.Sizeof(int(0)))
const Header=int(unsafe.Sizeof(""))
const Promoted=int(unsafe.Offsetof(packet.Value))
const Alignment=int(unsafe.Alignof(packet.Value))
type box[T any] struct{value T}
type item struct{value int}
func measure[T any](x T)int{size:=unsafe.Sizeof(x);p:=&size;return int(*p)}
func genericStruct[T any](x m.Generic[T])int{offset:=unsafe.Offsetof(x.Value);p:=&offset;return int(*p)}
func genericArray[T any](x [2]T)int{size:=unsafe.Sizeof(x);p:=&size;return int(*p)}
func genericSlice[T any](x []T)int{const size=unsafe.Sizeof(x);return int(size)}
func Layout()[]int{return []int{Word,Header,Promoted,Alignment,int(unsafe.Sizeof(packet)),Word}}
func Ignored()[]int{calls:=0;get:=func()*m.Outer{calls++;return nil};const x=unsafe.Sizeof(get().Value);const y=unsafe.Alignof(get().Value);const z=unsafe.Offsetof(get().Value);channel:=make(chan int);const received=unsafe.Sizeof(<-channel);array:=[1][2]int{{1,2}};index:=0;n:=len(array[index+int(unsafe.Sizeof(get().Value))-8]);p:=&n;return []int{calls,int(x),int(y),int(z),int(received),*p}}
func Generics()[]int{b:=box[int64]{1};native:=m.Generic[int64]{Pad:1,Value:2};xs:=[]int64{1,2};return []int{measure(int64(1)),measure(b),measure(&item{3}),genericStruct(native),genericArray([2]int64{1,2}),genericSlice(xs)}}
func Mutable()int{n:=unsafe.Sizeof(int64(0));copy:=n;p:=&copy;return int(*p)}
`
	comparison := `package unsafelayout_test
import("testing";"reflect";g "unsafe-layout-constants.test";r "unsafe-layout-constants.test/reference")
const importedOffset=g.Promoted
func TestLayout(t *testing.T){for _,pair:=range [][2][]int{{g.Layout(),r.Layout()},{g.Ignored(),r.Ignored()},{g.Generics(),r.Generics()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatal(pair)}};if g.Mutable()!=r.Mutable()||importedOffset!=r.Promoted{t.Fatal("constant/storage")}}
`
	runGeneratedGoDifferentialTestWithModule(t, root, "unsafe-layout-constants.test", module, generated, reference, comparison)
	invalid := `import go u from "unsafe";import go m from "example.com/layout";function f(x:m.ViaPointer):int{return int(u.Offsetof(x.Value));}`
	path := filepath.Join(root, "entry.km")
	checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: invalid})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range checked.Diagnostics {
		if strings.Contains(d.Message, "embedded through a pointer") && d.Span.Path == path {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing source pointer-embedding diagnostic: %v", checked.Diagnostics)
	}
}
