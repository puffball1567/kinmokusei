package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializedSlicesPreservePrivateGoTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module initialized-private.test\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden struct{Next *int}
type Values []hidden
type index uint64
var Count index
func Set(n uint64){Count=index(n)}
func Buffer()Values{return make(Values,1,3)}
`,
		"entry.km": `import go native from "initialized-private.test/native";
export function Private(n:int):int{const xs=native.Buffer();return len(xs[:n]);}
export function HiddenIndex(n:uint64):int{native.Set(n);const xs=native.Buffer();return len(xs[0:native.Count]);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "initialized")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), "native.hidden") || strings.Contains(string(generated), "native.index") {
		t.Fatalf("private type named in output:\n%s", generated)
	}
	reference := `package reference
func Private(n int)int{if n<0||n>1{panic("bounds")};return n}
func HiddenIndex(n uint64)int{if n>1{panic("bounds")};return int(n)}
`
	comparison := `package initialized_test
import("testing";g "initialized-private.test";r "initialized-private.test/reference")
func capture(f func()int)(value int,panicked bool){defer func(){panicked=recover()!=nil}();value=f();return}
func TestPrivate(t *testing.T){for _,n:=range []int{-1,0,1,2,3,4}{a,p:=capture(func()int{return g.Private(n)});b,q:=capture(func()int{return r.Private(n)});if a!=b||p!=q{t.Fatal("private",n,a,p,b,q)}};for _,n:=range []uint64{0,1,2,3,1<<32,^uint64(0)}{a,p:=capture(func()int{return g.HiddenIndex(n)});b,q:=capture(func()int{return r.HiddenIndex(n)});if a!=b||p!=q{t.Fatal("index",n,a,p,b,q)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "initialized-private.test", generated, reference, comparison, nil, nil)
}
