package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMapAbsencePreservesPrivateGoTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module map-absence-private.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden struct{N int}
type Lookup map[int]*hidden
func Values(state int)Lookup{values:=Lookup{};if state==1{values[0]=nil};if state==2{values[0]=&hidden{7}};return values}
`,
		"entry.km": `import go native from "example.com/native";
export function Run(state:int):int[]{
 const values=native.Values(state);const value=values[0];let n=0;if(value!==null){n=value.N;}
 const [snapshot,present]=values[0];let checked=0;if(present){if(snapshot===nil){checked=1;}else{checked=snapshot.N;}}
 return [n,checked];
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "mapprivate")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) {
		t.Fatalf("named private Go type:\n%s", generated)
	}
	reference := `package reference
type hidden struct{n int}
func Run(state int)[]int{values:=map[int]*hidden{};if state==1{values[0]=nil};if state==2{values[0]=&hidden{7}};v:=values[0];n:=0;if v!=nil{n=v.n};snapshot,present:=values[0];checked:=0;if present{if snapshot==nil{checked=1}else{checked=snapshot.n}};return []int{n,checked}}
`
	comparison := `package mapprivate_test
import("reflect";"testing";g "map-absence-private.test";r "map-absence-private.test/reference")
func TestPrivate(t *testing.T){for _,state:=range []int{0,1,2}{if got,want:=g.Run(state),r.Run(state);!reflect.DeepEqual(got,want){t.Fatal(state,got,want)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "map-absence-private.test", generated, reference, comparison, nil, nil)
}
