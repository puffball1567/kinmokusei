package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCABIGatewayNameHygiene(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shared library fixture targets Unix-like C toolchains")
	}
	cc := requireCCompiler(t)
	if output, err := exec.Command("go", "env", "CGO_ENABLED").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "1" {
		if requireCCompilerInCI() {
			t.Fatalf("required cgo toolchain unavailable: %v %s", err, output)
		}
		t.Skipf("cgo unavailable: %v %s", err, output)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "library.km")
	source := `
import go C from "fmt";
function log(): void { C.Println(1); }
function outResult(v: int32): int32 {
  if (v < 0) { throw new Exception("negative"); }
  return v + 3;
}
function arg0(v: int32): int32 { return v + 7; }
const arg1 = (v: int32, w: int32): int32 => v + w;
function recover(v: int32): int32 { return v + 11; }
export c("k_out", "GoMyThing", "k_args", "k_recover") {outResult, arg0, arg1, recover};
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitCABI([]string{path})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for name, data := range map[string][]byte{
		"go.mod":            []byte("module names.test\n\ngo 1.23\n"),
		"generated.go":      artifacts.GoSource,
		"generated_cabi.go": artifacts.Gateway,
		"kinmokusei_abi.h":  artifacts.Header,
		"caller.c": []byte(`#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include "kinmokusei_abi.h"
int main(void) {
  int32_t result = -1;
  if (k_out(10, &result) != KINMOKUSEI_ABI_OK || result != 13) return 1;
  if (GoMyThing(10, &result) != KINMOKUSEI_ABI_OK || result != 17) return 2;
  if (k_args(10, 20, &result) != KINMOKUSEI_ABI_OK || result != 30) return 3;
  if (k_recover(10, &result) != KINMOKUSEI_ABI_OK || result != 21) return 4;
  if (k_out(-1, &result) != KINMOKUSEI_ABI_PANIC) return 5;
  if (k_out(10, NULL) != KINMOKUSEI_ABI_INVALID_ARGUMENT) return 6;
  return 0;
}
`),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	library := "libnames.so"
	if runtime.GOOS == "darwin" {
		library = "libnames.dylib"
	}
	build := exec.Command("go", "build", "-buildmode=c-shared", "-buildvcs=false", "-o", library, "generated.go", "generated_cabi.go")
	build.Dir = directory
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("gateway build: %v\n%s", err, output)
	}
	for _, language := range []string{"C", "C++"} {
		t.Run(language, func(t *testing.T) {
			compiler := cc
			args := []string{}
			if language == "C++" {
				var err error
				compiler, err = exec.LookPath("c++")
				if err != nil {
					if requireCCompilerInCI() {
						t.Fatal("required C++ compiler is unavailable")
					}
					t.Skip("C++ compiler is unavailable")
				}
				args = append(args, "-x", "c++", "-std=c++17")
			}
			executable := filepath.Join(directory, "caller-"+language)
			args = append(args, "-I", directory, filepath.Join(directory, "caller.c"), "-L", directory, "-lnames", "-o", executable)
			compile := exec.Command(compiler, args...)
			if output, err := compile.CombinedOutput(); err != nil {
				t.Fatalf("%s caller build: %v\n%s", language, err, output)
			}
			caller := exec.Command(executable)
			caller.Env = append(os.Environ(), "LD_LIBRARY_PATH="+directory, "DYLD_LIBRARY_PATH="+directory)
			if output, err := caller.CombinedOutput(); err != nil {
				t.Fatalf("%s caller: %v\n%s", language, err, output)
			}
		})
	}
}
