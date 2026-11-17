package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceOrderPreservesGoContextualAndPrivateTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module source-order-private.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden byte
var Current hidden=1
func Bump()int{Current=10;return 5}
func Pair(a hidden,b int)int{return int(a)*100+b}
func Generic[T ~byte](a T,b int)int{return int(a)*100+b}
func Map()map[hidden]int{return map[hidden]int{1:1,2:2}}
func Tuple()(hidden,hidden){return 1,2}
func Two(a,b hidden)int{return int(a)*100+int(b)}
func Pointer(p *int,b int)int{if p==nil{return b};return *p*100+b}
func Wide(a uint64,b int)int{return int(a>>63)*100+b}
var Fn=Pair
func Reset(){Current=1;Fn=Pair}
func Swap()int{Current=10;Fn=func(a hidden,b int)int{return int(a)*1000+b};return 5}
`,
		"entry.km": `import go native from "example.com/native";
import go {Fn as importedFn} from "example.com/native";
export function Run(n:int):int[]{
 native.Current=1;const a=native.Pair(native.Current,native.Bump());
 native.Current=1;const b=native.Generic(native.Current,native.Bump());
 const rhs=():int=>{n=8;return 5;};const c=native.Pair(1<<n,rhs());
 const m=native.Map();let index=0;const value=():int=>{index=1;return 9;};m[1<<index]=value();
 const d=native.Two(native.Tuple());const e=native.Pointer(nil,native.Bump());const f=native.Wide(1<<63,native.Bump());
 native.Reset();const g=native.Fn(native.Current,native.Swap());
 native.Reset();const h=importedFn(native.Current,native.Swap());
 return [a,b,c,m[1],m[2],d,e,f,g,h];
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "ordered")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), "native.hidden") {
		t.Fatalf("generated output named a private Go type:\n%s", generated)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "ordered")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type hidden byte
func Run(n int)[]int{
 current:=hidden(1);first:=current;current=10;_=current;a:=int(first)*100+5
 current=1;first=current;current=10;_=current;b:=int(first)*100+5
 var shifted hidden=1<<n;n=8;_=n;c:=int(shifted)*100+5
 m:=map[hidden]int{1:1,2:2};index:=0;var key hidden=1<<index;index=1;_=index;m[key]=9
 return []int{a,b,c,m[1],m[2],102,5,105,105,105}
}
`
	comparison := `package ordered_test
import("reflect";"testing";g "source-order-private.test";r "source-order-private.test/reference")
func TestPrivate(t *testing.T){for _,n:=range []int{0,1,7,8,63,64}{if got,want:=g.Run(n),r.Run(n);!reflect.DeepEqual(got,want){t.Fatalf("n=%d: generated=%v reference=%v",n,got,want)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "source-order-private.test", generated, reference, comparison, nil, nil)
}
