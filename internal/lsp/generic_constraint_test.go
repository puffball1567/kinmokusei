package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestGenericConstraintDeclarationDetailsPreserveSourceSyntax(t *testing.T) {
	constraint := ast.TypeRef{Name: "comparable"}
	parameters := []ast.TypeParameter{{Name: "T", Constraint: &constraint}, {Name: "U"}}
	if got, want := formatTypeParameters(parameters), "<T extends comparable, U>"; got != want {
		t.Fatalf("formatTypeParameters() = %q, want %q", got, want)
	}
	function := &ast.FunctionDecl{
		Name: "equal", TypeParameters: parameters,
		Parameters: []ast.Parameter{{Name: "left", Type: ast.TypeRef{Name: "T"}}, {Name: "right", Type: ast.TypeRef{Name: "T"}}},
		ReturnType: ast.TypeRef{Name: "boolean"},
	}
	if got, want := functionDeclarationDetail(function), "function equal<T extends comparable, U>(left: T, right: T): boolean"; got != want {
		t.Fatalf("functionDeclarationDetail() = %q, want %q", got, want)
	}
}

func TestGoTypeSetConstraintNavigationAndImportRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go_constraint.km")
	uri := fileURI(path)
	text := `import go cmp from "cmp";
function minimum<T extends cmp.Ordered>(left: T, right: T): T {
  if (left < right) { return left; }
  return right;
}
function use(): int { return minimum(2, 1); }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "minimum", 1), ""),
		requestAt("textDocument/references", 3, uri, positionOf(text, "cmp", 2), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 4, uri, positionOf(text, "cmp", 2), `"newName":"ordering"`),
		requestAt("textDocument/prepareRename", 5, uri, positionOf(text, "Ordered", 0), ""),
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "function minimum<T extends cmp.Ordered>(left: T, right: T): T") {
		t.Fatalf("constraint hover = %q", hover)
	}
	if got := len(messages[3]["result"].([]any)); got != 2 {
		t.Fatalf("constraint alias references = %d, want import and constraint", got)
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 2 {
		t.Fatalf("constraint alias rename edits = %d, want import and constraint: %#v", got, changes)
	}
	if messages[5]["result"] != nil {
		t.Fatalf("external Go constraint prepareRename = %#v, want null", messages[5])
	}
}

func TestSourceTypeSetConstraintNavigationAndRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source_constraint.km")
	uri := fileURI(path)
	text := `constraint Integer = ~int | ~int8;
function doubled<T extends Integer>(value: T): T { return value + value; }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "Integer", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "Integer", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "Integer", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "Integer", 1), `"newName":"Whole"`),
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "constraint Integer = ~int | ~int8") {
		t.Fatalf("constraint hover = %q", hover)
	}
	definition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(0) || definition["character"] != float64(11) {
		t.Fatalf("constraint definition = %#v", definition)
	}
	if got := len(messages[4]["result"].([]any)); got != 2 {
		t.Fatalf("constraint references = %d, want declaration and use", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 2 {
		t.Fatalf("constraint rename edits = %d, want 2", got)
	}
	items := completionLabels(completionItemsAt(t, path, text, 0, 0))
	if detail := items["constraint"]["detail"]; detail != "keyword" {
		t.Fatalf("constraint keyword completion detail = %v", detail)
	}
	if detail := items["Integer"]["detail"]; detail != "constraint Integer = ~int | ~int8" {
		t.Fatalf("constraint declaration completion detail = %v", detail)
	}
}
