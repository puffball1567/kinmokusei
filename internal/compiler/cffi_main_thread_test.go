package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIncomingCFFIMainThread(t *testing.T) {
	requireCCompiler(t)
	cache := filepath.Join(t.TempDir(), "go-cache")
	for _, test := range []struct {
		name, declarations, header, implementation, main, binding string
	}{
		{
			name: "startup thread and scalar calls",
			binding: `import go ffi from "fixture.test";
function OnMain(): Result<boolean> {
  const startup = ffi.StartupThread()?;
  return ok(startup);
}
`,
			declarations: `"functions":[
  {"name":"StartupThread","symbol":"fixture_startup_thread","parameters":[],"result":"boolean","convention":"direct"},
  {"name":"Touch","symbol":"fixture_touch","parameters":[],"result":"void","convention":"direct"},
  {"name":"Touches","symbol":"fixture_touches","parameters":[],"result":"int32","convention":"direct"}
]`,
			header: "#include <stdbool.h>\n#include <stdint.h>\nbool fixture_startup_thread(void);\nvoid fixture_touch(void);\nint32_t fixture_touches(void);\n",
			implementation: `#include "fixture.h"
#if defined(__linux__)
#include <unistd.h>
#include <sys/syscall.h>
#elif defined(__APPLE__)
#include <pthread.h>
#endif
static int32_t touches;
bool fixture_startup_thread(void) {
#if defined(__linux__)
  return getpid() == syscall(SYS_gettid);
#elif defined(__APPLE__)
  return pthread_main_np() != 0;
#else
  return true;
#endif
}
void fixture_touch(void) { touches++; }
int32_t fixture_touches(void) { return touches; }
`,
			main: `package main
import (
  "errors"
  ffi "fixture.test"
  "fixture.test/app"
  "runtime"
)
func assert(ok bool) { if !ok { panic("mainThread scalar mismatch") } }
func main() {
  runtime.GOMAXPROCS(4)
  startup, err := ffi.StartupThread(); assert(startup && err == nil)
  startup, err = app.OnMain(); assert(startup && err == nil)
  assert(ffi.Touch() == nil)
  done := make(chan struct{})
  go func() {
    defer close(done)
    assert(errors.Is(ffi.Touch(), ffi.ErrWrongThread))
    startup, err := ffi.StartupThread(); assert(!startup && errors.Is(err, ffi.ErrWrongThread))
    startup, err = app.OnMain(); assert(!startup && errors.Is(err, ffi.ErrWrongThread))
    count, err := ffi.Touches(); assert(count == 0 && errors.Is(err, ffi.ErrWrongThread))
  }()
  <-done
  count, err := ffi.Touches(); assert(count == 1 && err == nil)
  for i := 0; i < 100; i++ {
    runtime.Gosched()
    startup, err = ffi.StartupThread(); assert(startup && err == nil)
  }
}
`,
		},
		{
			name: "handle and registration lifecycle",
			declarations: `"handles":[{"name":"Resource","cType":"fixture_resource","release":"fixture_resource_free"}],
"callbacks":[{"name":"Visit","lifetime":"registered","parameters":[{"name":"value","type":"int32"}],"result":"int32"}],
"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"resource","type":"Resource"}],"register":"fixture_watch_add","unregister":"fixture_watch_remove"}],
"functions":[
  {"name":"NewResource","symbol":"fixture_resource_new","parameters":[],"result":"Resource","convention":"statusOut"},
  {"name":"Draw","symbol":"fixture_draw","parameters":[{"name":"resource","type":"Resource"}],"result":"int32","convention":"statusOut"},
  {"name":"Blend","symbol":"fixture_blend","parameters":[{"name":"left","type":"Resource"},{"name":"right","type":"Resource"},{"name":"values","type":"borrowedArray","element":"int32"}],"result":"int32","convention":"statusOut"},
  {"name":"Fire","symbol":"fixture_fire","parameters":[],"result":"int32","convention":"direct"},
  {"name":"ReleaseCount","symbol":"fixture_release_count","parameters":[],"result":"int32","convention":"direct"}
]`,
			header: `#include <stdint.h>
#include <stddef.h>
typedef struct fixture_resource fixture_resource;
int32_t fixture_resource_new(fixture_resource **output);
void fixture_resource_free(fixture_resource *resource);
int32_t fixture_draw(fixture_resource *resource, int32_t *output);
int32_t fixture_blend(fixture_resource *left, fixture_resource *right, int32_t *values, size_t count, int32_t *output);
int32_t fixture_watch_add(fixture_resource *resource, int32_t (*visit)(int32_t, void *), void *context);
int32_t fixture_watch_remove(fixture_resource *resource, int32_t (*visit)(int32_t, void *), void *context);
int32_t fixture_fire(void);
int32_t fixture_release_count(void);
`,
			implementation: `#include "fixture.h"
#include <stdlib.h>
struct fixture_resource { int32_t value; };
static fixture_resource *watched;
static int32_t (*visitor)(int32_t, void *);
static void *visitor_context;
static int32_t releases;
int32_t fixture_resource_new(fixture_resource **output) {
  *output = calloc(1, sizeof(fixture_resource));
  if (!*output) return 1;
  (*output)->value = 42;
  return 0;
}
void fixture_resource_free(fixture_resource *resource) { releases++; free(resource); }
int32_t fixture_draw(fixture_resource *resource, int32_t *output) { *output = resource->value; return 0; }
int32_t fixture_blend(fixture_resource *left, fixture_resource *right, int32_t *values, size_t count, int32_t *output) {
  *output = left->value + right->value;
  for (size_t i = 0; i < count; i++) { *output += values[i]; values[i] = -1; }
  return 0;
}
int32_t fixture_watch_add(fixture_resource *resource, int32_t (*visit)(int32_t, void *), void *context) {
  if (watched) return 1;
  watched = resource; visitor = visit; visitor_context = context;
  return 0;
}
int32_t fixture_watch_remove(fixture_resource *resource, int32_t (*visit)(int32_t, void *), void *context) {
  if (watched != resource || visitor != visit || visitor_context != context) return 1;
  watched = NULL; visitor = NULL; visitor_context = NULL;
  return 0;
}
int32_t fixture_fire(void) { return visitor ? visitor(watched->value, visitor_context) : 0; }
int32_t fixture_release_count(void) { return releases; }
`,
			main: `package main
import (
  "errors"
  ffi "fixture.test"
)
func assert(ok bool) { if !ok { panic("mainThread lifecycle mismatch") } }
func main() {
  resource, err := ffi.NewResource(); assert(resource != nil && err == nil)
  values := []int32{3, 5}
  value, err := ffi.Blend(resource, resource, values); assert(value == 92 && err == nil && values[0] == 3 && values[1] == 5)
  callbacks := 0
  watch, err := ffi.RegisterWatch(resource, func(value int32) int32 { callbacks++; return value + 1 })
  assert(watch != nil && err == nil)
  assert(errors.Is(resource.Close(), ffi.ErrHandleHasActiveRegistrations))
  done := make(chan struct{})
  go func() {
    defer close(done)
    other, err := ffi.NewResource(); assert(other == nil && errors.Is(err, ffi.ErrWrongThread))
    value, err := ffi.Draw(resource); assert(value == 0 && errors.Is(err, ffi.ErrWrongThread))
    value, err = ffi.Blend(resource, resource, values); assert(value == 0 && errors.Is(err, ffi.ErrWrongThread))
    otherWatch, err := ffi.RegisterWatch(resource, func(value int32) int32 { return value })
    assert(otherWatch == nil && errors.Is(err, ffi.ErrWrongThread))
    assert(errors.Is(watch.Close(), ffi.ErrWrongThread))
    assert(errors.Is(resource.Close(), ffi.ErrWrongThread))
  }()
  <-done
  value, err = ffi.Fire(); assert(value == 43 && err == nil && callbacks == 1)
  assert(watch.CallbackError() == nil)
  assert(watch.Close() == nil)
  value, err = ffi.Draw(resource); assert(value == 42 && err == nil)
  assert(resource.Close() == nil)
  value, err = ffi.ReleaseCount(); assert(value == 1 && err == nil)
  value, err = ffi.Fire(); assert(value == 0 && err == nil && callbacks == 1)
}
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"mainThread",` + test.declarations + `}`
			artifacts, err := GenerateCFFI([]byte(manifest))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h":        test.header, "fixture.c": test.implementation, "cmd/main.go": test.main,
			}
			if test.binding != "" {
				files["app/binding.km"] = test.binding
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
			if test.binding != "" {
				generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.km")}, "app")
				if err != nil || len(diagnostics) != 0 {
					t.Fatalf("generate Kinmokusei mainThread wrapper: err=%v diagnostics=%v", err, diagnostics)
				}
				if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generated, 0o644); err != nil {
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
			command.Env = append(os.Environ(), "GOCACHE="+cache, "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("mainThread generated package failed: %v\n%s\n%s", err, output, artifacts.Source)
			}
		})
	}
}
