package compiler

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIncomingCFFIParameterNameHygiene(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			manifest := []byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",
  "handles":[{"name":"Resource","cType":"fixture_resource","release":"fixture_free"}],
  "structs":[{"name":"Point","cType":"fixture_point","fields":[{"name":"X","cName":"x","type":"int32"}]}],
  "callbacks":[
    {"name":"Visit","lifetime":"registered","parameters":[{"name":"int32","type":"int32"}],"result":"int32"},
    {"name":"InspectText","lifetime":"callScoped","parameters":[{"name":"runtime","type":"copiedCString"}],"result":"int32"}
  ],
  "callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"context","type":"int32"},{"name":"state","type":"int32"},{"name":"result","type":"int32"},{"name":"callback","type":"int32"},{"name":"Resource","type":"Resource"}],"register":"fixture_register","unregister":"fixture_unregister"}],
  "functions":[
    {"name":"Builtin","symbol":"fixture_sum","parameters":[{"name":"int32","type":"int32"},{"name":"len","type":"int32"},{"name":"nil","type":"int32"},{"name":"true","type":"int32"}],"result":"int32","convention":"statusOut"},
    {"name":"NewResource","symbol":"fixture_new","parameters":[{"name":"Resource","type":"int32"}],"result":"Resource","convention":"statusOut"},
    {"name":"Shift","symbol":"fixture_shift","parameters":[{"name":"Point","type":"Point"},{"name":"output","type":"int32"},{"name":"status","type":"int32"}],"result":"Point","convention":"statusOut"},
    {"name":"ArraySum","symbol":"fixture_array_sum","parameters":[{"name":"elementSize","type":"borrowedArray","element":"int32"},{"name":"kinmokuseiParameter0","type":"int32"},{"name":"unsafe","type":"int32"},{"name":"_","type":"int32"}],"result":"int32","convention":"direct"},
    {"name":"Inspect","symbol":"fixture_inspect","parameters":[{"name":"strings","type":"boolean"},{"name":"cgo","type":"InspectText"}],"result":"int32","convention":"direct"},
    {"name":"Fire","symbol":"fixture_fire","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"Echo","symbol":"fixture_echo","parameters":[{"name":"Echo","type":"int32"},{"name":"C","type":"int32"},{"name":"errors","type":"int32"},{"name":"fmt","type":"int32"},{"name":"runtime","type":"int32"}],"result":"int32","convention":"statusOut"}
  ]
}`)
			artifacts, err := GenerateCFFI(manifest)
			if err != nil {
				t.Fatal(err)
			}
			again, err := GenerateCFFI(manifest)
			if err != nil || !bytes.Equal(artifacts.Source, again.Source) {
				t.Fatalf("nondeterministic FFI parameter bindings: %v", err)
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef struct fixture_resource fixture_resource;
typedef struct { int32_t x; } fixture_point;
int32_t fixture_new(int32_t value, fixture_resource **output);
void fixture_free(fixture_resource *resource);
int32_t fixture_sum(int32_t a, int32_t b, int32_t c, int32_t d, int32_t *output);
int32_t fixture_shift(fixture_point point, int32_t a, int32_t b, fixture_point *output);
int32_t fixture_array_sum(const int32_t *values, size_t count, int32_t a, int32_t b, int32_t c);
int32_t fixture_inspect(bool missing, int32_t (*callback)(const char *, void *), void *context);
int32_t fixture_register(int32_t a, int32_t b, int32_t c, int32_t d, fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context);
int32_t fixture_unregister(int32_t a, int32_t b, int32_t c, int32_t d, fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context);
int32_t fixture_fire(int32_t *output);
int32_t fixture_echo(int32_t a, int32_t b, int32_t c, int32_t d, int32_t e, int32_t *output);
`,
				"fixture.c": `#include "fixture.h"
#include <stdlib.h>
#include <string.h>
struct fixture_resource { int32_t value; };
static fixture_resource *watched;
static int32_t captured;
static int32_t (*visitor)(int32_t, void *);
static void *visitor_context;
int32_t fixture_new(int32_t value, fixture_resource **output) {
  *output = calloc(1, sizeof(fixture_resource)); if (!*output) return 1;
  (*output)->value = value; return 0;
}
void fixture_free(fixture_resource *resource) { free(resource); }
int32_t fixture_sum(int32_t a, int32_t b, int32_t c, int32_t d, int32_t *output) { *output = a+10*b+100*c+1000*d; return 0; }
int32_t fixture_shift(fixture_point point, int32_t a, int32_t b, fixture_point *output) { output->x = point.x+10*a+100*b; return 0; }
int32_t fixture_array_sum(const int32_t *values, size_t count, int32_t a, int32_t b, int32_t c) {
  int32_t sum = a+10*b+100*c; for (size_t i = 0; i < count; i++) sum += values[i]; return sum;
}
int32_t fixture_inspect(bool missing, int32_t (*callback)(const char *, void *), void *context) {
  return callback(missing ? NULL : "hello", context);
}
int32_t fixture_register(int32_t a, int32_t b, int32_t c, int32_t d, fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context) {
  if (watched) return 1;
  captured = a+10*b+100*c+1000*d; watched = resource; visitor = callback; visitor_context = context; return 0;
}
int32_t fixture_unregister(int32_t a, int32_t b, int32_t c, int32_t d, fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context) {
  if (captured != a+10*b+100*c+1000*d || watched != resource || visitor != callback || visitor_context != context) return 1;
  watched = NULL; visitor = NULL; visitor_context = NULL; return 0;
}
int32_t fixture_fire(int32_t *output) { *output = visitor ? visitor(captured + watched->value, visitor_context) : 0; return 0; }
int32_t fixture_echo(int32_t a, int32_t b, int32_t c, int32_t d, int32_t e, int32_t *output) { *output = a+10*b+100*c+1000*d+10000*e; return 0; }
`,
				"app/binding.km": `import go ffi from "fixture.test";
function Sum(): Result<int32> {
  const value = ffi.Builtin(1, 2, 3, 4)?;
  return ok(value);
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  ffi "fixture.test"
  "fixture.test/app"
)
func assert(ok bool) { if !ok { panic("C FFI name collision changed arguments, results, or diagnostics") } }
func main() {
  value, err := app.Sum(); assert(value == 4321 && err == nil)
  value, err = ffi.Echo(1,2,3,4,5); assert(value == 54321 && err == nil)
  point, err := ffi.Shift(ffi.Point{X: 7}, 11, 13); assert(point.X == 1417 && err == nil)
  value, err = ffi.ArraySum([]int32{1,2,3}, 5, 7, 11); assert(value == 1181 && err == nil)
  value, err = ffi.ArraySum(nil, 5, 7, 11); assert(value == 1175 && err == nil)
  resource, err := ffi.NewResource(11); assert(resource != nil && err == nil)
  watch, err := ffi.RegisterWatch(2,3,5,7,resource,func(value int32) int32 { return value+1 }); assert(watch != nil && err == nil)
  assert(errors.Is(resource.Close(), ffi.ErrHandleHasActiveRegistrations))
  value, err = ffi.Fire(); assert(value == 7544 && err == nil)
  assert(watch.Close() == nil && resource.Close() == nil)
  value, err = ffi.Fire(); assert(value == 0 && err == nil)
  callbacks := 0
  value, err = ffi.Inspect(false, func(value string) int32 { callbacks++; assert(value == "hello"); return int32(len(value)) })
  assert(value == 5 && err == nil && callbacks == 1)
  value, err = ffi.Inspect(true, func(value string) int32 { callbacks++; return 99 })
  var input *ffi.CallbackInputError
  assert(value == 0 && errors.As(err, &input) && input.Function == "Inspect" && input.Parameter == "runtime" && callbacks == 1)
  value, err = ffi.Inspect(false, nil); assert(value == 0 && errors.Is(err, ffi.ErrNilCallback))
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
			generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.km")}, "app")
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("generate Kinmokusei name-hygiene wrapper: err=%v diagnostics=%v", err, diagnostics)
			}
			if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generated, 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			args := []string{"run", "-buildvcs=false", "./cmd"}
			if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE") == "1" {
				args = []string{"run", "-race", "-buildvcs=false", "./cmd"}
			}
			command := exec.CommandContext(ctx, "go", args...)
			command.Dir = root
			command.Env = append(os.Environ(), "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("parameter-hygiene generated package failed: %v\n%s", err, output)
			}
			command = exec.CommandContext(ctx, "go", "vet", "./...")
			command.Dir = root
			command.Env = append(os.Environ(), "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("parameter-hygiene generated package vet failed: %v\n%s", err, output)
			}
		})
	}
}
