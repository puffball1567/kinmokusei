package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoMethodExpressionSignatures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, needle, want string }{
		{`import go time from "time";function f(v:time.Time):boolean{return time.Time.IsZero(v);}`, "IsZero(v", "time.Time"},
		{`import go bytes from "bytes";function f():int{let b:bytes.Buffer=bytes.Buffer{};const length=(*bytes.Buffer).Len;return length(&b);}`, "length(&b", "*bytes.Buffer"},
		{`import go io from "io";function f(v:io.Reader,b:byte[]):int{const read=io.Reader.Read;const [n,e]=read(v,b);return n;}`, "read(v,b", "io.Reader"},
	} {
		t.Run(test.needle, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "entry.km")
			at := positionOf(test.input, test.needle, 0)
			at.Character += len(test.needle) - 1
			label, _, parameters := signatureResult(t, signatureHelpAt(t, path, test.input, at))
			if !strings.Contains(label, test.want) || len(parameters) == 0 {
				t.Fatalf("label=%q parameters=%v", label, parameters)
			}
		})
	}
}

func TestGoMethodExpressionIncompleteSignatures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input, receiver    string
		parameters, active int
	}{
		{`import go time from "time";function f():boolean{return time.Time.IsZero(|`, "time.Time", 1, 0},
		{`import go {Time as Instant} from "time";function f():boolean{return Instant.IsZero(|`, "time.Time", 1, 0},
		{`import go bytes from "bytes";function f():void{let b:bytes.Buffer=bytes.Buffer{};(*bytes.Buffer).WriteString(&b, |`, "*bytes.Buffer", 2, 1},
		{`import go io from "io";function f(v:io.Reader):void{io.Reader.Read(v, |`, "io.Reader", 2, 1},
		{`import go time from "time";function f():boolean{return time.Time /* receiver */ . IsZero(|`, "time.Time", 1, 0},
		{`import go bytes from "bytes";function f():void{let b:bytes.Buffer=bytes.Buffer{};((*bytes.Buffer)) /* receiver */ . WriteString(&b, |`, "*bytes.Buffer", 2, 1},
	} {
		t.Run(test.input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "entry.km")
			input := strings.Replace(test.input, "|", "", 1)
			label, active, parameters := signatureResult(t, signatureHelpAt(t, path, input, positionOf(test.input, "|", 0)))
			if !strings.Contains(label, "receiver: "+test.receiver) || len(parameters) != test.parameters || active != float64(test.active) {
				t.Fatalf("label=%q active=%v parameters=%v", label, active, parameters)
			}
		})
	}
}

func TestGoMethodExpressionIncompleteSignaturesRejectInvalidReceivers(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go bytes from "bytes";function f():void{bytes.Buffer.Len(`,
		`import go io from "io";function f():void{(*io.Reader).Read(`,
		`import go time from "time";function f():void{time.Time.Missing(`,
		`import go http from "net/http";function f():void{(*http.Request).Method(`,
		`import go {Time} from "time";function f(Time:int):void{Time.IsZero(`,
		`import go time from "time";function f():void{const time={Time:1};time.Time.IsZero(`,
	} {
		t.Run(input, func(t *testing.T) {
			message := signatureHelpAt(t, filepath.Join(t.TempDir(), "entry.km"), input, position{Character: len(input)})
			if message["result"] != nil {
				t.Fatalf("invalid receiver signature: %v", message)
			}
		})
	}
}

func TestGoMethodExpressionCompletions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want, hidden string }{
		{`import go time from "time";function f():void{const method=time.Time.Is|Zero;}`, "IsZero", ""},
		{`import go bytes from "bytes";function f():void{const method=(*bytes.Buffer).|;}`, "Len", ""},
		{`import go bytes from "bytes";const method=(*bytes.Buffer).Le|n;`, "Len", ""},
		{`import go bytes from "bytes";const method=((*bytes.Buffer)) /* receiver */ . Le|n;`, "Len", ""},
		{`import go bytes from "bytes";function f():void{const method=bytes.Buffer.|;}`, "", "Len"},
		{`import go { Time as Instant } from "time";function f():void{const method=Instant.Is|Zero;}`, "IsZero", ""},
		{`import go io from "io";function f():void{const method=io.Reader.Re|ad;}`, "Read", ""},
		{`import go io from "io";function f():void{const method=(*io.Reader).|;}`, "", "Read"},
		{`import go http from "net/http";function f():void{const method=(*http.Request).|;}`, "WithContext", "Method"},
		{`import go time from "time";function f():void{const time={Time:{IsLocal:1}};const value=time.Time.|;}`, "", "IsZero"},
		{`import go { Time } from "time";function f():void{const Time={IsLocal:1};const value=Time.|;}`, "IsLocal", "IsZero"},
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

func TestGoMethodExpressionDiagnosticRange(t *testing.T) {
	t.Parallel()
	input := "import go bytes from \"bytes\";\nconst callback=bytes.Buffer.Len;"
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		diagnostics := message["params"].(map[string]any)["diagnostics"].([]any)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics=%v", diagnostics)
		}
		diagnostic := diagnostics[0].(map[string]any)
		if !strings.Contains(diagnostic["message"].(string), "has no exported method") {
			t.Fatal(diagnostic)
		}
		start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
		want := positionOf(input, "Len", 0)
		if start["line"] != float64(want.Line) || start["character"] != float64(want.Character) {
			t.Fatalf("wrong source range: %v", diagnostic)
		}
		return
	}
	t.Fatal("missing diagnostic publication")
}
