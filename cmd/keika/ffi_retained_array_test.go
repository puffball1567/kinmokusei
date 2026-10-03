package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncomingCFFIGenerateRetainedArrayCommand(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "binding.json")
	output := filepath.Join(root, "generated")
	const declaration = `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove","parameters":[{"name":"values","type":"retainedArray","element":"int32"}]}]}`
	if err := os.WriteFile(manifest, []byte(declaration), 0o644); err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr := captureRun(t, "ffi", "generate", "--manifest", manifest, "-o", output)
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	generatedPath := filepath.Join(output, "generated_ffi.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "func RegisterWatch(values []int32, callback Visit) (*Watch, error)") {
		t.Fatalf("missing typed registration signature:\n%s", generated)
	}
	// A bad ownership/element declaration must not replace a usable binding.
	invalid := strings.Replace(declaration, `"element":"int32"`, `"element":"cstring"`, 1)
	if err := os.WriteFile(manifest, []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr = captureRun(t, "ffi", "generate", "--manifest", manifest, "-o", output)
	if status == 0 || stdout != "" || !strings.Contains(stderr, "requires a supported element") {
		t.Fatalf("invalid manifest status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	after, err := os.ReadFile(generatedPath)
	if err != nil || string(after) != string(generated) {
		t.Fatalf("invalid manifest changed generated binding: err=%v", err)
	}
}
