package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIncomingCFFIRetainedArray(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	cache := filepath.Join(t.TempDir(), "go-cache")
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"` + policy + `",
  "cFlags":["-Dcalloc=fixture_calloc","-Dfree=fixture_free"],
  "enums":[{"name":"Mode","cType":"fixture_mode","underlying":"uint16","values":[{"name":"ModeThree","symbol":"MODE_THREE"}]}],
  "structs":[
    {"name":"Point","cType":"fixture_point","fields":[{"name":"X","cName":"x","type":"float32"},{"name":"Y","cName":"y","type":"float32"}]},
    {"name":"Shape","cType":"fixture_shape","fields":[{"name":"Point","cName":"point","type":"Point"},{"name":"Enabled","cName":"enabled","type":"boolean"},{"name":"Mode","cName":"mode","type":"Mode"}]}
  ],
  "callbacks":[{"name":"Visit","lifetime":"registered","parameters":[{"name":"value","type":"int32"}],"result":"void"}],
  "callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"fixture_register","unregister":"fixture_unregister","parameters":[
    {"name":"label","type":"retainedCString"},{"name":"data","type":"retainedBytes"},
    {"name":"context","type":"retainedArray","element":"cInt32"},
    {"name":"int32","type":"retainedArray","element":"cUint32"},
    {"name":"elementSize","type":"retainedArray","element":"boolean"},
    {"name":"modes","type":"retainedArray","element":"Mode"},
    {"name":"shapes","type":"retainedArray","element":"Shape"},{"name":"option","type":"int32"}
  ]}],
  "functions":[
    {"name":"Fire","symbol":"fixture_fire","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"Live","symbol":"fixture_live","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"Frees","symbol":"fixture_frees","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"Registers","symbol":"fixture_registers","parameters":[],"result":"int32","convention":"statusOut"},
    {"name":"FailAllocation","symbol":"fixture_fail_allocation","parameters":[{"name":"after","type":"int32"}],"result":"void","convention":"status"}
  ]
}`))
			if err != nil {
				t.Fatal(err)
			}
			normalized := strings.Join(strings.Fields(string(artifacts.Source)), " ")
			for _, want := range []string{"kinmokusei_c_int_must_be_32_bits", "kinmokusei_c_uint_must_be_32_bits", "ErrRetainedArrayTooLarge", "ErrRetainedArrayAllocation", "parameter6 *C.fixture_shape"} {
				if !strings.Contains(normalized, want) {
					t.Fatalf("missing retained-array machinery %q", want)
				}
			}
			threadCheck := "return false"
			if policy == "mainThread" {
				threadCheck = "return errors.Is(err,ffi.ErrWrongThread)"
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
typedef enum __attribute__((packed)) fixture_mode { MODE_THREE = 3 } fixture_mode;
#else
typedef enum fixture_mode { MODE_THREE = 3 } fixture_mode;
#endif
typedef struct fixture_point { float x,y; } fixture_point;
typedef struct fixture_shape { fixture_point point; bool enabled; fixture_mode mode; } fixture_shape;
typedef void (*fixture_visit)(int32_t,void *);
#define FIXTURE_PARAMETERS char *label,uint8_t *data,size_t data_count,int *numbers,size_t number_count,unsigned int *masks,size_t mask_count,bool *flags,size_t flag_count,fixture_mode *modes,size_t mode_count,fixture_shape *shapes,size_t shape_count,int32_t option,fixture_visit visit,void *context
int32_t fixture_register(FIXTURE_PARAMETERS);
int32_t fixture_unregister(FIXTURE_PARAMETERS);
int32_t fixture_fire(int32_t *output);
int32_t fixture_live(int32_t *output);
int32_t fixture_frees(int32_t *output);
int32_t fixture_registers(int32_t *output);
int32_t fixture_fail_allocation(int32_t after);
#endif
`,
				"fixture.c": `#undef calloc
#undef free
#include <stdlib.h>
#include <string.h>
#include "fixture.h"
static void *allocations[16];
static int32_t live, frees, registers, fail_after=-1;
void *fixture_calloc(size_t count,size_t size) {
  if (fail_after==0) { fail_after=-1; return NULL; }
  if (fail_after>0) fail_after--;
  void *pointer=calloc(count,size);
  if (!pointer) return NULL;
  for (int i=0;i<16;i++) if (!allocations[i]) { allocations[i]=pointer; live++; return pointer; }
  abort();
}

void fixture_free(void *pointer) {
  if (!pointer) return;
  for (int i=0;i<16;i++) if (allocations[i]==pointer) { allocations[i]=NULL; live--; break; }
  frees++; free(pointer);
}
int32_t fixture_live(int32_t *output) { *output=live; return 0; }
int32_t fixture_frees(int32_t *output) { *output=frees; return 0; }
int32_t fixture_registers(int32_t *output) { *output=registers; return 0; }
int32_t fixture_fail_allocation(int32_t after) { fail_after=after; return 0; }
static struct {
  char *label; uint8_t *data; size_t data_count;
  int *numbers; size_t number_count; unsigned int *masks; size_t mask_count;
  bool *flags; size_t flag_count; fixture_mode *modes; size_t mode_count;
  fixture_shape *shapes; size_t shape_count; int32_t option; bool first_close;
  fixture_visit visit; void *context;
} current;
int32_t fixture_register(FIXTURE_PARAMETERS) {
  registers++;
  if (current.visit) return 41;
  if (strcmp(label,"label")!=0) return 42;
  if (option==3) {
    if (data || data_count || numbers || number_count || masks || mask_count || flags || flag_count || modes || mode_count || shapes || shape_count) return 43;
  } else {
    if (!data || data_count!=2 || data[0]!=1 || data[1]!=2 || !numbers || number_count!=2 || numbers[0]!=1 || numbers[1]!=2 ||
        !masks || mask_count!=1 || masks[0]!=9 || !flags || flag_count!=2 || !flags[0] || flags[1] ||
        !modes || mode_count!=1 || modes[0]!=MODE_THREE || !shapes || shape_count!=1) return 44;
    fixture_shape expected; memset(&expected,0,sizeof(expected));
    expected.point.x=1; expected.point.y=2; expected.enabled=true; expected.mode=MODE_THREE;
    if (memcmp(shapes,&expected,sizeof(expected))!=0) return 45; // Includes zeroed padding.
  }
  visit(7,context); // Context and all array storage must exist before return.
  if (option==1) return 31; // No pointers retained on register failure.
  current.label=label; current.data=data; current.data_count=data_count;
  current.numbers=numbers; current.number_count=number_count; current.masks=masks; current.mask_count=mask_count;
  current.flags=flags; current.flag_count=flag_count; current.modes=modes; current.mode_count=mode_count;
  current.shapes=shapes; current.shape_count=shape_count; current.option=option; current.first_close=true;
  current.visit=visit; current.context=context;
  if (number_count) numbers[0]=11; // Mutate only the registration-owned copy.
  return 0;
}
int32_t fixture_unregister(FIXTURE_PARAMETERS) {
  if (label!=current.label || data!=current.data || data_count!=current.data_count || numbers!=current.numbers || number_count!=current.number_count ||
      masks!=current.masks || mask_count!=current.mask_count || flags!=current.flags || flag_count!=current.flag_count ||
      modes!=current.modes || mode_count!=current.mode_count || shapes!=current.shapes || shape_count!=current.shape_count ||
      option!=current.option || visit!=current.visit || context!=current.context) return 46;
  if (shape_count && (shapes[0].point.x!=1 || numbers[0]!=11 || data[0]!=1 || masks[0]!=9 || !flags[0] || modes[0]!=MODE_THREE)) return 47;
  visit(99,context); // Close must suppress this entry.
  if (option==2 && current.first_close) { current.first_close=false; return 32; }
  current.visit=NULL; current.context=NULL; return 0;
}
int32_t fixture_fire(int32_t *output) {
  if (!current.visit) { *output=0; return 0; }
  int32_t value=current.number_count ? current.numbers[0]+(int32_t)current.shapes[0].point.x : 5;
  current.visit(value,current.context); *output=value; return 0;
}
`,
				"app/binding.km": `import go ffi from "fixture.test";
function Subscribe(): Result<int32> {
  let count: int32 = 0;
  const values: int32[] = [1, 2];
  const masks: uint32[] = [9];
  const flags: boolean[] = [true, false];
  const modes: ffi.Mode[] = [ffi.ModeThree];
  const point: ffi.Point = ffi.Point{X: 1, Y: 2};
  const shape: ffi.Shape = ffi.Shape{Point: point, Enabled: true, Mode: ffi.ModeThree};
  const shapes: ffi.Shape[] = [shape];
  const data: byte[] = [1, 2];
  const watch = ffi.RegisterWatch("label", data, values, masks, flags, modes, shapes, 0, (value: int32): void => { count += value; })?;
  const value = ffi.Fire()?;
  watch.Close()?;
  return ok(count + value);
}
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "fixture.test/app"
)
func check(ok bool,message string) { if !ok { panic(message) } }
func must(value int32,err error) int32 { if err!=nil { panic(err) }; return value }
func wrongThread(err error) bool { ` + threadCheck + ` }
func register(option int32, callback ffi.Visit) (*ffi.Watch,error) {
  return ffi.RegisterWatch("label",[]byte{1,2},[]int32{1,2},[]uint32{9},[]bool{true,false},[]ffi.Mode{ffi.ModeThree},[]ffi.Shape{{Point:ffi.Point{X:1,Y:2},Enabled:true,Mode:ffi.ModeThree}},option,callback)
}
func main() {
  calls:=0; visit:=func(int32){ calls++ }
  // Every allocation-failure position must roll back earlier arrays AND the
  // string/bytes allocated before them, without entering the native register.
  for position:=int32(0);position<5;position++ {
    beforeR,beforeF:=must(ffi.Registers()),must(ffi.Frees()); check(ffi.FailAllocation(position)==nil,"set allocation failure")
    watch,err:=register(0,visit)
    check(watch==nil && errors.Is(err,ffi.ErrRetainedArrayAllocation),"allocation failure returned")
    check(must(ffi.Live())==0 && must(ffi.Frees())==beforeF+position+2,"transactional allocation rollback")
    check(must(ffi.Registers())==beforeR && calls==0,"allocation failure has no native side effects")
  }
  data:=[]byte{1,2}; numbers:=[]int32{1,2}; masks:=[]uint32{9}; flags:=[]bool{true,false}
  modes:=[]ffi.Mode{ffi.ModeThree}; shapes:=[]ffi.Shape{{Point:ffi.Point{X:1,Y:2},Enabled:true,Mode:ffi.ModeThree}}
  beforeF:=must(ffi.Frees())
  watch,err:=ffi.RegisterWatch("label",data,numbers,masks,flags,modes,shapes,2,visit)
  check(err==nil && calls==1 && must(ffi.Live())==5 && numbers[0]==1,"initial notification and independent C storage")
  data[0]=100; numbers[0]=100; masks[0]=100; flags[0]=false; modes[0]=0; shapes[0].Point.X=100
  check(must(ffi.Fire())==12 && calls==2,"all snapshots retained independently of Go mutation")
  if "` + policy + `"=="mainThread" {
    done:=make(chan error,1); go func(){ _,err:=register(0,visit); done<-err }()
    check(wrongThread(<-done) && must(ffi.Live())==5 && must(ffi.Frees())==beforeF,"wrong thread allocated nothing")
  }
  copied:=*watch
  check(errors.Is(copied.Close(),ffi.ErrCopiedCallbackRegistration) && must(ffi.Live())==5,"copied close preserved arrays")
  var status *ffi.StatusError
  check(errors.As(watch.Close(),&status) && status.Code==32 && calls==2 && must(ffi.Live())==5 && must(ffi.Frees())==beforeF,"failed unregister retains every array")
  check(must(ffi.Fire())==12 && calls==3,"failed close resumes callback with retained arrays")
  check(watch.Close()==nil && calls==3 && must(ffi.Live())==0 && must(ffi.Frees())==beforeF+7,"successful unregister freed arrays string and bytes once")
  check(errors.Is(watch.Close(),ffi.ErrClosedCallbackRegistration) && must(ffi.Frees())==beforeF+7,"double close did not free twice")
  beforeF=must(ffi.Frees()); watch,err=register(1,visit)
  check(watch==nil && errors.As(err,&status) && status.Code==31 && calls==4 && must(ffi.Live())==0 && must(ffi.Frees())==beforeF+7,"native registration failure rollback")
  watch,err=register(0,func(int32){ panic(nil) })
  var callbackPanic *ffi.CallbackPanicError
  check(err==nil && errors.As(watch.CallbackError(),&callbackPanic) && must(ffi.Live())==5,"panic during registration retains live arrays")
  check(watch.Close()==nil && must(ffi.Live())==0,"callback panic cleanup")
  for _,nonNil:=range []bool{false,true} {
    var data []byte; var numbers []int32; var masks []uint32; var flags []bool; var modes []ffi.Mode; var shapes []ffi.Shape
    if nonNil { data=[]byte{}; numbers=[]int32{}; masks=[]uint32{}; flags=[]bool{}; modes=[]ffi.Mode{}; shapes=[]ffi.Shape{} }
    watch,err=ffi.RegisterWatch("label",data,numbers,masks,flags,modes,shapes,3,visit)
    check(err==nil && must(ffi.Live())==0 && must(ffi.Fire())==5 && watch.Close()==nil,"nil and empty arrays map to null zero")
  }
  result,err:=app.Subscribe(); check(err==nil && result==31 && must(ffi.Live())==0,"Kinmokusei retained-array API")
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
				t.Fatalf("generate retained-array wrapper: err=%v diagnostics=%v", err, diagnostics)
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
					t.Fatalf("retained-array fixture (%v): %v\n%s", arguments, err, output)
				}
			}
		})
	}
}
