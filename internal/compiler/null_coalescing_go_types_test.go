package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNullCoalescingPreservesPrivateGoTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module coalesce-private.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden struct{N int}
func(h *hidden)Read()int{if h==nil{return -1};return h.N}
type hiddenInterface interface{Read()int}
type Reader interface{Read()int}
var Calls int
func Pointer(p bool)*hidden{Calls++;if p{return &hidden{3}};return nil}
func Fallback()*hidden{Calls++;return &hidden{9}}
func Slice(p bool)[]hidden{if p{return []hidden{{3}}};return nil}
func FallbackSlice()[]hidden{Calls++;return []hidden{{9}}}
func Map(p bool)map[string]hidden{if p{return map[string]hidden{"x":{3}}};return nil}
func FallbackMap()map[string]hidden{Calls++;return map[string]hidden{"x":{9}}}
func Interface(p bool)hiddenInterface{if p{return &hidden{3}};return nil}
func FallbackInterface()hiddenInterface{Calls++;return &hidden{9}}
func Channel(p bool)chan hidden{if p{c:=make(chan hidden,1);c<-hidden{3};return c};return nil}
func FallbackChannel()chan hidden{Calls++;c:=make(chan hidden,1);c<-hidden{9};return c}
func Receive(p bool)<-chan hidden{return Channel(p)}
func FallbackReceive()<-chan hidden{return FallbackChannel()}
func Send(p bool)chan<- hidden{return Channel(p)}
func FallbackSend()chan<- hidden{return FallbackChannel()}
func LoadCallback(p bool)func()*hidden{if p{return Fallback};return nil}
`,
		"entry.km": `import go native from "example.com/native";
import go {Fallback as fallback} from "example.com/native";
function contextual(p:boolean):native.Reader{return native.Pointer(p)??native.Fallback();}
export function Run(p:boolean):int[]{
 native.Calls=0;
 const pointer=native.Pointer(p)??fallback();
 const slice=native.Slice(p)??native.FallbackSlice();
 const map=native.Map(p)??native.FallbackMap();
 const iface=native.Interface(p)??native.FallbackInterface();
 const channel=native.Channel(p)??native.FallbackChannel();
 const received=native.Receive(p)??native.FallbackReceive();
 const sent=native.Send(p)??native.FallbackSend();
 const result=[pointer.N,slice[0].N,map["x"].N,iface.Read(),(<-channel).N,(<-received).N,len(sent),contextual(p).Read(),native.Calls];
 return result;
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "coalesceprivate")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) {
		t.Fatalf("named private Go type:\n%s", generated)
	}
	reference := `package reference
func Run(p bool)[]int{value:=9;calls:=10;if p{value=3;calls=2};return []int{value,value,value,value,value,value,1,value,calls}}
`
	comparison := `package coalesceprivate_test
import("reflect";"testing";g "coalesce-private.test";r "coalesce-private.test/reference")
func TestPrivate(t *testing.T){for _,p:=range []bool{false,true}{if got,want:=g.Run(p),r.Run(p);!reflect.DeepEqual(got,want){t.Fatalf("present=%v got=%v want=%v",p,got,want)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "coalesce-private.test", generated, reference, comparison, nil, nil)

	// General lazy expressions whose private result cannot be named must fail
	// in the source checker rather than emit an invalid Go signature.
	invalid := `import go native from "example.com/native";function f():void{const v=native.Pointer(false)??native.Pointer(true);}`
	result, err := CheckFilesWithOverlay(paths, map[string]string{paths[0]: invalid})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range result.Diagnostics {
		found = found || bytes.Contains([]byte(d.Message), []byte("private Go type"))
	}
	if !found {
		t.Fatalf("missing source diagnostic: %v", result.Diagnostics)
	}
}
