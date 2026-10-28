package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestABIEnumCompatibilityCommand(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "library.km")
	writeSource := func(members string) {
		t.Helper()
		code := `enum Mode: int32 { ` + members + ` }
function open(mode: Mode): boolean { return mode == Mode.Write; }
export c("k_open") {open};`
		if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSource("Read = 0, Write = 1")
	output := filepath.Join(directory, "abi")
	if status, _, stderr := captureRun(t, "emit-c-abi", "-o", output, source); status != 0 {
		t.Fatalf("status=%d stderr=%s", status, stderr)
	}
	manifest := filepath.Join(output, "kinmokusei_abi.json")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		members string
		status  int
		want    string
	}{
		{"Write = 0, Read = 1", 1, `changed enum member "Write" of C ABI symbol "k_open" parameter 1 from 1 to 0`},
		{"Read = 0, Write = 1, Append = 2", 0, "added enum members"},
		{"Write = 1, Read = 0", 0, "exact fingerprint"},
	} {
		writeSource(test.members)
		status, stdout, stderr := captureRun(t, "abi", "check", "--baseline", manifest, source)
		if status != test.status || !strings.Contains(stdout+stderr, test.want) || test.status != 0 && stdout != "" || test.status == 0 && stderr != "" {
			t.Fatalf("members=%s status=%d stdout=%s stderr=%s", test.members, status, stdout, stderr)
		}
	}
	// A successful comparison must not rewrite the baseline into the new ABI.
	if data, err := os.ReadFile(manifest); err != nil || string(data) != string(original) {
		t.Fatalf("baseline modified: err=%v", err)
	}
	legacy := filepath.Join(directory, "legacy.json")
	if err := os.WriteFile(legacy, []byte(`{"schemaVersion":1,"functions":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "abi", "check", "--baseline", legacy, source); status != 1 || stdout != "" || !strings.Contains(stderr, "original baseline sources") {
		t.Fatalf("legacy status=%d stdout=%s stderr=%s", status, stdout, stderr)
	}
}
