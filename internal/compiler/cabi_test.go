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
function add(left: int32, right: int32): int32 { return left + right; }
const ping = (): void => {};
const panics = (): int32 => { const values: int32[] = []; return values[0]; };
const logicalNot = (value: boolean): boolean => !value;
const increment8 = (value: int8): int8 => value + int8(1);
function add16(left: int16, right: int16): int16 { return left + right; }
function incrementByte(value: uint8): uint8 { return value + uint8(1); }
enum Mode: uint16 { Off, On = 9 }
enum Delta: int8 { Minimum = -128, Zero = 0, Maximum = 127 }
function toggleMode(value: Mode): Mode {
  if (value === Mode.Off) { return Mode.On; }
  if (value === Mode.On) { return Mode.Off; }
  return value;
}
function echoDelta(value: Delta): Delta { return value; }
export c(
  "ontama_add",
  "ontama_add16",
  "ontama_echo_delta",
  "ontama_increment_byte",
  "ontama_increment8",
  "ontama_mode",
  "ontama_not",
  "ontama_ping",
  "ontama_panics",
) {
  add,
  add16,
  echoDelta,
  incrementByte,
  increment8,
  toggleMode,
  logicalNot,
  ping,
  panics,
};
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
  uint8_t flag = 99;
  if (ontama_not(0, &flag) != ONTAMA_ABI_OK || flag != 1) return 16;
  if (ontama_not(1, &flag) != ONTAMA_ABI_OK || flag != 0) return 17;
  if (ontama_not(2, &flag) != ONTAMA_ABI_OK || flag != 0) return 18;
  if (ontama_not(255, &flag) != ONTAMA_ABI_OK || flag != 0) return 19;
  if (ontama_not(0, NULL) != ONTAMA_ABI_INVALID_ARGUMENT) return 20;
  int8_t narrow8 = 0;
  if (ontama_increment8(127, &narrow8) != ONTAMA_ABI_OK || narrow8 != -128) return 21;
  if (ontama_increment8(-128, &narrow8) != ONTAMA_ABI_OK || narrow8 != -127) return 22;
  int16_t narrow16 = 0;
  if (ontama_add16(32760, 7, &narrow16) != ONTAMA_ABI_OK || narrow16 != 32767) return 23;
  if (ontama_add16(-32760, -8, &narrow16) != ONTAMA_ABI_OK || narrow16 != -32768) return 24;
  uint8_t alias_byte = 99;
  if (ontama_increment_byte(255, &alias_byte) != ONTAMA_ABI_OK || alias_byte != 0) return 25;
  uint16_t mode = 99;
  if (ontama_mode(0, &mode) != ONTAMA_ABI_OK || mode != 9) return 26;
  if (ontama_mode(9, &mode) != ONTAMA_ABI_OK || mode != 0) return 27;
  if (ontama_mode(22, &mode) != ONTAMA_ABI_OK || mode != 22) return 28;
  int8_t delta = 0;
  if (ontama_echo_delta(-128, &delta) != ONTAMA_ABI_OK || delta != -128) return 29;
  if (ontama_echo_delta(127, &delta) != ONTAMA_ABI_OK || delta != 127) return 30;
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
		{"invalid export list", `const value = (input: string): void => {}; export c("value") {value};`, "unsupported type string"},
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

func TestCABIExportListResolvesImportedFunctionAndArrow(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "operations.otm")
	entry := filepath.Join(directory, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`
function add(left: int32, right: int32): int32 { return left + right; }
const sub = (left: int32, right: int32): int32 => left - right;
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { add, sub } from "./operations";
export c("ontama_add", "ontama_sub") {add, sub};
`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitCABI([]string{entry})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"//export ontama_add", "//export ontama_sub", "__ontamaResult := add(", "__ontamaResult := sub("} {
		if !strings.Contains(string(artifacts.Gateway), want) {
			t.Errorf("gateway missing %q:\n%s", want, artifacts.Gateway)
		}
	}
}

func TestCABIImportedEnumUsesLinkedFixedWidthTransport(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "operations.otm")
	entry := filepath.Join(directory, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`
enum Code: int32 { Empty, Ready = 41 }
function preserve(value: Code): Code { return value; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
enum Code: uint16 { Local }
import { preserve } from "./operations";
export c("ontama_preserve") {preserve};
`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitCABI([]string{entry})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(artifacts.Header), "int32_t ontama_preserve(int32_t arg0, int32_t *out_result);") {
		t.Fatalf("imported enum header:\n%s", artifacts.Header)
	}
	gateway := string(artifacts.Gateway)
	if !strings.Contains(gateway, "__ontamaResult := preserve(") || !strings.Contains(gateway, "Code(arg0))") || !strings.Contains(gateway, "*outResult = C.int32_t(__ontamaResult)") {
		t.Fatalf("imported enum gateway:\n%s", artifacts.Gateway)
	}
}
