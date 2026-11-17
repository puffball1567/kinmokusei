package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFFIGeneratedNameCollisionDoesNotWriteOutput(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "binding.json")
	const invalid = `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Data","lifetime":"registered","parameters":[],"result":"ownedBytes"},{"name":"Data_release","lifetime":"callScoped","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Data","register":"fixture_register","unregister":"fixture_unregister"}],"functions":[{"name":"Run","symbol":"fixture_run","parameters":[{"name":"callback","type":"Data_release"}],"result":"void","convention":"direct"}]}`
	if err := os.WriteFile(manifest, []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new directory", true: "existing binding"}[existing], func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "generated")
			generated := filepath.Join(output, "generated_ffi.go")
			const original = "package binding\n// existing binding\n"
			if existing {
				if err := os.MkdirAll(output, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(generated, []byte(original), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			status, stdout, stderr := captureRun(t, "ffi", "generate", "--manifest", manifest, "-o", output)
			if status != 1 || stdout != "" || !strings.Contains(stderr, "kinmokusei_cffi_callback_Data_release_fn") || !strings.Contains(stderr, `callback "Data"`) || !strings.Contains(stderr, `callback "Data_release"`) {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
			}
			if !existing {
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("invalid manifest created output directory: %v", err)
				}
			} else if after, err := os.ReadFile(generated); err != nil || string(after) != original {
				t.Fatalf("invalid manifest changed existing binding: %v", err)
			}
		})
	}
}
