package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIncomingCFFIOwnedArrayLayoutBounds(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	cache := filepath.Join(t.TempDir(), "go-cache")
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",
  "enums":[{"name":"Mode","cType":"fixture_mode","underlying":"uint64","values":[{"name":"ModeSeven","symbol":"MODE_SEVEN"},{"name":"ModeEight","symbol":"MODE_EIGHT"}]}],
  "structs":[
    {"name":"Payload","cType":"fixture_payload","fields":[{"name":"Mode","cName":"mode","type":"Mode"},{"name":"Enabled","cName":"enabled","type":"boolean"}]},
    {"name":"Record","cType":"fixture_record","fields":[{"name":"Tag","cName":"tag","type":"byte"},{"name":"Payload","cName":"payload","type":"Payload"}]}
  ],
  "functions":[
    {"name":"Modes","symbol":"fixture_modes","parameters":[{"name":"mode","type":"int32"},{"name":"goLimit","type":"uint64"}],"result":"ownedArray","resultElement":"Mode","resultRelease":"fixture_modes_free","convention":"statusOut"},
    {"name":"Records","symbol":"fixture_records","parameters":[{"name":"mode","type":"int32"},{"name":"goLimit","type":"uint64"}],"result":"ownedArray","resultElement":"Record","resultRelease":"fixture_records_free","convention":"statusOut"},
    {"name":"ModesReleased","symbol":"fixture_modes_released","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"RecordsReleased","symbol":"fixture_records_released","parameters":[],"result":"int32","convention":"statusOut"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": string(artifacts.Source),
				"fixture.h": `#ifndef FIXTURE_H
#define FIXTURE_H
#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>
#if defined(__GNUC__) || defined(__clang__)
typedef enum __attribute__((packed)) fixture_mode { MODE_SEVEN=7,MODE_EIGHT=8 } fixture_mode;
#else
typedef enum fixture_mode { MODE_SEVEN=7,MODE_EIGHT=8 } fixture_mode;
#endif
typedef struct fixture_payload { fixture_mode mode; bool enabled; } fixture_payload;
typedef struct fixture_record { uint8_t tag; fixture_payload payload; } fixture_record;
int32_t fixture_modes(int32_t mode,uint64_t go_limit,fixture_mode **output,size_t *length);
int32_t fixture_records(int32_t mode,uint64_t go_limit,fixture_record **output,size_t *length);
void fixture_modes_free(fixture_mode *value);
void fixture_records_free(fixture_record *value);
int32_t fixture_modes_released(int32_t *output);
int32_t fixture_records_released(int32_t *output);
#endif
`,
				"fixture.c": `#include "fixture.h"
#include <stdlib.h>
#include <string.h>
static int32_t modes_released,records_released;
// Modes 4..6 deliberately describe an invalid native range. The generated
// bounds check must reject it BEFORE reading even its first element. Only two
// real elements are allocated, making accidental range traversal a test bug.
static size_t result_length(int32_t mode,uint64_t go_limit,size_t element_size) {
  switch (mode) {
    case 1: case 2: return 0;
    case 3: return 1;
    case 4: return (size_t)INTPTR_MAX/element_size+1;
    case 5: return (size_t)go_limit+1;
    case 6: case 7: return SIZE_MAX;
    default: return 2;
  }
}
int32_t fixture_modes(int32_t mode,uint64_t go_limit,fixture_mode **output,size_t *length) {
  *length=result_length(mode,go_limit,sizeof(fixture_mode));
  *output=NULL;
  if (mode==2 || mode==3 || mode==8) return mode==8 ? 29 : 0;
  *output=calloc(2,sizeof(fixture_mode)); if (!*output) return 30;
  (*output)[0]=MODE_SEVEN; (*output)[1]=MODE_EIGHT;
  return mode==7 ? 29 : 0;
}
int32_t fixture_records(int32_t mode,uint64_t go_limit,fixture_record **output,size_t *length) {
  *length=result_length(mode,go_limit,sizeof(fixture_record));
  *output=NULL;
  if (mode==2 || mode==3 || mode==8) return mode==8 ? 29 : 0;
  *output=calloc(2,sizeof(fixture_record)); if (!*output) return 30;
  (*output)[0].tag=11; (*output)[0].payload.mode=MODE_SEVEN; (*output)[0].payload.enabled=true;
  (*output)[1].tag=12; (*output)[1].payload.mode=MODE_EIGHT; (*output)[1].payload.enabled=false;
  return mode==7 ? 29 : 0;
}
void fixture_modes_free(fixture_mode *value) { modes_released++; memset(value,0,2*sizeof(*value)); free(value); }
void fixture_records_free(fixture_record *value) { records_released++; memset(value,0,2*sizeof(*value)); free(value); }
int32_t fixture_modes_released(int32_t *output) { *output=modes_released; return 0; }
int32_t fixture_records_released(int32_t *output) { *output=records_released; return 0; }
`,
				"app/binding.km": `import go ffi from "fixture.test";
function ReadModes(mode: int32, limit: uint64): Result<ffi.Mode[]> {
  const values = ffi.Modes(mode, limit)?;
  return ok(values);
}
function ReadRecords(mode: int32, limit: uint64): Result<ffi.Record[]> {
  const values = ffi.Records(mode, limit)?;
  return ok(values);
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "fixture.test/app"
  "unsafe"
)
func check(ok bool,message string) { if !ok { panic(message) } }
func must(value int32,err error) int32 { if err!=nil { panic(err) }; return value }
func run[T any](read func(int32,uint64)([]T,error), released func()(int32,error), limit uint64, inspect func([]T)) {
  for mode:=int32(0);mode<=8;mode++ {
    before:=must(released())
    // A Go allocation panic must not replace the documented FFI error.
    values,err:=read(mode,limit)
    switch mode {
      case 0: check(err==nil && len(values)==2,"normal owned-array result"); inspect(values)
      case 1,2: check(err==nil && values!=nil && len(values)==0,"normalized empty result")
      case 3: check(values==nil && errors.Is(err,ffi.ErrNullOwnedArray),"null nonempty result")
      case 4,5,6: check(values==nil && errors.Is(err,ffi.ErrOwnedArrayTooLarge),"C AND Go layout byte bounds")
      case 7,8:
        var status *ffi.StatusError
        check(values==nil && errors.As(err,&status) && status.Code==29,"native status precedes output validation")
    }
    expected:=before+1; if mode==2 || mode==3 || mode==8 { expected=before }
    check(must(released())==expected,"result released exactly once on success failure empty or oversized")
  }
}
func main() {
  maximum:=uint64(^uint(0)>>1)
  modesLimit:=maximum/uint64(unsafe.Sizeof(ffi.Mode(0)))
  recordsLimit:=maximum/uint64(unsafe.Sizeof(ffi.Record{}))
  inspectModes:=func(values []ffi.Mode) { check(values[0]==ffi.ModeSeven && values[1]==ffi.ModeEight,"enum copy survived native release") }
  inspectRecords:=func(values []ffi.Record) { check(values[0].Tag==11 && values[0].Payload.Mode==ffi.ModeSeven && values[0].Payload.Enabled && values[1].Tag==12 && values[1].Payload.Mode==ffi.ModeEight && !values[1].Payload.Enabled,"nested POD copy survived native release") }
  run(ffi.Modes,ffi.ModesReleased,modesLimit,inspectModes)
  run(ffi.Records,ffi.RecordsReleased,recordsLimit,inspectRecords)
  run(app.ReadModes,ffi.ModesReleased,modesLimit,inspectModes)
  run(app.ReadRecords,ffi.RecordsReleased,recordsLimit,inspectRecords)
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
				t.Fatalf("generate owned-array wrapper: err=%v diagnostics=%v", err, diagnostics)
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
				command.Env = append(os.Environ(), "GOCACHE="+cache, "CGO_ENABLED=1")
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("owned-array bounds fixture (%v): %v\n%s", arguments, err, output)
				}
			}
		})
	}
}
