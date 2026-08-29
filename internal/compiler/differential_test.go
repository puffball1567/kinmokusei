package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// runGeneratedGoDifferentialTest compiles generated OnsenTamago output beside
// an independently handwritten Go reference implementation and runs a test
// which compares their observable behavior. The reference lives in a separate
// package and may not import any package from the generated module, preventing
// generated declarations from being reused as the expected-value oracle.
func runGeneratedGoDifferentialTest(t *testing.T, root, modulePath string, generated []byte, referenceSource, testSource string) {
	t.Helper()
	runGeneratedGoDifferentialTestWithModule(t, root, modulePath, "module "+modulePath+"\n\ngo 1.23\n", generated, referenceSource, testSource)
}

func runGeneratedGoDifferentialTestWithModule(t *testing.T, root, modulePath, goModule string, generated []byte, referenceSource, testSource string) {
	t.Helper()
	runGeneratedGoDifferentialTestConfigured(t, root, modulePath, &goModule, generated, referenceSource, testSource, nil, nil)
}

// runGeneratedGoDifferentialTestInExistingModule adds the isolated reference
// package to an already generated module. It preserves that module's locked
// graph and accepts the exact test flags and target environment required by
// project, build-tag, cgo, and unsafe interop fixtures.
func runGeneratedGoDifferentialTestInExistingModule(
	t *testing.T,
	root, modulePath string,
	generated []byte,
	referenceSource, testSource string,
	testArguments, extraEnvironment []string,
) {
	t.Helper()
	runGeneratedGoDifferentialTestConfigured(t, root, modulePath, nil, generated, referenceSource, testSource, testArguments, extraEnvironment)
}

func runGeneratedGoDifferentialTestConfigured(
	t *testing.T,
	root, modulePath string,
	goModule *string,
	generated []byte,
	referenceSource, testSource string,
	testArguments, extraEnvironment []string,
) {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), "reference.go", referenceSource, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse independent Go reference: %v", err)
	}
	for _, imported := range parsed.Imports {
		path, unquoteErr := strconv.Unquote(imported.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("read independent Go reference import: %v", unquoteErr)
		}
		if path == modulePath || strings.HasPrefix(path, modulePath+"/") {
			t.Fatalf("independent Go reference must not import generated module %q", modulePath)
		}
	}
	testFile, err := parser.ParseFile(token.NewFileSet(), "generated_test.go", testSource, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse generated/reference comparison test: %v", err)
	}
	referenceImport := modulePath + "/reference"
	importsReference := false
	for _, imported := range testFile.Imports {
		path, unquoteErr := strconv.Unquote(imported.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("read comparison test import: %v", unquoteErr)
		}
		if path == referenceImport {
			importsReference = true
		}
	}
	if !importsReference {
		t.Fatalf("generated/reference comparison test must import independent reference package %q", referenceImport)
	}

	referenceDirectory := filepath.Join(root, "reference")
	if err := os.MkdirAll(referenceDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		filepath.Join(root, "generated.go"):               generated,
		filepath.Join(root, "generated_test.go"):          []byte(testSource),
		filepath.Join(referenceDirectory, "reference.go"): []byte(referenceSource),
	}
	if goModule != nil {
		files[filepath.Join(root, "go.mod")] = []byte(*goModule)
	}
	for path, contents := range files {
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	arguments := append([]string(nil), testArguments...)
	if len(arguments) == 0 {
		arguments = []string{"test", "./..."}
	}
	if os.Getenv("ONTAMA_DIFFERENTIAL_RACE") == "1" && environmentValue(extraEnvironment, "CGO_ENABLED") != "0" && len(arguments) != 0 && arguments[0] == "test" && !containsArgument(arguments, "-race") {
		arguments = append([]string{"test", "-race"}, arguments[1:]...)
	}
	command := exec.Command("go", arguments...)
	command.Dir = root
	goCache := os.Getenv("GOCACHE")
	if goCache == "" {
		goCache = filepath.Join(root, "go-cache")
	}
	command.Env = append(os.Environ(), "GOCACHE="+goCache)
	command.Env = append(command.Env, extraEnvironment...)
	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		t.Fatalf("generated/reference differential test failed: %v\n%s\n%s", commandErr, output, generated)
	}
}

func containsArgument(arguments []string, target string) bool {
	for _, argument := range arguments {
		if argument == target {
			return true
		}
	}
	return false
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for index := len(environment) - 1; index >= 0; index-- {
		if strings.HasPrefix(environment[index], prefix) {
			return strings.TrimPrefix(environment[index], prefix)
		}
	}
	return ""
}
