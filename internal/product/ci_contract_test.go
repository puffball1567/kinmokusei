package product

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompatibilityWorkflowQualityGates(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	workflowPath := filepath.Join(filepath.Dir(current), "..", "..", ".github", "workflows", "compatibility.yml")
	contents, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(contents)
	for _, version := range []string{"1.23.x", "1.24.x", "1.25.x", "1.26.x", "1.27.x"} {
		if !strings.Contains(workflow, "go-version: "+version) || !strings.Contains(workflow, "- "+version) {
			t.Errorf("Go %s is not covered by both runtime and release-toolchain matrices", version)
		}
	}
	for _, target := range []string{
		"FuzzLexNeverPanics",
		"FuzzParseNeverPanics",
		"FuzzApplyContentChangesNeverPanics",
		"FuzzRelatedDeclarationsNeverPanics",
		"FuzzCallContextAtNeverPanics",
		"FuzzCompilePipelineNeverPanics",
	} {
		if !strings.Contains(workflow, "target: "+target) {
			t.Errorf("compatibility workflow does not explore %s", target)
		}
	}
	for _, required := range []string{
		"-fuzz=${{ matrix.target }}",
		"KINMOKUSEI_REQUIRE_C_TESTS: \"1\"",
		"TestCABISharedLibraryCompileAndCCallerMatrix|TestIncomingCFFI.*",
		"BenchmarkKinmokusei(Check|Codegen)",
		"TestServerLifecycleDoesNotLeakGoroutines",
		"TestStructuredTasksMatchIndependentGo",
		"C_FFI_RESULT: ${{ needs.c_ffi.result }}",
		"GOROUTINE_LEAK_RESULT: ${{ needs.goroutine_leaks.result }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("compatibility workflow is missing required quality gate %q", required)
		}
	}
}
