package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestAnonymousGoInterfaceCompletionAndSignature(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "contracts")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): "[project]\nname = \"anonymous\"\nversion = \"0.1.0\"\ngo-module = \"anonymous-editor.test\"\ngo-version = \"1.23\"\n",
		filepath.Join(library, "go.mod"):             "module anonymous-editor.test/contracts\n\ngo 1.23\n",
		filepath.Join(library, "contracts.go"):       "package contracts\nfunc New()interface{Read(int)string;Pair()(int,string)}{return nil}\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.AddDependency(root, "anonymous-editor.test/contracts", "v0.0.0", "./contracts", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.km")
	input := `import go c from "anonymous-editor.test/contracts";
function use():string{const value=c.New();return value.Read(1);}
`
	position := positionOf(input, "Read(1)", 0)
	completionText := strings.Replace(input, "value.Read(1)", "value.", 1)
	items := completionLabels(completionItemsAt(t, path, completionText, position.Line, position.Character))
	if items["Read"] == nil || !strings.Contains(items["Read"]["detail"].(string), "(int) => string") {
		t.Fatalf("completion=%#v", items)
	}
	if items["Pair"] == nil || !strings.Contains(items["Pair"]["detail"].(string), "(int, string)") {
		t.Fatalf("multiple-result completion=%#v", items)
	}
	position.Character += len("Read(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, position))
	if !strings.Contains(label, "value.Read(") || !strings.Contains(label, "int") || !strings.HasSuffix(label, ": string") {
		t.Fatalf("signature=%q", label)
	}
}
