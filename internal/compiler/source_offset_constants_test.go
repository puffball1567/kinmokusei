package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestSourceOffsetConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":          "module source-offset-constants.test\n\ngo 1.23\n",
		"kinmokusei.toml": "[project]\nname=\"source-offset-constants\"\nversion=\"0.1.0\"\ngo-module=\"source-offset-constants.test\"\ngo-version=\"1.23\"\n[go.interop]\nunsafe=\"allow\"\n",
		"packet.km": `import go {Offsetof as offset} from "unsafe";
export struct Packet{public pad:byte;private value:int64;public tail:byte;}
const packet:Packet=Packet{pad:1,value:2,tail:3};
export const ValueOffset=int(offset(packet.value));
export const TailOffset=int(offset(packet.tail));
export struct Box<T>{private pad:byte;public value:T;}
export struct SliceBox<T>{private pad:byte;public values:T[];}
`,
		"bridge.km": `export {Packet as Record,ValueOffset,TailOffset,Box,SliceBox} from "./packet";`,
		"entry.km": `import {Record,ValueOffset,TailOffset,Box,SliceBox} from "./bridge";
import go u from "unsafe";
alias A=Record;
type Defined=distinct Record;
struct Node{private next:*Node;public value:int;}
struct ZeroTail{private first:byte;public empty:[0]int64;}
class Limits{public static const value:int=ValueOffset;}
function genericOffset<T>(box:Box<T>):int{const n=u.Offsetof(box.value);const p=&n;return int(*p);}
function genericPrefix<T>(box:Box<T>):int{const n=u.Offsetof(box.pad);const p=&n;return int(*p);}
function genericHeader<T>(box:SliceBox<T>):int{const n=u.Offsetof(box.values);return int(n);}
export const PublicOffset=ValueOffset;
export function Offsets():int[]{const packet:A=A{pad:1,value:2,tail:3};const defined=Defined(packet);const p:*A=nil;const d:*Defined=nil;const zero:*ZeroTail=nil;const object:{z:int64,a:byte}={z:2,a:1};return [ValueOffset,TailOffset,int(u.Offsetof(packet.pad)),int(u.Offsetof(defined.value)),int(u.Offsetof(p.value)),int(u.Offsetof(d.tail)),int(u.Offsetof(zero.empty)),int(u.Offsetof(object.z)),Limits.value];}
export function Generics():int[]{const a:Box<int64>=Box<int64>{pad:1,value:2};const b:Box<byte>=Box<byte>{pad:1,value:2};const c:SliceBox<int64>=SliceBox<int64>{pad:1,values:[]};return [genericOffset(a),genericOffset(b),genericPrefix(a),genericHeader(c),int(u.Offsetof(a.value))];}
export function Ignored():int[]{let calls=0;const get=():*A=>{calls++;return nil;};const offset=u.Offsetof(get().value);const nodes=goChannel<Node>();const received=u.Offsetof((<-nodes).value);const node:*Node=nil;const chain=u.Offsetof(node.next.value);return [calls,int(offset),int(received),int(chain)];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "sourceoffsets")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const PublicOffset", "const ValueOffset", "const TailOffset", "const n = ", "var n = "} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import "unsafe"
type packet struct{pad byte;value int64;tail byte}
var p packet
const ValueOffset=int(unsafe.Offsetof(p.value))
const TailOffset=int(unsafe.Offsetof(p.tail))
type alias=packet
type defined packet
type node struct{next *node;value int}
type zeroTail struct{first byte;empty [0]int64}
type box[T any] struct{pad byte;value T}
type sliceBox[T any] struct{pad byte;values []T}
func genericOffset[T any](b box[T])int{n:=unsafe.Offsetof(b.value);p:=&n;return int(*p)}
func genericPrefix[T any](b box[T])int{n:=unsafe.Offsetof(b.pad);p:=&n;return int(*p)}
func genericHeader[T any](b sliceBox[T])int{const n=unsafe.Offsetof(b.values);return int(n)}
func Offsets()[]int{a:=alias{1,2,3};b:=defined(a);var p *alias;var d *defined;var z *zeroTail;object:=struct{a byte;z int64}{1,2};return []int{ValueOffset,TailOffset,int(unsafe.Offsetof(a.pad)),int(unsafe.Offsetof(b.value)),int(unsafe.Offsetof(p.value)),int(unsafe.Offsetof(d.tail)),int(unsafe.Offsetof(z.empty)),int(unsafe.Offsetof(object.z)),ValueOffset}}
func Generics()[]int{a:=box[int64]{1,2};b:=box[byte]{1,2};c:=sliceBox[int64]{1,nil};return []int{genericOffset(a),genericOffset(b),genericPrefix(a),genericHeader(c),int(unsafe.Offsetof(a.value))}}
func Ignored()[]int{calls:=0;get:=func()*alias{calls++;return nil};const offset=unsafe.Offsetof(get().value);nodes:=make(chan node);const received=unsafe.Offsetof((<-nodes).value);var n *node;const chain=unsafe.Offsetof(n.next.value);return []int{calls,int(offset),int(received),int(chain)}}
`
	comparison := `package sourceoffsets_test
import("testing";"reflect";g "source-offset-constants.test";r "source-offset-constants.test/reference")
const offset=g.PublicOffset
func TestOffsets(t *testing.T){for _,pair:=range [][2][]int{{g.Offsets(),r.Offsets()},{g.Generics(),r.Generics()},{g.Ignored(),r.Ignored()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatal(pair)}};if offset!=r.ValueOffset{t.Fatal(offset)}}
`
	runGeneratedGoDifferentialTest(t, root, "source-offset-constants.test", generated, reference, comparison)
}
