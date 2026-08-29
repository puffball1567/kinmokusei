package main

import (
	"os"
	"path/filepath"
	"testing"

	"ontama.local/ontama/internal/product"
)

func TestBuildAndRunCommands(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("GOCACHE", filepath.Join(temp, "go-cache"))
	source := filepath.Join(temp, "main.otm")
	if err := os.WriteFile(source, []byte(`
function answer(): int { return 20 + 22; }
function main(): void { answer(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(temp, "program")
	if status := run([]string{"build", "-o", output, source}); status != 0 {
		t.Fatalf("build exit status = %d", status)
	}
	generated := filepath.Join(product.GeneratedDirectory(temp), "generated.go")
	if _, err := os.Stat(generated); err != nil {
		t.Fatalf("generated Go is missing: %v", err)
	}
	if info, err := os.Stat(output); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("built executable is missing or not executable: info=%v err=%v", info, err)
	}
	if status := run([]string{"run", source}); status != 0 {
		t.Fatalf("run exit status = %d", status)
	}
}
