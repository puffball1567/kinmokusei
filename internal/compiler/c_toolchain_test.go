package compiler

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const requireCCompilerEnvironment = "KINMOKUSEI_REQUIRE_C_TESTS"

func requireCCompilerInCI() bool {
	return os.Getenv(requireCCompilerEnvironment) == "1"
}

func requireCCompiler(t *testing.T) string {
	t.Helper()
	compiler := os.Getenv("CC")
	if compiler == "" {
		compiler = "cc"
	}
	if _, err := exec.LookPath(compiler); err != nil {
		if requireCCompilerInCI() {
			t.Fatalf("required C compiler %q is unavailable: %v", compiler, err)
		}
		t.Skipf("C compiler %q is unavailable", compiler)
	}
	return compiler
}

func TestRequiredCCompilerModeCannotSkip(t *testing.T) {
	const childEnvironment = "KINMOKUSEI_MISSING_C_COMPILER_CHILD"
	if os.Getenv(childEnvironment) == "1" {
		requireCCompiler(t)
		t.Fatal("required C compiler check unexpectedly returned")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestRequiredCCompilerModeCannotSkip$")
	command.Env = append(os.Environ(),
		childEnvironment+"=1",
		requireCCompilerEnvironment+"=1",
		"CC=kinmokusei-intentionally-missing-c-compiler",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("required C compiler child unexpectedly passed:\n%s", output)
	}
	if !strings.Contains(string(output), "required C compiler") || !strings.Contains(string(output), "kinmokusei-intentionally-missing-c-compiler") {
		t.Fatalf("required C compiler failure is not actionable: err=%v output=%s", err, output)
	}
}
