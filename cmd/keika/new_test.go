package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCommandArguments(t *testing.T) {
	for _, args := range [][]string{
		{"new"}, {"new", "unknown"}, {"new", "app"}, {"new", "library", "one", "two"},
		{"new", "app", "--unknown"}, {"new", "app", "--license", "MIT", "app"},
		{"new", "app", "app", "--name", "wrong-order"},
	} {
		if status, _, stderr := captureRun(t, args...); status != 2 || stderr == "" {
			t.Fatalf("%v: status=%d stderr=%s", args, status, stderr)
		}
	}
	if status, _, stderr := captureRun(t, "new", "library", "--help"); status != 0 || !strings.Contains(stderr, "module") {
		t.Fatalf("help: status=%d stderr=%s", status, stderr)
	}
	if status, _, stderr := captureRun(t, "new", "app", t.TempDir()); status != 1 || !strings.Contains(stderr, "already exists") {
		t.Fatalf("existing: status=%d stderr=%s", status, stderr)
	}
}

func TestNewProjectsConnectThroughLocalReplacement(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "myapp")
	library := filepath.Join(base, "mylib")
	t.Setenv("GOPROXY", "off")
	for _, args := range [][]string{
		{"new", "app", app},
		{"new", "library", "--module", "pkg.test/greeting", "--license", "MIT", library},
	} {
		if status, out, stderr := captureRun(t, args...); status != 0 || !strings.Contains(out, "created ") || stderr != "" {
			t.Fatalf("%v: status=%d out=%s stderr=%s", args, status, out, stderr)
		}
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(app); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "check"); status != 0 {
		t.Fatalf("new app check: %s", stderr)
	}
	if status, out, stderr := captureRun(t, "run"); status != 0 || strings.TrimSpace(out) != "Hello from myapp" {
		t.Fatalf("new app run: %d %s %s", status, out, stderr)
	}
	if err := os.Chdir(library); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "check"); status != 0 {
		t.Fatalf("new library check: %s", stderr)
	}
	if status, out, stderr := captureRun(t, "emit-go", "-package", "library"); status != 0 || !strings.Contains(out, "package library") {
		t.Fatalf("new library emission: %d %s %s", status, out, stderr)
	}
	if err := os.Chdir(app); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "deps", "add", "--offline", "--replace", "../mylib", "pkg.test/greeting@v0.1.0"); status != 0 {
		t.Fatalf("new library dependency: %s", stderr)
	}
	input := "import { greet } from \"pkg.test/greeting\"\nimport go { Println } from \"fmt\"\nconst main = (): void => { Println(greet(\"Kinmokusei\")) }\n"
	if err := os.WriteFile(filepath.Join(app, "main.km"), []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "check"); status != 0 {
		t.Fatalf("consumer check: %s", stderr)
	}
	if status, out, stderr := captureRun(t, "run"); status != 0 || strings.TrimSpace(out) != "Hello, Kinmokusei!" {
		t.Fatalf("consumer run: %d %s %s", status, out, stderr)
	}
	if status, _, stderr := captureRun(t, "build"); status != 0 {
		t.Fatalf("consumer build: %s", stderr)
	}
}
