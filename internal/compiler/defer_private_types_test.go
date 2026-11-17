package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeferredGoPrivateTypesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module defer-private.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden byte
func Token()hidden{return 7}
func Accept(event func(int),v hidden){event(int(v))}
func Variadic(event func(int),v ...hidden){for _,n:=range v{event(int(n))}}
func Generic[T ~byte](event func(int),seed T,value T){event(int(seed));event(int(value))}
func Pair()(hidden,hidden){return 4,5}
func Consume(a,b hidden){}
func Wide(event func(int),n uint64){event(int(n>>63))}
func Empty[T any](){}
func Map()map[hidden]int{return map[hidden]int{1:11,2:22}}
`,
		"entry.km": `import go native from "example.com/native";
export function Run(event:(value:int)=>void,n:uint):void{
 const m=native.Map();
 const observe=():void=>{event(len(m));};defer observe();
 try {
  defer native.Accept(event,native.Token());
  defer native.Accept(event,1<<n);
  defer native.Variadic(event,1<<n,2<<n);
  defer native.Generic(event,native.Token(),1<<n);
  defer native.Consume(native.Pair());
  defer native.Wide(event,1<<63);
  defer native.Empty<int>();
  defer delete(m,1<<n);
 } finally {event(-1);}
 event(len(m));
}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "defers")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), "native.hidden") {
		t.Fatalf("defer named an unexported Go type:\n%s", generated)
	}
	reference := `package reference
type hidden byte
func token()hidden{return 7}
func accept(event func(int),v hidden){event(int(v))}
func variadic(event func(int),v ...hidden){for _,n:=range v{event(int(n))}}
func generic[T ~byte](event func(int),seed T,value T){event(int(seed));event(int(value))}
func pair()(hidden,hidden){return 4,5}
func consume(a,b hidden){}
func wide(event func(int),n uint64){event(int(n>>63))}
func empty[T any](){}
func Run(event func(int),n uint){
 m:=map[hidden]int{1:11,2:22};defer func(){event(len(m))}();defer accept(event,token());defer accept(event,1<<n);defer variadic(event,1<<n,2<<n)
 defer generic(event,token(),1<<n);defer consume(pair());defer wide(event,1<<63);defer empty[int]();defer delete(m,1<<n)
 event(-1);event(len(m))
}
`
	comparison := `package defers_test
import("reflect";"testing";g "defer-private.test";r "defer-private.test/reference")
func collect(call func(func(int),uint),n uint)(trace []int){call(func(v int){trace=append(trace,v)},n);return}
func TestPrivate(t *testing.T){for _,n:=range []uint{0,1,7,8,64}{gt,rt:=collect(g.Run,n),collect(r.Run,n);if !reflect.DeepEqual(gt,rt){t.Fatalf("n=%d generated=%v reference=%v",n,gt,rt)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "defer-private.test", generated, reference, comparison, nil, nil)
}
