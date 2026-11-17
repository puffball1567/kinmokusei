package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericInstantiationCycleCheckAndEmit(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "cycle.km")
	if err := os.WriteFile(input, []byte("function f<T>(x:T):void {\n f<T[]>([x]);\n}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"check", input}, {"check", "--json", input}, {"emit-go", input}} {
		status, out, stderr := captureRun(t, args...)
		if status != 1 || !strings.Contains(out+stderr, "instantiation cycle") || strings.Contains(out+stderr, "generated.go:") {
			t.Fatalf("%v: status=%d out=%s stderr=%s", args, status, out, stderr)
		}
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 1 || files[0].Name() != "cycle.km" {
		t.Fatalf("failed checks wrote artifacts: %v %v", files, err)
	}
	output, metadata := filepath.Join(root, "generated.go"), filepath.Join(root, "generated.go.map.json")
	for _, path := range []string{output, metadata} {
		if err := os.WriteFile(path, []byte("keep original output"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if status, _, stderr := captureRun(t, "emit-go", "-o", output, "-source-map", metadata, input); status != 1 || !strings.Contains(stderr, "instantiation cycle") {
		t.Fatalf("emit-go: status=%d stderr=%s", status, stderr)
	}
	for _, path := range []string{output, metadata} {
		contents, err := os.ReadFile(path)
		if err != nil || string(contents) != "keep original output" {
			t.Fatalf("failed cycle check replaced output: %v", err)
		}
	}
}
