package compiler

import (
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type navigationPositionImporter struct {
	types.Importer
	position token.Position
}

func (i navigationPositionImporter) GoSourcePosition(int) token.Position { return i.position }

func TestGoSymbolSourceFallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.go")
	text := "package api\n// Visible returns a value.\nfunc Visible() int {return 1}\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	reference := ast.GoReference{Name: "Visible", Detail: "func api.Visible() int", Position: 1}
	for _, test := range []struct {
		name, path string
		line       int
		valid      bool
	}{
		{"available", path, 3, true},
		{"missing source", path + ".missing", 3, false},
		{"wrong line", path, 2, false},
		{"no position", "", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := Result{goImporter: navigationPositionImporter{position: token.Position{Filename: test.path, Line: test.line}}}
			symbol := r.GoSymbol(reference)
			if symbol.Detail != reference.Detail {
				t.Fatal(symbol)
			}
			if (symbol.Declaration.Path != "") != test.valid {
				t.Fatal(symbol)
			}
			if test.valid && !strings.Contains(symbol.Documentation, "returns a value") {
				t.Fatal(symbol)
			}
		})
	}
	if symbol := (Result{}).GoSymbol(reference); symbol.Detail != reference.Detail || symbol.Declaration.Path != "" {
		t.Fatal(symbol)
	}
	if err := os.WriteFile(path, []byte("package api\nfunc Visible("), 0644); err != nil {
		t.Fatal(err)
	}
	if symbol := (Result{goImporter: navigationPositionImporter{position: token.Position{Filename: path, Line: 2}}}).GoSymbol(reference); symbol.Declaration.Path != "" {
		t.Fatal(symbol)
	}
}

func TestGoDeclarationDocumentationAndExactSpans(t *testing.T) {
	text := `package api
// Constants document the group.
const (
 // Answer is the result.
 Answer = 42
 Grouped = 43
)
// Values document variables.
var Value int
// Container holds fields.
type Container struct {
 // Item is documented.
 Item int
 Other int // Other has a trailing comment.
 *Embedded[int]
}
// Embedded is a generic type.
type Embedded[T any] struct{}
// Alias describes an alias.
type Alias = Container
// Reader describes the method contract.
type Reader interface {
 Read() int
}
`
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "api.go", text, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, needle, doc string }{
		{"Answer", "Answer =", "Answer is"}, {"Grouped", "Grouped =", "Constants document"},
		{"Value", "Value int", "Values document"}, {"Container", "Container struct", "Container holds"},
		{"Item", "Item int", "Item is"}, {"Other", "Other int", "trailing comment"},
		{"Embedded", "Embedded[int]", ""}, {"Alias", "Alias =", "Alias describes"},
		{"Read", "Read()", "Reader describes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			offset := strings.Index(text, test.needle)
			line := 1 + strings.Count(text[:offset], "\n")
			span, doc := goDeclarationAt(files, file, test.name, line)
			if span.Path != "api.go" || text[span.Start.Offset:span.End.Offset] != test.name || !strings.Contains(doc, test.doc) {
				t.Fatalf("%+v doc %q", span, doc)
			}
		})
	}
	for _, test := range []struct{ text, name string }{
		{"package api; type A struct{Value int};type B struct{Value int}", "Value"},
		{"package api; func A(){}", "Missing"},
	} {
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, "ambiguous.go", test.text, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if span, _ := goDeclarationAt(fs, f, test.name, 1); span.Path != "" {
			t.Fatal(span)
		}
	}
}

func TestGoEmbeddedIdentifiers(t *testing.T) {
	for _, test := range []struct{ input, want string }{{"Value", "Value"}, {"pkg.Value", "Value"}, {"*pkg.Value", "Value"}, {"Value[int]", "Value"}, {"Value[int,string]", "Value"}, {"[]int", ""}} {
		expression, err := parser.ParseExpr(test.input)
		if err != nil {
			t.Fatal(err)
		}
		id := goEmbeddedIdentifier(expression)
		if test.want == "" {
			if id != nil {
				t.Fatal(id)
			}
		} else if id == nil || id.Name != test.want {
			t.Fatalf("%s: %v", test.input, id)
		}
	}
}

func TestGoSourcePositionMissingToolchain(t *testing.T) {
	if p := goSourcePosition(nil, 1); p.IsValid() {
		t.Fatal(p)
	}
	if p := goSourcePosition(token.NewFileSet(), 0); p.IsValid() {
		t.Fatal(p)
	}
	fs := token.NewFileSet()
	file := fs.AddFile("$GOROOT/src/api.go", -1, 1)
	t.Setenv("PATH", t.TempDir())
	if p := goSourcePosition(fs, int(file.Pos(0))); p.IsValid() {
		t.Fatal(p)
	}
}
