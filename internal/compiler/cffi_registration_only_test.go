package compiler

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIncomingCFFIRegistrationOnlyValidation(t *testing.T) {
	t.Parallel()
	const prefix = `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe",`
	const callback = `"callbacks":[{"name":"Notice","lifetime":"registered","parameters":[],"result":"void"}],`
	const registration = `"callbackRegistrations":[{"name":"Watch","callback":"Notice","register":"watch_add","unregister":"watch_remove"}]`
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{"empty", `"functions":[]`, "at least one function or callback registration"},
		{"null", `"functions":null,"callbackRegistrations":null`, "at least one function or callback registration"},
		{"empty registrations", `"callbackRegistrations":[]`, "at least one function or callback registration"},
		{"callback without operation", strings.TrimSuffix(callback, ","), "at least one function or callback registration"},
		{"type without operation", `"structs":[{"name":"Point","cType":"point","fields":[{"name":"X","cName":"x","type":"int32"}]}]`, "at least one function or callback registration"},
		{"missing callback", registration, "must reference a registered callback"},
		{"call scoped callback", strings.Replace(callback, "registered", "callScoped", 1) + registration, "must reference a registered callback"},
		{"invalid symbol", callback + strings.Replace(registration, "watch_remove", "watch_remove();", 1), "register/unregister symbols must be identifiers"},
		{"duplicate registration", callback + strings.Replace(registration, `}]`, `},{"name":"Watch","callback":"Notice","register":"other_add","unregister":"other_remove"}]`, 1), "unique exported identifier"},
		{"borrowed registration input", callback + strings.Replace(registration, `"register":`, `"parameters":[{"name":"data","type":"borrowedBytes"}],"register":`, 1), "unsupported value type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateCFFI([]byte(prefix + test.body + `}`))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("GenerateCFFI error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestIncomingCFFIRegistrationOnly(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	cache := filepath.Join(t.TempDir(), "go-cache")
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			manifest := []byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",
  "callbacks":[{"name":"Notice","lifetime":"registered","parameters":[{"name":"events","type":"copiedArray","element":"int32"}],"result":"boolean"}],
  "callbackRegistrations":[{"name":"Watch","callback":"Notice","register":"fixture_register","unregister":"fixture_unregister","parameters":[{"name":"label","type":"retainedCString"},{"name":"data","type":"retainedBytes"},{"name":"context","type":"int32"},{"name":"reject","type":"boolean"},{"name":"retry","type":"boolean"}]}]
}`)
			artifacts, err := GenerateCFFI(manifest)
			if err != nil {
				t.Fatal(err)
			}
			// An explicitly empty operation list must generate the same binding
			// as omission; callers should not need a dummy C operation either way.
			explicit, err := GenerateCFFI(bytes.Replace(manifest, []byte(`"callbacks":`), []byte(`"functions":[],"callbacks":`), 1))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(artifacts.Source, explicit.Source) {
				t.Fatal("omitted and empty functions generated different bindings")
			}
			threadCheck := "return false"
			if policy == "mainThread" {
				threadCheck = "return errors.Is(err, ffi.ErrWrongThread)"
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef bool (*fixture_notice)(const int32_t *, size_t, void *);
int32_t fixture_register(char *label, uint8_t *data, size_t count, int32_t tag, bool reject, bool retry, fixture_notice callback, void *context);
int32_t fixture_unregister(char *label, uint8_t *data, size_t count, int32_t tag, bool reject, bool retry, fixture_notice callback, void *context);
int32_t fixture_fire(void);
void fixture_counts(int32_t *registers, int32_t *unregisters);
`,
				"fixture.c": `#include "fixture.h"
#include <string.h>
static struct {
  char *label;
  uint8_t *data;
  size_t count;
  int32_t tag;
  bool retry, first_close;
  fixture_notice callback;
  void *context;
} current;
static int32_t registers, unregisters;
int32_t fixture_register(char *label, uint8_t *data, size_t count, int32_t tag, bool reject, bool retry, fixture_notice callback, void *context) {
  registers++;
  if (current.callback) return 42;
  int32_t events[] = {7,8};
  // Some subscription APIs deliver an initial notification before returning
  // the registration. Its Go context must already be alive at this point.
  callback(strcmp(label,"invalid") == 0 ? NULL : events,2,context);
  if (events[0] != 7 || events[1] != 8) return 43;
  if (reject) return 31; // Failure retains no argument or future callback.
  current.label=label; current.data=data; current.count=count; current.tag=tag;
  current.retry=retry; current.first_close=true;
  current.callback=callback; current.context=context;
  return 0;
}
int32_t fixture_unregister(char *label, uint8_t *data, size_t count, int32_t tag, bool reject, bool retry, fixture_notice callback, void *context) {
  unregisters++;
  if (current.label!=label || current.data!=data || current.count!=count || current.tag!=tag ||
      current.retry!=retry || current.callback!=callback || current.context!=context || reject) return 44;
  if (strcmp(label,"normal") == 0 && (count!=2 || data[0]!=1 || data[1]!=2)) return 45;
  // Closing must suppress user code even if unregister invokes the bridge.
  int32_t events[] = {7,8}; if (callback(events,2,context)) return 46;
  if (retry && current.first_close) { current.first_close=false; return 32; }
  current.callback=NULL; current.context=NULL; return 0;
}
int32_t fixture_fire(void) {
  if (!current.callback) return 0;
  int32_t events[] = {7,8};
  bool accepted = current.callback(events,2,current.context);
  return events[0]==7 && events[1]==8 ? (accepted ? 1 : 0) : -1;
}
void fixture_counts(int32_t *r, int32_t *u) { *r=registers; *u=unregisters; }
`,
				// Test-only observations of the native fixture are deliberately not
				// generated manifest operations. Applications need only RegisterWatch,
				// Close and CallbackError to use this subscription-only binding.
				"probe.go": `package ffi
/*
#include "fixture.h"
*/
import "C"
func FixtureFire() int32 { return int32(C.fixture_fire()) }
func FixtureCounts() (int32,int32) { var r,u C.int32_t; C.fixture_counts(&r,&u); return int32(r),int32(u) }
`,
				"app/binding.km": `import go ffi from "fixture.test";
function Subscribe(): Result<int32> {
  let count: int32 = 0;
  const payload: byte[] = [1, 2];
  const watch = ffi.RegisterWatch("normal", payload, 42, false, false, (events: int32[]): boolean => { count += events[0]; return true; })?;
  watch.Close()?;
  return ok(count);
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "fixture.test/app"
)
const policy = "` + policy + `"
func wrongThread(err error) bool { ` + threadCheck + ` }
func check(ok bool,message string) { if !ok { panic(message) } }
func main() {
  calls := 0
  visit := func(events []int32) bool { check(len(events)==2 && events[0]==7 && events[1]==8,"initial notification snapshot"); calls++; events[0]=99; return true }
  r,u := ffi.FixtureCounts()
  watch, err := ffi.RegisterWatch("normal",[]byte{1,2},42,false,false,nil)
  check(watch==nil && errors.Is(err,ffi.ErrNilCallback),"nil callback rejected")
  watch, err = ffi.RegisterWatch("bad\x00label",nil,42,false,false,visit)
  check(watch==nil && errors.Is(err,ffi.ErrEmbeddedNUL),"embedded NUL rejected")
  afterR,afterU := ffi.FixtureCounts(); check(afterR==r && afterU==u,"validation entered no native code")
  payload := []byte{1,2}
  watch, err = ffi.RegisterWatch("normal",payload,42,false,false,visit)
  check(err==nil && calls==1 && watch.CallbackError()==nil,"registration-only constructor and initial notification")
  payload[0]=100; check(ffi.FixtureFire()==1 && calls==2,"retained input and copied notification isolation")
  copied := *watch
  check(errors.Is(copied.Close(),ffi.ErrCopiedCallbackRegistration),"copied registration rejected")
  check(ffi.FixtureFire()==1 && calls==3,"copied close preserved subscription")
  if policy=="mainThread" {
    done:=make(chan error,1)
    go func(){ _,err:=ffi.RegisterWatch("normal",nil,42,false,false,visit); done<-err }()
    check(wrongThread(<-done),"worker registration rejected")
    go func(){ done<-watch.Close() }()
    check(wrongThread(<-done),"worker close rejected")
  }
  check(watch.Close()==nil && calls==3 && ffi.FixtureFire()==0,"unregister suppresses its own callback entry")
  check(errors.Is(watch.Close(),ffi.ErrClosedCallbackRegistration),"double close")
  var empty ffi.Watch
  check(errors.Is(empty.Close(),ffi.ErrClosedCallbackRegistration),"zero registration")
  var absent *ffi.Watch
  check(errors.Is(absent.Close(),ffi.ErrClosedCallbackRegistration),"nil registration")
  watch, err = ffi.RegisterWatch("rejected",nil,9,true,false,visit)
  var status *ffi.StatusError
  check(watch==nil && errors.As(err,&status) && status.Code==31 && ffi.FixtureFire()==0 && calls==4,"register failure does not retain callback")
  watch, err = ffi.RegisterWatch("retry",nil,9,false,true,visit)
  check(err==nil && calls==5,"register after failure")
  check(errors.As(watch.Close(),&status) && status.Code==32 && calls==5,"unregister failure suppresses in-close notification")
  check(ffi.FixtureFire()==1 && calls==6,"failed close resumes callback")
  check(watch.Close()==nil && calls==6 && ffi.FixtureFire()==0,"unregister retry")
  watch, err = ffi.RegisterWatch("invalid",nil,9,false,false,visit)
  var input *ffi.CallbackInputError
  check(err==nil && errors.As(watch.CallbackError(),&input) && input.Parameter=="events" && calls==6,"initial notification input failure retained")
  check(ffi.FixtureFire()==0 && watch.Close()==nil,"failed initial notification can be unregistered")
  watch, err = ffi.RegisterWatch("panic",nil,9,false,false,func([]int32) bool { panic(nil) })
  var recovered *ffi.CallbackPanicError
  check(err==nil && errors.As(watch.CallbackError(),&recovered),"initial notification nil panic retained")
  check(ffi.FixtureFire()==0 && watch.Close()==nil,"panicking initial notification can be unregistered")
  count, err := app.Subscribe(); check(err==nil && count==7,"Kinmokusei subscription-only API")
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
				t.Fatalf("generate subscription-only wrapper: err=%v diagnostics=%v", err, diagnostics)
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
					t.Fatalf("subscription-only fixture (%v): %v\n%s", arguments, err, output)
				}
			}
		})
	}
}
