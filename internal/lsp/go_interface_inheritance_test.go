package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoInterfaceInheritanceCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contracts.km")
	input := `import go fmt from "fmt";
interface Named extends fmt.Stringer {}
interface Child<T> extends Named {function read():T;}
function use(value:Child<int>):string{return value.String();}
`
	position := positionOf(input, "String();", 0)
	completionText := strings.Replace(input, "value.String()", "value.", 1)
	items := completionLabels(completionItemsAt(t, path, completionText, position.Line, position.Character))
	if items["String"] == nil || !strings.Contains(items["String"]["detail"].(string), "String(): string") {
		t.Fatalf("completion=%#v", items)
	}
	if items["read"] == nil {
		t.Fatalf("source member missing: %#v", items)
	}
	position.Character += len("String(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, position))
	if label != "value.String(): string" {
		t.Fatalf("signature=%q", label)
	}
}
