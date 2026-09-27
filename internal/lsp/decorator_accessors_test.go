package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessorDecoratorInvocationTooling(t *testing.T) {
	t.Parallel()
	for _, context := range []string{"GetterDecoratorContext", "SetterDecoratorContext"} {
		t.Run(context, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "accessors.km")
			input := "function Capture(context:" + context + "):void{const invoke=context.invoke;const invokeStatic=context.invokeStatic;}"
			position := positionOf(input, "invoke;", 0)
			messages := serveMessages(t, openDocument(fileURI(path), input), requestAt("textDocument/hover", 2, fileURI(path), position, ""))
			hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
			if !strings.Contains(hover, "invoke:") || !strings.Contains(hover, "DecoratorValue") || !strings.Contains(hover, "Result<DecoratorValue>") {
				t.Fatalf("invoke hover=%q", hover)
			}
			completion := strings.Replace(input, "context.invoke;", "context.;", 1)
			position = positionOf(completion, "context.;", 0)
			position.Character += len("context.")
			items := completionLabels(completionItemsAt(t, path, completion, position.Line, position.Character))
			for _, field := range []string{"invoke", "invocable", "invokeStatic", "staticInvocable", "invokeUnavailableReason", "staticInvokeUnavailableReason"} {
				if items[field] == nil {
					t.Errorf("missing %s in %s completion", field, context)
				}
			}
		})
	}
}
