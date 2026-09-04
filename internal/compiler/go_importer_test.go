package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestEnvironmentWithReplacesExistingValue(t *testing.T) {
	t.Setenv("KINMOKUSEI_TEST_SETTING", "first")
	environment := environmentWith("KINMOKUSEI_TEST_SETTING", "final")
	replaced := environmentWithValues(environment, "KINMOKUSEI_TEST_SETTING", "final")
	if replaced != 1 {
		t.Fatalf("matching final values = %d, want 1", replaced)
	}
}

func TestContextualGoPackageDiagnosticMatrix(t *testing.T) {
	disabled := &moduleGoImporter{target: &project.BuildTarget{GOOS: "linux", GOARCH: "amd64", Tags: []string{}}}
	enabled := &moduleGoImporter{target: &project.BuildTarget{GOOS: "linux", GOARCH: "amd64", CGOEnabled: true, Tags: []string{}}}
	plain := &moduleGoImporter{}
	for _, test := range []struct {
		name     string
		importer *moduleGoImporter
		message  string
		want     string
	}{
		{"target constraints", disabled, "build constraints exclude all Go files", "has no files for target linux/amd64"},
		{"missing C compiler", enabled, `cgo: C compiler "missing-cc" not found`, "requires a working C toolchain"},
		{"target raw error", enabled, "native header missing", "CGO_ENABLED=1"},
		{"nonproject raw error", plain, "missing", `go list failed for "example.com/library"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.importer.contextualLoadError("example.com/library", test.message, nil).Error(); !strings.Contains(got, test.want) {
				t.Fatalf("diagnostic=%q, want %q", got, test.want)
			}
		})
	}
}

func TestIgnoredCgoFileDetectionMatrix(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		"cgo.go":       "package fixture\nimport \"C\"\n",
		"ordinary.go":  "package fixture\nimport \"fmt\"\nvar _ = fmt.Sprint\n",
		"malformed.go": "not Go source",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name  string
		files []string
		want  bool
	}{
		{"cgo", []string{"cgo.go"}, true},
		{"ordinary", []string{"ordinary.go"}, false},
		{"malformed", []string{"malformed.go"}, false},
		{"mixed", []string{"ordinary.go", "cgo.go"}, true},
		{"missing", []string{"missing.go"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			listing := listedGoPackage{Dir: root, IgnoredGoFiles: test.files}
			if got := packageListingHasIgnoredCgoFile(listing); got != test.want {
				t.Fatalf("detected=%v, want %v", got, test.want)
			}
		})
	}
}

func environmentWithValues(environment []string, name, value string) int {
	want := name + "=" + value
	prefix := name + "="
	count := 0
	for _, item := range environment {
		if item == want {
			count++
		} else if strings.HasPrefix(item, prefix) {
			return -1
		}
	}
	return count
}
