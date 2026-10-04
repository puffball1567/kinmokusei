package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestIncomingCFFIMultipleCallScopedCallbacks(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	flags := `"cFlags":["-pthread"],"ldFlags":["-pthread"],`
	if runtime.GOOS == "windows" {
		flags = ""
	}
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",` + flags + `
  "callbacks":[
    {"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"int32"}],"result":"int32"},
    {"name":"Text","lifetime":"callScoped","parameters":[{"name":"path","type":"copiedCString"}],"result":"int32"}
  ],
  "functions":[
    {"name":"Pair","symbol":"fixture_pair","parameters":[{"name":"left","type":"Visit"},{"name":"seed","type":"int32"},{"name":"right","type":"Visit"}],"result":"int32","convention":"direct"},
    {"name":"CheckedPair","symbol":"fixture_checked_pair","parameters":[{"name":"left","type":"Visit"},{"name":"seed","type":"int32"},{"name":"right","type":"Visit"}],"result":"int32","convention":"statusOut"},
    {"name":"StatusPair","symbol":"fixture_status_pair","parameters":[{"name":"left","type":"Visit"},{"name":"seed","type":"int32"},{"name":"right","type":"Visit"}],"result":"void","convention":"status"},
    {"name":"VoidPair","symbol":"fixture_void_pair","parameters":[{"name":"left","type":"Visit"},{"name":"right","type":"Visit"}],"result":"void","convention":"direct"},
    {"name":"Triple","symbol":"fixture_triple","parameters":[{"name":"first","type":"Visit"},{"name":"second","type":"Visit"},{"name":"third","type":"Visit"}],"result":"int32","convention":"direct"},
    {"name":"Mixed","symbol":"fixture_mixed","parameters":[{"name":"missing","type":"boolean"},{"name":"runtime","type":"Text"},{"name":"visit","type":"Visit"}],"result":"int32","convention":"direct"},
    {"name":"ParallelPair","symbol":"fixture_parallel_pair","parameters":[{"name":"left","type":"Visit"},{"name":"seed","type":"int32"},{"name":"right","type":"Visit"}],"result":"int32","convention":"direct"},
    {"name":"ArrayPair","symbol":"fixture_array_pair","parameters":[{"name":"left","type":"Visit"},{"name":"values","type":"borrowedArray","element":"int32"},{"name":"right","type":"Visit"}],"result":"int32","convention":"direct"},
    {"name":"CallCount","symbol":"fixture_call_count","parameters":[],"result":"int32","convention":"statusOut"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef int32_t (*fixture_visit)(int32_t, void *);
typedef int32_t (*fixture_text)(const char *, void *);
int32_t fixture_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc);
int32_t fixture_checked_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc, int32_t *output);
int32_t fixture_status_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc);
void fixture_void_pair(fixture_visit left, void *lc, fixture_visit right, void *rc);
int32_t fixture_triple(fixture_visit first, void *fc, fixture_visit second, void *sc, fixture_visit third, void *tc);
int32_t fixture_mixed(bool missing, fixture_text text, void *tc, fixture_visit visit, void *vc);
int32_t fixture_parallel_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc);
int32_t fixture_array_pair(fixture_visit left, void *lc, const int32_t *values, size_t count, fixture_visit right, void *rc);
int32_t fixture_call_count(int32_t *output);
`,
				"fixture.c": `#include "fixture.h"
#ifdef _WIN32
#include <windows.h>
#else
#include <pthread.h>
#endif
static int32_t calls;
int32_t fixture_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc) {
  calls++;
  // Explicit sequence: C does not specify operand evaluation order.
  int32_t a = left(seed, lc);
  int32_t b = right(seed+1, rc);
  int32_t c = left(seed+2, lc);
  int32_t d = right(seed+3, rc);
  return a + 10*b + 100*c + 1000*d;
}
int32_t fixture_checked_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc, int32_t *output) {
  *output = fixture_pair(left, lc, seed, right, rc); return seed < 0 ? 31 : 0;
}
int32_t fixture_status_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc) {
  fixture_pair(left, lc, seed, right, rc); return seed < 0 ? 31 : 0;
}
void fixture_void_pair(fixture_visit left, void *lc, fixture_visit right, void *rc) {
  fixture_pair(left, lc, 1, right, rc);
}
int32_t fixture_triple(fixture_visit first, void *fc, fixture_visit second, void *sc, fixture_visit third, void *tc) {
  calls++;
  int32_t a = first(1, fc);
  int32_t b = second(2, sc);
  int32_t c = third(3, tc);
  return a+10*b+100*c;
}
int32_t fixture_mixed(bool missing, fixture_text text, void *tc, fixture_visit visit, void *vc) {
  calls++;
  int32_t a = text(missing ? NULL : "ok", tc);
  int32_t b = visit(9, vc); return 10*a+b;
}
typedef struct { fixture_visit visit; void *context; int32_t value, result; } fixture_job;
static void fixture_do_job(fixture_job *job) { job->result = job->visit(job->value, job->context); }
#ifdef _WIN32
static DWORD WINAPI fixture_job_entry(LPVOID pointer) { fixture_do_job(pointer); return 0; }
#else
static void *fixture_job_entry(void *pointer) { fixture_do_job(pointer); return NULL; }
#endif
int32_t fixture_parallel_pair(fixture_visit left, void *lc, int32_t seed, fixture_visit right, void *rc) {
  calls++; fixture_job a = {left,lc,seed,0}, b = {right,rc,seed+1,0};
#ifdef _WIN32
  HANDLE first = CreateThread(NULL,0,fixture_job_entry,&a,0,NULL);
  if (!first) return -1;
  HANDLE second = CreateThread(NULL,0,fixture_job_entry,&b,0,NULL);
  if (second) { WaitForSingleObject(second,INFINITE); CloseHandle(second); }
  WaitForSingleObject(first,INFINITE); CloseHandle(first);
  if (!second) return -1;
#else
  pthread_t first, second;
  if (pthread_create(&first,NULL,fixture_job_entry,&a) != 0) return -1;
  int status = pthread_create(&second,NULL,fixture_job_entry,&b);
  if (status == 0) pthread_join(second,NULL);
  pthread_join(first,NULL);
  if (status != 0) return -1;
#endif
  return 10*a.result+b.result;
}
int32_t fixture_array_pair(fixture_visit left, void *lc, const int32_t *values, size_t count, fixture_visit right, void *rc) {
  calls++; int32_t result = 0;
  for (size_t i = 0; i < count; i++) { int32_t a = left(values[i],lc); int32_t b = right(values[i]+1,rc); result += a+b; }
  return result;
}
int32_t fixture_call_count(int32_t *output) { *output = calls; return 0; }
`,
				"app/binding.km": `import go ffi from "fixture.test";
function Sample(): Result<int32> {
  const value = ffi.Pair((value: int32): int32 => value + 10, 1, (value: int32): int32 => value + 20)?;
  return ok(value);
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  ffi "fixture.test"
  "fixture.test/app"
  "sync/atomic"
)
func assert(ok bool) { if !ok { panic("multiple C FFI callback contract mismatch") } }
func joined(err error) []error {
  tree, ok := err.(interface { Unwrap() []error }); assert(ok); return tree.Unwrap()
}
func panicAt(err error, function string, want any) {
  var failure *ffi.CallbackPanicError
  assert(errors.As(err, &failure) && failure.Function == function && failure.Value == want)
}
func argumentAt(err error, function, parameter string) {
  var failure *ffi.CallbackArgumentError
  assert(errors.As(err,&failure) && failure.Function == function && failure.Parameter == parameter && failure.Err != nil)
}
func main() {
  identity := func(value int32) int32 { return value }
  // Nil in any slot prevents entering C or invoking any other callback.
  var observed atomic.Int32
  observer := func(value int32) int32 { observed.Add(1); return value }
  value, err := ffi.Pair(nil,1,observer); assert(value == 0 && errors.Is(err, ffi.ErrNilCallback))
  value, err = ffi.Pair(observer,1,nil); assert(value == 0 && errors.Is(err, ffi.ErrNilCallback))
  value, err = ffi.Triple(observer,nil,observer); assert(value == 0 && errors.Is(err, ffi.ErrNilCallback))
  count, err := ffi.CallCount(); assert(count == 0 && err == nil && observed.Load() == 0)
  value, err = app.Sample(); assert(value == 25531 && err == nil)
  value, err = ffi.Pair(observer,1,observer); assert(value == 4321 && err == nil && observed.Load() == 4)
  value, err = ffi.Triple(identity,identity,identity); assert(value == 321 && err == nil)
  value, err = ffi.ArrayPair(identity,nil,identity); assert(value == 0 && err == nil)
  values := []int32{1,2}
  value, err = ffi.ArrayPair(func(v int32) int32 { return 2*v },values,func(v int32) int32 { return 3*v })
  assert(value == 21 && err == nil && values[0] == 1 && values[1] == 2)
  leftCalls, rightCalls := 0, 0
  leftPanic := func(value int32) int32 { leftCalls++; panic("left") }
  rightPanic := func(value int32) int32 { rightCalls++; panic("right") }
  value, err = ffi.Pair(leftPanic,1,observer); assert(value == 0 && observed.Load() == 6 && leftCalls == 1)
  failures := joined(err); assert(len(failures) == 1); panicAt(failures[0],"Pair","left")
  value, err = ffi.Pair(identity,1,rightPanic); assert(value == 0 && rightCalls == 1)
  failures = joined(err); assert(len(failures) == 1); panicAt(failures[0],"Pair","right")
  value, err = ffi.Pair(leftPanic,1,rightPanic); assert(value == 0 && leftCalls == 2 && rightCalls == 2)
  failures = joined(err); assert(len(failures) == 2)
  panicAt(failures[0],"Pair","left"); panicAt(failures[1],"Pair","right")
  argumentAt(failures[0],"Pair","left"); argumentAt(failures[1],"Pair","right")
  value, err = ffi.Triple(leftPanic,rightPanic,func(int32) int32 { panic("third") }); assert(value == 0)
  failures = joined(err); assert(len(failures) == 3)
  panicAt(failures[0],"Triple","left"); panicAt(failures[1],"Triple","right"); panicAt(failures[2],"Triple","third")
  value, err = ffi.Pair(identity,1,identity); assert(value == 4321 && err == nil)
  value, err = ffi.CheckedPair(identity,1,identity); assert(value == 4321 && err == nil)
  value, err = ffi.CheckedPair(identity,-1,identity)
  var status *ffi.StatusError
  assert(value == 0 && errors.As(err,&status) && status.Function == "CheckedPair" && status.Code == 31)
  value, err = ffi.CheckedPair(leftPanic,-1,rightPanic); assert(value == 0)
  failures = joined(err); assert(len(failures) == 2)
  panicAt(failures[0],"CheckedPair","left"); panicAt(failures[1],"CheckedPair","right")
  assert(ffi.StatusPair(identity,1,identity) == nil && ffi.VoidPair(identity,identity) == nil)
  err = ffi.StatusPair(identity,-1,identity); assert(errors.As(err,&status) && status.Code == 31)
  failures = joined(ffi.StatusPair(leftPanic,1,rightPanic)); assert(len(failures) == 2)
  panicAt(failures[0],"StatusPair","left"); panicAt(failures[1],"StatusPair","right")
  failures = joined(ffi.VoidPair(leftPanic,rightPanic)); assert(len(failures) == 2)
  panicAt(failures[0],"VoidPair","left"); panicAt(failures[1],"VoidPair","right")
  textCalls := 0
  text := func(value string) int32 { textCalls++; assert(value == "ok"); return int32(len(value)) }
  value, err = ffi.Mixed(false,text,identity); assert(value == 29 && err == nil && textCalls == 1)
  value, err = ffi.Mixed(true,text,rightPanic); assert(value == 0 && textCalls == 1)
  failures = joined(err); assert(len(failures) == 2)
  var input *ffi.CallbackInputError
  assert(errors.As(failures[0],&input) && input.Function == "Mixed" && input.Parameter == "path")
  argumentAt(failures[0],"Mixed","runtime"); argumentAt(failures[1],"Mixed","visit")
  panicAt(failures[1],"Mixed","right")
  var concurrent atomic.Int32
  for i := 0; i < 10; i++ {
    value, err = ffi.ParallelPair(func(v int32) int32 { concurrent.Add(1); return v+100 },3,func(v int32) int32 { concurrent.Add(1); return v+200 })
    assert(value == 1234 && err == nil)
  }
  assert(concurrent.Load() == 20)
  value, err = ffi.ParallelPair(func(int32) int32 { panic("parallel-left") },3,func(int32) int32 { panic("parallel-right") })
  assert(value == 0)
  failures = joined(err); assert(len(failures) == 2)
  panicAt(failures[0],"ParallelPair","parallel-left"); panicAt(failures[1],"ParallelPair","parallel-right")
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
				t.Fatalf("generate Kinmokusei multiple-callback wrapper: err=%v diagnostics=%v", err, diagnostics)
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
				t.Fatalf("multiple-callback generated package failed: %v\n%s", err, output)
			}
			command = exec.CommandContext(ctx, "go", "vet", "./...")
			command.Dir = root
			command.Env = append(os.Environ(), "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("multiple-callback generated package vet failed: %v\n%s", err, output)
			}
		})
	}
}
