package compiler

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCABISharedLibraryCompileAndCCallerMatrix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shared library fixture currently targets Unix-like C toolchains")
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skipf("C compiler %q is unavailable", cc)
	}
	if output, err := exec.Command("go", "env", "CGO_ENABLED").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "1" {
		t.Skipf("cgo is unavailable: %v %s", err, output)
	}

	directory := t.TempDir()
	source := filepath.Join(directory, "library.otm")
	input := `
export c("ontama_add") function add(left: int32, right: int32): int32 { return left + right; }
export c("ontama_ping") function ping(): void {}
export c("ontama_panics") function panics(): int32 { const values: int32[] = []; return values[0]; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitCABI([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	var manifest struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(artifacts.Manifest, &manifest); err != nil || manifest.Fingerprint != artifacts.Fingerprint || !strings.Contains(string(artifacts.Header), `#define ONTAMA_ABI_FINGERPRINT "`+artifacts.Fingerprint+`"`) {
		t.Fatalf("manifest/header fingerprint mismatch: manifest=%#v artifact=%q err=%v", manifest, artifacts.Fingerprint, err)
	}
	files := map[string][]byte{
		"generated.go":      artifacts.GoSource,
		"generated_cabi.go": artifacts.Gateway,
		"ontama_abi.h":      artifacts.Header,
		"ontama_abi.json":   artifacts.Manifest,
		"go.mod":            []byte("module cabi.test\n\ngo 1.23\n"),
		"caller.c": []byte(`#include <stdint.h>
#include <stdio.h>
#include <pthread.h>
#include <string.h>
#include "ontama_abi.h"

static void *call_add_repeatedly(void *raw_id) {
  int32_t id = (int32_t)(intptr_t)raw_id;
  for (int32_t index = 0; index < 100; index++) {
    int32_t result = 0;
    if (ontama_add(id, index, &result) != ONTAMA_ABI_OK || result != id + index) return (void *)(intptr_t)1;
  }
  return NULL;
}

int main(void) {
	if (strncmp(ONTAMA_ABI_FINGERPRINT, "sha256:", 7) != 0 || strlen(ONTAMA_ABI_FINGERPRINT) != 71) return 9;
  int32_t result = 0;
  if (ontama_add(20, 22, &result) != ONTAMA_ABI_OK || result != 42) return 10;
  if (ontama_add(1, 2, NULL) != ONTAMA_ABI_INVALID_ARGUMENT) return 11;
  if (ontama_ping() != ONTAMA_ABI_OK) return 12;
  if (ontama_panics(&result) != ONTAMA_ABI_PANIC) return 13;
  pthread_t threads[8];
  for (intptr_t index = 0; index < 8; index++) {
    if (pthread_create(&threads[index], NULL, call_add_repeatedly, (void *)index) != 0) return 14;
  }
  for (int index = 0; index < 8; index++) {
    void *thread_result = NULL;
    if (pthread_join(threads[index], &thread_result) != 0 || thread_result != NULL) return 15;
  }
  puts("c-abi-ok");
  return 0;
}
`),
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(directory, name), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	libraryName := "libontama.so"
	if runtime.GOOS == "darwin" {
		libraryName = "libontama.dylib"
	}
	build := exec.Command("go", "build", "-buildmode=c-shared", "-buildvcs=false", "-o", libraryName, ".")
	build.Dir = directory
	build.Env = append(os.Environ(), "GOCACHE="+filepath.Join(directory, "go-cache"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("c-shared build failed: %v\n%s\n--- gateway ---\n%s", err, output, artifacts.Gateway)
	}
	compile := exec.Command(cc, "-pthread", "-I", directory, filepath.Join(directory, "caller.c"), "-L", directory, "-lontama", "-o", filepath.Join(directory, "caller"))
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("C caller compilation failed: %v\n%s\n--- header ---\n%s", err, output, artifacts.Header)
	}
	caller := exec.Command(filepath.Join(directory, "caller"))
	caller.Env = append(os.Environ(), "LD_LIBRARY_PATH="+directory, "DYLD_LIBRARY_PATH="+directory)
	if output, err := caller.CombinedOutput(); err != nil || string(output) != "c-abi-ok\n" {
		t.Fatalf("C caller failed: %v output=%q", err, output)
	}
}

func TestEmitCABIFailureMatrix(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"no exports", `function main(): void {}`, "does not declare any C ABI exports"},
		{"invalid export", `export c("value") function value(input: string): void {}`, "unsupported type string"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(directory, test.name+".otm")
			if err := os.WriteFile(path, []byte(test.source), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitCABI([]string{path})
			combined := ""
			if err != nil {
				combined = err.Error()
			}
			for _, item := range diagnostics {
				combined += "\n" + item.Message
			}
			if !strings.Contains(combined, test.want) {
				t.Fatalf("err=%v diagnostics=%v want=%q", err, diagnostics, test.want)
			}
		})
	}
}
