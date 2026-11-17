package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestNullCoalescingCompletionType(t *testing.T) {
	t.Parallel()
	ref := ast.TypeRef{Name: "Reader", Interface: true, Nullable: true, GenericArguments: []ast.TypeRef{{Name: "int"}}}
	got, ok := expressionCompletionType(&ast.Program{}, &ast.BinaryExpr{Operator: "??", CoalesceType: &ref})
	if !ok || got.Name != "Reader" || !got.Nullable || !got.Interface || len(got.GenericArguments) != 1 || got.GenericArguments[0].Name != "int" {
		t.Fatalf("lost checked result type: %#v %v", got, ok)
	}
}

func TestNullCoalescingInferredLocalNavigation(t *testing.T) {
	t.Parallel()
	input := `class Box{constructor(public value:int){}}function f(box:Box|null):int{const checked=box??new Box(9);return checked.value;}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, positionOf(input, "checked.value", 0), ""), requestAt("textDocument/definition", 3, uri, positionOf(input, "value;", 0), ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Box") {
		t.Fatalf("hover: %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "value")) {
		t.Fatalf("definition: %v", messages[3])
	}
}

func TestNullCoalescingCompletionDoesNotUseCallArgument(t *testing.T) {
	t.Parallel()
	input := `class Box{public value:int=1;}class Other{public name:string="other";}function convert(v:Box):Other{return new Other();}function f(box:Box|null):void{convert(box??new Box()).va;}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	at := positionOf(input, ".va;", 0)
	at.Character += 3
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/completion", 2, uri, at, ""))
	for _, raw := range messages[2]["result"].([]any) {
		if raw.(map[string]any)["label"] == "value" {
			t.Fatalf("argument type leaked into call result: %v", messages[2])
		}
	}
}
