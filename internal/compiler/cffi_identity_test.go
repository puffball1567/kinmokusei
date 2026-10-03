package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Resource values contain a native pointer and synchronization state. Copying
// those values must never create a second owner, even through a Go consumer.
func TestIncomingCFFICopiedResourceIdentity(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",
  "handles":[{"name":"Resource","cType":"fixture_resource","release":"fixture_release"}],
  "callbacks":[{"name":"Visit","lifetime":"registered","parameters":[{"name":"value","type":"int32"}],"result":"int32"}],
  "callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"resource","type":"Resource"}],"register":"fixture_register","unregister":"fixture_unregister"}],
  "functions":[
    {"name":"NewResource","symbol":"fixture_new","parameters":[],"result":"Resource","convention":"statusOut"},
    {"name":"Read","symbol":"fixture_read","parameters":[{"name":"resource","type":"Resource"}],"result":"int32","convention":"statusOut"},
    {"name":"Check","symbol":"fixture_check","parameters":[{"name":"resource","type":"Resource"}],"result":"void","convention":"status"},
    {"name":"Blend","symbol":"fixture_blend","parameters":[{"name":"left","type":"Resource"},{"name":"right","type":"Resource"}],"result":"int32","convention":"statusOut"},
    {"name":"Fire","symbol":"fixture_fire","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"Releases","symbol":"fixture_releases","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"NativeCalls","symbol":"fixture_calls","parameters":[],"result":"int32","convention":"statusOut"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h": `#include <stdint.h>
typedef struct fixture_resource fixture_resource;
int32_t fixture_new(fixture_resource **output);
void fixture_release(fixture_resource *resource);
int32_t fixture_read(fixture_resource *resource, int32_t *output);
int32_t fixture_check(fixture_resource *resource);
int32_t fixture_blend(fixture_resource *left, fixture_resource *right, int32_t *output);
int32_t fixture_register(fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context);
int32_t fixture_unregister(fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context);
int32_t fixture_fire(int32_t *output);
int32_t fixture_releases(int32_t *output);
int32_t fixture_calls(int32_t *output);
`,
				"fixture.c": `#include "fixture.h"
#include <stdlib.h>
struct fixture_resource { int32_t value; };
static int32_t releases, calls;
static fixture_resource *watched;
static int32_t (*visitor)(int32_t, void *);
static void *visitor_context;
int32_t fixture_new(fixture_resource **output) {
  *output = calloc(1, sizeof(fixture_resource));
  if (!*output) return 1;
  (*output)->value = 42;
  return 0;
}
void fixture_release(fixture_resource *resource) { releases++; free(resource); }
int32_t fixture_read(fixture_resource *resource, int32_t *output) { calls++; *output = resource->value; return 0; }
int32_t fixture_check(fixture_resource *resource) { calls++; return resource->value == 42 ? 0 : 1; }
int32_t fixture_blend(fixture_resource *left, fixture_resource *right, int32_t *output) {
  calls++; *output = left->value + right->value; return 0;
}
int32_t fixture_register(fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context) {
  calls++; if (watched) return 1;
  watched = resource; visitor = callback; visitor_context = context; return 0;
}
int32_t fixture_unregister(fixture_resource *resource, int32_t (*callback)(int32_t, void *), void *context) {
  calls++; if (watched != resource || visitor != callback || visitor_context != context) return 1;
  watched = NULL; visitor = NULL; visitor_context = NULL; return 0;
}
int32_t fixture_fire(int32_t *output) { *output = visitor ? visitor(watched->value, visitor_context) : 0; return 0; }
int32_t fixture_releases(int32_t *output) { *output = releases; return 0; }
int32_t fixture_calls(int32_t *output) { *output = calls; return 0; }
`,
				"cmd/main.go": `package main
import (
  "errors"
  ffi "fixture.test"
  "sync"
)
const mainThreadPolicy = "` + policy + `" == "mainThread"
func assert(ok bool) { if !ok { panic("copied C FFI resource accepted or original damaged") } }
func main() {
  resource, err := ffi.NewResource(); assert(err == nil)
  alias := resource
  copied := *resource
  value, err := ffi.Read(&copied); assert(value == 0 && errors.Is(err, ffi.ErrCopiedHandle))
  assert(errors.Is(ffi.Check(&copied), ffi.ErrCopiedHandle))
  value, err = ffi.Blend(resource, &copied); assert(value == 0 && errors.Is(err, ffi.ErrCopiedHandle))
  value, err = ffi.Blend(&copied, resource); assert(value == 0 && errors.Is(err, ffi.ErrCopiedHandle))
  watch, err := ffi.RegisterWatch(&copied, func(value int32) int32 { return value })
  assert(watch == nil && errors.Is(err, ffi.ErrCopiedHandle))
  assert(errors.Is(copied.Close(), ffi.ErrCopiedHandle))
  value, err = ffi.NativeCalls(); assert(value == 0 && err == nil)
  value, err = ffi.Releases(); assert(value == 0 && err == nil)
  value, err = ffi.Read(alias); assert(value == 42 && err == nil)
  value, err = ffi.Blend(resource, alias); assert(value == 84 && err == nil)
  watch, err = ffi.RegisterWatch(resource, func(value int32) int32 { return value + 1 }); assert(err == nil)
  copiedWatch := *watch
  assert(errors.Is(copiedWatch.CallbackError(), ffi.ErrCopiedCallbackRegistration))
  assert(errors.Is(copiedWatch.Close(), ffi.ErrCopiedCallbackRegistration))
  var workers sync.WaitGroup
  for i := 0; i < 8; i++ {
    workers.Add(1)
    go func() {
      defer workers.Done()
      for j := 0; j < 50; j++ {
        err := copied.Close()
        // mainThread rejects the wrong thread before inspecting identity.
        if mainThreadPolicy { assert(err != nil) } else { assert(errors.Is(err, ffi.ErrCopiedHandle)) }
        err = copiedWatch.Close()
        if mainThreadPolicy { assert(err != nil) } else { assert(errors.Is(err, ffi.ErrCopiedCallbackRegistration)) }
        assert(errors.Is(copiedWatch.CallbackError(), ffi.ErrCopiedCallbackRegistration))
      }
    }()
  }
  workers.Wait()
  value, err = ffi.NativeCalls(); assert(value == 3 && err == nil)
  assert(errors.Is(resource.Close(), ffi.ErrHandleHasActiveRegistrations))
  value, err = ffi.Fire(); assert(value == 43 && err == nil && watch.CallbackError() == nil)
  watchAlias := watch
  assert(watchAlias.Close() == nil)
  *watch = copiedWatch
  // Restoring a snapshot to the original address must not revive its context.
  assert(errors.Is(watch.Close(), ffi.ErrClosedCallbackRegistration))
  assert(errors.Is(copiedWatch.Close(), ffi.ErrCopiedCallbackRegistration))
  value, err = ffi.NativeCalls(); assert(value == 4 && err == nil)
  value, err = ffi.Fire(); assert(value == 0 && err == nil)
  assert(alias.Close() == nil)
  *resource = copied
  // Identity alone is insufficient: this snapshot has the original address.
  assert(errors.Is(resource.Close(), ffi.ErrClosedHandle))
  value, err = ffi.Read(resource); assert(value == 0 && errors.Is(err, ffi.ErrClosedHandle))
  assert(errors.Is(copied.Close(), ffi.ErrCopiedHandle))
  value, err = ffi.Read(&copied); assert(value == 0 && errors.Is(err, ffi.ErrCopiedHandle))
  value, err = ffi.Releases(); assert(value == 1 && err == nil)
  var zero ffi.Resource
  value, err = ffi.Read(&zero); assert(value == 0 && errors.Is(err, ffi.ErrClosedHandle))
  var zeroWatch ffi.Watch
  assert(errors.Is(zeroWatch.Close(), ffi.ErrClosedCallbackRegistration))
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
				t.Fatalf("copied-resource generated package failed: %v\n%s", err, output)
			}
			command = exec.CommandContext(ctx, "go", "vet", "./...")
			command.Dir = root
			command.Env = append(os.Environ(), "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("copied-resource generated package vet failed: %v\n%s", err, output)
			}
		})
	}
}
