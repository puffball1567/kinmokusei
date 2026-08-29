package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorImportedConstantProofsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	proofs := filepath.Join(temp, "proofs.otm")
	entry := filepath.Join(temp, "entry.otm")
	if err := os.WriteFile(proofs, []byte(`
const internalPrefix = "温";
const importedEnabled = 2 * 3 === 6;
const importedCount = 1 + 2;
const importedText = internalPrefix + "泉";
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { importedEnabled, importedCount, importedText } from "./proofs";

class User { constructor(public name: string) {} }
class ImportedHolder {
  private booleanUser: User;
  private sliceUser: User;
  private stringUser: User;
  private sliceVisits: int[];
  private runes: int32[];
  constructor() {
    this.sliceVisits = [];
    this.runes = [];
    while (importedEnabled) {
      this.booleanUser = new User("imported-boolean");
      break;
    }
    const source = makeSlice[int](importedCount);
    for (const value of source) {
      this.sliceUser = new User("imported-cardinality");
      this.sliceVisits = append(this.sliceVisits, value);
    }
    for (const rune of importedText) {
      this.stringUser = new User("imported-string");
      this.runes = append(this.runes, rune);
    }
  }
  public function names(): string {
    return this.booleanUser.name + ":" + this.sliceUser.name + ":" + this.stringUser.name;
  }
  public function counts(): int { return len(this.sliceVisits) * 10 + len(this.runes); }
}

function importedNames(): string { return new ImportedHolder().names(); }
function importedCounts(): int { return new ImportedHolder().counts(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	generated, diagnostics, err := EmitGo([]string{entry}, "constructorimportedconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "for importedEnabled") {
		t.Fatalf("generated Go does not retain the imported boolean condition:\n%s", generated)
	}

	referenceSource := `package reference

const internalPrefix = "温"
const importedEnabled = 2*3 == 6
const importedCount = 1+2
const importedText = internalPrefix + "泉"

func Imported() (string, int) {
  booleanName, sliceName, stringName := "", "", ""
  sliceVisits, runes := 0, 0
  for importedEnabled { booleanName = "imported-boolean"; break }
  source := make([]int, importedCount)
  for range source { sliceName = "imported-cardinality"; sliceVisits++ }
  for range importedText { stringName = "imported-string"; runes++ }
  return booleanName + ":" + sliceName + ":" + stringName, sliceVisits*10 + runes
}
`
	testSource := `package constructorimportedconstants

import (
  "testing"
  reference "constructor-imported-constants.test/reference"
)

func TestConstructorImportedConstants(t *testing.T) {
  wantNames, wantCounts := reference.Imported()
  if got := importedNames(); got != wantNames { t.Fatalf("importedNames = %q, equivalent Go = %q", got, wantNames) }
  if got := importedCounts(); got != wantCounts { t.Fatalf("importedCounts = %d, equivalent Go = %d", got, wantCounts) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-imported-constants.test", generated, referenceSource, testSource)
}

func TestConstructorImportedConstantProofsRejectDynamicOrEmptyValues(t *testing.T) {
	tests := []struct {
		name       string
		dependency string
		entry      string
	}{
		{
			name:       "dynamic imported boolean",
			dependency: `function readEnabled(): boolean { return true; } const importedEnabled = readEnabled();`,
			entry:      `import { importedEnabled } from "./proofs"; class User {} class Holder { private user: User; constructor() { while (importedEnabled) { this.user = new User(); break; } } }`,
		},
		{
			name:       "empty imported string",
			dependency: `const internalText = ""; const importedText = internalText + "";`,
			entry:      `import { importedText } from "./proofs"; class User {} class Holder { private user: User; constructor() { for (const rune of importedText) { this.user = new User(); } } }`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temp := t.TempDir()
			proofs := filepath.Join(temp, "proofs.otm")
			entry := filepath.Join(temp, "entry.otm")
			if err := os.WriteFile(proofs, []byte(test.dependency), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(entry, []byte(test.entry), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := CheckFiles([]string{entry})
			if err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(result.Diagnostics))
			for index, diagnostic := range result.Diagnostics {
				messages[index] = diagnostic.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), "every constructor path") {
				t.Fatalf("diagnostics = %v, want incomplete constructor path", messages)
			}
		})
	}
}
