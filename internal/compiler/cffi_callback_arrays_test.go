package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIncomingCFFICallbackArrays(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	cache := filepath.Join(t.TempDir(), "go-cache")
	flags := `"cFlags":["-pthread"],"ldFlags":["-pthread"],`
	if runtime.GOOS == "windows" {
		flags = ""
	}
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",` + flags + `
  "enums":[{"name":"Mode","cType":"fixture_mode","underlying":"int32","values":[{"name":"ModeOn","symbol":"FIXTURE_ON"}]}],
  "structs":[
    {"name":"Coord","cType":"fixture_coord","fields":[{"name":"X","cName":"x","type":"int32"},{"name":"Y","cName":"y","type":"int32"}]},
    {"name":"Point","cType":"fixture_point","fields":[{"name":"Position","cName":"position","type":"Coord"},{"name":"Mode","cName":"mode","type":"Mode"},{"name":"Visible","cName":"visible","type":"boolean"}]}
  ],
  "callbacks":[
    {"name":"Scan","lifetime":"callScoped","parameters":[{"name":"values","type":"copiedArray","element":"int32"},{"name":"modes","type":"copiedArray","element":"Mode"},{"name":"points","type":"copiedArray","element":"Point"},{"name":"flags","type":"copiedArray","element":"boolean"},{"name":"wide","type":"copiedArray","element":"cInt32"}],"result":"int32"},
    {"name":"Update","lifetime":"callScoped","parameters":[{"name":"points","type":"inoutArray","element":"Point"},{"name":"modes","type":"inoutArray","element":"Mode"}],"result":"boolean"},
    {"name":"Observe","lifetime":"callScoped","parameters":[{"name":"points","type":"inoutArray","element":"Point"}],"result":"void"},
    {"name":"Overlap","lifetime":"callScoped","parameters":[{"name":"first","type":"inoutArray","element":"int32"},{"name":"second","type":"inoutArray","element":"int32"}],"result":"void"},
    {"name":"StoredScan","lifetime":"registered","parameters":[{"name":"points","type":"copiedArray","element":"Point"}],"result":"int32"},
    {"name":"StoredUpdate","lifetime":"registered","parameters":[{"name":"points","type":"inoutArray","element":"Point"},{"name":"modes","type":"inoutArray","element":"Mode"}],"result":"boolean"},
    {"name":"Owned","lifetime":"registered","parameters":[{"name":"points","type":"copiedArray","element":"Point"}],"result":"ownedArray","resultElement":"Point"}
  ],
  "callbackRegistrations":[
    {"name":"PointWatch","callback":"StoredScan","register":"fixture_watch_register","unregister":"fixture_watch_unregister"},
    {"name":"UpdateWatch","callback":"StoredUpdate","register":"fixture_update_watch_register","unregister":"fixture_update_watch_unregister"},
    {"name":"OwnedWatch","callback":"Owned","register":"fixture_owned_register","unregister":"fixture_owned_unregister"}
  ],
  "functions":[
    {"name":"RunScan","symbol":"fixture_scan","parameters":[{"name":"mode","type":"int32"},{"name":"visit","type":"Scan"}],"result":"int32","convention":"direct"},
    {"name":"RunCheckedScan","symbol":"fixture_checked_scan","parameters":[{"name":"mode","type":"int32"},{"name":"visit","type":"Scan"}],"result":"int32","convention":"statusOut"},
    {"name":"RunUpdate","symbol":"fixture_update","parameters":[{"name":"invalid","type":"boolean"},{"name":"visit","type":"Update"}],"result":"int32","convention":"direct"},
    {"name":"RunObserve","symbol":"fixture_observe","parameters":[{"name":"visit","type":"Observe"}],"result":"int32","convention":"direct"},
    {"name":"RunOverlap","symbol":"fixture_overlap","parameters":[{"name":"visit","type":"Overlap"}],"result":"int32","convention":"direct"},
    {"name":"RunPair","symbol":"fixture_pair","parameters":[{"name":"first","type":"Scan"},{"name":"second","type":"Scan"}],"result":"int32","convention":"direct"},
    {"name":"RunParallel","symbol":"fixture_parallel","parameters":[{"name":"visit","type":"Scan"}],"result":"int32","convention":"direct"},
    {"name":"FireWatch","symbol":"fixture_watch_fire","parameters":[{"name":"mode","type":"int32"}],"result":"int32","convention":"statusOut"},
    {"name":"FireUpdate","symbol":"fixture_update_watch_fire","parameters":[{"name":"invalid","type":"boolean"}],"result":"int32","convention":"statusOut"},
    {"name":"FireOwned","symbol":"fixture_owned_fire","parameters":[{"name":"mode","type":"int32"}],"result":"int32","convention":"statusOut"},
    {"name":"Mutation","symbol":"fixture_mutation","parameters":[],"result":"int32","convention":"statusOut"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"kinmokusei_c_int_must_be_32_bits", "type Scan func(values []int32", "points []Point", "const fixture_point *", "array byte size exceeds Go int"} {
				if !strings.Contains(string(artifacts.Source), want) {
					t.Fatalf("missing callback-array contract %q", want)
				}
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
// GCC/Clang's packed enum forces different C/Go layouts without expanding
// cgo's permitted compiler-flag set. Other compilers use an ordinary enum.
#if defined(__GNUC__) || defined(__clang__)
typedef enum __attribute__((packed)) { FIXTURE_ON = 7 } fixture_mode;
#else
typedef enum { FIXTURE_ON = 7 } fixture_mode;
#endif
typedef struct { int32_t x, y; } fixture_coord;
typedef struct { fixture_coord position; fixture_mode mode; bool visible; } fixture_point;
typedef int32_t (*fixture_scan_callback)(const int32_t *, size_t, const fixture_mode *, size_t, const fixture_point *, size_t, const bool *, size_t, const int *, size_t, void *);
typedef bool (*fixture_update_callback)(fixture_point *, size_t, fixture_mode *, size_t, void *);
typedef void (*fixture_observe_callback)(fixture_point *, size_t, void *);
typedef void (*fixture_overlap_callback)(int32_t *, size_t, int32_t *, size_t, void *);
typedef int32_t (*fixture_stored_callback)(const fixture_point *, size_t, void *);
typedef fixture_point *(*fixture_owned_callback)(const fixture_point *, size_t, size_t *, void *);
typedef void (*fixture_owned_release)(fixture_point *);
int32_t fixture_scan(int32_t mode, fixture_scan_callback visit, void *context);
int32_t fixture_checked_scan(int32_t mode, fixture_scan_callback visit, void *context, int32_t *output);
int32_t fixture_update(bool invalid, fixture_update_callback visit, void *context);
int32_t fixture_observe(fixture_observe_callback visit, void *context);
int32_t fixture_overlap(fixture_overlap_callback visit, void *context);
int32_t fixture_pair(fixture_scan_callback first, void *fc, fixture_scan_callback second, void *sc);
int32_t fixture_parallel(fixture_scan_callback visit, void *context);
int32_t fixture_watch_register(fixture_stored_callback visit, void *context);
int32_t fixture_watch_unregister(fixture_stored_callback visit, void *context);
int32_t fixture_watch_fire(int32_t mode, int32_t *output);
int32_t fixture_update_watch_register(fixture_update_callback visit, void *context);
int32_t fixture_update_watch_unregister(fixture_update_callback visit, void *context);
int32_t fixture_update_watch_fire(bool invalid, int32_t *output);
int32_t fixture_owned_register(fixture_owned_callback visit, fixture_owned_release release, void *context);
int32_t fixture_owned_unregister(fixture_owned_callback visit, fixture_owned_release release, void *context);
int32_t fixture_owned_fire(int32_t mode, int32_t *output);
int32_t fixture_mutation(int32_t *output);
`,
				"fixture.c": `#include "fixture.h"
#include <stdint.h>
#ifdef _WIN32
#include <windows.h>
#else
#include <pthread.h>
#endif
static fixture_point point(int32_t x) { return (fixture_point){{x,5},FIXTURE_ON,true}; }
static int32_t mutation;
int32_t fixture_scan(int32_t mode, fixture_scan_callback visit, void *context) {
  int32_t values[] = {2,3}; fixture_mode modes[] = {FIXTURE_ON,(fixture_mode)99};
  fixture_point points[] = {point(4)}; bool flags[] = {true,false}; int wide[] = {6};
  const fixture_point *input = points; size_t count = 1;
  if (mode == 1) { input = NULL; count = 0; }
  if (mode == 2) { input = NULL; }
  if (mode == 3) count = (size_t)INTPTR_MAX / sizeof(fixture_point) + 1;
  if (mode == 4) count = SIZE_MAX;
  if (mode == 5) count = 0;
  size_t mode_count = mode == 6 ? (size_t)INTPTR_MAX / sizeof(int32_t) + 1 : 2;
  int32_t a = visit(values,2,modes,mode_count,input,count,flags,2,wide,1,context);
  // Go mutations of copied inputs must never reach C; saved Go snapshots
  // must not change when C subsequently reuses or mutates this storage.
  if (values[0] != 2 || modes[0] != FIXTURE_ON || points[0].position.x != 4 || !flags[0] || wide[0] != 6) return -100;
  values[0] = 20; points[0].position.x = 40;
  int32_t b = visit(values,2,modes,mode_count,input,count,flags,2,wide,1,context);
  return a+b;
}
int32_t fixture_checked_scan(int32_t mode, fixture_scan_callback visit, void *context, int32_t *output) {
  *output = fixture_scan(mode,visit,context); return mode >= 2 ? 31 : 0;
}
int32_t fixture_update(bool invalid, fixture_update_callback visit, void *context) {
  fixture_point points[] = {point(4)}; fixture_mode modes[] = {FIXTURE_ON};
  bool value = visit(points,1,invalid ? NULL : modes,1,context);
  mutation = points[0].position.x + (int32_t)modes[0];
  return mutation + (value ? 1000 : 0);
}
int32_t fixture_observe(fixture_observe_callback visit, void *context) {
  fixture_point points[] = {point(4)}; visit(points,1,context); return points[0].position.x;
}
int32_t fixture_overlap(fixture_overlap_callback visit, void *context) {
  int32_t values[] = {1,2}; visit(values,2,values+1,1,context); return 10*values[0]+values[1];
}
int32_t fixture_pair(fixture_scan_callback first, void *fc, fixture_scan_callback second, void *sc) {
  int32_t a = fixture_scan(2,first,fc), b = fixture_scan(3,second,sc); return a+b;
}
typedef struct { fixture_scan_callback visit; void *context; int32_t result; } fixture_job;
static void run_job(fixture_job *job) { job->result = fixture_scan(0,job->visit,job->context); }
#ifdef _WIN32
static DWORD WINAPI job_entry(LPVOID input) { run_job(input); return 0; }
#else
static void *job_entry(void *input) { run_job(input); return NULL; }
#endif
int32_t fixture_parallel(fixture_scan_callback visit, void *context) {
  fixture_job a = {visit,context,0}, b = {visit,context,0};
#ifdef _WIN32
  HANDLE first = CreateThread(NULL,0,job_entry,&a,0,NULL); if (!first) return -1;
  HANDLE second = CreateThread(NULL,0,job_entry,&b,0,NULL);
  if (second) { WaitForSingleObject(second,INFINITE); CloseHandle(second); }
  WaitForSingleObject(first,INFINITE); CloseHandle(first); if (!second) return -1;
#else
  pthread_t first, second; if (pthread_create(&first,NULL,job_entry,&a) != 0) return -1;
  int status = pthread_create(&second,NULL,job_entry,&b);
  if (status == 0) pthread_join(second,NULL);
  pthread_join(first,NULL); if (status != 0) return -1;
#endif
  return a.result+b.result;
}
static fixture_stored_callback stored;
static void *stored_context;
int32_t fixture_watch_register(fixture_stored_callback visit, void *context) { stored = visit; stored_context = context; return 0; }
int32_t fixture_watch_unregister(fixture_stored_callback visit, void *context) { if (visit != stored || context != stored_context) return 1; stored = NULL; return 0; }
static const fixture_point *input_points(int32_t mode, fixture_point *points, size_t *count) {
  *count = mode == 1 || mode == 5 ? 0 : 1;
  if (mode == 3) *count = (size_t)INTPTR_MAX / sizeof(fixture_point) + 1;
  if (mode == 4) *count = SIZE_MAX;
  return mode == 1 || mode == 2 ? NULL : points;
}
int32_t fixture_watch_fire(int32_t mode, int32_t *output) {
  fixture_point points[] = {point(4)}; size_t count;
  const fixture_point *input = input_points(mode,points,&count);
  *output = stored(input,count,stored_context);
  if (points[0].position.x != 4) return 32;
  points[0].position.x = 400; return 0;
}
static fixture_update_callback stored_update;
static void *stored_update_context;
int32_t fixture_update_watch_register(fixture_update_callback visit, void *context) { stored_update=visit; stored_update_context=context; return 0; }
int32_t fixture_update_watch_unregister(fixture_update_callback visit, void *context) { if (visit != stored_update || context != stored_update_context) return 1; stored_update=NULL; return 0; }
int32_t fixture_update_watch_fire(bool invalid, int32_t *output) { *output=fixture_update(invalid,stored_update,stored_update_context); return 0; }
static fixture_owned_callback owned;
static fixture_owned_release owned_release;
static void *owned_context;
int32_t fixture_owned_register(fixture_owned_callback visit, fixture_owned_release release, void *context) { owned=visit; owned_release=release; owned_context=context; return 0; }
int32_t fixture_owned_unregister(fixture_owned_callback visit, fixture_owned_release release, void *context) { if (visit != owned || release != owned_release || context != owned_context) return 1; owned=NULL; return 0; }
int32_t fixture_owned_fire(int32_t mode, int32_t *output) {
  fixture_point points[] = {point(4)}; size_t count, length = 99;
  const fixture_point *input = input_points(mode,points,&count);
  fixture_point *result = owned(input,count,&length,owned_context);
  if (!result && length) return 33;
  *output = length ? result[0].position.x : 0;
  if (result) owned_release(result); return 0;
}
int32_t fixture_mutation(int32_t *output) { *output = mutation; return 0; }
`,
				"app/binding.km": `import go ffi from "fixture.test";
function Update(): Result<int32> {
  const result = ffi.RunObserve((points: ffi.Point[]): void => { points[0].Position.X = 42; })?;
  return ok(result);
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "fixture.test/app"
  "sync/atomic"
)
func check(ok bool, message string) { if !ok { panic(message) } }
func inputError(err error, parameter string) {
  var failure *ffi.CallbackInputError
  check(errors.As(err,&failure) && failure.Parameter == parameter, "array input error identity")
}
func scan(values []int32, modes []ffi.Mode, points []ffi.Point, flags []bool, wide []int32) int32 {
  check(len(values)==2 && values[1]==3 && modes[0]==ffi.ModeOn && modes[1]==99 && flags[0] && !flags[1] && wide[0]==6, "scalar/enum/bool/C-int array inputs")
  if len(points) != 0 { check(points[0].Position.Y==5 && points[0].Mode==ffi.ModeOn && points[0].Visible, "nested POD array") }
  return values[0]
}
func main() {
  var snapshots [][]ffi.Point
  calls := 0
  result, err := ffi.RunScan(0,func(values []int32,modes []ffi.Mode,points []ffi.Point,flags []bool,wide []int32) int32 {
    scan(values,modes,points,flags,wide); snapshots=append(snapshots,points); calls++
    original := values[0]; values[0]=900; modes[0]=100; flags[0]=false; wide[0]=100
    return original
  })
  check(err==nil && result==22 && calls==2, "copied arrays runtime")
  check(snapshots[0][0].Position.X==4 && snapshots[1][0].Position.X==40, "saved snapshots survive native mutation")
  result, err = ffi.RunCheckedScan(0,scan); check(err==nil && result==22, "status-out array callback")
  for _, mode := range []int32{1,5} {
    result, err = ffi.RunScan(mode,func(values []int32,modes []ffi.Mode,points []ffi.Point,flags []bool,wide []int32) int32 {
      scan(values,modes,points,flags,wide); check(points!=nil && len(points)==0, "non-nil empty array"); return 1
    }); check(err==nil && result==2, "null/non-null empty arrays")
  }
  for _, mode := range []int32{2,3,4} {
    result, err = ffi.RunCheckedScan(mode,func([]int32,[]ffi.Mode,[]ffi.Point,[]bool,[]int32) int32 { panic("invalid array reached callback") })
    check(result==0, "failed callback zero result"); inputError(err,"points")
  }
  result, err = ffi.RunCheckedScan(6,scan); check(result==0, "Go layout overflow zero result"); inputError(err,"modes")
  result, err = ffi.RunUpdate(false,func(points []ffi.Point,modes []ffi.Mode) bool {
    points[0].Position.X=42; modes[0]=99; return false
  }); check(err==nil && result==141, "copy-back on false result")
  result, err = ffi.RunUpdate(false,func(points []ffi.Point,modes []ffi.Mode) bool {
    points[0].Position.X=42; modes[0]=99; return true
  }); check(err==nil && result==1141, "copy-back on true result")
  result, err = ffi.RunObserve(func(points []ffi.Point) { points[0].Position.X=42 }); check(err==nil && result==42, "void array copy-back")
  result, err = ffi.RunOverlap(func(first,second []int32) { check(second[0]==2, "independent overlapping inputs"); first[0]=7; first[1]=8; second[0]=9 })
  check(err==nil && result==79, "overlap writes in declaration order")
  result, err = ffi.RunUpdate(true,func([]ffi.Point,[]ffi.Mode) bool { panic("second invalid array reached user code") })
  check(result==0, "invalid writable input zero result"); inputError(err,"modes")
  mutation, err := ffi.Mutation(); check(err==nil && mutation==11, "no write after validation failure")
  result, err = ffi.RunUpdate(false,func(points []ffi.Point,modes []ffi.Mode) bool { points[0].Position.X=42; modes[0]=99; panic(nil) })
  var recovered *ffi.CallbackPanicError
  check(result==0 && errors.As(err,&recovered), "array panic captured")
  mutation, err = ffi.Mutation(); check(err==nil && mutation==11, "no write after panic")
  result, err = ffi.RunPair(scan,scan)
  tree, ok := err.(interface { Unwrap() []error }); check(result==0 && ok && len(tree.Unwrap())==2, "joined array failures")
  for _, failure := range tree.Unwrap() { inputError(failure,"points") }
  var parallelCalls atomic.Int32
  result, err = ffi.RunParallel(func(values []int32,modes []ffi.Mode,points []ffi.Point,flags []bool,wide []int32) int32 {
    parallelCalls.Add(1); return scan(values,modes,points,flags,wide)
  }); check(err==nil && result==44 && parallelCalls.Load()==4, "concurrent native array callbacks")
  var saved []ffi.Point
  watch, err := ffi.RegisterPointWatch(func(points []ffi.Point) int32 { saved=points; return points[0].Position.X })
  check(err==nil, "register copied arrays")
  result, err = ffi.FireWatch(0); check(err==nil && result==4 && saved[0].Position.X==4, "registered snapshot isolation")
  check(watch.Close()==nil && saved[0].Position.X==4, "snapshot survives unregister")
  for _, mode := range []int32{2,3,4} {
    watch, err = ffi.RegisterPointWatch(func([]ffi.Point) int32 { panic("invalid registered array reached callback") }); check(err==nil, "register invalid array probe")
    result, err = ffi.FireWatch(mode); check(err==nil && result==0, "registered invalid zero result")
    inputError(watch.CallbackError(),"points"); check(watch.Close()==nil, "close failed array registration")
  }
  update, err := ffi.RegisterUpdateWatch(func(points []ffi.Point,modes []ffi.Mode) bool { points[0].Position.X=42; modes[0]=99; return false })
  check(err==nil, "register mutable arrays")
  result, err = ffi.FireUpdate(false); check(err==nil && result==141, "registered false result copy-back")
  check(update.Close()==nil, "close mutable array registration")
  calls=0
  update, err = ffi.RegisterUpdateWatch(func(points []ffi.Point,modes []ffi.Mode) bool { calls++; points[0].Position.X=42; modes[0]=99; panic(nil) })
  check(err==nil, "register panicking mutable arrays")
  for i:=0; i<2; i++ {
    result, err = ffi.FireUpdate(false); check(err==nil && result==11, "registered panic leaves C arrays unchanged")
  }
  check(calls==1 && errors.As(update.CallbackError(),&recovered), "registered array panic disables user code")
  check(update.Close()==nil, "close panicking mutable array registration")
  owned, err := ffi.RegisterOwnedWatch(func(points []ffi.Point) []ffi.Point { points[0].Position.X=42; return points }); check(err==nil, "register owned array conversion")
  result, err = ffi.FireOwned(0); check(err==nil && result==42, "copied input with owned array output")
  check(owned.Close()==nil, "close owned result registration")
  owned, err = ffi.RegisterOwnedWatch(func([]ffi.Point) []ffi.Point { panic("invalid owned input reached callback") }); check(err==nil, "register invalid owned input")
  result, err = ffi.FireOwned(2); check(err==nil && result==0, "zero owned output length on input error")
  inputError(owned.CallbackError(),"points"); check(owned.Close()==nil, "close invalid owned input")
  result, err = app.Update(); check(err==nil && result==42, "Kinmokusei typed-array callback wrapper")
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
				t.Fatalf("generate typed-array callback wrapper: err=%v diagnostics=%v", err, diagnostics)
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
			for _, arguments := range [][]string{args, {"vet", "./..."}} {
				command := exec.CommandContext(ctx, "go", arguments...)
				command.Dir = root
				command.Env = append(os.Environ(), "GOCACHE="+cache, "CGO_ENABLED=1", "GODEBUG=panicnil=1")
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("callback arrays fixture (%v): %v\n%s", arguments, err, output)
				}
			}
		})
	}
}

func TestIncomingCFFICallbackArrayValidation(t *testing.T) {
	for _, kind := range []string{"copiedArray", "inoutArray"} {
		for _, element := range []string{"int8", "int16", "int32", "int64", "byte", "uint16", "uint32", "uint64", "float32", "float64", "boolean", "cInt32", "cUint32"} {
			t.Run(kind+"/accepted/"+element, func(t *testing.T) {
				manifest := `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"values","type":"` + kind + `","element":"` + element + `"}],"result":"void"}],"functions":[{"name":"Value","symbol":"fixture_value","parameters":[{"name":"visit","type":"Visit"}],"result":"void","convention":"direct"}]}`
				if _, err := GenerateCFFI([]byte(manifest)); err != nil {
					t.Fatalf("supported scalar array rejected: %v", err)
				}
			})
		}
		for _, element := range []string{"", "void", "cstring", "borrowedArray", "copiedBytes", "Handle", "Visit", "Missing"} {
			t.Run(kind+"/"+element, func(t *testing.T) {
				manifest := `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe","handles":[{"name":"Handle","cType":"fixture_handle","release":"fixture_free"}],"callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"values","type":"` + kind + `","element":"` + element + `"}],"result":"void"}],"functions":[{"name":"Value","symbol":"fixture_value","parameters":[{"name":"visit","type":"Visit"}],"result":"void","convention":"direct"}]}`
				if _, err := GenerateCFFI([]byte(manifest)); err == nil || !strings.Contains(err.Error(), "requires a supported element") {
					t.Fatalf("error=%v; expected unsupported callback array element", err)
				}
			})
		}
		for _, test := range []struct{ name, declaration, want string }{
			{"function input", `"functions":[{"name":"Value","symbol":"fixture_value","parameters":[{"name":"values","type":"` + kind + `","element":"int32"}],"result":"void","convention":"direct"}]`, "element only for borrowedArray"},
			{"function result", `"functions":[{"name":"Value","symbol":"fixture_value","parameters":[],"result":"` + kind + `","convention":"direct"}]`, "unsupported result type"},
			{"registration input", `"callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"values","type":"` + kind + `","element":"int32"}],"register":"fixture_register","unregister":"fixture_unregister"}],"functions":[{"name":"Value","symbol":"fixture_value","parameters":[],"result":"void","convention":"direct"}]`, "may not declare element"},
			{"callback result", `"callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"` + kind + `"}],"functions":[{"name":"Value","symbol":"fixture_value","parameters":[],"result":"void","convention":"direct"}]`, "unsupported scalar or enum result"},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				manifest := `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe",` + test.declaration + `}`
				if _, err := GenerateCFFI([]byte(manifest)); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error=%v; want %q", err, test.want)
				}
			})
		}
	}
}
