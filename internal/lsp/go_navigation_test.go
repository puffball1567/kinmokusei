package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func requireGoNavigation(t *testing.T, path, input, needle, detail, documentation, name string) {
	t.Helper()
	at := positionOf(input, needle, 0)
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, at, ""), requestAt("textDocument/definition", 3, uri, at, ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("hover: %v", messages[2])
	}
	text := hover["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(text, detail) || !strings.Contains(text, documentation) {
		t.Fatalf("hover %q: want %q / %q", text, detail, documentation)
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition: %v", messages[3])
	}
	target, err := filePath(definition["uri"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(target) != ".go" {
		t.Fatalf("expected Go declaration, got %s", target)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	span := definition["range"].(map[string]any)
	start := span["start"].(map[string]any)
	end := span["end"].(map[string]any)
	lines := strings.Split(string(content), "\n")
	line := int(start["line"].(float64))
	if line >= len(lines) || end["line"] != start["line"] {
		t.Fatalf("range: %v", span)
	}
	lo, startErr := byteOffsetAtPosition(lines[line], position{Character: int(start["character"].(float64))})
	if startErr != nil {
		t.Fatal("invalid start")
	}
	hi, endErr := byteOffsetAtPosition(lines[line], position{Character: int(end["character"].(float64))})
	if endErr != nil || lines[line][lo:hi] != name {
		t.Fatalf("target: %v %s", span, lines[line])
	}
}

func TestGoHoverAndDefinitionStandardLibrary(t *testing.T) {
	for _, test := range []struct{ input, needle, detail, doc, name string }{
		{`import go fmt from "fmt";function f():string{return fmt.Sprint(42);}`, "Sprint(42)", "func fmt.Sprint", "Sprint formats", "Sprint"},
		{`import go math from "math";function f():float64{return math.Pi;}`, "Pi;", "const math.Pi", "Mathematical constants", "Pi"},
		{`import go os from "os";function f():void{const stream=os.Stdout;}`, "Stdout;", "var os.Stdout", "Stdout", "Stdout"},
		{`import go http from "net/http";function f(r:*http.Request):void{const method=r.Method;}`, "Method;", "field Method string", "HTTP method", "Method"},
		{`import go http from "net/http";function f(r:*http.Request):void{const ctx=r.Context();}`, "Context()", "func (*http.Request).Context", "Context returns", "Context"},
		{`import go time from "time";function f(t:time.Time):boolean{return t.IsZero();}`, "Time):", "type time.Time struct", "A Time represents", "Time"},
		{`import go time from "time";function f(t:time.Time):boolean{return time.Time.IsZero(t);}`, "IsZero(t)", "IsZero() bool", "IsZero reports", "IsZero"},
		{`import go bufio from "bufio";function f(r:*bufio.ReadWriter):void{const [text,err]=r.ReadString(10);}`, "ReadString(10)", "ReadString", "ReadString reads", "ReadString"},
		{`import go io from "io";function f(r:io.Reader,b:byte[]):void{const [n,err]=r.Read(b);}`, "Read(b)", "Read(p []byte)", "Read reads", "Read"},
		{`import go {Sprint as Format} from "fmt";function f():string{return Format(42);}`, "Format(42)", "func fmt.Sprint", "Sprint formats", "Sprint"},
		{`import go {Time as Instant} from "time";function f(t:Instant):boolean{return t.IsZero();}`, "Instant):", "type time.Time struct", "A Time represents", "Time"},
		{`import go atomic from "sync/atomic";function f(p:*atomic.Pointer<int>):void{p.Store(nil);}`, "Store(nil)", "Store", "Store atomically", "Store"},
	} {
		t.Run(test.needle, func(t *testing.T) {
			requireGoNavigation(t, filepath.Join(t.TempDir(), "entry.km"), test.input, test.needle, test.detail, test.doc, test.name)
		})
	}
}

func TestGoHoverAndDefinitionExternalLockedModule(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "api")
	if err := os.Mkdir(library, 0755); err != nil {
		t.Fatal(err)
	}
	goSource := "package api\n\n// Visible is a documented external function.\nfunc Visible() int {return 42}\nvar _ = \"🌸\"; func Unicode() int {return 1}\n\n// Record stores external data.\ntype Record struct {\n // Value is the stored integer.\n Value int\n}\n// Read returns the stored integer.\nfunc(r *Record) Read() int{return r.Value}\n"
	for path, text := range map[string]string{
		filepath.Join(root, product.ProjectFileName): "[project]\nname=\"navigation\"\nversion=\"0.1.0\"\ngo-module=\"example.com/navigation\"\ngo-version=\"1.23\"\n",
		filepath.Join(library, "go.mod"):             "module example.com/api\n\ngo 1.23\n",
		filepath.Join(library, "api.go"):             goSource,
	} {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.AddDependency(root, "example.com/api", "v0.0.0", "./api", true); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ input, needle, detail, doc, name string }{
		{`import go api from "example.com/api";function f():int{return api.Visible();}`, "Visible()", "func api.Visible", "documented external function", "Visible"},
		{`import go api from "example.com/api";function f():int{return api.Unicode();}`, "Unicode()", "func api.Unicode", "", "Unicode"},
		{`import go api from "example.com/api";function f(r:*api.Record):int{return r.Value;}`, "Value;", "field Value int", "stored integer", "Value"},
		{`import go api from "example.com/api";function f(r:*api.Record):int{return r.Read();}`, "Read()", "Read() int", "stored integer", "Read"},
	} {
		t.Run(test.needle, func(t *testing.T) {
			requireGoNavigation(t, filepath.Join(root, "main.km"), test.input, test.needle, test.detail, test.doc, test.name)
		})
	}
}

func TestGoNavigationRespectsShadowingAndNonCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	input := `import go {Sprint} from "fmt";function f(Sprint:int):int{return Sprint;} // Sprint`
	at := positionOf(input, "Sprint;", 0)
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, at, ""), requestAt("textDocument/definition", 3, uri, at, ""))
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "Sprint: int") || strings.Contains(hover, "func fmt") {
		t.Fatalf("shadowing: %s", hover)
	}
	if messages[3]["result"].(map[string]any)["uri"] != uri {
		t.Fatalf("shadowing definition: %v", messages[3])
	}
	for _, text := range []string{`// fmt.Sprint`, `const s="fmt.Sprint";`} {
		doc := document{Path: path, Text: text}
		if _, found := goReferenceAt(&ast.Program{}, doc, strings.Index(text, "Sprint")); found {
			t.Fatal("noncode reference")
		}
	}
}
