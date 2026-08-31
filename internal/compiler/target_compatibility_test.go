package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/project"
)

func TestGeneratedGoTargetCompatibilityMatrix(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "main.otm")
	contents := `
import go strconv from "strconv";
import go strings from "strings";
function maximumUnsigned(): uint { return ^uint(0); }
function aliases(value: uint8, codepoint: int32): int32 { return int32(value) + codepoint; }
function main(): void { strings.ToUpper(strconv.Itoa(int(maximumUnsigned() >> 1))); aliases(255, int32(65)); }
`
	if err := os.WriteFile(source, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "main")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: err=%v diagnostics=%v", err, diagnostics)
	}
	if err := os.WriteFile(filepath.Join(directory, "generated.go"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module target-matrix.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		goos   string
		goarch string
		magic  string
	}{
		{goos: "linux", goarch: "amd64", magic: "\x7fELF"},
		{goos: "linux", goarch: "arm64", magic: "\x7fELF"},
		{goos: "darwin", goarch: "arm64", magic: "\xcf\xfa\xed\xfe"},
		{goos: "windows", goarch: "amd64", magic: "MZ"},
	}
	for _, test := range tests {
		t.Run(test.goos+"_"+test.goarch, func(t *testing.T) {
			target := project.BuildTarget{
				GOOS: test.goos, GOARCH: test.goarch, Tags: []string{},
			}
			output := filepath.Join(directory, test.goos+"-"+test.goarch)
			command := exec.Command("go", "build", "-buildvcs=false", "-trimpath", "-o", output, ".")
			command.Dir = directory
			command.Env = target.Environment(os.Environ())
			if buildOutput, buildErr := command.CombinedOutput(); buildErr != nil {
				t.Fatalf("cross-build: %v\n%s\n--- generated ---\n%s", buildErr, buildOutput, generated)
			}
			binary, readErr := os.ReadFile(output)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !strings.HasPrefix(string(binary), test.magic) {
				t.Fatalf("binary prefix = % x, want % x", binary[:min(4, len(binary))], test.magic)
			}
		})
	}
}
