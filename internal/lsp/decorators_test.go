package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDecoratorContextHoverAndCompletion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "decorators.km")
	uri := fileURI(path)
	input := `function Register(): (context:DecoratorContext)=>void {
  return (context) => {
    const kind = context.kind;
  };
}
@Register() class Service{}
`
	typePosition := positionOf(input, "DecoratorContext", 0)
	memberPosition := positionOf(input, "kind;", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, typePosition, ""),
		requestAt("textDocument/hover", 3, uri, memberPosition, ""),
		requestAt("textDocument/definition", 4, uri, typePosition, ""),
	)
	typeHover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(typeHover, "type DecoratorContext") || !strings.Contains(typeHover, "parameterIndex: int") {
		t.Fatalf("type hover=%q", typeHover)
	}
	memberHover := messages[3]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(memberHover, "kind: string") {
		t.Fatalf("member hover=%q", memberHover)
	}
	if messages[4]["result"] != nil {
		t.Fatalf("built-in definition=%v", messages[4])
	}

	completion := strings.Replace(input, "context.kind", "context.", 1)
	position := positionOf(completion, "context.", 0)
	position.Character += len("context.")
	items := completionLabels(completionItemsAt(t, path, completion, position.Line, position.Character))
	for _, field := range []string{"kind", "identity", "className", "memberName", "parameterName", "parameterIndex", "static", "visibility", "valueType"} {
		if items[field] == nil {
			t.Errorf("missing %s completion: %#v", field, items)
		}
	}
	lexical := completionLabels(completionItemsAt(t, path, input, 0, 0))
	if lexical["DecoratorContext"] == nil {
		t.Fatalf("missing built-in type completion: %#v", lexical)
	}
}
