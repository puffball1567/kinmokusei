package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestChannelAbsencePreservesPrivateGoTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module channel-absence-private.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden struct{N int}
type Stream chan *hidden
func Make(state int)Stream{ch:=make(Stream,1);if state==1{ch<-nil};if state==2{ch<-&hidden{7}};close(ch);return ch}
`,
		"entry.km": `import go native from "example.com/native";
export function Run(state:int):int[]{
 const value=<-native.Make(state);let n=0;if(value!==null){n=value.N;}
 let selected=<-native.Make(0);select{case selected=<-native.Make(state){}}
 let selectedN=0;if(selected!==null){selectedN=selected.N;}
 let checked=0;select{case const [snapshot,open]=<-native.Make(state){if(open){if(snapshot===nil){checked=1;}else{checked=snapshot.N;}}}}
 return [n,selectedN,checked];
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "channelprivate")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) {
		t.Fatalf("named private Go type:\n%s", generated)
	}
	reference := `package reference
type hidden struct{n int}
type stream chan *hidden
func makeStream(state int)stream{ch:=make(stream,1);if state==1{ch<-nil};if state==2{ch<-&hidden{7}};close(ch);return ch}
func Run(state int)[]int{value:=<-makeStream(state);n:=0;if value!=nil{n=value.n};selected:=<-makeStream(0);select{case selected=<-makeStream(state):};selectedN:=0;if selected!=nil{selectedN=selected.n};checked:=0;select{case snapshot,open:=<-makeStream(state):if open{if snapshot==nil{checked=1}else{checked=snapshot.n}}};return []int{n,selectedN,checked}}
`
	comparison := `package channelprivate_test
import("reflect";"testing";g "channel-absence-private.test";r "channel-absence-private.test/reference")
func TestPrivate(t *testing.T){for _,state:=range []int{0,1,2}{if got,want:=g.Run(state),r.Run(state);!reflect.DeepEqual(got,want){t.Fatal(state,got,want)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "channel-absence-private.test", generated, reference, comparison, nil, nil)
}
