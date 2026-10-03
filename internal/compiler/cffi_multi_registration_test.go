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

func TestIncomingCFFIMultipleHandleRegistrations(t *testing.T) {
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
  "handles":[
    {"name":"Image","cType":"fixture_image","release":"fixture_image_free"},
    {"name":"Canvas","cType":"fixture_canvas","release":"fixture_canvas_free"}
  ],
  "callbacks":[{"name":"Visit","lifetime":"registered","parameters":[{"name":"value","type":"int32"}],"result":"boolean"}],
  "callbackRegistrations":[
    {"name":"Pair","callback":"Visit","register":"fixture_pair_register","unregister":"fixture_pair_unregister","parameters":[{"name":"left","type":"Image"},{"name":"right","type":"Image"},{"name":"key","type":"int32"}]},
    {"name":"Across","callback":"Visit","register":"fixture_across_register","unregister":"fixture_across_unregister","parameters":[{"name":"image","type":"Image"},{"name":"canvas","type":"Canvas"},{"name":"key","type":"int32"},{"name":"label","type":"retainedCString"},{"name":"data","type":"retainedBytes"}]}
  ],
  "functions":[
    {"name":"NewImage","symbol":"fixture_image_new","parameters":[{"name":"value","type":"int32"}],"result":"Image","convention":"statusOut"},
    {"name":"NewCanvas","symbol":"fixture_canvas_new","parameters":[{"name":"value","type":"int32"}],"result":"Canvas","convention":"statusOut"},
    {"name":"Fire","symbol":"fixture_fire","parameters":[{"name":"key","type":"int32"}],"result":"int32","convention":"statusOut"},
    {"name":"FailNextUnregister","symbol":"fixture_fail_next_unregister","parameters":[{"name":"key","type":"int32"}],"result":"void","convention":"status"},
    {"name":"ReleaseCount","symbol":"fixture_release_count","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"UnregisterCount","symbol":"fixture_unregister_count","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"Begin","symbol":"fixture_begin","parameters":[{"name":"key","type":"int32"}],"result":"void","convention":"status"},
    {"name":"Join","symbol":"fixture_join","parameters":[],"result":"void","convention":"status"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			// No ordinary function takes multiple handles: registrations alone
			// must enable the shared creation-ID lock order.
			normalized := strings.Join(strings.Fields(string(artifacts.Source)), " ")
			for _, want := range []string{"atomic.Uint64", "sort.Slice(kinmokuseiHandleLocks", "parameter0 *kinmokusei_cffi_handle_state_Image", "parameter1 *kinmokusei_cffi_handle_state_Canvas", "(*item.registrations)++", "(*item.registrations)--"} {
				if !strings.Contains(normalized, want) {
					t.Fatalf("missing multi-registration ownership machinery %q", want)
				}
			}
			threadCheck := "return false"
			if policy == "mainThread" {
				threadCheck = "return errors.Is(err, ffi.ErrWrongThread)"
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"app/binding.km": `import go ffi from "fixture.test";
function Sample(): Result<int32> {
  const left = ffi.NewImage(13)?;
  const right = ffi.NewImage(14)?;
  const watch = ffi.RegisterPair(left, right, 9, (value: int32): boolean => value == 27)?;
  const value = ffi.Fire(9)?;
  watch.Close()?;
  left.Close()?;
  right.Close()?;
  return ok(value);
}
`,
				"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef struct fixture_image fixture_image;
typedef struct fixture_canvas fixture_canvas;
typedef bool (*fixture_visit)(int32_t value, void *context);
int32_t fixture_image_new(int32_t value, fixture_image **output);
int32_t fixture_canvas_new(int32_t value, fixture_canvas **output);
void fixture_image_free(fixture_image *image);
void fixture_canvas_free(fixture_canvas *canvas);
int32_t fixture_pair_register(fixture_image *left, fixture_image *right, int32_t key, fixture_visit visit, void *context);
int32_t fixture_pair_unregister(fixture_image *left, fixture_image *right, int32_t key, fixture_visit visit, void *context);
int32_t fixture_across_register(fixture_image *image, fixture_canvas *canvas, int32_t key, char *label, uint8_t *data, size_t count, fixture_visit visit, void *context);
int32_t fixture_across_unregister(fixture_image *image, fixture_canvas *canvas, int32_t key, char *label, uint8_t *data, size_t count, fixture_visit visit, void *context);
int32_t fixture_fire(int32_t key, int32_t *output);
int32_t fixture_fail_next_unregister(int32_t key);
int32_t fixture_release_count(int32_t *output);
int32_t fixture_unregister_count(int32_t *output);
int32_t fixture_begin(int32_t key);
int32_t fixture_join(void);
`,
				"fixture.c": `#include "fixture.h"
#include <stdlib.h>
#include <stdatomic.h>
#ifdef _WIN32
#include <windows.h>
#else
#include <pthread.h>
#endif
struct fixture_image { int32_t value; };
struct fixture_canvas { int32_t value; };
typedef struct {
  fixture_image *left, *right;
  fixture_canvas *canvas;
  char *label;
  uint8_t *data;
  size_t count;
  fixture_visit visit;
  void *context;
  bool fail;
} fixture_entry;
static fixture_entry entries[64];
static atomic_flag mutex = ATOMIC_FLAG_INIT;
static _Atomic int32_t releases;
static _Atomic int32_t unregisters;
static void lock(void) { while (atomic_flag_test_and_set(&mutex)) {} }
static void unlock(void) { atomic_flag_clear(&mutex); }
int32_t fixture_image_new(int32_t value, fixture_image **output) {
  *output = malloc(sizeof(fixture_image)); if (!*output) return 1;
  (*output)->value = value; return 0;
}
int32_t fixture_canvas_new(int32_t value, fixture_canvas **output) {
  *output = malloc(sizeof(fixture_canvas)); if (!*output) return 1;
  (*output)->value = value; return 0;
}
void fixture_image_free(fixture_image *image) { atomic_fetch_add(&releases, 1); free(image); }
void fixture_canvas_free(fixture_canvas *canvas) { atomic_fetch_add(&releases, 1); free(canvas); }
static int32_t put(int32_t key, fixture_entry entry) {
  if (key < 0 || key >= 64) return 41;
  lock();
  if (entries[key].visit) { unlock(); return 42; }
  entries[key] = entry; unlock(); return 0;
}
static int32_t remove_entry(int32_t key, fixture_entry expected) {
  if (key < 0 || key >= 64) return 41;
  lock(); fixture_entry *entry = &entries[key];
  if (entry->visit != expected.visit || entry->context != expected.context ||
      entry->left != expected.left || entry->right != expected.right || entry->canvas != expected.canvas ||
      entry->label != expected.label || entry->data != expected.data || entry->count != expected.count) {
    unlock(); return 43;
  }
  if (entry->fail) { entry->fail = false; unlock(); return 31; }
  *entry = (fixture_entry){0}; atomic_fetch_add(&unregisters, 1); unlock(); return 0;
}
int32_t fixture_pair_register(fixture_image *left, fixture_image *right, int32_t key, fixture_visit visit, void *context) {
  return put(key, (fixture_entry){.left=left,.right=right,.visit=visit,.context=context});
}
int32_t fixture_pair_unregister(fixture_image *left, fixture_image *right, int32_t key, fixture_visit visit, void *context) {
  return remove_entry(key, (fixture_entry){.left=left,.right=right,.visit=visit,.context=context});
}
int32_t fixture_across_register(fixture_image *image, fixture_canvas *canvas, int32_t key, char *label, uint8_t *data, size_t count, fixture_visit visit, void *context) {
  return put(key, (fixture_entry){.left=image,.canvas=canvas,.label=label,.data=data,.count=count,.visit=visit,.context=context});
}
int32_t fixture_across_unregister(fixture_image *image, fixture_canvas *canvas, int32_t key, char *label, uint8_t *data, size_t count, fixture_visit visit, void *context) {
  return remove_entry(key, (fixture_entry){.left=image,.canvas=canvas,.label=label,.data=data,.count=count,.visit=visit,.context=context});
}
int32_t fixture_fire(int32_t key, int32_t *output) {
  if (key < 0 || key >= 64) return 41;
  lock(); fixture_entry *entry = &entries[key];
  if (!entry->visit) { unlock(); return 44; }
  int32_t value = entry->left->value + (entry->right ? entry->right->value : entry->canvas->value);
  if (entry->label) value += (unsigned char)entry->label[0];
  for (size_t i = 0; i < entry->count; i++) value += entry->data[i];
  *output = entry->visit(value, entry->context) ? value : 0;
  unlock(); return 0;
}
int32_t fixture_fail_next_unregister(int32_t key) {
  if (key < 0 || key >= 64) return 41;
  lock(); entries[key].fail = true; unlock(); return 0;
}
int32_t fixture_release_count(int32_t *output) { *output = atomic_load(&releases); return 0; }
int32_t fixture_unregister_count(int32_t *output) { *output = atomic_load(&unregisters); return 0; }
static fixture_entry job;
static bool job_result;
static void run_job(void) {
  // Read retained resources before entering Go. Successful unregister ends
  // all further native resource/context access except this admitted bridge.
  int32_t value = job.left->value + job.right->value;
  job_result = job.visit(value, job.context);
}
#ifdef _WIN32
static HANDLE worker;
static DWORD WINAPI worker_entry(LPVOID ignored) { run_job(); return 0; }
#else
static pthread_t worker;
static void *worker_entry(void *ignored) { run_job(); return NULL; }
#endif
int32_t fixture_begin(int32_t key) {
  if (key < 0 || key >= 64) return 41;
  lock(); job = entries[key]; unlock();
  if (!job.visit || !job.right) return 44;
#ifdef _WIN32
  worker = CreateThread(NULL, 0, worker_entry, NULL, 0, NULL);
  return worker ? 0 : 45;
#else
  return pthread_create(&worker, NULL, worker_entry, NULL) == 0 ? 0 : 45;
#endif
}
int32_t fixture_join(void) {
#ifdef _WIN32
  WaitForSingleObject(worker, INFINITE); CloseHandle(worker);
#else
  pthread_join(worker, NULL);
#endif
  return job_result ? 0 : 46;
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "fixture.test/app"
  "sync"
  "time"
)
const policy = "` + policy + `"
func wrongThread(err error) bool { ` + threadCheck + ` }
func check(ok bool, message string) { if !ok { panic(message) } }
func image(value int32) *ffi.Image { h, err := ffi.NewImage(value); check(err == nil, "create image"); return h }
func canvas(value int32) *ffi.Canvas { h, err := ffi.NewCanvas(value); check(err == nil, "create canvas"); return h }
func callback(value int32) bool { return value > 0 }
func fire(key, expected int32) { value, err := ffi.Fire(key); check(err == nil && value == expected, "callback value") }
func leased(h interface { Close() error }) { check(errors.Is(h.Close(), ffi.ErrHandleHasActiveRegistrations), "active resource lease") }
func closed(h interface { Close() error }) { check(h.Close() == nil, "release resource") }
func main() {
  left, right, target := image(2), image(3), canvas(10)
  pair, err := ffi.RegisterPair(left, right, 1, callback); check(err == nil, "register pair")
  data := []byte{1,2}
  across, err := ffi.RegisterAcross(left, target, 2, "A", data, callback); check(err == nil, "register across")
  data[0] = 100 // Retained bytes must be independent of caller-owned storage.
  fire(1, 5); fire(2, 80)
  leased(left); leased(right); leased(target)
  check(ffi.FailNextUnregister(1) == nil, "arm unregister failure")
  var status *ffi.StatusError
  check(errors.As(pair.Close(), &status) && status.Code == 31, "unregister failure")
  fire(1, 5); leased(left); leased(right)
  check(pair.Close() == nil, "retry unregister")
  leased(left); leased(target); closed(right)
  check(ffi.FailNextUnregister(2) == nil, "arm cross-type unregister failure")
  check(errors.As(across.Close(), &status) && status.Code == 31, "retained cross-type unregister failure")
  fire(2, 80); leased(left); leased(target)
  check(across.Close() == nil, "close overlapping registration")
  closed(left); closed(target)

  // Repeated arguments acquire exactly one lease, not two unmatched leases.
  same := image(4)
  pair, err = ffi.RegisterPair(same, same, 3, callback); check(err == nil, "same object registration")
  fire(3, 8); leased(same)
  check(pair.Close() == nil, "same object unregister"); closed(same)

  // Every invalid argument and C-level register failure is transactional.
  left, right = image(5), image(6)
  pair, err = ffi.RegisterPair(left, nil, 4, callback); check(pair == nil && errors.Is(err, ffi.ErrClosedHandle), "nil second handle")
  var empty ffi.Image
  pair, err = ffi.RegisterPair(left, &empty, 4, callback); check(pair == nil && errors.Is(err, ffi.ErrClosedHandle), "zero second handle")
  copy := *right
  pair, err = ffi.RegisterPair(left, &copy, 4, callback); check(pair == nil && errors.Is(err, ffi.ErrCopiedHandle), "copied second handle")
  pair, err = ffi.RegisterPair(left, right, 4, nil); check(pair == nil && errors.Is(err, ffi.ErrNilCallback), "nil callback")
  pair, err = ffi.RegisterPair(left, right, -1, callback); check(pair == nil && errors.As(err, &status) && status.Code == 41, "register failure")
  closed(right)
  pair, err = ffi.RegisterPair(left, right, 4, callback); check(pair == nil && errors.Is(err, ffi.ErrClosedHandle), "closed second handle")
  closed(left)

  // A registration retains the private ownership state, not a public wrapper
  // that can be assigned a different value while the C registration survives.
  left, right = image(7), image(8)
  pair, err = ffi.RegisterPair(left, right, 5, callback); check(err == nil, "snapshot registration")
  old := *right; *right = ffi.Image{}
  fire(5, 15)
  check(pair.Close() == nil, "unregister with overwritten public handle")
  *right = old; closed(left); closed(right)

  left, right = image(9), image(10)
  pair, err = ffi.RegisterPair(left, right, 6, func(int32) bool { panic("callback failure") }); check(err == nil, "panicking registration")
  fire(6, 0)
  var recovered *ffi.CallbackPanicError
  check(errors.As(pair.CallbackError(), &recovered), "callback panic boundary")
  check(pair.Close() == nil, "unregister after callback panic"); closed(left); closed(right)

  left, right = image(11), image(12)
  if policy == "mainThread" {
    // Rejected worker calls must not acquire leases or unregister live ones.
    pair, err = ffi.RegisterPair(left, right, 7, callback); check(err == nil, "main registration")
    done := make(chan error, 1)
    go func() { _, err := ffi.RegisterPair(right, left, 8, callback); done <- err }()
    check(wrongThread(<-done), "worker registration rejected")
    go func() { done <- pair.Close() }()
    check(wrongThread(<-done), "worker unregister rejected")
    leased(left); leased(right); fire(7, 23)
    check(pair.Close() == nil, "main unregister")
  } else {
    var workers sync.WaitGroup
    for i := 0; i < 12; i++ {
      workers.Add(1)
      go func(i int) {
        defer workers.Done()
        a, b := left, right; if i%2 != 0 { a, b = b, a }
        for j := 0; j < 20; j++ {
          registration, err := ffi.RegisterPair(a, b, int32(i+10), callback); check(err == nil, "concurrent reversed registration")
          leased(a); leased(b); fire(int32(i+10), 23)
          check(registration.Close() == nil, "concurrent reversed unregister")
        }
      }(i)
    }
    workers.Wait()
    // Racing closes must unregister and remove each lease only once.
    pair, err = ffi.RegisterPair(left, right, 7, callback); check(err == nil, "race close registration")
    results := make(chan error, 2)
    go func() { results <- pair.Close() }(); go func() { results <- pair.Close() }()
    a, b := <-results, <-results
    check((a == nil && errors.Is(b, ffi.ErrClosedCallbackRegistration)) || (b == nil && errors.Is(a, ffi.ErrClosedCallbackRegistration)), "single successful unregister")
  }
  closed(left); closed(right)
  extraReleases := int32(0)
  if policy == "threadSafe" || policy == "serialized" {
    // Native unregister may return before an admitted Go callback finishes.
    // Keep all leases while draining, but do not hold their mutexes or the
    // serialized binding mutex: the callback can still finish its work.
    left, right = image(20), image(21)
    entered, release := make(chan struct{}), make(chan struct{})
    pair, err = ffi.RegisterPair(left, right, 8, func(int32) bool {
      close(entered); <-release; leased(left); leased(right); return true
    }); check(err == nil, "draining registration")
    before, err := ffi.UnregisterCount(); check(err == nil, "initial unregister count")
    check(ffi.Begin(8) == nil, "begin native asynchronous callback"); <-entered
    done := make(chan error, 1); go func() { done <- pair.Close() }()
    deadline := time.Now().Add(3*time.Second)
    for {
      after, err := ffi.UnregisterCount(); check(err == nil, "observe native unregister")
      if after != before { break }
      check(time.Now().Before(deadline), "native unregister deadline")
      time.Sleep(time.Millisecond)
    }
    select { case <-done: panic("Close returned before callback drained"); default: }
    leased(left); leased(right)
    close(release); check(<-done == nil, "drain registered callback")
    check(ffi.Join() == nil, "join native callback worker")
    closed(left); closed(right); extraReleases = 2
  }
  value, err := app.Sample(); check(err == nil && value == 27, "Kinmokusei Result wrapper")
  count, err := ffi.ReleaseCount(); check(err == nil && count == 14+extraReleases, "exact native release count")
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
				t.Fatalf("generate Kinmokusei multiple-handle registration wrapper: err=%v diagnostics=%v", err, diagnostics)
			}
			if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generated, 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			arguments := []string{"run", "-buildvcs=false", "./cmd"}
			if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE") == "1" {
				arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
			}
			for _, args := range [][]string{arguments, {"vet", "./..."}} {
				command := exec.CommandContext(ctx, "go", args...)
				command.Dir = root
				command.Env = append(os.Environ(), "GOCACHE="+cache, "CGO_ENABLED=1")
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("multi-handle registration fixture (%v): %v\n%s", args, err, output)
				}
			}
		})
	}
}
