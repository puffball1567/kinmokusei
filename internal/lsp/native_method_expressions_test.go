package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeMethodExpressionSignatures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, receiver string }{
		{`struct S{public function read(n:int):int{return n;}}function f(v:S):int{return S.read(v, |`, "S"},
		{`struct S{public pointer function set(n:int):void{}}function f(v:*S):void{(*S).set(v, |`, "*S"},
		{`type N=distinct int;public function add(this:N,n:int):int{return n;}function f(v:N):int{return N.add(v, |`, "N"},
		{`enum S:int8{A,B}public function read(this:S,n:int):int{return n;}function f(v:S):int{return S.read(v, |`, "S"},
		{`enum S:int8{A,B}public function read(this:*S,n:int):int{return n;}function f(v:*S):int{return (*S).read(v, |`, "*S"},
		{`interface I{function read(n:int):int;}function f(v:I):int{return I.read(v, |`, "I"},
		{`struct S{public function read(n:int):int{return n;}}alias A=S;function f(v:A):int{return A.read(v, |`, "S"},
		{`struct S{public function read(n:int):int{return n;}}function f(v:*S):int{return ((*S)) /* type */ . read(v, |`, "*S"},
	} {
		t.Run(test.input, func(t *testing.T) {
			input := strings.Replace(test.input, "|", "", 1)
			path := filepath.Join(t.TempDir(), "entry.km")
			label, active, parameters := signatureResult(t, signatureHelpAt(t, path, input, positionOf(test.input, "|", 0)))
			if !strings.Contains(label, "receiver: "+test.receiver) || !strings.Contains(label, "n: int") || len(parameters) != 2 || active != float64(1) {
				t.Fatalf("label=%q active=%v parameters=%v", label, active, parameters)
			}
		})
	}
}

func TestNativeMethodExpressionCompletions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want, hidden string }{
		{`struct S{public x:int;public function read():int{return this.x;}public pointer function set():void{}}function f():void{const read=S.|;}`, "read", "set"},
		{`struct S{public x:int;public function read():int{return this.x;}public pointer function set():void{}}function f():void{const read=(*S).|;}`, "set", "x"},
		{`enum S{A,B}public function read(this:S):int{return int(this);}public function set(this:*S):void{}const read=S.|;`, "read", "set"},
		{`enum S{A,B}public function read(this:S):int{return int(this);}const read=S.|;`, "A", ""},
		{`enum S{A,B}public function read(this:S):int{return int(this);}public function set(this:*S):void{}const read=(*S).|;`, "set", "A"},
		{`enum S{A,B}public function read(this:S):int{return int(this);}alias A=S;const read=A.|;`, "read", "B"},
		{`enum S{A,B}public function read(this:S):int{return int(this);}function f():void{const S={other:1};const read=S.|;}`, "Other", "read"},
		{`struct S{public function read():int{return 1;}}alias A=S;const read=A.re|ad;`, "read", ""},
		{`interface I{function read():int;get value():int;}const read=I.|;`, "read", "get value"},
		{`struct S{public function read():int{return 1;}public function echo<T>(v:T):T{return v;}}const read=S.|;`, "read", "echo"},
		{`struct S{public function read():int{return 1;}}function f():void{const S={other:1};const value=S.|;}`, "Other", "read"},
	} {
		t.Run(test.input, func(t *testing.T) {
			input := strings.Replace(test.input, "|", "", 1)
			uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
			messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/completion", 2, uri, positionOf(test.input, "|", 0), ""))
			labels := map[string]bool{}
			for _, raw := range messages[2]["result"].([]any) {
				labels[raw.(map[string]any)["label"].(string)] = true
			}
			if test.want != "" && !labels[test.want] || test.hidden != "" && labels[test.hidden] {
				t.Fatalf("completion=%v", messages[2])
			}
		})
	}
}

func TestNativeMethodExpressionInvalidSignature(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`struct S{public pointer function set():void{}}function f():void{S.set(`,
		`struct S{public x:int;}function f():void{S.x(`,
		`struct S{public function read():int{return 1;}}function f(S:int):void{S.read(`,
		`enum S{A}function f():void{S.A(`,
		`enum S{A}public function set(this:*S):void{}function f():void{S.set(`,
		`enum S{read}public function read(this:S):int{return int(this);}function f():void{S.read(`,
		`struct S<T>{public function read():int{return 1;}}function f():void{S.read(`,
	} {
		t.Run(input, func(t *testing.T) {
			message := signatureHelpAt(t, filepath.Join(t.TempDir(), "entry.km"), input, position{Character: len(input)})
			if message["result"] != nil {
				t.Fatalf("invalid method expression signature: %v", message)
			}
		})
	}
}

func TestNativeMethodExpressionNavigationAndRename(t *testing.T) {
	t.Parallel()
	inputs := []string{`struct S {
 public function read():int{return 1;}
}
const first=S.read;
const second=(*S).read;`, `enum S{A,B}
public function read(this:S):int{return int(this);}
const first=S.read;
const second=(*S).read;`}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
			messages := serveMessages(t, openDocument(uri, input),
				requestAt("textDocument/definition", 2, uri, positionOf(input, "read", 1), ""),
				requestAt("textDocument/rename", 3, uri, positionOf(input, "read", 2), `"newName":"snapshot"`),
				requestAt("textDocument/definition", 4, uri, positionOf(input, "S", 2), ""),
			)
			definition := messages[2]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
			if definition["line"] != float64(1) {
				t.Fatalf("method definition=%v", definition)
			}
			changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
			if len(changes[uri].([]any)) != 3 {
				t.Fatalf("rename must include declaration and both expressions: %v", changes)
			}
			typeDefinition := messages[4]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
			if typeDefinition["line"] != float64(0) {
				t.Fatalf("receiver definition=%v", typeDefinition)
			}
		})
	}
}
