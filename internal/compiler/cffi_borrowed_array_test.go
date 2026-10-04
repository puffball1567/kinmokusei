package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIncomingCFFIBorrowedArray(t *testing.T) {
	requireCCompiler(t)
	root := t.TempDir()
	manifest := []byte(`{
  "schemaVersion":1,
  "package":"ffi",
  "header":"fixture.h",
  "threadPolicy":"threadSafe",
  "enums":[{"name":"Mode","cType":"fixture_mode","underlying":"cInt32","values":[{"name":"ModeOne","symbol":"MODE_ONE"}]}],
  "structs":[{"name":"Point","cType":"fixture_point","fields":[{"name":"X","cName":"x","type":"float32"},{"name":"Y","cName":"y","type":"float32"}]}],
  "callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"count","type":"int32"}],"result":"int32"}],
  "functions":[
    {"name":"SumPoints","symbol":"fixture_sum_points","parameters":[{"name":"points","type":"borrowedArray","element":"Point"}],"result":"int32","convention":"direct"},
    {"name":"SumModes","symbol":"fixture_sum_modes","parameters":[{"name":"modes","type":"borrowedArray","element":"Mode"}],"result":"int32","convention":"direct"},
    {"name":"SumNumbers","symbol":"fixture_sum_numbers","parameters":[{"name":"numbers","type":"borrowedArray","element":"uint32"}],"result":"uint64","convention":"direct"},
    {"name":"ReadPoints","symbol":"fixture_const_points","parameters":[{"name":"points","type":"borrowedArray","element":"Point"}],"result":"int32","convention":"direct"},
    {"name":"AcceptPoints","symbol":"fixture_accept_points","parameters":[{"name":"points","type":"borrowedArray","element":"Point"}],"result":"void","convention":"status"},
    {"name":"VisitPoints","symbol":"fixture_visit_points","parameters":[{"name":"points","type":"borrowedArray","element":"Point"},{"name":"visit","type":"Visit"}],"result":"int32","convention":"direct"}
  ]
}`)
	artifacts, err := GenerateCFFI(manifest)
	if err != nil {
		t.Fatal(err)
	}
	generated := string(artifacts.Source)
	for _, want := range []string{
		"func SumPoints(points []Point) (int32, error)",
		"func SumModes(modes []Mode) (int32, error)",
		"func SumNumbers(numbers []uint32) (uint64, error)",
		"func ReadPoints(points []Point) (int32, error)",
		"func AcceptPoints(points []Point) error",
		"func VisitPoints(points []Point, visit Visit) (int32, error)",
		"ErrBorrowedArrayTooLarge", "ErrBorrowedArrayAllocation",
		"C.kinmokusei_cffi_alloc_array", "unsafe.Slice(kinmokuseiArray0",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated FFI missing %q", want)
		}
	}
	files := map[string]string{
		"go.mod":           "module fixture.test\n\ngo 1.23\n",
		"generated_ffi.go": generated,
		"fixture.h": `#include <stdint.h>
#include <stddef.h>
typedef enum fixture_mode { MODE_ONE = 1 } fixture_mode;
typedef struct fixture_point { float x; float y; } fixture_point;
int32_t fixture_sum_points(fixture_point *points, size_t count);
int32_t fixture_sum_modes(fixture_mode *modes, size_t count);
uint64_t fixture_sum_numbers(uint32_t *numbers, size_t count);
int32_t fixture_const_points(const fixture_point *points, size_t count);
int32_t fixture_accept_points(fixture_point *points, size_t count);
int32_t fixture_visit_points(fixture_point *points, size_t count, int32_t (*visit)(int32_t, void *), void *context);
`,
		"fixture.c": `#include "fixture.h"
int32_t fixture_sum_points(fixture_point *points, size_t count) {
  if (count == 0) return points == NULL ? -1 : -2;
  int32_t sum = 0;
  for (size_t i = 0; i < count; i++) sum += (int32_t)(points[i].x + points[i].y);
  points[0].x = 999;
  return sum;
}
int32_t fixture_sum_modes(fixture_mode *modes, size_t count) {
  int32_t sum = 0;
  for (size_t i = 0; i < count; i++) sum += modes[i];
  return sum;
}
uint64_t fixture_sum_numbers(uint32_t *numbers, size_t count) {
  uint64_t sum = 0;
  for (size_t i = 0; i < count; i++) sum += numbers[i];
  return sum;
}
int32_t fixture_const_points(const fixture_point *points, size_t count) {
  return count == 0 ? 0 : (int32_t)points[0].x;
}
int32_t fixture_accept_points(fixture_point *points, size_t count) {
  return count == 1 && points != NULL ? 0 : 7;
}
int32_t fixture_visit_points(fixture_point *points, size_t count, int32_t (*visit)(int32_t, void *), void *context) {
  return visit((int32_t)count, context) + (points == NULL ? 0 : (int32_t)points[0].x);
}
`,
		"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
)
func assert(ok bool) { if !ok { panic("borrowedArray mismatch") } }
func main() {
  points := []ffi.Point{{X: 1, Y: 2}, {X: 3, Y: 4}}
  sum, err := ffi.SumPoints(points)
  assert(err == nil && sum == 10 && points[0].X == 1)
  sum, err = ffi.SumPoints(nil)
  assert(err == nil && sum == -1)
  sum, err = ffi.SumPoints([]ffi.Point{})
  assert(err == nil && sum == -1)
  modes, err := ffi.SumModes([]ffi.Mode{ffi.ModeOne, ffi.ModeOne})
  assert(err == nil && modes == 2)
  numbers, err := ffi.SumNumbers([]uint32{1, 2, 3})
  assert(err == nil && numbers == 6)
  first, err := ffi.ReadPoints(points)
  assert(err == nil && first == 1)
  assert(ffi.AcceptPoints(points[:1]) == nil)
  var status *ffi.StatusError
  assert(errors.As(ffi.AcceptPoints(nil), &status) && status.Code == 7)
  visited, err := ffi.VisitPoints(points, func(count int32) int32 { return count * 10 })
  assert(err == nil && visited == 21 && points[0].X == 1)
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := filepath.Join(root, "borrowed-array-runner")
	if runtime.GOOS == "windows" {
		runner += ".exe"
	}
	command := exec.Command("go", "build", "-buildvcs=false", "-o", runner, "./cmd")
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("borrowedArray generated package failed: %v\n%s\n%s", err, output, generated)
	}
	if output, err := exec.Command(runner).CombinedOutput(); err != nil {
		t.Fatalf("borrowedArray runtime failed: %v\n%s", err, output)
	}
}

func TestIncomingCFFIBorrowedArrayValidation(t *testing.T) {
	tests := []struct{ name, parameter, want string }{
		{"missing element", `{"name":"values","type":"borrowedArray"}`, "requires a supported element"},
		{"unknown element", `{"name":"values","type":"borrowedArray","element":"Missing"}`, "requires a supported element"},
		{"string element", `{"name":"values","type":"borrowedArray","element":"cstring"}`, "requires a supported element"},
		{"extra element", `{"name":"values","type":"int32","element":"int32"}`, "only for borrowedArray"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"fixture_value","parameters":[` + test.parameter + `],"result":"int32","convention":"direct"}]}`
			if _, err := GenerateCFFI([]byte(manifest)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v; want %q", err, test.want)
			}
		})
	}
	for _, test := range []struct{ name, declaration, want string }{
		{"callback array", `"callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"values","type":"borrowedArray"}],"result":"void"}],`, "unsupported scalar, enum, POD, or tagged-union type"},
		{"callback extra element", `"callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"int32","element":"int32"}],"result":"void"}],`, "may not declare element"},
		{"registration array", `"callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"values","type":"borrowedArray","element":"int32"}],"register":"watch_add","unregister":"watch_remove"}],`, "may not declare element"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe",` + test.declaration + `"functions":[{"name":"Value","symbol":"fixture_value","parameters":[],"result":"int32","convention":"direct"}]}`
			if _, err := GenerateCFFI([]byte(manifest)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v; want %q", err, test.want)
			}
		})
	}
}
