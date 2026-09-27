package lsp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemberCompletionEditingExistingName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "library.km"), []byte(`export class Box<T>{constructor(public value:T){}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, input, want, hidden string }{
		{"source", `class Box{constructor(public value:int){}}function run(box:Box):int{return box.va|lue;}`, "value", ""},
		{"source selector start", `class Box{constructor(public value:int){}}function run(box:Box):int{return box.|value;}`, "value", ""},
		{"source call argument", `class Box{constructor(public value:int){}}function consume(n:int):int{return n;}function run(box:Box):int{return consume(box.va|lue);}`, "value", ""},
		{"import alias", `import {Box as Crate} from "./library";function run():int{const box=new Crate<int>(42);return box.va|lue;}`, "value", ""},
		{"Go value", `import go {Buffer as Bytes} from "bytes";function run():string{let box:Bytes=Bytes{};return box.Str|ing();}`, "String", "Reset"},
		{"Go package", `import go fmt from "fmt";function run():string{return fmt.Spr|int(42);}`, "Sprint", "Println"},
		{"Unicode CRLF", "class 箱{constructor(public 値段:int){}}\r\nfunction f(箱物:箱):int{return 箱物.値|段;}", "値段", ""},
		{"private filtering", `class Box{private value:int=1;public valid:int=2;}function f(box:Box):int{return box.va|lid;}`, "valid", "value"},
		{"getter", `class Box{public get value():int{return 42;}}function f():int{const box=new Box();return box.va|lue;}`, "value", ""},
		{"static filtering", `class Box{public static value:int=42;public variant:int=1;}function f():int{return Box.va|lue;}`, "value", "variant"},
		{"unterminated function", `class Box{constructor(public value:int){}}function f(box:Box):int{return box.va|`, "value", ""},
		{"unterminated call", `class Box{constructor(public value:int){}}function take(n:int):int{return n;}function f(box:Box):int{return take(box.va|`, "value", ""},
		{"unterminated index", `class Box{constructor(public value:int){}}function f(box:Box,xs:int[]):int{return xs[box.va|`, "value", ""},
		{"trailing whitespace", "class Box{constructor(public value:int){}}function f(box:Box):int{return box.va|\r\n  ", "value", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			marker := strings.Index(test.input, "|")
			input := strings.Replace(test.input, "|", "", 1)
			at := positionOf(test.input, "|", 0)
			uri := fileURI(filepath.Join(root, "entry.km"))
			messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/completion", 2, uri, at, ""))
			labels := map[string]bool{}
			for _, raw := range messages[2]["result"].([]any) {
				labels[raw.(map[string]any)["label"].(string)] = true
			}
			if !labels[test.want] || test.hidden != "" && labels[test.hidden] {
				t.Fatalf("offset=%d completion=%v", marker, messages[2])
			}
		})
	}
}

func TestMemberCompletionRecoveryPreservesDocument(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	input := "class 箱{constructor(public 値段:int){}}\r\nfunction f(箱物:箱):int{return 箱物.値段;}"
	at := positionOf(input, "値段", 1)
	at.Character++
	messages := []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input),
		requestAt("textDocument/completion", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		`{"jsonrpc":"2.0","id":900,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`}
	var output bytes.Buffer
	server := newServer(strings.NewReader(framed(messages...)), &output)
	if err := server.serve(); err != nil {
		t.Fatal(err)
	}
	if got := server.documents[uri].Text; got != input {
		t.Fatalf("document changed: %q", got)
	}
	want := positionOf(input, "値段", 0)
	found := false
	for _, message := range decodeMessages(t, output.String()) {
		if message["id"] != float64(3) {
			continue
		}
		result, ok := message["result"].(map[string]any)
		if !ok {
			t.Fatalf("definition: %v", message)
		}
		start := result["range"].(map[string]any)["start"].(map[string]any)
		if start["line"] != float64(want.Line) || start["character"] != float64(want.Character) {
			t.Fatalf("definition moved: %v", result)
		}
		found = true
	}
	if !found {
		t.Fatal("definition response missing")
	}
}

func TestCompletionRecoveryText(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input        string
		offset       int
		prefix, want string
	}{
		{"box.value", 6, "va", "box      "},
		{"box.値段;next()", len("box.値"), "値", "box       ;next()"},
		{"box.value()", 4, "", "box      ()"},
		{"box.value\r\nnext()", 6, "va", "box      \r\nnext()"},
		{"box.value", -1, "", "box.value"},
		{"box.value", 100, "", "box.value"},
		{"box", 3, "box", "box"},
	} {
		if got := memberCompletionAnalysisText(test.input, test.offset, test.prefix); got != test.want {
			t.Errorf("%q: got %q want %q", test.input, got, test.want)
		}
	}
	for _, test := range []struct{ input, want string }{
		{"f(xs[box", "f(xs[box\n])"},
		{"function f(){return box", "function f(){return box\n}"},
		{"f(\"[\"", "f(\"[\"\n)"},
		{"f(/* ] */box", "f(/* ] */box\n)"},
		{"f([)", "f([)"},
		{"f(\"unfinished", "f(\"unfinished"},
		{"f(/* unfinished", "f(/* unfinished"},
		{"box", "box"},
	} {
		if got := closeCompletionDelimiters(test.input); got != test.want {
			t.Errorf("%q: got %q want %q", test.input, got, test.want)
		}
	}
}

func FuzzMemberCompletionRecovery(f *testing.F) {
	for _, seed := range []string{"box.value", "return f(box.値段", "function f(){return xs[box.va", "box.\r\n", "f(/*", "f([)"} {
		f.Add(seed, uint32(len(seed)))
	}
	f.Fuzz(func(t *testing.T, input string, rawOffset uint32) {
		if len(input) > 1<<16 {
			t.Skip()
		}
		offset := int(rawOffset % uint32(len(input)+1))
		_, prefix, member := completionContext(input, offset)
		if !member {
			return
		}
		got := memberCompletionAnalysisText(input, offset, prefix)
		if len(got) < len(input) || len(got) > 2*len(input)+1 {
			t.Fatal("unbounded recovery size")
		}
		start := offset - len(prefix) - 1
		if got[:start] != input[:start] {
			t.Fatal("changed receiver or earlier source")
		}
		for i := start; i < len(input); i++ {
			if input[i] == '\r' || input[i] == '\n' {
				if got[i] != input[i] {
					t.Fatal("changed line endings")
				}
			} else if got[i] != input[i] && got[i] != ' ' {
				t.Fatal("changed source outside blanking")
			}
		}
	})
}
