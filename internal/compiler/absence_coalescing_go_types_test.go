package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbsenceCoalescingPreservesPrivateGoTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module absence-fallback-private.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden struct{n int}
func(v hidden)Value()int{return v.n}
func Values(p bool)map[int]hidden{m:=map[int]hidden{};if p{m[0]=hidden{7}};return m}
func Stream(p bool)chan hidden{ch:=make(chan hidden,1);if p{ch<-hidden{5}};close(ch);return ch}
func Fallback()hidden{return hidden{9}}
`,
		"entry.km": `import go native from "example.com/native";
alias View=interface{Value():int;};
export function Lookup(present:boolean):View{return native.Values(present)[0]??native.Fallback();}
export function Receive(present:boolean):View{return (<-native.Stream(present))??native.Fallback();}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := filepath.Join(root, "entry.km")
	bad := `import go native from "example.com/native";function f():void{const value=native.Values(false)[0]??native.Fallback();}`
	checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: bad})
	if err != nil {
		t.Fatal(err)
	}
	if len(checked.Diagnostics) != 1 || !strings.Contains(checked.Diagnostics[0].Message, "private Go type") {
		t.Fatalf("unnameable result: %v %v", err, checked.Diagnostics)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "absenceprivate")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) {
		t.Fatalf("named private Go type:\n%s", generated)
	}
	reference := `package reference
type View interface{Value()int}
type hidden struct{n int}
func(v hidden)Value()int{return v.n}
func Lookup(p bool)View{m:=map[int]hidden{};if p{m[0]=hidden{7}};v,ok:=m[0];if !ok{return hidden{9}};return v}
func Receive(p bool)View{ch:=make(chan hidden,1);if p{ch<-hidden{5}};close(ch);v,open:=<-ch;if !open{return hidden{9}};return v}
`
	comparison := `package absenceprivate_test
import("testing";g "absence-fallback-private.test";r "absence-fallback-private.test/reference")
func TestPrivate(t *testing.T){for _,p:=range []bool{false,true}{if got,want:=g.Lookup(p).Value(),r.Lookup(p).Value();got!=want{t.Fatal(got,want)};if got,want:=g.Receive(p).Value(),r.Receive(p).Value();got!=want{t.Fatal(got,want)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "absence-fallback-private.test", generated, reference, comparison, nil, nil)
}
