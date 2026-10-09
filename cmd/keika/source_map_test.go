package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/compiler"
)

func TestEmitGoSourceMap(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.km")
	input := "function main(): void { let value = 1; }"
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	output, sidecar := filepath.Join(root, "output.go"), filepath.Join(root, "output.go.map.json")
	status, stdout, stderr := captureRun(t, "emit-go", "-o", output, "-source-map", sidecar, source)
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("emit: %d %s %s", status, stdout, stderr)
	}
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	var metadata compiler.GoSourceMap
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Version != 1 || metadata.Generated.File != "output.go" || metadata.SourceRoot != "." || len(metadata.Mappings) == 0 {
		t.Fatalf("invalid map: %s", data)
	}
	for _, args := range [][]string{
		{"emit-go", "-source-map", sidecar, source},
		{"emit-go", "-o", output, "-source-map", output, source},
		{"emit-go", "-o", output, "-source-map", source, source},
		{"emit-go", "-o", source, "-source-map", sidecar, source},
	} {
		if status, _, stderr := captureRun(t, args...); status == 0 || stderr == "" {
			t.Fatalf("bad output paths accepted: %v", args)
		}
	}
	after, _ := os.ReadFile(source)
	if string(after) != input {
		t.Fatal("source overwritten")
	}
	status, plain, stderr := captureRun(t, "emit-go", source)
	if status != 0 || stderr != "" || !strings.Contains(plain, "func main()") {
		t.Fatalf("legacy emission failed: %d %s %s", status, plain, stderr)
	}
}
