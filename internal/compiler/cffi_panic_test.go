package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIncomingCFFICallbackNilPanic(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	cache := filepath.Join(t.TempDir(), "go-cache")
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",
  "callbacks":[
    {"name":"Number","lifetime":"callScoped","parameters":[],"result":"int32"},
    {"name":"Predicate","lifetime":"callScoped","parameters":[],"result":"boolean"},
    {"name":"Notify","lifetime":"callScoped","parameters":[],"result":"void"},
    {"name":"Mutate","lifetime":"callScoped","parameters":[{"name":"data","type":"inoutBytes"}],"result":"int32"},
    {"name":"Bytes","lifetime":"registered","parameters":[],"result":"ownedBytes"},
    {"name":"Text","lifetime":"registered","parameters":[{"name":"path","type":"copiedCString"}],"result":"ownedCString"},
    {"name":"Values","lifetime":"registered","parameters":[],"result":"ownedArray","resultElement":"int32"}
  ],
  "callbackRegistrations":[
    {"name":"ByteWatch","callback":"Bytes","register":"fixture_bytes_register","unregister":"fixture_bytes_unregister"},
    {"name":"TextWatch","callback":"Text","register":"fixture_text_register","unregister":"fixture_text_unregister"},
    {"name":"ValueWatch","callback":"Values","register":"fixture_values_register","unregister":"fixture_values_unregister"}
  ],
  "functions":[
    {"name":"InvokeNumber","symbol":"fixture_number","parameters":[{"name":"callback","type":"Number"}],"result":"int32","convention":"direct"},
    {"name":"InvokePredicate","symbol":"fixture_predicate","parameters":[{"name":"callback","type":"Predicate"}],"result":"boolean","convention":"direct"},
    {"name":"InvokeNotify","symbol":"fixture_notify","parameters":[{"name":"callback","type":"Notify"}],"result":"void","convention":"direct"},
    {"name":"InvokeMutate","symbol":"fixture_mutate","parameters":[{"name":"invalid","type":"boolean"},{"name":"callback","type":"Mutate"}],"result":"int32","convention":"direct"},
    {"name":"InvokeBoth","symbol":"fixture_both","parameters":[{"name":"first","type":"Number"},{"name":"second","type":"Number"}],"result":"int32","convention":"direct"},
    {"name":"ReadMutation","symbol":"fixture_read_mutation","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"FireBytes","symbol":"fixture_bytes_fire","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"FireText","symbol":"fixture_text_fire","parameters":[{"name":"invalid","type":"boolean"}],"result":"int32","convention":"statusOut"},
    {"name":"FireValues","symbol":"fixture_values_fire","parameters":[],"result":"int32","convention":"statusOut"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			executor := "return true"
			if policy == "threadAffine" {
				executor = `returned := false
  defer func() { recover(); preserved = !returned }()
  kinmokuseiCFFIDo(func() { panic(nil) })
  returned = true
  return false`
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"probe.go":         "package ffi\nfunc ExecutorPanicIsPreserved() (preserved bool) { " + executor + " }\n",
				"fixture.h": `#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>
typedef int32_t (*fixture_number_callback)(void *);
typedef bool (*fixture_predicate_callback)(void *);
typedef void (*fixture_notify_callback)(void *);
typedef int32_t (*fixture_mutate_callback)(uint8_t *, size_t, void *);
int32_t fixture_number(fixture_number_callback callback, void *context);
bool fixture_predicate(fixture_predicate_callback callback, void *context);
void fixture_notify(fixture_notify_callback callback, void *context);
int32_t fixture_mutate(bool invalid, fixture_mutate_callback callback, void *context);
int32_t fixture_both(fixture_number_callback first, void *fc, fixture_number_callback second, void *sc);
int32_t fixture_read_mutation(int32_t *output);
typedef uint8_t *(*fixture_bytes_callback)(size_t *, void *);
typedef void (*fixture_bytes_release)(uint8_t *);
typedef char *(*fixture_text_callback)(const char *, void *);
typedef void (*fixture_text_release)(char *);
typedef int32_t *(*fixture_values_callback)(size_t *, void *);
typedef void (*fixture_values_release)(int32_t *);
int32_t fixture_bytes_register(fixture_bytes_callback callback, fixture_bytes_release release, void *context);
int32_t fixture_bytes_unregister(fixture_bytes_callback callback, fixture_bytes_release release, void *context);
int32_t fixture_bytes_fire(int32_t *output);
int32_t fixture_text_register(fixture_text_callback callback, fixture_text_release release, void *context);
int32_t fixture_text_unregister(fixture_text_callback callback, fixture_text_release release, void *context);
int32_t fixture_text_fire(bool invalid, int32_t *output);
int32_t fixture_values_register(fixture_values_callback callback, fixture_values_release release, void *context);
int32_t fixture_values_unregister(fixture_values_callback callback, fixture_values_release release, void *context);
int32_t fixture_values_fire(int32_t *output);
`,
				"fixture.c": `#include "fixture.h"
#include <string.h>
int32_t fixture_number(fixture_number_callback callback, void *context) {
  int32_t a = callback(context), b = callback(context); return a+b;
}
bool fixture_predicate(fixture_predicate_callback callback, void *context) {
  bool a = callback(context), b = callback(context); return a || b;
}
void fixture_notify(fixture_notify_callback callback, void *context) { callback(context); callback(context); }
static int32_t mutation;
int32_t fixture_mutate(bool invalid, fixture_mutate_callback callback, void *context) {
  uint8_t data[] = {7};
  int32_t a = callback(invalid ? NULL : data, 1, context);
  int32_t b = callback(invalid ? NULL : data, 1, context);
  mutation = data[0]; return a+b;
}
int32_t fixture_read_mutation(int32_t *output) { *output = mutation; return 0; }
int32_t fixture_both(fixture_number_callback first, void *fc, fixture_number_callback second, void *sc) {
  int32_t a = first(fc), b = second(sc), c = first(fc), d = second(sc); return a+b+c+d;
}
#define WATCH(kind) \
static fixture_##kind##_callback kind##_callback; \
static fixture_##kind##_release kind##_release; \
static void *kind##_context; \
int32_t fixture_##kind##_register(fixture_##kind##_callback callback, fixture_##kind##_release release, void *context) { \
  kind##_callback = callback; kind##_release = release; kind##_context = context; return 0; \
} \
int32_t fixture_##kind##_unregister(fixture_##kind##_callback callback, fixture_##kind##_release release, void *context) { \
  if (kind##_callback != callback || kind##_release != release || kind##_context != context) return 1; \
  kind##_callback = NULL; kind##_release = NULL; kind##_context = NULL; return 0; \
}
WATCH(bytes)
WATCH(text)
WATCH(values)
int32_t fixture_bytes_fire(int32_t *output) {
  *output = 0;
  for (int i = 0; i < 2; i++) {
    size_t length = 99; uint8_t *data = bytes_callback(&length, bytes_context);
    if (!data && length) return 2;
    *output += (int32_t)length;
    if (data) bytes_release(data);
  }
  return 0;
}
int32_t fixture_text_fire(bool invalid, int32_t *output) {
  *output = 0;
  for (int i = 0; i < 2; i++) {
    char *text = text_callback(invalid ? NULL : "ok", text_context);
    if (text) { *output += (int32_t)strlen(text); text_release(text); }
  }
  return 0;
}
int32_t fixture_values_fire(int32_t *output) {
  *output = 0;
  for (int i = 0; i < 2; i++) {
    size_t length = 99; int32_t *data = values_callback(&length, values_context);
    if (!data && length) return 2;
    *output += (int32_t)length;
    if (data) values_release(data);
  }
  return 0;
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "os"
)
func check(ok bool, message string) { if !ok { panic(message) } }
func panicError(err error, function string) {
  var failure *ffi.CallbackPanicError
  check(errors.As(err, &failure), "nil panic was swallowed: " + function)
  check(failure.Function == function, "panic function identity")
  check((failure.Value == nil) == (os.Getenv("GODEBUG") == "panicnil=1"), "preserve recovered panic value")
}
func main() {
  calls := 0
  value, err := ffi.InvokeNumber(func() int32 { calls++; panic(nil) })
  check(value == 0 && calls == 1, "suppress numeric callback after nil panic"); panicError(err, "InvokeNumber")
  calls = 0
  predicate, err := ffi.InvokePredicate(func() bool { calls++; panic(nil) })
  check(!predicate && calls == 1, "suppress bool callback after nil panic"); panicError(err, "InvokePredicate")
  calls = 0
  err = ffi.InvokeNotify(func() { calls++; panic(nil) })
  check(calls == 1, "suppress void callback after nil panic"); panicError(err, "InvokeNotify")
  calls = 0
  value, err = ffi.InvokeMutate(false, func(data []byte) int32 { calls++; data[0] = 42; panic(nil) })
  check(value == 0 && calls == 1, "suppress mutable callback after nil panic"); panicError(err, "InvokeMutate")
  mutation, err := ffi.ReadMutation(); check(err == nil && mutation == 7, "no copy-back after nil panic")
  value, err = ffi.InvokeMutate(true, func([]byte) int32 { panic("invalid input reached user code") })
  var input *ffi.CallbackInputError
  var recovered *ffi.CallbackPanicError
  check(value == 0 && errors.As(err,&input) && !errors.As(err,&recovered), "input rejection is not a panic")
  first, second := 0, 0
  value, err = ffi.InvokeBoth(func() int32 { first++; panic(nil) }, func() int32 { second++; panic(nil) })
  check(value == 0 && first == 1 && second == 1, "independent nil-panic slots")
  tree, ok := err.(interface { Unwrap() []error }); check(ok && len(tree.Unwrap()) == 2, "joined nil panics")
  for i, slot := range tree.Unwrap() {
    var argument *ffi.CallbackArgumentError
    name := "first"; if i != 0 { name = "second" }
    check(errors.As(slot,&argument) && argument.Parameter == name, "nil-panic slot identity")
    panicError(slot, "InvokeBoth")
  }
  calls = 0
  bytes, err := ffi.RegisterByteWatch(func() []byte { calls++; panic(nil) }); check(err == nil, "register bytes")
  value, err = ffi.FireBytes(); check(value == 0 && err == nil && calls == 1, "owned bytes nil-panic result")
  panicError(bytes.CallbackError(), "ByteWatch"); check(bytes.Close() == nil, "close bytes after nil panic")
  calls = 0
  text, err := ffi.RegisterTextWatch(func(string) string { calls++; panic(nil) }); check(err == nil, "register text")
  value, err = ffi.FireText(false); check(value == 0 && err == nil && calls == 1, "owned text nil-panic result")
  panicError(text.CallbackError(), "TextWatch"); check(text.Close() == nil, "close text after nil panic")
  calls = 0
  values, err := ffi.RegisterValueWatch(func() []int32 { calls++; panic(nil) }); check(err == nil, "register values")
  value, err = ffi.FireValues(); check(value == 0 && err == nil && calls == 1, "owned array nil-panic result")
  panicError(values.CallbackError(), "ValueWatch"); check(values.Close() == nil, "close values after nil panic")
  // Legitimate early returns must remain input errors, not completion failures.
  text, err = ffi.RegisterTextWatch(func(string) string { return "bad\x00text" }); check(err == nil, "register invalid text result")
  value, err = ffi.FireText(false); check(value == 0 && err == nil, "reject embedded NUL result")
  check(errors.As(text.CallbackError(),&input) && !errors.As(text.CallbackError(),&recovered), "owned result rejection is not a panic")
  check(text.Close() == nil, "close invalid text result")
  text, err = ffi.RegisterTextWatch(func(string) string { panic("null input reached user code") }); check(err == nil, "register copied input")
  value, err = ffi.FireText(true); check(value == 0 && err == nil, "reject null copied input")
  check(errors.As(text.CallbackError(),&input) && !errors.As(text.CallbackError(),&recovered), "copied input rejection is not a panic")
  check(text.Close() == nil, "close invalid copied input")
  // Recovery inside user code is an ordinary successful callback return.
  value, err = ffi.InvokeNumber(func() (result int32) {
    defer func() { recover(); result = 9 }()
    panic(nil)
  }); check(value == 18 && err == nil, "user-recovered nil panic is successful")
  check(ffi.ExecutorPanicIsPreserved(), "affine executor swallowed nil panic")
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
			for _, mode := range []string{"panicnil=1", "panicnil=0"} {
				t.Run(mode, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()
					args := []string{"run", "-buildvcs=false", "./cmd"}
					if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE") == "1" {
						args = []string{"run", "-race", "-buildvcs=false", "./cmd"}
					}
					command := exec.CommandContext(ctx, "go", args...)
					command.Dir = root
					command.Env = append(os.Environ(), "GOCACHE="+cache, "CGO_ENABLED=1", "GODEBUG="+mode)
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("callback nil-panic fixture: %v\n%s", err, output)
					}
				})
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "go", "vet", "./...")
			command.Dir = root
			command.Env = append(os.Environ(), "GOCACHE="+cache, "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("callback nil-panic fixture vet: %v\n%s", err, output)
			}
		})
	}
}
