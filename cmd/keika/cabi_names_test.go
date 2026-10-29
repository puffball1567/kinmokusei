package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitCABIReservedNamesDoNotWriteOutput(t *testing.T) {
	for _, symbol := range []string{"double", "int32_t", "GoString", "malloc", "recover", "nil", "KINMOKUSEI_ABI_OK", "Exception"} {
		t.Run(symbol, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "library.km")
			source := fmt.Sprintf(`function impl(): int32 { throw new Exception("fail"); } export c(%q) {impl};`, symbol)
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(directory, "abi")
			checkFailure := func() {
				t.Helper()
				status, stdout, stderr := captureRun(t, "emit-c-abi", "-o", output, path)
				if status != 1 || stdout != "" || !strings.Contains(stderr, symbol) || !strings.Contains(stderr, "library.km:") {
					t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
				}
			}
			checkFailure()
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("invalid ABI created output: %v", err)
			}
			if err := os.Mkdir(output, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"generated.go", "generated_cabi.go", "kinmokusei_abi.h", "kinmokusei_abi.json"} {
				if err := os.WriteFile(filepath.Join(output, name), []byte("keep"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			checkFailure()
			for _, name := range []string{"generated.go", "generated_cabi.go", "kinmokusei_abi.h", "kinmokusei_abi.json"} {
				if data, err := os.ReadFile(filepath.Join(output, name)); err != nil || string(data) != "keep" {
					t.Fatalf("%s was modified: %v", name, err)
				}
			}
		})
	}
}
