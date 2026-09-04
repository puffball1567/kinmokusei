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
