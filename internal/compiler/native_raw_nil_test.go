package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
)

func TestNativeRawNilDiagnosticInImportedModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, library := filepath.Join(root, "entry.km"), filepath.Join(root, "models.km")
	files := map[string]string{
		entry:   `import {load} from "./models";function main():void{const user=load();}`,
		library: "export class User{}\nexport function load():User{return nil;}",
	}
	result, err := CheckFilesWithOverlay([]string{entry}, files)
	if err != nil || len(result.Diagnostics) != 1 {
		t.Fatalf("check err=%v diagnostics=%v", err, result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Span.Path != library || diagnostic.Span.Start.Line != 2 || !strings.Contains(diagnostic.Message, "cannot use nil as") {
		t.Fatalf("diagnostic lost original module location: %v", diagnostic)
	}
	files[library] = "export class User{}\nexport function load():User|null{return null;}"
	result, err = CheckFilesWithOverlay([]string{entry}, files)
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("nullable replacement rejected: %v %v", err, result.Diagnostics)
	}
	if _, err := codegen.GenerateWithTarget(result.Program, "fixture", result.goImporter, result.goSizes); err != nil {
		t.Fatalf("nullable replacement generated invalid Go: %v", err)
	}
}
