package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIncomingCFFIGeneratesAndRunsScalarStatusMatrix(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler is not available")
	}
	root := t.TempDir()
	manifest := []byte(`{
  "schemaVersion": 1,
  "package": "fixtureffi",
  "header": "fixture.h",
  "threadPolicy": "serialized",
  "targets": [{"goos":"linux", "goarch":"amd64", "cFlags":["-DONTAMA_FFI_TARGET=1"], "ldFlags":["-lm"]}],
  "enums": [{"name":"Mode", "cType":"fixture_mode", "underlying":"cInt32", "values":[{"name":"ModeAdd","symbol":"FIXTURE_MODE_ADD"},{"name":"ModeMultiply","symbol":"FIXTURE_MODE_MULTIPLY"}]},{"name":"ValueKind", "cType":"fixture_value_kind", "underlying":"cInt32", "values":[{"name":"ValueInteger","symbol":"FIXTURE_VALUE_INTEGER"},{"name":"ValueNumber","symbol":"FIXTURE_VALUE_NUMBER"}]}],
  "structs": [
    {"name":"Point", "cType":"fixture_point", "fields":[{"name":"X","cName":"x","type":"float32"},{"name":"Y","cName":"y","type":"float32"}]},
    {"name":"Pose", "cType":"fixture_pose", "fields":[{"name":"Position","cName":"position","type":"Point"},{"name":"Layer","cName":"layer","type":"int32"},{"name":"Mode","cName":"mode","type":"Mode"},{"name":"Visible","cName":"visible","type":"boolean"}]},
    {"name":"KeyEvent", "cType":"fixture_key_event", "fields":[{"name":"Key","cName":"key","type":"int32"},{"name":"Pressed","cName":"pressed","type":"boolean"}]},
    {"name":"MotionEvent", "cType":"fixture_motion_event", "fields":[{"name":"X","cName":"x","type":"float32"},{"name":"Y","cName":"y","type":"float32"}]}
  ],
  "taggedUnions": [{"name":"Event", "cType":"fixture_event", "tag":{"name":"Type","cName":"type","type":"uint32"}, "overlaidTag":true, "variants":[{"name":"Key","cName":"key","type":"KeyEvent","tags":["FIXTURE_EVENT_KEY_DOWN","FIXTURE_EVENT_KEY_UP"]},{"name":"Motion","cName":"motion","type":"MotionEvent","tags":["FIXTURE_EVENT_MOTION"]}]},{"name":"Value", "cType":"fixture_value", "tag":{"name":"Kind","cName":"kind","type":"ValueKind"}, "variants":[{"name":"Integer","cName":"integer","type":"int64","tags":["FIXTURE_VALUE_INTEGER"]},{"name":"Number","cName":"number","type":"float64","tags":["FIXTURE_VALUE_NUMBER"]}]}],
  "handles": [{"name":"Counter", "cType":"fixture_counter", "release":"fixture_counter_free"}],
  "functions": [
    {"name":"Add", "symbol":"fixture_add", "parameters":[{"name":"left","type":"int32"},{"name":"right","type":"int32"}], "result":"int32", "convention":"direct"},
    {"name":"Scale", "symbol":"fixture_scale", "parameters":[{"name":"value","type":"float64"},{"name":"factor","type":"float64"}], "result":"float64", "convention":"direct"},
    {"name":"Record", "symbol":"fixture_record", "parameters":[{"name":"value","type":"int64"}], "result":"void", "convention":"direct"},
    {"name":"Last", "symbol":"fixture_last", "parameters":[], "result":"int64", "convention":"direct"},
    {"name":"CheckedDouble", "symbol":"fixture_checked_double", "parameters":[{"name":"value","type":"int32"}], "result":"int32", "convention":"statusOut"},
    {"name":"SerializedProbe", "symbol":"fixture_serialized_probe", "parameters":[{"name":"rounds","type":"int32"}], "result":"int32", "convention":"direct"},
    {"name":"NewCounter", "symbol":"fixture_counter_new", "parameters":[{"name":"initial","type":"int64"}], "result":"Counter", "convention":"statusOut"},
    {"name":"CounterAdd", "symbol":"fixture_counter_add", "parameters":[{"name":"counter","type":"Counter"},{"name":"delta","type":"int64"}], "result":"int64", "convention":"statusOut"},
    {"name":"TitleLength", "symbol":"fixture_title_length", "parameters":[{"name":"title","type":"cstring"}], "result":"cInt32", "convention":"direct"},
    {"name":"SetTitle", "symbol":"fixture_set_title", "parameters":[{"name":"title","type":"cstring"}], "result":"void", "convention":"direct"},
    {"name":"UnsignedMix", "symbol":"fixture_unsigned_mix", "parameters":[{"name":"small","type":"uint16"},{"name":"medium","type":"uint32"},{"name":"large","type":"uint64"}], "result":"uint64", "convention":"direct"},
    {"name":"Invert", "symbol":"fixture_invert", "parameters":[{"name":"value","type":"boolean"}], "result":"boolean", "convention":"direct"},
    {"name":"CIntAdd", "symbol":"fixture_c_int_add", "parameters":[{"name":"left","type":"cInt32"},{"name":"right","type":"cInt32"}], "result":"cInt32", "convention":"direct"},
    {"name":"OffsetPoint", "symbol":"fixture_offset_point", "parameters":[{"name":"point","type":"Point"},{"name":"delta","type":"float32"}], "result":"Point", "convention":"direct"},
    {"name":"TransformPose", "symbol":"fixture_transform_pose", "parameters":[{"name":"pose","type":"Pose"}], "result":"Pose", "convention":"direct"},
    {"name":"CheckedPose", "symbol":"fixture_checked_pose", "parameters":[{"name":"valid","type":"boolean"}], "result":"Pose", "convention":"statusOut"},
    {"name":"NextMode", "symbol":"fixture_next_mode", "parameters":[{"name":"mode","type":"Mode"}], "result":"Mode", "convention":"direct"},
    {"name":"Label", "symbol":"fixture_label", "parameters":[{"name":"missing","type":"boolean"}], "result":"cstring", "convention":"direct"}
    ,{"name":"MakeName", "symbol":"fixture_make_name", "parameters":[{"name":"mode","type":"byte"}], "result":"ownedCString", "resultRelease":"fixture_string_free", "convention":"statusOut"}
    ,{"name":"StringReleaseCount", "symbol":"fixture_string_release_count", "parameters":[], "result":"cInt32", "convention":"direct"}
    ,{"name":"Checksum", "symbol":"fixture_checksum", "parameters":[{"name":"data","type":"borrowedBytes"}], "result":"uint64", "convention":"direct"}
    ,{"name":"AcceptBytes", "symbol":"fixture_accept_bytes", "parameters":[{"name":"data","type":"borrowedBytes"}], "result":"void", "convention":"status"}
    ,{"name":"MakeBytes", "symbol":"fixture_make_bytes", "parameters":[{"name":"seed","type":"byte"},{"name":"count","type":"uint32"}], "result":"ownedBytes", "resultRelease":"fixture_bytes_free", "convention":"statusOut"}
    ,{"name":"BytesReleaseCount", "symbol":"fixture_bytes_release_count", "parameters":[], "result":"cInt32", "convention":"direct"}
    ,{"name":"MakePoints", "symbol":"fixture_make_points", "parameters":[{"name":"count","type":"uint32"},{"name":"mode","type":"byte"}], "result":"ownedArray", "resultElement":"Point", "resultRelease":"fixture_points_free", "convention":"statusOut"}
    ,{"name":"PointsReleaseCount", "symbol":"fixture_points_release_count", "parameters":[], "result":"cInt32", "convention":"direct"}
    ,{"name":"MakeNumbers", "symbol":"fixture_make_numbers", "parameters":[], "result":"ownedArray", "resultElement":"uint32", "resultRelease":"fixture_numbers_free", "convention":"statusOut"}
    ,{"name":"MakeModes", "symbol":"fixture_make_modes", "parameters":[], "result":"ownedArray", "resultElement":"Mode", "resultRelease":"fixture_modes_free", "convention":"statusOut"}
    ,{"name":"TypedReleaseCount", "symbol":"fixture_typed_release_count", "parameters":[], "result":"cInt32", "convention":"direct"}
    ,{"name":"MakeEvent", "symbol":"fixture_make_event", "parameters":[{"name":"eventType","type":"uint32"}], "result":"Event", "convention":"direct"}
    ,{"name":"CheckedEvent", "symbol":"fixture_checked_event", "parameters":[{"name":"eventType","type":"uint32"}], "result":"Event", "convention":"statusOut"}
    ,{"name":"EventScore", "symbol":"fixture_event_score", "parameters":[{"name":"event","type":"Event"}], "result":"int32", "convention":"direct"}
    ,{"name":"EchoValue", "symbol":"fixture_echo_value", "parameters":[{"name":"value","type":"Value"}], "result":"Value", "convention":"direct"}
  ]
}`)
	artifacts, err := GenerateCFFI(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Package != "fixtureffi" {
		t.Fatalf("package = %q", artifacts.Package)
	}
	generated := string(artifacts.Source)
	for _, want := range []string{"import \"C\"", "ontamaCFFIMutex.Lock()", "func CheckedDouble(value int32) (int32, error)", "&StatusError{Function: \"CheckedDouble\", Code: status}", "type Counter struct", "func (handle *Counter) Close() error", "func CounterAdd(counter *Counter, delta int64) (int64, error)", "func TitleLength(title string) (int32, error)", "ErrEmbeddedNUL", "ErrNullCString", "ontama_c_int_must_be_32_bits", "#cgo linux,amd64 CFLAGS:", "#cgo linux,amd64 LDFLAGS:", "type Point struct", "type Pose struct", "type Mode int32", "func OffsetPoint(point Point, delta float32) Point", "func Label(missing bool) (string, error)", "func MakeName(mode byte) (string, error)", "defer C.fixture_string_free(output)", "ErrNullOwnedCString", "C.GoString(output)", "func Checksum(data []byte) uint64", "C.CBytes(data)", "C.size_t(len(data))", "ontama_cffi_free_bytes", "func MakeBytes(seed byte, count uint32) ([]byte, error)", "defer C.fixture_bytes_free(output)", "ErrNullOwnedBuffer", "ErrOwnedBufferTooLarge", "func MakePoints(count uint32, mode byte) ([]Point, error)", "unsafe.Slice(output", "ontamaCFFIFromPoint(value)", "defer C.fixture_points_free(output)", "type Event struct", "ontama_cffi_Event_get_tag", "func EventScore(event Event) int32", "func CheckedEvent(eventType uint32) (Event, error)", "type Value struct", "type ValueKind int32", "func EchoValue(value Value) Value"} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated C FFI does not contain %q:\n%s", want, generated)
		}
	}
	files := map[string]string{
		"go.mod":           "module fixtureffi.test\n\ngo 1.23\n",
		"generated_ffi.go": generated,
		"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef enum fixture_mode { FIXTURE_MODE_ADD = 1, FIXTURE_MODE_MULTIPLY = 2 } fixture_mode;
typedef enum fixture_value_kind { FIXTURE_VALUE_INTEGER = 1, FIXTURE_VALUE_NUMBER = 2 } fixture_value_kind;
typedef struct fixture_point { float x; float y; } fixture_point;
typedef struct fixture_pose { fixture_point position; int32_t layer; fixture_mode mode; bool visible; } fixture_pose;
typedef struct fixture_key_event { uint32_t type; int32_t key; bool pressed; } fixture_key_event;
typedef struct fixture_motion_event { uint32_t type; float x; float y; } fixture_motion_event;
enum { FIXTURE_EVENT_KEY_DOWN = 1, FIXTURE_EVENT_KEY_UP = 2, FIXTURE_EVENT_MOTION = 3 };
typedef union fixture_event { uint32_t type; fixture_key_event key; fixture_motion_event motion; } fixture_event;
typedef struct fixture_value { fixture_value_kind kind; union { int64_t integer; double number; }; } fixture_value;
int32_t fixture_add(int32_t left, int32_t right);
double fixture_scale(double value, double factor);
void fixture_record(int64_t value);
int64_t fixture_last(void);
int32_t fixture_checked_double(int32_t value, int32_t *output);
int32_t fixture_serialized_probe(int32_t rounds);
typedef struct fixture_counter fixture_counter;
int32_t fixture_counter_new(int64_t initial, fixture_counter **output);
int32_t fixture_counter_add(fixture_counter *counter, int64_t delta, int64_t *output);
void fixture_counter_free(fixture_counter *counter);
int fixture_title_length(const char *title);
void fixture_set_title(const char *title);
uint64_t fixture_unsigned_mix(uint16_t small, uint32_t medium, uint64_t large);
bool fixture_invert(bool value);
int fixture_c_int_add(int left, int right);
fixture_point fixture_offset_point(fixture_point point, float delta);
fixture_pose fixture_transform_pose(fixture_pose pose);
int32_t fixture_checked_pose(bool valid, fixture_pose *output);
fixture_mode fixture_next_mode(fixture_mode mode);
const char *fixture_label(bool missing);
int32_t fixture_make_name(uint8_t mode, char **output);
void fixture_string_free(char *value);
int fixture_string_release_count(void);
uint64_t fixture_checksum(const uint8_t *data, size_t length);
int32_t fixture_accept_bytes(const uint8_t *data, size_t length);
int32_t fixture_make_bytes(uint8_t seed, uint32_t count, uint8_t **output, size_t *output_length);
void fixture_bytes_free(uint8_t *data);
int fixture_bytes_release_count(void);
int32_t fixture_make_points(uint32_t count, uint8_t mode, fixture_point **output, size_t *output_length);
void fixture_points_free(fixture_point *data);
int fixture_points_release_count(void);
int32_t fixture_make_numbers(uint32_t **output, size_t *output_length);
void fixture_numbers_free(uint32_t *data);
int32_t fixture_make_modes(fixture_mode **output, size_t *output_length);
void fixture_modes_free(fixture_mode *data);
int fixture_typed_release_count(void);
fixture_event fixture_make_event(uint32_t event_type);
int32_t fixture_checked_event(uint32_t event_type, fixture_event *output);
int32_t fixture_event_score(fixture_event event);
fixture_value fixture_echo_value(fixture_value value);
`,
		"fixture.c": `#include "fixture.h"
#include <stdlib.h>
static int64_t last_value;
static int active;
static int title_length;
static int string_release_count;
static int bytes_release_count;
static int points_release_count;
static int typed_release_count;
struct fixture_counter { int64_t value; };
int32_t fixture_add(int32_t left, int32_t right) { return left + right; }
double fixture_scale(double value, double factor) { return value * factor; }
void fixture_record(int64_t value) { last_value = value; }
int64_t fixture_last(void) { return last_value; }
int32_t fixture_checked_double(int32_t value, int32_t *output) { if (value < 0) return 7; *output = value * 2; return 0; }
int32_t fixture_serialized_probe(int32_t rounds) {
  if (__sync_fetch_and_add(&active, 1) != 0) { __sync_fetch_and_sub(&active, 1); return -1; }
  volatile int32_t value = 0;
  for (int32_t index = 0; index < rounds; index++) value += index & 1;
  __sync_fetch_and_sub(&active, 1);
  return value < 0 ? -2 : 0;
}
int32_t fixture_counter_new(int64_t initial, fixture_counter **output) {
  fixture_counter *counter = (fixture_counter *)malloc(sizeof(fixture_counter));
  if (!counter) return 12;
  counter->value = initial;
  *output = counter;
  return 0;
}
int32_t fixture_counter_add(fixture_counter *counter, int64_t delta, int64_t *output) {
  if (!counter) return 13;
  counter->value += delta;
  *output = counter->value;
  return 0;
}
void fixture_counter_free(fixture_counter *counter) { free(counter); }
int fixture_title_length(const char *title) { int length = 0; while (title[length] != 0) length++; return length; }
void fixture_set_title(const char *title) { title_length = fixture_title_length(title); }
uint64_t fixture_unsigned_mix(uint16_t small, uint32_t medium, uint64_t large) { return (uint64_t)small + medium + large; }
bool fixture_invert(bool value) { return !value; }
int fixture_c_int_add(int left, int right) { return left + right + title_length * 0; }
fixture_point fixture_offset_point(fixture_point point, float delta) { point.x += delta; point.y -= delta; return point; }
fixture_pose fixture_transform_pose(fixture_pose pose) { pose.position.x += 1; pose.layer += 2; pose.mode = fixture_next_mode(pose.mode); pose.visible = !pose.visible; return pose; }
int32_t fixture_checked_pose(bool valid, fixture_pose *output) { if (!valid) return 23; *output = (fixture_pose){{3, 4}, 5, FIXTURE_MODE_ADD, true}; return 0; }
fixture_mode fixture_next_mode(fixture_mode mode) { return mode == FIXTURE_MODE_ADD ? FIXTURE_MODE_MULTIPLY : FIXTURE_MODE_ADD; }
const char *fixture_label(bool missing) { return missing ? NULL : "borrowed-label"; }
int32_t fixture_make_name(uint8_t mode, char **output) {
  if (mode == 2) { *output = NULL; return 0; }
  const char *source = mode == 0 ? "" : mode == 4 ? "copied-name" : "\xE6\xB8\xA9\xE6\xB3\x89tamago";
  size_t length = 0; while (source[length] != 0) length++;
  char *value = (char *)malloc(length + 1);
  if (!value) return 73;
  for (size_t index = 0; index <= length; index++) value[index] = source[index];
  *output = value;
  return mode == 3 ? 72 : 0;
}
void fixture_string_free(char *value) { string_release_count++; free(value); }
int fixture_string_release_count(void) { return string_release_count; }
uint64_t fixture_checksum(const uint8_t *data, size_t length) {
  if (length == 0) return data == NULL ? 0 : UINT64_MAX;
  uint64_t total = 0;
  for (size_t index = 0; index < length; index++) total += data[index];
  return total;
}
int32_t fixture_accept_bytes(const uint8_t *data, size_t length) {
  if (length == 0) return data == NULL ? 0 : 63;
  if (data == NULL) return 64;
  return length == 3 && data[0] == 255 ? 62 : 0;
}
int32_t fixture_make_bytes(uint8_t seed, uint32_t count, uint8_t **output, size_t *output_length) {
  if (seed == 254) { *output = NULL; *output_length = 2; return 0; }
  uint8_t *data = (uint8_t *)malloc(count == 0 ? 1 : count);
  if (!data) return 71;
  *output = data;
  if (seed == 255) { *output_length = 1; return 70; }
  if (seed == 253) { *output_length = SIZE_MAX; return 0; }
  *output_length = count;
  for (uint32_t index = 0; index < count; index++) data[index] = (uint8_t)(seed + index);
  return 0;
}
void fixture_bytes_free(uint8_t *data) { bytes_release_count++; free(data); }
int fixture_bytes_release_count(void) { return bytes_release_count; }
int32_t fixture_make_points(uint32_t count, uint8_t mode, fixture_point **output, size_t *output_length) {
  if (mode == 2) { *output = NULL; *output_length = 2; return 0; }
  fixture_point *points = (fixture_point *)malloc((count == 0 ? 1 : count) * sizeof(fixture_point));
  if (!points) return 81;
  *output = points;
  if (mode == 1) { *output_length = 1; return 80; }
  if (mode == 3) { *output_length = SIZE_MAX; return 0; }
  *output_length = count;
  for (uint32_t index = 0; index < count; index++) { points[index].x = (float)index + 0.5f; points[index].y = (float)index * -2.0f; }
  return 0;
}
void fixture_points_free(fixture_point *data) { points_release_count++; free(data); }
int fixture_points_release_count(void) { return points_release_count; }
int32_t fixture_make_numbers(uint32_t **output, size_t *output_length) {
  uint32_t *values = (uint32_t *)malloc(3 * sizeof(uint32_t));
  if (!values) return 90;
  values[0] = 0; values[1] = 42; values[2] = UINT32_MAX;
  *output = values; *output_length = 3; return 0;
}
void fixture_numbers_free(uint32_t *data) { typed_release_count++; free(data); }
int32_t fixture_make_modes(fixture_mode **output, size_t *output_length) {
  fixture_mode *values = (fixture_mode *)malloc(2 * sizeof(fixture_mode));
  if (!values) return 91;
  values[0] = FIXTURE_MODE_ADD; values[1] = FIXTURE_MODE_MULTIPLY;
  *output = values; *output_length = 2; return 0;
}
void fixture_modes_free(fixture_mode *data) { typed_release_count++; free(data); }
int fixture_typed_release_count(void) { return typed_release_count; }
fixture_event fixture_make_event(uint32_t event_type) {
  fixture_event event = {0}; event.type = event_type;
  if (event_type == FIXTURE_EVENT_KEY_DOWN || event_type == FIXTURE_EVENT_KEY_UP) { event.key.key = 42; event.key.pressed = event_type == FIXTURE_EVENT_KEY_DOWN; }
  else if (event_type == FIXTURE_EVENT_MOTION) { event.motion.x = 1.5f; event.motion.y = -2.5f; }
  return event;
}
int32_t fixture_checked_event(uint32_t event_type, fixture_event *output) { if (event_type == 99) return 92; *output = fixture_make_event(event_type); return 0; }
int32_t fixture_event_score(fixture_event event) {
  if (event.type == FIXTURE_EVENT_KEY_DOWN || event.type == FIXTURE_EVENT_KEY_UP) return event.key.key + (event.key.pressed ? 100 : 0);
  if (event.type == FIXTURE_EVENT_MOTION) return (int32_t)(event.motion.x * 10 + event.motion.y * -10);
  return (int32_t)event.type;
}
fixture_value fixture_echo_value(fixture_value value) { return value; }
`,
		"app/binding.otm": `import go ffi from "fixtureffi.test";
function Checked(value: int32): Result<int32> {
  const doubled = ffi.CheckedDouble(value)?;
  return ok(doubled);
}
function Count(initial: int64, delta: int64): Result<int64> {
  const counter = ffi.NewCounter(initial)?;
  const value = ffi.CounterAdd(counter, delta)?;
  counter.Close()?;
  return ok(value);
}
function TitleSize(title: string): Result<int32> {
  const size = ffi.TitleLength(title)?;
  return ok(size);
}
function MovePoint(point: ffi.Point, delta: float32): ffi.Point {
  return ffi.OffsetPoint(point, delta);
}
function ByteSum(data: byte[]): uint64 { return ffi.Checksum(data); }
function ValidateBytes(data: byte[]): Result<void> {
  ffi.AcceptBytes(data)?;
  return ok();
}
function LoadBytes(seed: byte, count: uint32): Result<byte[]> {
  const data = ffi.MakeBytes(seed, count)?;
  return ok(data);
}
function LoadName(mode: byte): Result<string> {
  const name = ffi.MakeName(mode)?;
  return ok(name);
}
function LoadPoints(count: uint32): Result<ffi.Point[]> {
  const points = ffi.MakePoints(count, 0)?;
  return ok(points);
}
function ScoreEvent(event: ffi.Event): int32 { return ffi.EventScore(event); }
function LoadEvent(eventType: uint32): Result<ffi.Event> {
  const event = ffi.CheckedEvent(eventType)?;
  return ok(event);
}
function EchoValue(value: ffi.Value): ffi.Value { return ffi.EchoValue(value); }
`,
		"cmd/main.go": `package main
import (
  "errors"
	app "fixtureffi.test/app"
	ffi "fixtureffi.test"
  "sync"
)
func assert(ok bool) { if !ok { panic("incoming C FFI assertion failed") } }
func main() {
  assert(ffi.Add(20, 22) == 42)
  assert(ffi.Scale(1.5, 2) == 3)
  ffi.Record(9223372036854770000)
  assert(ffi.Last() == 9223372036854770000)
  got, err := ffi.CheckedDouble(21); assert(got == 42 && err == nil)
  got, err = ffi.CheckedDouble(-1)
  var status *ffi.StatusError
  assert(got == 0 && errors.As(err, &status) && status.Code == 7 && status.Function == "CheckedDouble")
  var wait sync.WaitGroup
  failures := make(chan int32, 32)
  for index := 0; index < 32; index++ { wait.Add(1); go func() { defer wait.Done(); if got := ffi.SerializedProbe(100000); got != 0 { failures <- got } }() }
  wait.Wait(); close(failures)
  for range failures { panic("serialized C FFI call overlapped") }
  counter, err := ffi.NewCounter(40); assert(err == nil)
  got64, err := ffi.CounterAdd(counter, 2); assert(got64 == 42 && err == nil)
  var addWait sync.WaitGroup
  addErrors := make(chan error, 16)
  for index := 0; index < 16; index++ { addWait.Add(1); go func() { defer addWait.Done(); if _, err := ffi.CounterAdd(counter, 1); err != nil { addErrors <- err } }() }
  addWait.Wait()
  close(addErrors); for range addErrors { panic("concurrent handle call failed") }
  got64, err = ffi.CounterAdd(counter, 0); assert(got64 == 58 && err == nil)
  assert(counter.Close() == nil)
  assert(errors.Is(counter.Close(), ffi.ErrClosedHandle))
  got64, err = ffi.CounterAdd(counter, 1); assert(got64 == 0 && errors.Is(err, ffi.ErrClosedHandle))
  got64, err = ffi.CounterAdd(nil, 1); assert(got64 == 0 && errors.Is(err, ffi.ErrClosedHandle))
  got, err = app.Checked(21); assert(got == 42 && err == nil)
  got64, err = app.Count(40, 2); assert(got64 == 42 && err == nil)
  length, err := ffi.TitleLength("温泉tamago"); assert(length == 12 && err == nil)
  assert(ffi.SetTitle("hello") == nil)
  _, err = ffi.TitleLength("bad\x00title"); assert(errors.Is(err, ffi.ErrEmbeddedNUL))
  assert(errors.Is(ffi.SetTitle("bad\x00title"), ffi.ErrEmbeddedNUL))
  assert(ffi.UnsignedMix(2, 3, 4) == 9)
  assert(ffi.Invert(true) == false && ffi.Invert(false) == true)
  assert(ffi.CIntAdd(20, 22) == 42)
  length, err = app.TitleSize("tamago"); assert(length == 6 && err == nil)
  point := ffi.OffsetPoint(ffi.Point{X: 2, Y: 5}, 3); assert(point.X == 5 && point.Y == 2)
  point = app.MovePoint(ffi.Point{X: 10, Y: 20}, 4); assert(point.X == 14 && point.Y == 16)
  pose := ffi.TransformPose(ffi.Pose{Position: ffi.Point{X: 1, Y: 2}, Layer: 3, Mode: ffi.ModeAdd, Visible: true})
  assert(pose.Position.X == 2 && pose.Position.Y == 2 && pose.Layer == 5 && pose.Mode == ffi.ModeMultiply && !pose.Visible)
  pose, err = ffi.CheckedPose(true); assert(err == nil && pose.Position.X == 3 && pose.Position.Y == 4 && pose.Layer == 5 && pose.Mode == ffi.ModeAdd && pose.Visible)
  pose, err = ffi.CheckedPose(false); assert(pose == (ffi.Pose{}) && errors.As(err, &status) && status.Code == 23)
  assert(ffi.NextMode(ffi.ModeAdd) == ffi.ModeMultiply && ffi.NextMode(ffi.ModeMultiply) == ffi.ModeAdd)
  label, err := ffi.Label(false); assert(label == "borrowed-label" && err == nil)
  label, err = ffi.Label(true); assert(label == "" && errors.Is(err, ffi.ErrNullCString))
  assert(ffi.StringReleaseCount() == 0)
  name, err := ffi.MakeName(1); assert(name == "温泉tamago" && err == nil && ffi.StringReleaseCount() == 1)
  name, err = ffi.MakeName(0); assert(name == "" && err == nil && ffi.StringReleaseCount() == 2)
  name, err = ffi.MakeName(2); assert(name == "" && errors.Is(err, ffi.ErrNullOwnedCString) && ffi.StringReleaseCount() == 2)
  name, err = ffi.MakeName(3); assert(name == "" && errors.As(err, &status) && status.Code == 72 && status.Function == "MakeName" && ffi.StringReleaseCount() == 3)
  name, err = app.LoadName(4); assert(name == "copied-name" && err == nil && ffi.StringReleaseCount() == 4)
  assert(ffi.Checksum(nil) == 0)
  assert(ffi.Checksum([]byte{}) == 0)
  assert(ffi.Checksum([]byte{0, 1, 255}) == 256)
  assert(app.ByteSum([]byte{10, 20, 30, 40}) == 100)
  assert(app.ValidateBytes([]byte{1, 2, 3}) == nil)
  err = app.ValidateBytes([]byte{255, 1, 2}); assert(errors.As(err, &status) && status.Code == 62 && status.Function == "AcceptBytes")
  assert(ffi.BytesReleaseCount() == 0)
  owned, err := ffi.MakeBytes(10, 4); assert(err == nil && len(owned) == 4 && owned[0] == 10 && owned[3] == 13 && ffi.BytesReleaseCount() == 1)
  owned[0] = 99
  empty, err := ffi.MakeBytes(1, 0); assert(err == nil && empty != nil && len(empty) == 0 && ffi.BytesReleaseCount() == 2)
  owned, err = ffi.MakeBytes(255, 1); assert(owned == nil && errors.As(err, &status) && status.Code == 70 && ffi.BytesReleaseCount() == 3)
  owned, err = ffi.MakeBytes(254, 1); assert(owned == nil && errors.Is(err, ffi.ErrNullOwnedBuffer) && ffi.BytesReleaseCount() == 3)
  owned, err = ffi.MakeBytes(253, 1); assert(owned == nil && errors.Is(err, ffi.ErrOwnedBufferTooLarge) && ffi.BytesReleaseCount() == 4)
  owned, err = app.LoadBytes(20, 3); assert(err == nil && len(owned) == 3 && owned[0] == 20 && owned[2] == 22 && ffi.BytesReleaseCount() == 5)
  assert(ffi.PointsReleaseCount() == 0)
  points, err := ffi.MakePoints(3, 0); assert(err == nil && len(points) == 3 && points[0].X == 0.5 && points[2].X == 2.5 && points[2].Y == -4 && ffi.PointsReleaseCount() == 1)
  points[0].X = 99
  points, err = ffi.MakePoints(0, 0); assert(err == nil && points != nil && len(points) == 0 && ffi.PointsReleaseCount() == 2)
  points, err = ffi.MakePoints(1, 1); assert(points == nil && errors.As(err, &status) && status.Code == 80 && ffi.PointsReleaseCount() == 3)
  points, err = ffi.MakePoints(1, 2); assert(points == nil && errors.Is(err, ffi.ErrNullOwnedArray) && ffi.PointsReleaseCount() == 3)
  points, err = ffi.MakePoints(1, 3); assert(points == nil && errors.Is(err, ffi.ErrOwnedArrayTooLarge) && ffi.PointsReleaseCount() == 4)
  points, err = app.LoadPoints(2); assert(err == nil && len(points) == 2 && points[1].X == 1.5 && points[1].Y == -2 && ffi.PointsReleaseCount() == 5)
  numbers, err := ffi.MakeNumbers(); assert(err == nil && len(numbers) == 3 && numbers[0] == 0 && numbers[1] == 42 && numbers[2] == ^uint32(0))
  modes, err := ffi.MakeModes(); assert(err == nil && len(modes) == 2 && modes[0] == ffi.ModeAdd && modes[1] == ffi.ModeMultiply)
  assert(ffi.TypedReleaseCount() == 2)
  event := ffi.MakeEvent(1); assert(event.Type == 1 && event.Key.Key == 42 && event.Key.Pressed && ffi.EventScore(event) == 142)
  event = ffi.MakeEvent(2); assert(event.Type == 2 && event.Key.Key == 42 && !event.Key.Pressed && app.ScoreEvent(event) == 42)
  event = ffi.MakeEvent(3); assert(event.Type == 3 && event.Motion.X == 1.5 && event.Motion.Y == -2.5 && ffi.EventScore(event) == 40)
  event = ffi.MakeEvent(77); assert(event.Type == 77 && event.Key == (ffi.KeyEvent{}) && event.Motion == (ffi.MotionEvent{}) && ffi.EventScore(event) == 77)
  event = ffi.Event{Type: 1, Key: ffi.KeyEvent{Key: 7, Pressed: true}}; assert(ffi.EventScore(event) == 107)
  event = ffi.Event{Type: 2, Key: ffi.KeyEvent{Key: -9, Pressed: true}}; assert(ffi.EventScore(event) == 91)
  event = ffi.Event{Type: 3, Motion: ffi.MotionEvent{X: -1.5, Y: 2.5}}; assert(ffi.EventScore(event) == -40)
  event = ffi.Event{Type: 88, Key: ffi.KeyEvent{Key: 999, Pressed: true}, Motion: ffi.MotionEvent{X: 99, Y: 99}}; assert(ffi.EventScore(event) == 88)
  event, err = ffi.CheckedEvent(1); assert(err == nil && event.Type == 1 && event.Key.Key == 42 && event.Key.Pressed)
  event, err = ffi.CheckedEvent(3); assert(err == nil && event.Type == 3 && event.Motion.X == 1.5)
  event, err = ffi.CheckedEvent(78); assert(err == nil && event.Type == 78 && event.Key == (ffi.KeyEvent{}) && event.Motion == (ffi.MotionEvent{}))
  event, err = app.LoadEvent(2); assert(err == nil && event.Type == 2 && event.Key.Key == 42 && !event.Key.Pressed)
  event, err = ffi.CheckedEvent(99); assert(event == (ffi.Event{}) && errors.As(err, &status) && status.Code == 92)
  event, err = app.LoadEvent(99); assert(event == (ffi.Event{}) && errors.As(err, &status) && status.Code == 92)
  value := ffi.EchoValue(ffi.Value{Kind: ffi.ValueInteger, Integer: -9223372036854770000}); assert(value.Kind == ffi.ValueInteger && value.Integer == -9223372036854770000 && value.Number == 0)
  value = app.EchoValue(ffi.Value{Kind: ffi.ValueNumber, Number: 3.25}); assert(value.Kind == ffi.ValueNumber && value.Integer == 0 && value.Number == 3.25)
  value = ffi.EchoValue(ffi.Value{Kind: ffi.ValueKind(77), Integer: 42, Number: 8.5}); assert(value.Kind == ffi.ValueKind(77) && value.Integer == 0 && value.Number == 0)
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generatedApp, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.otm")}, "app")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("generate OnsenTamago FFI wrapper: err=%v diagnostics=%v", err, diagnostics)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generatedApp, 0o644); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(root, "ffi-runner")
	if runtime.GOOS == "windows" {
		runner += ".exe"
	}
	arguments := []string{"build", "-buildvcs=false", "-o", runner, "./cmd"}
	if os.Getenv("ONTAMA_DIFFERENTIAL_RACE") == "1" {
		arguments = []string{"build", "-race", "-buildvcs=false", "-o", runner, "./cmd"}
	}
	command := exec.Command("go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated incoming C FFI build failed: %v\n%s\n%s", err, output, artifacts.Source)
	}
	if output, err := exec.Command(runner).CombinedOutput(); err != nil {
		t.Fatalf("generated incoming C FFI execution failed: %v\n%s", err, output)
	}
}

func TestIncomingCFFICallScopedCallbacks(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler is not available")
	}
	root := t.TempDir()
	artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,
  "package":"callbackffi",
  "header":"fixture.h",
  "threadPolicy":"serialized",
  "cFlags":["-pthread"],
  "ldFlags":["-pthread"],
  "enums":[{"name":"Decision","cType":"fixture_decision","underlying":"cInt32","values":[{"name":"DecisionContinue","symbol":"FIXTURE_CONTINUE"},{"name":"DecisionStop","symbol":"FIXTURE_STOP"}]},{"name":"PayloadKind","cType":"fixture_payload_kind","underlying":"cInt32","values":[{"name":"PayloadPoint","symbol":"FIXTURE_PAYLOAD_POINT"},{"name":"PayloadCode","symbol":"FIXTURE_PAYLOAD_CODE"}]}],
  "structs":[{"name":"PayloadPointValue","cType":"fixture_payload_point","fields":[{"name":"X","cName":"x","type":"int32"},{"name":"Y","cName":"y","type":"int32"}]},{"name":"OverlayKeyValue","cType":"fixture_overlay_key","fields":[{"name":"Key","cName":"key","type":"int32"}]},{"name":"OverlayMotionValue","cType":"fixture_overlay_motion","fields":[{"name":"X","cName":"x","type":"float32"}]}],
  "taggedUnions":[{"name":"PayloadEvent","cType":"fixture_payload_event","tag":{"name":"Kind","cName":"kind","type":"PayloadKind"},"variants":[{"name":"Point","cName":"point","type":"PayloadPointValue","tags":["FIXTURE_PAYLOAD_POINT"]},{"name":"Code","cName":"code","type":"int32","tags":["FIXTURE_PAYLOAD_CODE"]}]},{"name":"OverlayEvent","cType":"fixture_overlay_event","tag":{"name":"Type","cName":"type","type":"int32"},"overlaidTag":true,"variants":[{"name":"Key","cName":"key","type":"OverlayKeyValue","tags":["FIXTURE_OVERLAY_KEY"]},{"name":"Motion","cName":"motion","type":"OverlayMotionValue","tags":["FIXTURE_OVERLAY_MOTION"]}]}],
  "callbacks":[
    {"name":"VisitCallback","lifetime":"callScoped","parameters":[{"name":"value","type":"int32"},{"name":"index","type":"uint32"}],"result":"boolean"},
    {"name":"ObserveCallback","lifetime":"callScoped","parameters":[{"name":"value","type":"int64"}],"result":"void"},
    {"name":"DecideCallback","lifetime":"callScoped","parameters":[{"name":"value","type":"int32"}],"result":"Decision"},
    {"name":"TickCallback","lifetime":"callScoped","parameters":[],"result":"void"},
    {"name":"WidthCallback","lifetime":"callScoped","parameters":[{"name":"value","type":"cUint32"}],"result":"cInt32"},
    {"name":"StoredCallback","lifetime":"registered","parameters":[{"name":"value","type":"int32"}],"result":"boolean"},
    {"name":"PayloadCallback","lifetime":"callScoped","parameters":[{"name":"point","type":"PayloadPointValue"},{"name":"event","type":"PayloadEvent"}],"result":"int32"},
    {"name":"StoredPayloadCallback","lifetime":"registered","parameters":[{"name":"point","type":"PayloadPointValue"},{"name":"event","type":"PayloadEvent"}],"result":"int32"},
    {"name":"OverlayCallback","lifetime":"callScoped","parameters":[{"name":"event","type":"OverlayEvent"}],"result":"int32"},
    {"name":"TextBufferCallback","lifetime":"callScoped","parameters":[{"name":"title","type":"copiedCString"},{"name":"optional","type":"nullableCopiedCString"},{"name":"data","type":"copiedBytes"}],"result":"int32"},
    {"name":"StoredTextBufferCallback","lifetime":"registered","parameters":[{"name":"title","type":"copiedCString"},{"name":"optional","type":"nullableCopiedCString"},{"name":"data","type":"copiedBytes"}],"result":"int32"},
    {"name":"ObserveTextCallback","lifetime":"callScoped","parameters":[{"name":"title","type":"copiedCString"}],"result":"void"},
    {"name":"TransformBytesCallback","lifetime":"callScoped","parameters":[{"name":"data","type":"inoutBytes"},{"name":"salt","type":"copiedBytes"}],"result":"boolean"},
    {"name":"StoredTransformBytesCallback","lifetime":"registered","parameters":[{"name":"data","type":"inoutBytes"}],"result":"void"},
    {"name":"DualTransformCallback","lifetime":"callScoped","parameters":[{"name":"first","type":"inoutBytes"},{"name":"second","type":"inoutBytes"}],"result":"void"},
    {"name":"OwnedDataCallback","lifetime":"registered","parameters":[{"name":"seed","type":"byte"}],"result":"ownedBytes"},
    {"name":"OwnedTextCallback","lifetime":"registered","parameters":[{"name":"path","type":"copiedCString"}],"result":"ownedCString"},
    {"name":"OwnedPointArrayCallback","lifetime":"registered","parameters":[{"name":"seed","type":"int32"}],"result":"ownedArray","resultElement":"PayloadPointValue"},
    {"name":"OwnedDecisionArrayCallback","lifetime":"registered","parameters":[{"name":"empty","type":"boolean"}],"result":"ownedArray","resultElement":"Decision"}
  ],
  "callbackRegistrations":[{"name":"StoredWatch","callback":"StoredCallback","register":"fixture_stored_register","unregister":"fixture_stored_unregister"},{"name":"StoredPayloadWatch","callback":"StoredPayloadCallback","register":"fixture_stored_payload_register","unregister":"fixture_stored_payload_unregister"},{"name":"StoredTextBufferWatch","callback":"StoredTextBufferCallback","register":"fixture_stored_text_buffer_register","unregister":"fixture_stored_text_buffer_unregister"},{"name":"StoredTransformBytesWatch","callback":"StoredTransformBytesCallback","register":"fixture_stored_transform_register","unregister":"fixture_stored_transform_unregister"},{"name":"OwnedDataWatch","callback":"OwnedDataCallback","register":"fixture_owned_data_register","unregister":"fixture_owned_data_unregister"},{"name":"OwnedTextWatch","callback":"OwnedTextCallback","register":"fixture_owned_text_register","unregister":"fixture_owned_text_unregister"},{"name":"OwnedPointArrayWatch","callback":"OwnedPointArrayCallback","register":"fixture_owned_point_array_register","unregister":"fixture_owned_point_array_unregister"},{"name":"OwnedDecisionArrayWatch","callback":"OwnedDecisionArrayCallback","register":"fixture_owned_decision_array_register","unregister":"fixture_owned_decision_array_unregister"}],
  "functions":[
    {"name":"Fold","symbol":"fixture_fold","parameters":[{"name":"seed","type":"int32"},{"name":"count","type":"uint32"},{"name":"visit","type":"VisitCallback"}],"result":"int32","convention":"direct"},
    {"name":"Around","symbol":"fixture_around","parameters":[{"name":"prefix","type":"int32"},{"name":"visit","type":"VisitCallback"},{"name":"suffix","type":"int32"}],"result":"int32","convention":"direct"},
    {"name":"CheckedFold","symbol":"fixture_checked_fold","parameters":[{"name":"seed","type":"int32"},{"name":"visit","type":"VisitCallback"}],"result":"int32","convention":"statusOut"},
    {"name":"Validate","symbol":"fixture_validate","parameters":[{"name":"count","type":"uint32"},{"name":"visit","type":"VisitCallback"}],"result":"void","convention":"status"},
    {"name":"Observe","symbol":"fixture_observe","parameters":[{"name":"start","type":"int64"},{"name":"count","type":"uint32"},{"name":"observe","type":"ObserveCallback"}],"result":"void","convention":"direct"},
    {"name":"DecideSum","symbol":"fixture_decide_sum","parameters":[{"name":"count","type":"int32"},{"name":"decide","type":"DecideCallback"}],"result":"int32","convention":"direct"},
    {"name":"Parallel","symbol":"fixture_parallel","parameters":[{"name":"rounds","type":"uint32"},{"name":"visit","type":"VisitCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"Tick","symbol":"fixture_tick","parameters":[{"name":"tick","type":"TickCallback"}],"result":"void","convention":"direct"}
    ,{"name":"Inspect","symbol":"fixture_inspect","parameters":[{"name":"label","type":"cstring"},{"name":"data","type":"borrowedBytes"},{"name":"visit","type":"VisitCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"FireStored","symbol":"fixture_stored_fire","parameters":[{"name":"value","type":"int32"}],"result":"boolean","convention":"direct"}
    ,{"name":"EmitPayload","symbol":"fixture_emit_payload","parameters":[{"name":"kind","type":"PayloadKind"},{"name":"callback","type":"PayloadCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"FireStoredPayload","symbol":"fixture_stored_payload_fire","parameters":[{"name":"kind","type":"PayloadKind"}],"result":"int32","convention":"direct"}
    ,{"name":"EmitOverlay","symbol":"fixture_emit_overlay","parameters":[{"name":"eventType","type":"int32"},{"name":"callback","type":"OverlayCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"EmitTextBuffer","symbol":"fixture_emit_text_buffer","parameters":[{"name":"mode","type":"byte"},{"name":"callback","type":"TextBufferCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"FireStoredTextBuffer","symbol":"fixture_stored_text_buffer_fire","parameters":[{"name":"mode","type":"byte"}],"result":"int32","convention":"direct"}
    ,{"name":"ObserveCopiedText","symbol":"fixture_observe_copied_text","parameters":[{"name":"missing","type":"boolean"},{"name":"callback","type":"ObserveTextCallback"}],"result":"void","convention":"direct"}
    ,{"name":"TransformBytes","symbol":"fixture_transform_bytes","parameters":[{"name":"mode","type":"byte"},{"name":"callback","type":"TransformBytesCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"LastTransformedChecksum","symbol":"fixture_last_transformed_checksum","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"FireStoredTransform","symbol":"fixture_stored_transform_fire","parameters":[{"name":"mode","type":"byte"}],"result":"int32","convention":"direct"}
    ,{"name":"TransformAliased","symbol":"fixture_transform_aliased","parameters":[{"name":"callback","type":"DualTransformCallback"}],"result":"int32","convention":"direct"}
    ,{"name":"FireOwnedData","symbol":"fixture_owned_data_fire","parameters":[{"name":"seed","type":"byte"},{"name":"nullLength","type":"boolean"}],"result":"int32","convention":"direct"}
    ,{"name":"HoldOwnedData","symbol":"fixture_owned_data_hold","parameters":[{"name":"seed","type":"byte"}],"result":"int32","convention":"direct"}
    ,{"name":"ReleaseHeldOwnedData","symbol":"fixture_owned_data_release_held","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"OwnedDataReleaseCount","symbol":"fixture_owned_data_release_count","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"FireOwnedText","symbol":"fixture_owned_text_fire","parameters":[{"name":"path","type":"cstring"}],"result":"int32","convention":"direct"}
    ,{"name":"HoldOwnedText","symbol":"fixture_owned_text_hold","parameters":[{"name":"path","type":"cstring"}],"result":"int32","convention":"direct"}
    ,{"name":"ReleaseHeldOwnedText","symbol":"fixture_owned_text_release_held","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"OwnedTextReleaseCount","symbol":"fixture_owned_text_release_count","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"FireOwnedPointArray","symbol":"fixture_owned_point_array_fire","parameters":[{"name":"seed","type":"int32"},{"name":"nullLength","type":"boolean"}],"result":"int32","convention":"direct"}
    ,{"name":"HoldOwnedPointArray","symbol":"fixture_owned_point_array_hold","parameters":[{"name":"seed","type":"int32"}],"result":"int32","convention":"direct"}
    ,{"name":"ReleaseHeldOwnedPointArray","symbol":"fixture_owned_point_array_release_held","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"OwnedPointArrayReleaseCount","symbol":"fixture_owned_point_array_release_count","parameters":[],"result":"int32","convention":"direct"}
    ,{"name":"FireOwnedDecisionArray","symbol":"fixture_owned_decision_array_fire","parameters":[{"name":"empty","type":"boolean"}],"result":"int32","convention":"direct"}
    ,{"name":"OwnedDecisionArrayReleaseCount","symbol":"fixture_owned_decision_array_release_count","parameters":[],"result":"int32","convention":"direct"}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(artifacts.Source)
	for _, want := range []string{"type VisitCallback func(value int32, index uint32) bool", "type ObserveCallback func(value int64)", "type DecideCallback func(value int32) Decision", "type PayloadCallback func(point PayloadPointValue, event PayloadEvent) int32", "type StoredPayloadCallback func(point PayloadPointValue, event PayloadEvent) int32", "type OverlayCallback func(event OverlayEvent) int32", "type TextBufferCallback func(title string, optional *string, data []byte) int32", "type StoredTextBufferCallback func(title string, optional *string, data []byte) int32", "type TransformBytesCallback func(data []byte, salt []byte) bool", "type StoredTransformBytesCallback func(data []byte)", "type OwnedDataCallback func(seed byte) []byte", "ontama_cffi_callback_OwnedDataCallback_release", "C.CBytes(ontamaCallbackResult)", "*outputLength = C.size_t(len(ontamaCallbackResult))", "type OwnedTextCallback func(path string) string", "ontama_cffi_callback_OwnedTextCallback_release", "strings.ContainsRune(ontamaCallbackResult", "return C.CString(ontamaCallbackResult)", "type OwnedPointArrayCallback func(seed int32) []PayloadPointValue", "ontama_cffi_callback_OwnedPointArrayCallback_release", "ontamaOutput[index] = ontamaCFFIToPayloadPointValue(value)", "owned array size exceeds address space", "ontamaCallbackResult := state.callback", "copy(unsafe.Slice((*byte)(unsafe.Pointer(value0))", "type CallbackInputError struct", "const char *value0", "const uint8_t *value2", "C.GoString(value0)", "unsafe.Slice((*byte)(unsafe.Pointer(value2))", "ontamaCFFIFromPayloadPointValue(value0)", "ontamaCFFIFromPayloadEvent(value1)", "ontamaCFFIFromOverlayEvent(value0)", "runtime/cgo", "cgo.NewHandle", "defer ontamaCallbackHandle", "//export ontama_cffi_callback_VisitCallback_go", "ontama_cffi_call_Around", "ontama_c_int_must_be_32_bits", "ontama_c_uint_must_be_32_bits", "ErrNilCallback", "type CallbackPanicError struct", "func Fold(seed int32, count uint32, visit VisitCallback) (int32, error)", "func Observe(start int64, count uint32, observe ObserveCallback) error"} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated callback FFI does not contain %q:\n%s", want, generated)
		}
	}
	files := map[string]string{
		"go.mod":           "module callbackffi.test\n\ngo 1.23\n",
		"generated_ffi.go": generated,
		"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef enum fixture_decision { FIXTURE_CONTINUE = 1, FIXTURE_STOP = 2 } fixture_decision;
typedef enum fixture_payload_kind { FIXTURE_PAYLOAD_POINT = 1, FIXTURE_PAYLOAD_CODE = 2 } fixture_payload_kind;
typedef struct fixture_payload_point { int32_t x; int32_t y; } fixture_payload_point;
typedef struct fixture_payload_event { fixture_payload_kind kind; union { fixture_payload_point point; int32_t code; }; } fixture_payload_event;
enum { FIXTURE_OVERLAY_KEY = 10, FIXTURE_OVERLAY_MOTION = 11 };
typedef struct fixture_overlay_key { int32_t type; int32_t key; } fixture_overlay_key;
typedef struct fixture_overlay_motion { int32_t type; float x; } fixture_overlay_motion;
typedef union fixture_overlay_event { int32_t type; fixture_overlay_key key; fixture_overlay_motion motion; } fixture_overlay_event;
typedef bool (*fixture_visit_callback)(int32_t value, uint32_t index, void *context);
typedef void (*fixture_observe_callback)(int64_t value, void *context);
typedef fixture_decision (*fixture_decide_callback)(int32_t value, void *context);
int32_t fixture_fold(int32_t seed, uint32_t count, fixture_visit_callback visit, void *context);
int32_t fixture_around(int32_t prefix, fixture_visit_callback visit, void *context, int32_t suffix);
int32_t fixture_checked_fold(int32_t seed, fixture_visit_callback visit, void *context, int32_t *output);
int32_t fixture_validate(uint32_t count, fixture_visit_callback visit, void *context);
void fixture_observe(int64_t start, uint32_t count, fixture_observe_callback observe, void *context);
typedef void (*fixture_tick_callback)(void *context);
int32_t fixture_decide_sum(int32_t count, fixture_decide_callback decide, void *context);
int32_t fixture_parallel(uint32_t rounds, fixture_visit_callback visit, void *context);
void fixture_tick(fixture_tick_callback tick, void *context);
int32_t fixture_inspect(const char *label, const uint8_t *data, size_t length, fixture_visit_callback visit, void *context);
typedef bool (*fixture_stored_callback)(int32_t value, void *context);
int32_t fixture_stored_register(fixture_stored_callback callback, void *context);
int32_t fixture_stored_unregister(fixture_stored_callback callback, void *context);
bool fixture_stored_fire(int32_t value);
typedef int32_t (*fixture_payload_callback)(fixture_payload_point point, fixture_payload_event event, void *context);
int32_t fixture_emit_payload(fixture_payload_kind kind, fixture_payload_callback callback, void *context);
typedef int32_t (*fixture_stored_payload_callback)(fixture_payload_point point, fixture_payload_event event, void *context);
int32_t fixture_stored_payload_register(fixture_stored_payload_callback callback, void *context);
int32_t fixture_stored_payload_unregister(fixture_stored_payload_callback callback, void *context);
int32_t fixture_stored_payload_fire(fixture_payload_kind kind);
typedef int32_t (*fixture_overlay_callback)(fixture_overlay_event event, void *context);
int32_t fixture_emit_overlay(int32_t event_type, fixture_overlay_callback callback, void *context);
typedef int32_t (*fixture_text_buffer_callback)(const char *title, const char *optional, const uint8_t *data, size_t data_length, void *context);
int32_t fixture_emit_text_buffer(uint8_t mode, fixture_text_buffer_callback callback, void *context);
typedef int32_t (*fixture_stored_text_buffer_callback)(const char *title, const char *optional, const uint8_t *data, size_t data_length, void *context);
int32_t fixture_stored_text_buffer_register(fixture_stored_text_buffer_callback callback, void *context);
int32_t fixture_stored_text_buffer_unregister(fixture_stored_text_buffer_callback callback, void *context);
int32_t fixture_stored_text_buffer_fire(uint8_t mode);
typedef void (*fixture_observe_text_callback)(const char *title, void *context);
void fixture_observe_copied_text(bool missing, fixture_observe_text_callback callback, void *context);
typedef bool (*fixture_transform_bytes_callback)(uint8_t *data, size_t data_length, const uint8_t *salt, size_t salt_length, void *context);
int32_t fixture_transform_bytes(uint8_t mode, fixture_transform_bytes_callback callback, void *context);
int32_t fixture_last_transformed_checksum(void);
typedef void (*fixture_stored_transform_callback)(uint8_t *data, size_t data_length, void *context);
int32_t fixture_stored_transform_register(fixture_stored_transform_callback callback, void *context);
int32_t fixture_stored_transform_unregister(fixture_stored_transform_callback callback, void *context);
int32_t fixture_stored_transform_fire(uint8_t mode);
typedef void (*fixture_dual_transform_callback)(uint8_t *first, size_t first_length, uint8_t *second, size_t second_length, void *context);
int32_t fixture_transform_aliased(fixture_dual_transform_callback callback, void *context);
typedef uint8_t *(*fixture_owned_data_callback)(uint8_t seed, size_t *output_length, void *context);
typedef void (*fixture_owned_data_release)(uint8_t *data);
int32_t fixture_owned_data_register(fixture_owned_data_callback callback, fixture_owned_data_release release, void *context);
int32_t fixture_owned_data_unregister(fixture_owned_data_callback callback, fixture_owned_data_release release, void *context);
int32_t fixture_owned_data_fire(uint8_t seed, bool null_length);
int32_t fixture_owned_data_hold(uint8_t seed);
int32_t fixture_owned_data_release_held(void);
int32_t fixture_owned_data_release_count(void);
typedef char *(*fixture_owned_text_callback)(const char *path, void *context);
typedef void (*fixture_owned_text_release)(char *text);
int32_t fixture_owned_text_register(fixture_owned_text_callback callback, fixture_owned_text_release release, void *context);
int32_t fixture_owned_text_unregister(fixture_owned_text_callback callback, fixture_owned_text_release release, void *context);
int32_t fixture_owned_text_fire(const char *path);
int32_t fixture_owned_text_hold(const char *path);
int32_t fixture_owned_text_release_held(void);
int32_t fixture_owned_text_release_count(void);
typedef fixture_payload_point *(*fixture_owned_point_array_callback)(int32_t seed, size_t *output_length, void *context);
typedef void (*fixture_owned_point_array_release)(fixture_payload_point *points);
int32_t fixture_owned_point_array_register(fixture_owned_point_array_callback callback, fixture_owned_point_array_release release, void *context);
int32_t fixture_owned_point_array_unregister(fixture_owned_point_array_callback callback, fixture_owned_point_array_release release, void *context);
int32_t fixture_owned_point_array_fire(int32_t seed, bool null_length);
int32_t fixture_owned_point_array_hold(int32_t seed);
int32_t fixture_owned_point_array_release_held(void);
int32_t fixture_owned_point_array_release_count(void);
typedef fixture_decision *(*fixture_owned_decision_array_callback)(bool empty, size_t *output_length, void *context);
typedef void (*fixture_owned_decision_array_release)(fixture_decision *values);
int32_t fixture_owned_decision_array_register(fixture_owned_decision_array_callback callback, fixture_owned_decision_array_release release, void *context);
int32_t fixture_owned_decision_array_unregister(fixture_owned_decision_array_callback callback, fixture_owned_decision_array_release release, void *context);
int32_t fixture_owned_decision_array_fire(bool empty);
int32_t fixture_owned_decision_array_release_count(void);
`,
		"fixture.c": `#include "fixture.h"
#include <pthread.h>
#include <string.h>
int32_t fixture_fold(int32_t seed, uint32_t count, fixture_visit_callback visit, void *context) {
  int32_t total = 0;
  for (uint32_t index = 0; index < count; index++) { int32_t value = seed + (int32_t)index; if (!visit(value, index, context)) break; total += value; }
  return total;
}
int32_t fixture_around(int32_t prefix, fixture_visit_callback visit, void *context, int32_t suffix) { return visit(prefix + suffix, 7, context) ? prefix * 100 + suffix : -1; }
int32_t fixture_checked_fold(int32_t seed, fixture_visit_callback visit, void *context, int32_t *output) {
  if (seed < 0) return 31;
  *output = visit(seed, 9, context) ? seed * 2 : seed;
  return 0;
}
int32_t fixture_validate(uint32_t count, fixture_visit_callback visit, void *context) {
  for (uint32_t index = 0; index < count; index++) if (!visit((int32_t)index, index, context)) return 44;
  return 0;
}
void fixture_observe(int64_t start, uint32_t count, fixture_observe_callback observe, void *context) { for (uint32_t index = 0; index < count; index++) observe(start + index, context); }
int32_t fixture_decide_sum(int32_t count, fixture_decide_callback decide, void *context) { int32_t total = 0; for (int32_t value = 0; value < count; value++) { if (decide(value, context) == FIXTURE_STOP) break; total += value; } return total; }
typedef struct fixture_parallel_job { uint32_t start; uint32_t rounds; fixture_visit_callback visit; void *context; int32_t accepted; } fixture_parallel_job;
static void *fixture_parallel_worker(void *raw) { fixture_parallel_job *job = (fixture_parallel_job *)raw; for (uint32_t index = 0; index < job->rounds; index++) if (job->visit((int32_t)(job->start + index), index, job->context)) job->accepted++; return NULL; }
int32_t fixture_parallel(uint32_t rounds, fixture_visit_callback visit, void *context) {
  fixture_parallel_job jobs[2] = {{0, rounds, visit, context, 0}, {rounds, rounds, visit, context, 0}};
  pthread_t threads[2];
  if (pthread_create(&threads[0], NULL, fixture_parallel_worker, &jobs[0]) != 0) return -1;
  if (pthread_create(&threads[1], NULL, fixture_parallel_worker, &jobs[1]) != 0) { pthread_join(threads[0], NULL); return -2; }
  pthread_join(threads[0], NULL); pthread_join(threads[1], NULL);
  return jobs[0].accepted + jobs[1].accepted;
}
void fixture_tick(fixture_tick_callback tick, void *context) { tick(context); }
int32_t fixture_inspect(const char *label, const uint8_t *data, size_t length, fixture_visit_callback visit, void *context) {
  int32_t sum = 0; size_t label_length = 0;
  while (label[label_length] != 0) label_length++;
  for (size_t index = 0; index < length; index++) sum += data[index];
  return visit(sum, (uint32_t)length, context) ? (int32_t)(label_length + length) : -1;
}
static fixture_stored_callback stored_callback; static void *stored_context;
int32_t fixture_stored_register(fixture_stored_callback callback, void *context) { if (stored_callback) return 101; stored_callback = callback; stored_context = context; return 0; }
int32_t fixture_stored_unregister(fixture_stored_callback callback, void *context) { if (callback != stored_callback || context != stored_context) return 102; stored_callback = NULL; stored_context = NULL; return 0; }
bool fixture_stored_fire(int32_t value) { return stored_callback ? stored_callback(value, stored_context) : false; }
static fixture_payload_event fixture_payload(fixture_payload_kind kind) { fixture_payload_event event = {0}; event.kind = kind; if (kind == FIXTURE_PAYLOAD_POINT) { event.point.x = 11; event.point.y = 12; } else if (kind == FIXTURE_PAYLOAD_CODE) { event.code = -55; } return event; }
int32_t fixture_emit_payload(fixture_payload_kind kind, fixture_payload_callback callback, void *context) { fixture_payload_point point = {-3, 7}; return 1000 + callback(point, fixture_payload(kind), context); }
static fixture_stored_payload_callback stored_payload_callback; static void *stored_payload_context;
int32_t fixture_stored_payload_register(fixture_stored_payload_callback callback, void *context) { if (stored_payload_callback) return 111; stored_payload_callback = callback; stored_payload_context = context; return 0; }
int32_t fixture_stored_payload_unregister(fixture_stored_payload_callback callback, void *context) { if (callback != stored_payload_callback || context != stored_payload_context) return 112; stored_payload_callback = NULL; stored_payload_context = NULL; return 0; }
int32_t fixture_stored_payload_fire(fixture_payload_kind kind) { if (!stored_payload_callback) return -1; fixture_payload_point point = {-3, 7}; return stored_payload_callback(point, fixture_payload(kind), stored_payload_context); }
int32_t fixture_emit_overlay(int32_t event_type, fixture_overlay_callback callback, void *context) { fixture_overlay_event event = {0}; event.type = event_type; if (event_type == FIXTURE_OVERLAY_KEY) event.key.key = 42; else if (event_type == FIXTURE_OVERLAY_MOTION) event.motion.x = -1.5f; return 500 + callback(event, context); }
static int32_t fixture_call_text_buffer(uint8_t mode, fixture_text_buffer_callback callback, void *context) {
  static char unicode_title[] = "\xE6\xB8\xA9\xE6\xB3\x89"; static char empty_title[] = ""; static char optional[] = "note"; static char mutable_title[] = "copy";
  static uint8_t binary[] = {0, 1, 255}; static uint8_t empty_sentinel = 7; static uint8_t mutable_data[] = {5, 6}; static uint8_t repeated[] = {1, 2};
  if (mode == 0) return callback(unicode_title, NULL, binary, 3, context);
  if (mode == 1) return callback(empty_title, optional, &empty_sentinel, 0, context);
  if (mode == 2) return callback(NULL, optional, binary, 3, context);
  if (mode == 3) return callback(unicode_title, NULL, NULL, 2, context);
  if (mode == 4) return callback(unicode_title, NULL, &empty_sentinel, SIZE_MAX, context);
  if (mode == 5) { mutable_title[0] = 'c'; mutable_data[0] = 5; int32_t value = callback(mutable_title, optional, mutable_data, 2, context); mutable_title[0] = 'X'; mutable_data[0] = 99; return value; }
  repeated[0] = 1; int32_t first = callback(unicode_title, NULL, repeated, 2, context); repeated[0] = 9; int32_t second = callback(unicode_title, NULL, repeated, 2, context); return first + second;
}
int32_t fixture_emit_text_buffer(uint8_t mode, fixture_text_buffer_callback callback, void *context) { return 700 + fixture_call_text_buffer(mode, callback, context); }
static fixture_stored_text_buffer_callback stored_text_buffer_callback; static void *stored_text_buffer_context;
int32_t fixture_stored_text_buffer_register(fixture_stored_text_buffer_callback callback, void *context) { if (stored_text_buffer_callback) return 121; stored_text_buffer_callback = callback; stored_text_buffer_context = context; return 0; }
int32_t fixture_stored_text_buffer_unregister(fixture_stored_text_buffer_callback callback, void *context) { if (callback != stored_text_buffer_callback || context != stored_text_buffer_context) return 122; stored_text_buffer_callback = NULL; stored_text_buffer_context = NULL; return 0; }
int32_t fixture_stored_text_buffer_fire(uint8_t mode) { return stored_text_buffer_callback ? fixture_call_text_buffer(mode, stored_text_buffer_callback, stored_text_buffer_context) : -1; }
void fixture_observe_copied_text(bool missing, fixture_observe_text_callback callback, void *context) { static char title[] = "observe"; callback(missing ? NULL : title, context); }
static uint8_t transformed_data[3]; static int32_t last_transformed_checksum;
static void fixture_reset_transformed(void) { transformed_data[0] = 1; transformed_data[1] = 2; transformed_data[2] = 3; }
static int32_t fixture_transformed_checksum(void) { return transformed_data[0] + transformed_data[1] + transformed_data[2]; }
int32_t fixture_transform_bytes(uint8_t mode, fixture_transform_bytes_callback callback, void *context) { static uint8_t salt[2]; fixture_reset_transformed(); salt[0] = 10; salt[1] = 20; bool accepted; if (mode == 2) accepted = callback(NULL, 2, salt, 2, context); else if (mode == 3) accepted = callback(transformed_data, SIZE_MAX, salt, 2, context); else if (mode == 4) accepted = callback(transformed_data, 0, salt, 2, context); else accepted = callback(transformed_data, 3, salt, 2, context); last_transformed_checksum = fixture_transformed_checksum(); salt[0] = 99; return (accepted ? 1000 : 0) + last_transformed_checksum; }
int32_t fixture_last_transformed_checksum(void) { return last_transformed_checksum; }
static fixture_stored_transform_callback stored_transform_callback; static void *stored_transform_context;
int32_t fixture_stored_transform_register(fixture_stored_transform_callback callback, void *context) { if (stored_transform_callback) return 131; stored_transform_callback = callback; stored_transform_context = context; return 0; }
int32_t fixture_stored_transform_unregister(fixture_stored_transform_callback callback, void *context) { if (callback != stored_transform_callback || context != stored_transform_context) return 132; stored_transform_callback = NULL; stored_transform_context = NULL; return 0; }
int32_t fixture_stored_transform_fire(uint8_t mode) { if (!stored_transform_callback) return -1; fixture_reset_transformed(); if (mode == 2) stored_transform_callback(NULL, 2, stored_transform_context); else if (mode == 3) stored_transform_callback(transformed_data, SIZE_MAX, stored_transform_context); else stored_transform_callback(transformed_data, mode == 4 ? 0 : 3, stored_transform_context); last_transformed_checksum = fixture_transformed_checksum(); return last_transformed_checksum; }
int32_t fixture_transform_aliased(fixture_dual_transform_callback callback, void *context) { transformed_data[0] = 1; callback(transformed_data, 1, transformed_data, 1, context); last_transformed_checksum = transformed_data[0]; return transformed_data[0]; }
static fixture_owned_data_callback owned_data_callback; static fixture_owned_data_release owned_data_release; static void *owned_data_context; static uint8_t *held_owned_data; static size_t held_owned_data_length; static int32_t owned_data_release_count;
int32_t fixture_owned_data_register(fixture_owned_data_callback callback, fixture_owned_data_release release, void *context) { if (owned_data_callback) return 141; owned_data_callback = callback; owned_data_release = release; owned_data_context = context; return 0; }
int32_t fixture_owned_data_unregister(fixture_owned_data_callback callback, fixture_owned_data_release release, void *context) { if (callback != owned_data_callback || release != owned_data_release || context != owned_data_context) return 142; owned_data_callback = NULL; owned_data_context = NULL; return 0; }
int32_t fixture_owned_data_fire(uint8_t seed, bool null_length) { if (!owned_data_callback) return -1; size_t length = 99; uint8_t *data = owned_data_callback(seed, null_length ? NULL : &length, owned_data_context); if (null_length) return data == NULL ? 0 : -2; if (length == 0) return data == NULL ? 0 : -3; if (!data) return -4; int32_t checksum = 0; for (size_t index = 0; index < length; index++) checksum += data[index]; owned_data_release(data); owned_data_release_count++; return checksum; }
int32_t fixture_owned_data_hold(uint8_t seed) { if (!owned_data_callback || held_owned_data) return -1; held_owned_data_length = 0; held_owned_data = owned_data_callback(seed, &held_owned_data_length, owned_data_context); return (int32_t)held_owned_data_length; }
int32_t fixture_owned_data_release_held(void) { if (!held_owned_data) return held_owned_data_length == 0 ? 0 : -1; int32_t checksum = 0; for (size_t index = 0; index < held_owned_data_length; index++) checksum += held_owned_data[index]; owned_data_release(held_owned_data); owned_data_release_count++; held_owned_data = NULL; held_owned_data_length = 0; return checksum; }
int32_t fixture_owned_data_release_count(void) { return owned_data_release_count; }
static fixture_owned_text_callback owned_text_callback; static fixture_owned_text_release owned_text_release; static void *owned_text_context; static char *held_owned_text; static int32_t owned_text_release_count;
int32_t fixture_owned_text_register(fixture_owned_text_callback callback, fixture_owned_text_release release, void *context) { if (owned_text_callback) return 151; owned_text_callback = callback; owned_text_release = release; owned_text_context = context; return 0; }
int32_t fixture_owned_text_unregister(fixture_owned_text_callback callback, fixture_owned_text_release release, void *context) { if (callback != owned_text_callback || release != owned_text_release || context != owned_text_context) return 152; owned_text_callback = NULL; owned_text_context = NULL; return 0; }
int32_t fixture_owned_text_fire(const char *path) { if (!owned_text_callback) return -1; char *text = owned_text_callback(path, owned_text_context); if (!text) return 0; int32_t length = (int32_t)strlen(text); owned_text_release(text); owned_text_release_count++; return 100 + length; }
int32_t fixture_owned_text_hold(const char *path) { if (!owned_text_callback || held_owned_text) return -1; held_owned_text = owned_text_callback(path, owned_text_context); return held_owned_text ? (int32_t)strlen(held_owned_text) : -2; }
int32_t fixture_owned_text_release_held(void) { if (!held_owned_text) return 0; int32_t length = (int32_t)strlen(held_owned_text); owned_text_release(held_owned_text); owned_text_release_count++; held_owned_text = NULL; return length; }
int32_t fixture_owned_text_release_count(void) { return owned_text_release_count; }
static fixture_owned_point_array_callback owned_point_array_callback; static fixture_owned_point_array_release owned_point_array_release; static void *owned_point_array_context; static fixture_payload_point *held_owned_points; static size_t held_owned_point_count; static int32_t owned_point_array_release_count;
int32_t fixture_owned_point_array_register(fixture_owned_point_array_callback callback, fixture_owned_point_array_release release, void *context) { if (owned_point_array_callback) return 161; owned_point_array_callback = callback; owned_point_array_release = release; owned_point_array_context = context; return 0; }
int32_t fixture_owned_point_array_unregister(fixture_owned_point_array_callback callback, fixture_owned_point_array_release release, void *context) { if (callback != owned_point_array_callback || release != owned_point_array_release || context != owned_point_array_context) return 162; owned_point_array_callback = NULL; owned_point_array_context = NULL; return 0; }
static int32_t fixture_owned_point_array_sum(fixture_payload_point *points, size_t count) { int32_t sum = 0; for (size_t index = 0; index < count; index++) sum += points[index].x + points[index].y; return sum; }
int32_t fixture_owned_point_array_fire(int32_t seed, bool null_length) { if (!owned_point_array_callback) return -1; size_t count = 99; fixture_payload_point *points = owned_point_array_callback(seed, null_length ? NULL : &count, owned_point_array_context); if (null_length) return points == NULL ? 0 : -2; if (count == 0) return points == NULL ? 0 : -3; if (!points) return -4; int32_t sum = fixture_owned_point_array_sum(points, count); owned_point_array_release(points); owned_point_array_release_count++; return sum; }
int32_t fixture_owned_point_array_hold(int32_t seed) { if (!owned_point_array_callback || held_owned_points) return -1; held_owned_point_count = 0; held_owned_points = owned_point_array_callback(seed, &held_owned_point_count, owned_point_array_context); return (int32_t)held_owned_point_count; }
int32_t fixture_owned_point_array_release_held(void) { if (!held_owned_points) return held_owned_point_count == 0 ? 0 : -1; int32_t sum = fixture_owned_point_array_sum(held_owned_points, held_owned_point_count); owned_point_array_release(held_owned_points); owned_point_array_release_count++; held_owned_points = NULL; held_owned_point_count = 0; return sum; }
int32_t fixture_owned_point_array_release_count(void) { return owned_point_array_release_count; }
static fixture_owned_decision_array_callback owned_decision_array_callback; static fixture_owned_decision_array_release owned_decision_array_release; static void *owned_decision_array_context; static int32_t owned_decision_array_release_count;
int32_t fixture_owned_decision_array_register(fixture_owned_decision_array_callback callback, fixture_owned_decision_array_release release, void *context) { if (owned_decision_array_callback) return 171; owned_decision_array_callback = callback; owned_decision_array_release = release; owned_decision_array_context = context; return 0; }
int32_t fixture_owned_decision_array_unregister(fixture_owned_decision_array_callback callback, fixture_owned_decision_array_release release, void *context) { if (callback != owned_decision_array_callback || release != owned_decision_array_release || context != owned_decision_array_context) return 172; owned_decision_array_callback = NULL; owned_decision_array_context = NULL; return 0; }
int32_t fixture_owned_decision_array_fire(bool empty) { if (!owned_decision_array_callback) return -1; size_t count = 0; fixture_decision *values = owned_decision_array_callback(empty, &count, owned_decision_array_context); if (count == 0) return values == NULL ? 0 : -2; if (!values) return -3; int32_t sum = 0; for (size_t index = 0; index < count; index++) sum += (int32_t)values[index]; owned_decision_array_release(values); owned_decision_array_release_count++; return sum; }
int32_t fixture_owned_decision_array_release_count(void) { return owned_decision_array_release_count; }
`,
		"app/binding.otm": `import go ffi from "callbackffi.test";
function FoldThree(seed: int32, count: uint32): Result<int32> {
  const total = ffi.Fold(seed, count, (value: int32, index: uint32): boolean => index < 3)?;
  return ok(total);
}
function ReadPayload(kind: ffi.PayloadKind): Result<int32> {
  const value = ffi.EmitPayload(kind, (point: ffi.PayloadPointValue, event: ffi.PayloadEvent): int32 => point.X + point.Y + event.Point.X + event.Point.Y + event.Code + int32(event.Kind))?;
  return ok(value);
}
function ReadCopiedCallback(mode: byte): Result<int32> {
  const value = ffi.EmitTextBuffer(mode, (title: string, optional: *string, data: byte[]): int32 => int32(len(title) + len(data)))?;
  return ok(value);
}
function MutateBytes(): Result<int32> {
  const value = ffi.TransformBytes(0, (data: byte[], salt: byte[]): boolean => { data[0] += salt[0]; data[2] = 30; return true; })?;
  return ok(value);
}
function OwnedByte(value: byte): Result<int32> {
  const watch = ffi.RegisterOwnedDataWatch((current: byte): byte[] => [current])?;
  const checksum = ffi.FireOwnedData(value, false);
  watch.Close()?;
  return ok(checksum);
}
function OwnedText(value: string): Result<int32> {
  const watch = ffi.RegisterOwnedTextWatch((path: string): string => path)?;
  const length = ffi.FireOwnedText(value)?;
  watch.Close()?;
  return ok(length);
}
`,
		"cmd/main.go": `package main
import (
  "errors"
  "sync/atomic"
  app "callbackffi.test/app"
  ffi "callbackffi.test"
)
func assert(ok bool) { if !ok { panic("callScoped callback assertion failed") } }
func main() {
  total, err := ffi.Fold(10, 0, func(value int32, index uint32) bool { panic("must not run") }); assert(total == 0 && err == nil)
  total, err = ffi.Fold(10, 5, func(value int32, index uint32) bool { return index < 3 }); assert(total == 33 && err == nil)
  total, err = app.FoldThree(20, 10); assert(total == 63 && err == nil)
  total, err = ffi.Around(4, func(value int32, index uint32) bool { return value == 9 && index == 7 }, 5); assert(total == 405 && err == nil)
  total, err = ffi.Around(4, func(value int32, index uint32) bool { return false }, 5); assert(total == -1 && err == nil)
  total, err = ffi.CheckedFold(21, func(value int32, index uint32) bool { return value == 21 && index == 9 }); assert(total == 42 && err == nil)
  total, err = ffi.CheckedFold(-1, func(value int32, index uint32) bool { panic("not called") }); var status *ffi.StatusError; assert(total == 0 && errors.As(err, &status) && status.Code == 31)
  err = ffi.Validate(3, func(value int32, index uint32) bool { return index < 2 }); assert(errors.As(err, &status) && status.Code == 44)
  var observed int64
  err = ffi.Observe(-2, 5, func(value int64) { observed += value }); assert(err == nil && observed == 0)
  ticks := 0; err = ffi.Tick(func() { ticks++ }); assert(err == nil && ticks == 1)
  inspected, err := ffi.Inspect("温泉", []byte{0, 1, 255}, func(value int32, index uint32) bool { return value == 256 && index == 3 }); assert(inspected == 9 && err == nil)
  inspected, err = ffi.Inspect("empty", nil, func(value int32, index uint32) bool { return value == 0 && index == 0 }); assert(inspected == 5 && err == nil)
  inspectCalls := 0; inspected, err = ffi.Inspect("bad\x00label", []byte{1}, func(value int32, index uint32) bool { inspectCalls++; return true }); assert(inspected == 0 && inspectCalls == 0 && errors.Is(err, ffi.ErrEmbeddedNUL))
  storedValue := int32(0); stored, err := ffi.RegisterStoredWatch(func(value int32) bool { storedValue = value; return value == 42 }); assert(err == nil && ffi.FireStored(42) && storedValue == 42)
  assert(stored.Close() == nil && !ffi.FireStored(7) && storedValue == 42)
  payload, err := ffi.EmitPayload(ffi.PayloadPoint, func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { assert(point == (ffi.PayloadPointValue{X: -3, Y: 7}) && event.Kind == ffi.PayloadPoint && event.Point == (ffi.PayloadPointValue{X: 11, Y: 12}) && event.Code == 0); return 42 }); assert(payload == 1042 && err == nil)
  payload, err = ffi.EmitPayload(ffi.PayloadCode, func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { assert(point.X == -3 && point.Y == 7 && event.Kind == ffi.PayloadCode && event.Point == (ffi.PayloadPointValue{}) && event.Code == -55); return -51 }); assert(payload == 949 && err == nil)
  payload, err = ffi.EmitPayload(ffi.PayloadKind(77), func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { assert(event.Kind == ffi.PayloadKind(77) && event.Point == (ffi.PayloadPointValue{}) && event.Code == 0); return int32(event.Kind) }); assert(payload == 1077 && err == nil)
  payload, err = app.ReadPayload(ffi.PayloadPoint); assert(payload == 1028 && err == nil)
  var payloadPanic *ffi.CallbackPanicError
  payload, err = ffi.EmitPayload(ffi.PayloadPoint, func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { panic("payload exploded") }); assert(payload == 0 && errors.As(err, &payloadPanic) && payloadPanic.Function == "EmitPayload")
  storedPayload, err := ffi.RegisterStoredPayloadWatch(func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { return point.X + point.Y + event.Point.X + event.Point.Y + event.Code }); assert(err == nil)
  assert(ffi.FireStoredPayload(ffi.PayloadPoint) == 27 && ffi.FireStoredPayload(ffi.PayloadCode) == -51 && ffi.FireStoredPayload(ffi.PayloadKind(91)) == 4)
  rejectedStoredPayload, err := ffi.RegisterStoredPayloadWatch(func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { return 0 }); assert(rejectedStoredPayload == nil && errors.As(err, &status) && status.Code == 111)
  assert(storedPayload.Close() == nil && ffi.FireStoredPayload(ffi.PayloadPoint) == -1)
  storedPayloadPanicCalls := 0
  panickingStoredPayload, err := ffi.RegisterStoredPayloadWatch(func(point ffi.PayloadPointValue, event ffi.PayloadEvent) int32 { storedPayloadPanicCalls++; panic("stored payload exploded") }); assert(err == nil && ffi.FireStoredPayload(ffi.PayloadPoint) == 0 && storedPayloadPanicCalls == 1)
  assert(errors.As(panickingStoredPayload.CallbackError(), &payloadPanic) && payloadPanic.Function == "StoredPayloadWatch" && payloadPanic.Value == "stored payload exploded")
  assert(ffi.FireStoredPayload(ffi.PayloadCode) == 0 && storedPayloadPanicCalls == 1 && panickingStoredPayload.Close() == nil)
  overlay, err := ffi.EmitOverlay(10, func(event ffi.OverlayEvent) int32 { assert(event.Type == 10 && event.Key.Key == 42 && event.Motion == (ffi.OverlayMotionValue{})); return event.Key.Key }); assert(overlay == 542 && err == nil)
  overlay, err = ffi.EmitOverlay(11, func(event ffi.OverlayEvent) int32 { assert(event.Type == 11 && event.Key == (ffi.OverlayKeyValue{}) && event.Motion.X == -1.5); return int32(event.Motion.X * -10) }); assert(overlay == 515 && err == nil)
  overlay, err = ffi.EmitOverlay(99, func(event ffi.OverlayEvent) int32 { assert(event.Type == 99 && event.Key == (ffi.OverlayKeyValue{}) && event.Motion == (ffi.OverlayMotionValue{})); return event.Type }); assert(overlay == 599 && err == nil)
  copied, err := ffi.EmitTextBuffer(0, func(title string, optional *string, data []byte) int32 { assert(title == "温泉" && optional == nil && data != nil && len(data) == 3 && data[0] == 0 && data[2] == 255); return int32(len(title) + len(data)) }); assert(copied == 709 && err == nil)
  var capturedOptional *string
  copied, err = ffi.EmitTextBuffer(1, func(title string, optional *string, data []byte) int32 { capturedOptional = optional; assert(title == "" && optional != nil && *optional == "note" && data != nil && len(data) == 0); return 1 }); assert(copied == 701 && err == nil && capturedOptional != nil && *capturedOptional == "note")
  capturedTitle := ""; var capturedData []byte
  copied, err = ffi.EmitTextBuffer(5, func(title string, optional *string, data []byte) int32 { capturedTitle = title; capturedData = data; return int32(data[0] + data[1]) }); assert(copied == 711 && err == nil && capturedTitle == "copy" && len(capturedData) == 2 && capturedData[0] == 5 && capturedData[1] == 6)
  var repeatedCopies [][]byte
  copied, err = ffi.EmitTextBuffer(6, func(title string, optional *string, data []byte) int32 { repeatedCopies = append(repeatedCopies, data); return int32(data[0]) }); assert(copied == 710 && err == nil && len(repeatedCopies) == 2 && repeatedCopies[0][0] == 1 && repeatedCopies[1][0] == 9)
  copiedCalls := 0; var inputError *ffi.CallbackInputError
  copied, err = ffi.EmitTextBuffer(2, func(title string, optional *string, data []byte) int32 { copiedCalls++; return 1 }); assert(copied == 0 && copiedCalls == 0 && errors.As(err, &inputError) && inputError.Function == "EmitTextBuffer" && inputError.Parameter == "title" && inputError.Reason == "null string pointer")
  copied, err = ffi.EmitTextBuffer(3, func(title string, optional *string, data []byte) int32 { copiedCalls++; return 1 }); assert(copied == 0 && copiedCalls == 0 && errors.As(err, &inputError) && inputError.Parameter == "data" && inputError.Reason == "null byte pointer with non-zero length")
  copied, err = ffi.EmitTextBuffer(4, func(title string, optional *string, data []byte) int32 { copiedCalls++; return 1 }); assert(copied == 0 && copiedCalls == 0 && errors.As(err, &inputError) && inputError.Parameter == "data" && inputError.Reason == "byte length exceeds Go int")
  copied, err = app.ReadCopiedCallback(0); assert(copied == 709 && err == nil)
  observedCopiedText := ""; err = ffi.ObserveCopiedText(false, func(title string) { observedCopiedText = title }); assert(err == nil && observedCopiedText == "observe")
  observedCopiedCalls := 0; err = ffi.ObserveCopiedText(true, func(title string) { observedCopiedCalls++ }); assert(observedCopiedCalls == 0 && errors.As(err, &inputError) && inputError.Function == "ObserveCopiedText" && inputError.Parameter == "title")
  storedTextCalls := 0
  storedText, err := ffi.RegisterStoredTextBufferWatch(func(title string, optional *string, data []byte) int32 { storedTextCalls++; return int32(len(title) + len(data)) }); assert(err == nil && ffi.FireStoredTextBuffer(0) == 9 && storedTextCalls == 1)
  assert(ffi.FireStoredTextBuffer(3) == 0 && storedTextCalls == 1 && errors.As(storedText.CallbackError(), &inputError) && inputError.Function == "StoredTextBufferWatch" && inputError.Parameter == "data")
  assert(ffi.FireStoredTextBuffer(0) == 0 && storedTextCalls == 1 && storedText.Close() == nil && ffi.FireStoredTextBuffer(0) == -1)
  var capturedSalt []byte
  transformed, err := ffi.TransformBytes(0, func(data []byte, salt []byte) bool { capturedSalt = salt; assert(data != nil && len(data) == 3 && data[0] == 1 && salt[0] == 10); data[0] += salt[0]; data[2] = 30; return true }); assert(transformed == 1043 && err == nil && ffi.LastTransformedChecksum() == 43 && capturedSalt[0] == 10)
  transformed, err = ffi.TransformBytes(1, func(data []byte, salt []byte) bool { data[0] = 8; return false }); assert(transformed == 13 && err == nil && ffi.LastTransformedChecksum() == 13)
  transformCalls := 0
  transformed, err = ffi.TransformBytes(2, func(data []byte, salt []byte) bool { transformCalls++; return true }); assert(transformed == 0 && transformCalls == 0 && errors.As(err, &inputError) && inputError.Parameter == "data" && ffi.LastTransformedChecksum() == 6)
  transformed, err = ffi.TransformBytes(3, func(data []byte, salt []byte) bool { transformCalls++; return true }); assert(transformed == 0 && transformCalls == 0 && errors.As(err, &inputError) && inputError.Reason == "byte length exceeds Go int" && ffi.LastTransformedChecksum() == 6)
  transformed, err = ffi.TransformBytes(4, func(data []byte, salt []byte) bool { assert(data != nil && len(data) == 0); return true }); assert(transformed == 1006 && err == nil && ffi.LastTransformedChecksum() == 6)
  var transformPanic *ffi.CallbackPanicError
  transformed, err = ffi.TransformBytes(0, func(data []byte, salt []byte) bool { data[0] = 200; panic("transform exploded") }); assert(transformed == 0 && errors.As(err, &transformPanic) && transformPanic.Function == "TransformBytes" && ffi.LastTransformedChecksum() == 6)
  transformed, err = app.MutateBytes(); assert(transformed == 1043 && err == nil && ffi.LastTransformedChecksum() == 43)
  storedTransformCalls := 0
  storedTransform, err := ffi.RegisterStoredTransformBytesWatch(func(data []byte) { storedTransformCalls++; data[1] = 40 }); assert(err == nil && ffi.FireStoredTransform(0) == 44 && storedTransformCalls == 1)
  assert(ffi.FireStoredTransform(2) == 6 && storedTransformCalls == 1 && errors.As(storedTransform.CallbackError(), &inputError) && inputError.Function == "StoredTransformBytesWatch" && inputError.Parameter == "data")
  assert(ffi.FireStoredTransform(0) == 6 && storedTransformCalls == 1 && storedTransform.Close() == nil && ffi.FireStoredTransform(0) == -1)
  panickingTransform, err := ffi.RegisterStoredTransformBytesWatch(func(data []byte) { data[0] = 200; panic("stored transform exploded") }); assert(err == nil && ffi.FireStoredTransform(0) == 6)
  assert(errors.As(panickingTransform.CallbackError(), &transformPanic) && transformPanic.Function == "StoredTransformBytesWatch" && ffi.LastTransformedChecksum() == 6 && panickingTransform.Close() == nil)
  aliased, err := ffi.TransformAliased(func(first []byte, second []byte) { first[0] = 10; second[0] = 20 }); assert(aliased == 20 && err == nil && ffi.LastTransformedChecksum() == 20)
  aliased, err = ffi.TransformAliased(func(first []byte, second []byte) { first[0] = 10; second[0] = 20; panic("aliased transform exploded") }); assert(aliased == 0 && errors.As(err, &transformPanic) && transformPanic.Function == "TransformAliased" && ffi.LastTransformedChecksum() == 1)
  assert(ffi.OwnedDataReleaseCount() == 0)
  ownedData, err := ffi.RegisterOwnedDataWatch(func(seed byte) []byte { if seed == 0 { return []byte{} }; return []byte{seed, seed + 1, seed + 2} }); assert(err == nil)
  rejectedOwnedData, err := ffi.RegisterOwnedDataWatch(func(seed byte) []byte { return []byte{1} }); assert(rejectedOwnedData == nil && errors.As(err, &status) && status.Code == 141)
  assert(ffi.FireOwnedData(42, false) == 129 && ffi.OwnedDataReleaseCount() == 1)
  assert(ffi.FireOwnedData(0, false) == 0 && ffi.OwnedDataReleaseCount() == 1)
  assert(ffi.HoldOwnedData(7) == 3 && ownedData.Close() == nil && ffi.FireOwnedData(7, false) == -1)
  assert(ffi.ReleaseHeldOwnedData() == 24 && ffi.OwnedDataReleaseCount() == 2)
  panickingOwnedData, err := ffi.RegisterOwnedDataWatch(func(seed byte) []byte { panic("owned data exploded") }); assert(err == nil && ffi.FireOwnedData(255, false) == 0)
  assert(errors.As(panickingOwnedData.CallbackError(), &transformPanic) && transformPanic.Function == "OwnedDataWatch" && ffi.OwnedDataReleaseCount() == 2 && panickingOwnedData.Close() == nil)
  ownedDataCalls := 0
  invalidOwnedData, err := ffi.RegisterOwnedDataWatch(func(seed byte) []byte { ownedDataCalls++; return []byte{seed} }); assert(err == nil && ffi.FireOwnedData(9, true) == 0 && ownedDataCalls == 0)
  assert(errors.As(invalidOwnedData.CallbackError(), &inputError) && inputError.Function == "OwnedDataWatch" && inputError.Parameter == "$result" && inputError.Reason == "null owned byte length pointer" && invalidOwnedData.Close() == nil)
  ownedChecksum, err := app.OwnedByte(9); assert(ownedChecksum == 9 && err == nil && ffi.OwnedDataReleaseCount() == 3)
  assert(ffi.OwnedTextReleaseCount() == 0)
  ownedText, err := ffi.RegisterOwnedTextWatch(func(path string) string { return path + "!" }); assert(err == nil)
  rejectedOwnedText, err := ffi.RegisterOwnedTextWatch(func(path string) string { return path }); assert(rejectedOwnedText == nil && errors.As(err, &status) && status.Code == 151)
  ownedTextLength, err := ffi.FireOwnedText("温泉"); assert(ownedTextLength == 107 && err == nil && ffi.OwnedTextReleaseCount() == 1)
  assert(ownedText.Close() == nil)
  emptyOwnedText, err := ffi.RegisterOwnedTextWatch(func(path string) string { return "" }); assert(err == nil)
  ownedTextLength, err = ffi.FireOwnedText(""); assert(ownedTextLength == 100 && err == nil && ffi.OwnedTextReleaseCount() == 2 && emptyOwnedText.Close() == nil)
  heldOwnedText, err := ffi.RegisterOwnedTextWatch(func(path string) string { return path }); assert(err == nil)
  heldTextLength, err := ffi.HoldOwnedText("late"); assert(heldTextLength == 4 && err == nil && heldOwnedText.Close() == nil)
  assert(ffi.ReleaseHeldOwnedText() == 4 && ffi.OwnedTextReleaseCount() == 3)
  panickingOwnedText, err := ffi.RegisterOwnedTextWatch(func(path string) string { panic("owned text exploded") }); assert(err == nil)
  ownedTextLength, err = ffi.FireOwnedText("panic"); assert(ownedTextLength == 0 && err == nil && errors.As(panickingOwnedText.CallbackError(), &transformPanic) && transformPanic.Function == "OwnedTextWatch" && ffi.OwnedTextReleaseCount() == 3 && panickingOwnedText.Close() == nil)
  invalidOwnedText, err := ffi.RegisterOwnedTextWatch(func(path string) string { return "bad\x00tail" }); assert(err == nil)
  ownedTextLength, err = ffi.FireOwnedText("nul"); assert(ownedTextLength == 0 && err == nil && errors.As(invalidOwnedText.CallbackError(), &inputError) && inputError.Function == "OwnedTextWatch" && inputError.Parameter == "$result" && inputError.Reason == "embedded NUL in owned string result" && ffi.OwnedTextReleaseCount() == 3 && invalidOwnedText.Close() == nil)
  ownedTextLength, err = app.OwnedText("steam"); assert(ownedTextLength == 105 && err == nil && ffi.OwnedTextReleaseCount() == 4)
  assert(ffi.OwnedPointArrayReleaseCount() == 0)
  ownedPoints, err := ffi.RegisterOwnedPointArrayWatch(func(seed int32) []ffi.PayloadPointValue { if seed == 0 { return []ffi.PayloadPointValue{} }; return []ffi.PayloadPointValue{{X: seed, Y: seed + 1}, {X: seed + 2, Y: seed + 3}} }); assert(err == nil)
  rejectedOwnedPoints, err := ffi.RegisterOwnedPointArrayWatch(func(seed int32) []ffi.PayloadPointValue { return nil }); assert(rejectedOwnedPoints == nil && errors.As(err, &status) && status.Code == 161)
  assert(ffi.FireOwnedPointArray(10, false) == 46 && ffi.OwnedPointArrayReleaseCount() == 1)
  assert(ffi.FireOwnedPointArray(0, false) == 0 && ffi.OwnedPointArrayReleaseCount() == 1)
  assert(ffi.HoldOwnedPointArray(20) == 2 && ownedPoints.Close() == nil && ffi.FireOwnedPointArray(20, false) == -1)
  assert(ffi.ReleaseHeldOwnedPointArray() == 86 && ffi.OwnedPointArrayReleaseCount() == 2)
  panickingOwnedPoints, err := ffi.RegisterOwnedPointArrayWatch(func(seed int32) []ffi.PayloadPointValue { panic("owned point array exploded") }); assert(err == nil && ffi.FireOwnedPointArray(1, false) == 0)
  assert(errors.As(panickingOwnedPoints.CallbackError(), &transformPanic) && transformPanic.Function == "OwnedPointArrayWatch" && ffi.OwnedPointArrayReleaseCount() == 2 && panickingOwnedPoints.Close() == nil)
  ownedPointCalls := 0
  invalidOwnedPoints, err := ffi.RegisterOwnedPointArrayWatch(func(seed int32) []ffi.PayloadPointValue { ownedPointCalls++; return []ffi.PayloadPointValue{{X: seed}} }); assert(err == nil && ffi.FireOwnedPointArray(9, true) == 0 && ownedPointCalls == 0)
  assert(errors.As(invalidOwnedPoints.CallbackError(), &inputError) && inputError.Function == "OwnedPointArrayWatch" && inputError.Parameter == "$result" && inputError.Reason == "null owned array length pointer" && invalidOwnedPoints.Close() == nil)
  ownedDecisions, err := ffi.RegisterOwnedDecisionArrayWatch(func(empty bool) []ffi.Decision { if empty { return []ffi.Decision{} }; return []ffi.Decision{ffi.DecisionContinue, ffi.DecisionStop} }); assert(err == nil)
  assert(ffi.FireOwnedDecisionArray(false) == 3 && ffi.OwnedDecisionArrayReleaseCount() == 1)
  assert(ffi.FireOwnedDecisionArray(true) == 0 && ffi.OwnedDecisionArrayReleaseCount() == 1 && ownedDecisions.Close() == nil)
  decided, err := ffi.DecideSum(10, func(value int32) ffi.Decision { if value == 3 { return ffi.DecisionStop }; return ffi.DecisionContinue }); assert(decided == 3 && err == nil)
  var concurrent atomic.Int32
  accepted, err := ffi.Parallel(1000, func(value int32, index uint32) bool { concurrent.Add(1); return true }); assert(accepted == 2000 && concurrent.Load() == 2000 && err == nil)
  total, err = ffi.Fold(0, 1, nil); assert(total == 0 && errors.Is(err, ffi.ErrNilCallback))
  err = ffi.Observe(0, 1, nil); assert(errors.Is(err, ffi.ErrNilCallback))
  calls := 0
  total, err = ffi.Fold(10, 8, func(value int32, index uint32) bool { calls++; if index == 2 { panic("visit exploded") }; return true })
  var callbackPanic *ffi.CallbackPanicError
  assert(total == 0 && calls == 3 && errors.As(err, &callbackPanic) && callbackPanic.Function == "Fold" && callbackPanic.Value == "visit exploded")
  err = ffi.Validate(2, func(value int32, index uint32) bool { panic("status panic") }); assert(errors.As(err, &callbackPanic) && callbackPanic.Function == "Validate")
  total, err = ffi.CheckedFold(4, func(value int32, index uint32) bool { panic("status-out panic") }); assert(total == 0 && errors.As(err, &callbackPanic) && callbackPanic.Function == "CheckedFold")
  err = ffi.Observe(1, 3, func(value int64) { if value == 2 { panic("void panic") } }); assert(errors.As(err, &callbackPanic) && callbackPanic.Function == "Observe")
  accepted, err = ffi.Parallel(100, func(value int32, index uint32) bool { if value == 50 { panic("parallel panic") }; return true }); assert(accepted == 0 && errors.As(err, &callbackPanic) && callbackPanic.Function == "Parallel")
  total, err = ffi.Fold(1, 2, func(value int32, index uint32) bool { return true }); assert(total == 3 && err == nil)
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generatedApp, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.otm")}, "app")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("generate callScoped callback OnsenTamago wrapper: err=%v diagnostics=%v", err, diagnostics)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generatedApp, 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"run", "-buildvcs=false", "./cmd"}
	if os.Getenv("ONTAMA_DIFFERENTIAL_RACE") == "1" {
		arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
	}
	command := exec.Command("go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("callScoped callback incoming C FFI failed: %v\n%s\n%s", err, output, artifacts.Source)
	}
}

func TestIncomingCFFIRegisteredCallbacks(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler is not available")
	}
	root := t.TempDir()
	artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,
  "package":"registeredffi",
  "header":"fixture.h",
  "threadPolicy":"threadSafe",
  "cFlags":["-pthread"],
  "ldFlags":["-pthread"],
  "enums":[{"name":"Topic","cType":"fixture_topic","underlying":"cInt32","values":[{"name":"TopicInput","symbol":"FIXTURE_TOPIC_INPUT"},{"name":"TopicWindow","symbol":"FIXTURE_TOPIC_WINDOW"}]}],
  "structs":[{"name":"Filter","cType":"fixture_filter","fields":[{"name":"Minimum","cName":"minimum","type":"int32"},{"name":"Maximum","cName":"maximum","type":"int32"}]}],
  "handles":[{"name":"Resource","cType":"fixture_resource","release":"fixture_resource_free"}],
  "callbacks":[{"name":"EventCallback","lifetime":"registered","parameters":[{"name":"value","type":"int32"}],"result":"boolean"}],
  "callbackRegistrations":[{"name":"Watch","callback":"EventCallback","register":"fixture_watch_register","unregister":"fixture_watch_unregister"},{"name":"TopicWatch","callback":"EventCallback","parameters":[{"name":"topic","type":"Topic"}],"register":"fixture_topic_watch_register","unregister":"fixture_topic_watch_unregister"},{"name":"FilteredWatch","callback":"EventCallback","parameters":[{"name":"topic","type":"Topic"},{"name":"filter","type":"Filter"}],"register":"fixture_filtered_watch_register","unregister":"fixture_filtered_watch_unregister"},{"name":"ResourceWatch","callback":"EventCallback","parameters":[{"name":"resource","type":"Resource"},{"name":"topic","type":"Topic"}],"register":"fixture_resource_watch_register","unregister":"fixture_resource_watch_unregister"},{"name":"RetainedWatch","callback":"EventCallback","parameters":[{"name":"label","type":"retainedCString"},{"name":"data","type":"retainedBytes"}],"register":"fixture_retained_watch_register","unregister":"fixture_retained_watch_unregister"}],
  "functions":[
    {"name":"Fire","symbol":"fixture_fire","parameters":[{"name":"value","type":"int32"}],"result":"int32","convention":"direct"},
    {"name":"FireParallel","symbol":"fixture_fire_parallel","parameters":[{"name":"start","type":"int32"},{"name":"rounds","type":"uint32"}],"result":"int32","convention":"direct"},
    {"name":"FailNextUnregister","symbol":"fixture_fail_next_unregister","parameters":[],"result":"void","convention":"direct"},
    {"name":"FireTopic","symbol":"fixture_topic_fire","parameters":[{"name":"topic","type":"Topic"},{"name":"value","type":"int32"}],"result":"int32","convention":"direct"},
    {"name":"FireFiltered","symbol":"fixture_filtered_fire","parameters":[{"name":"topic","type":"Topic"},{"name":"value","type":"int32"}],"result":"boolean","convention":"direct"},
    {"name":"NewResource","symbol":"fixture_resource_new","parameters":[],"result":"Resource","convention":"statusOut"},
    {"name":"FireResource","symbol":"fixture_resource_fire","parameters":[{"name":"resource","type":"Resource"},{"name":"topic","type":"Topic"},{"name":"value","type":"int32"}],"result":"int32","convention":"statusOut"},
    {"name":"FailNextResourceUnregister","symbol":"fixture_resource_fail_next_unregister","parameters":[],"result":"void","convention":"direct"},
    {"name":"FireRetained","symbol":"fixture_retained_fire","parameters":[],"result":"int32","convention":"direct"},
    {"name":"FailNextRetainedUnregister","symbol":"fixture_retained_fail_next_unregister","parameters":[],"result":"void","convention":"direct"}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(artifacts.Source)
	for _, want := range []string{"type EventCallback func(value int32) bool", "type Watch struct", "func RegisterWatch(callback EventCallback) (*Watch, error)", "func RegisterTopicWatch(topic Topic, callback EventCallback) (*TopicWatch, error)", "func RegisterFilteredWatch(topic Topic, filter Filter, callback EventCallback) (*FilteredWatch, error)", "func RegisterResourceWatch(resource *Resource, topic Topic, callback EventCallback) (*ResourceWatch, error)", "func RegisterRetainedWatch(label string, data []byte, callback EventCallback) (*RetainedWatch, error)", "parameter0 Topic", "parameter1 Filter", "parameter0 *Resource", "*C.char", "unsafe.Pointer", "C.CString(label)", "C.CBytes(data)", "C.ontama_cffi_free_string(registration.parameter0)", "C.ontama_cffi_free_bytes(registration.parameter1)", "resource.registrations++", "registration.parameter0.registrations--", "ErrHandleHasActiveRegistrations", "func (registration *Watch) Close() error", "func (registration *Watch) CallbackError() error", "ErrClosedCallbackRegistration", "state.inFlight.Add(1)", "registration.state.stop()", "registration.state.wait()", "registration.state.resume()", "registration.context.Delete()", "ontama_cffi_register_Watch", "ontama_cffi_unregister_Watch"} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated registered callback FFI does not contain %q:\n%s", want, generated)
		}
	}
	files := map[string]string{
		"go.mod":           "module registeredffi.test\n\ngo 1.23\n",
		"generated_ffi.go": generated,
		"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef bool (*fixture_event_callback)(int32_t value, void *context);
typedef enum fixture_topic { FIXTURE_TOPIC_INPUT = 1, FIXTURE_TOPIC_WINDOW = 2 } fixture_topic;
typedef struct fixture_filter { int32_t minimum; int32_t maximum; } fixture_filter;
typedef struct fixture_resource fixture_resource;
int32_t fixture_watch_register(fixture_event_callback callback, void *context);
int32_t fixture_watch_unregister(fixture_event_callback callback, void *context);
int32_t fixture_fire(int32_t value);
int32_t fixture_fire_parallel(int32_t start, uint32_t rounds);
void fixture_fail_next_unregister(void);
int32_t fixture_topic_watch_register(fixture_topic topic, fixture_event_callback callback, void *context);
int32_t fixture_topic_watch_unregister(fixture_topic topic, fixture_event_callback callback, void *context);
int32_t fixture_topic_fire(fixture_topic topic, int32_t value);
int32_t fixture_filtered_watch_register(fixture_topic topic, fixture_filter filter, fixture_event_callback callback, void *context);
int32_t fixture_filtered_watch_unregister(fixture_topic topic, fixture_filter filter, fixture_event_callback callback, void *context);
bool fixture_filtered_fire(fixture_topic topic, int32_t value);
int32_t fixture_resource_new(fixture_resource **output);
void fixture_resource_free(fixture_resource *resource);
int32_t fixture_resource_watch_register(fixture_resource *resource, fixture_topic topic, fixture_event_callback callback, void *context);
int32_t fixture_resource_watch_unregister(fixture_resource *resource, fixture_topic topic, fixture_event_callback callback, void *context);
int32_t fixture_resource_fire(fixture_resource *resource, fixture_topic topic, int32_t value, int32_t *output);
void fixture_resource_fail_next_unregister(void);
int32_t fixture_retained_watch_register(char *label, uint8_t *data, size_t data_length, fixture_event_callback callback, void *context);
int32_t fixture_retained_watch_unregister(char *label, uint8_t *data, size_t data_length, fixture_event_callback callback, void *context);
int32_t fixture_retained_fire(void);
void fixture_retained_fail_next_unregister(void);
`,
		"fixture.c": `#include "fixture.h"
#include <pthread.h>
#include <stddef.h>
#include <stdlib.h>
typedef struct fixture_watcher { fixture_event_callback callback; void *context; bool active; uint32_t in_flight; } fixture_watcher;
static fixture_watcher watchers[4];
static pthread_mutex_t watchers_mutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t watchers_changed = PTHREAD_COND_INITIALIZER;
static bool fail_next_unregister;
typedef struct fixture_topic_watcher { fixture_topic topic; fixture_event_callback callback; void *context; } fixture_topic_watcher;
static fixture_topic_watcher topic_watchers[4];
static fixture_topic filtered_topic; static fixture_filter filtered_filter; static fixture_event_callback filtered_callback; static void *filtered_context;
struct fixture_resource { bool released; };
static fixture_resource *resource_watch_resource; static fixture_topic resource_watch_topic; static fixture_event_callback resource_watch_callback; static void *resource_watch_context; static bool fail_next_resource_unregister;
static char *retained_label; static uint8_t *retained_data; static size_t retained_data_length; static fixture_event_callback retained_callback; static void *retained_context; static bool fail_next_retained_unregister;
int32_t fixture_watch_register(fixture_event_callback callback, void *context) {
  pthread_mutex_lock(&watchers_mutex);
  for (size_t index = 0; index < 4; index++) {
    if (!watchers[index].active && watchers[index].in_flight == 0) {
      watchers[index] = (fixture_watcher){callback, context, true, 0};
      pthread_mutex_unlock(&watchers_mutex);
      return 0;
    }
  }
  pthread_mutex_unlock(&watchers_mutex);
  return 61;
}
int32_t fixture_watch_unregister(fixture_event_callback callback, void *context) {
  pthread_mutex_lock(&watchers_mutex);
  if (fail_next_unregister) { fail_next_unregister = false; pthread_mutex_unlock(&watchers_mutex); return 62; }
  for (size_t index = 0; index < 4; index++) {
    fixture_watcher *watcher = &watchers[index];
    if (watcher->active && watcher->callback == callback && watcher->context == context) {
      watcher->active = false;
      while (watcher->in_flight != 0) pthread_cond_wait(&watchers_changed, &watchers_mutex);
      watcher->callback = NULL; watcher->context = NULL;
      pthread_mutex_unlock(&watchers_mutex);
      return 0;
    }
  }
  pthread_mutex_unlock(&watchers_mutex);
  return 63;
}
int32_t fixture_fire(int32_t value) {
  fixture_watcher *selected[4]; size_t selected_count = 0;
  pthread_mutex_lock(&watchers_mutex);
  for (size_t index = 0; index < 4; index++) if (watchers[index].active) { watchers[index].in_flight++; selected[selected_count++] = &watchers[index]; }
  pthread_mutex_unlock(&watchers_mutex);
  int32_t accepted = 0;
  for (size_t index = 0; index < selected_count; index++) {
    fixture_watcher *watcher = selected[index];
    if (watcher->callback(value, watcher->context)) accepted++;
    pthread_mutex_lock(&watchers_mutex); watcher->in_flight--; pthread_cond_broadcast(&watchers_changed); pthread_mutex_unlock(&watchers_mutex);
  }
  return accepted;
}
typedef struct fixture_fire_job { int32_t start; uint32_t rounds; int32_t accepted; } fixture_fire_job;
static void *fixture_fire_worker(void *raw) { fixture_fire_job *job = (fixture_fire_job *)raw; for (uint32_t index = 0; index < job->rounds; index++) job->accepted += fixture_fire(job->start + (int32_t)index); return NULL; }
int32_t fixture_fire_parallel(int32_t start, uint32_t rounds) {
  fixture_fire_job jobs[2] = {{start, rounds, 0}, {start + (int32_t)rounds, rounds, 0}}; pthread_t threads[2];
  if (pthread_create(&threads[0], NULL, fixture_fire_worker, &jobs[0]) != 0) return -1;
  if (pthread_create(&threads[1], NULL, fixture_fire_worker, &jobs[1]) != 0) { pthread_join(threads[0], NULL); return -2; }
  pthread_join(threads[0], NULL); pthread_join(threads[1], NULL); return jobs[0].accepted + jobs[1].accepted;
}
void fixture_fail_next_unregister(void) { pthread_mutex_lock(&watchers_mutex); fail_next_unregister = true; pthread_mutex_unlock(&watchers_mutex); }
int32_t fixture_topic_watch_register(fixture_topic topic, fixture_event_callback callback, void *context) { for (size_t index = 0; index < 4; index++) if (!topic_watchers[index].callback) { topic_watchers[index] = (fixture_topic_watcher){topic, callback, context}; return 0; } return 81; }
int32_t fixture_topic_watch_unregister(fixture_topic topic, fixture_event_callback callback, void *context) { for (size_t index = 0; index < 4; index++) if (topic_watchers[index].topic == topic && topic_watchers[index].callback == callback && topic_watchers[index].context == context) { topic_watchers[index] = (fixture_topic_watcher){0}; return 0; } return 82; }
int32_t fixture_topic_fire(fixture_topic topic, int32_t value) { int32_t accepted = 0; for (size_t index = 0; index < 4; index++) if (topic_watchers[index].callback && topic_watchers[index].topic == topic && topic_watchers[index].callback(value, topic_watchers[index].context)) accepted++; return accepted; }
int32_t fixture_filtered_watch_register(fixture_topic topic, fixture_filter filter, fixture_event_callback callback, void *context) { if (filtered_callback) return 83; filtered_topic = topic; filtered_filter = filter; filtered_callback = callback; filtered_context = context; return 0; }
int32_t fixture_filtered_watch_unregister(fixture_topic topic, fixture_filter filter, fixture_event_callback callback, void *context) { if (topic != filtered_topic || filter.minimum != filtered_filter.minimum || filter.maximum != filtered_filter.maximum || callback != filtered_callback || context != filtered_context) return 84; filtered_callback = NULL; filtered_context = NULL; return 0; }
bool fixture_filtered_fire(fixture_topic topic, int32_t value) { return filtered_callback && topic == filtered_topic && value >= filtered_filter.minimum && value <= filtered_filter.maximum ? filtered_callback(value, filtered_context) : false; }
int32_t fixture_resource_new(fixture_resource **output) { fixture_resource *resource = (fixture_resource *)malloc(sizeof(fixture_resource)); if (!resource) return 90; resource->released = false; *output = resource; return 0; }
void fixture_resource_free(fixture_resource *resource) { resource->released = true; free(resource); }
int32_t fixture_resource_watch_register(fixture_resource *resource, fixture_topic topic, fixture_event_callback callback, void *context) { if (!resource || resource->released) return 91; if (resource_watch_callback) return 92; resource_watch_resource = resource; resource_watch_topic = topic; resource_watch_callback = callback; resource_watch_context = context; return 0; }
int32_t fixture_resource_watch_unregister(fixture_resource *resource, fixture_topic topic, fixture_event_callback callback, void *context) { if (fail_next_resource_unregister) { fail_next_resource_unregister = false; return 96; } if (resource != resource_watch_resource || topic != resource_watch_topic || callback != resource_watch_callback || context != resource_watch_context) return 93; resource_watch_resource = NULL; resource_watch_callback = NULL; resource_watch_context = NULL; return 0; }
int32_t fixture_resource_fire(fixture_resource *resource, fixture_topic topic, int32_t value, int32_t *output) { if (!resource || resource->released) return 94; if (resource != resource_watch_resource || topic != resource_watch_topic || !resource_watch_callback) { *output = 0; return 0; } *output = resource_watch_callback(value, resource_watch_context) ? 1 : 0; return 0; }
void fixture_resource_fail_next_unregister(void) { fail_next_resource_unregister = true; }
int32_t fixture_retained_watch_register(char *label, uint8_t *data, size_t data_length, fixture_event_callback callback, void *context) { if (retained_callback) return 101; if (!label || (data_length == 0 ? data != NULL : data == NULL)) return 104; retained_label = label; retained_data = data; retained_data_length = data_length; retained_callback = callback; retained_context = context; return 0; }
int32_t fixture_retained_watch_unregister(char *label, uint8_t *data, size_t data_length, fixture_event_callback callback, void *context) { if (fail_next_retained_unregister) { fail_next_retained_unregister = false; return 102; } if (label != retained_label || data != retained_data || data_length != retained_data_length || callback != retained_callback || context != retained_context) return 103; retained_label = NULL; retained_data = NULL; retained_data_length = 0; retained_callback = NULL; retained_context = NULL; return 0; }
int32_t fixture_retained_fire(void) { if (!retained_callback) return 0; int32_t value = 0; while (retained_label[value] != 0) value++; for (size_t index = 0; index < retained_data_length; index++) value += retained_data[index]; return retained_callback(value, retained_context) ? 1 : 0; }
void fixture_retained_fail_next_unregister(void) { fail_next_retained_unregister = true; }
`,
		"app/binding.otm": `import go ffi from "registeredffi.test";
function CountOnce(value: int32): Result<int32> {
  let observed: int32 = 0;
  const watch = ffi.RegisterWatch((current: int32): boolean => { observed += current; return true; })?;
  const accepted = ffi.Fire(value);
  watch.Close()?;
  return ok(observed + accepted);
}
function CountTopic(topic: ffi.Topic, value: int32): Result<int32> {
  let observed: int32 = 0;
  const watch = ffi.RegisterTopicWatch(topic, (current: int32): boolean => { observed = current; return true; })?;
  const accepted = ffi.FireTopic(topic, value);
  watch.Close()?;
  return ok(observed + accepted);
}
function CountResource(value: int32): Result<int32> {
  const resource = ffi.NewResource()?;
  let observed: int32 = 0;
  const watch = ffi.RegisterResourceWatch(resource, ffi.TopicWindow, (current: int32): boolean => { observed = current; return true; })?;
  const accepted = ffi.FireResource(resource, ffi.TopicWindow, value)?;
  watch.Close()?;
  resource.Close()?;
  return ok(observed + accepted);
}
function CountRetained(label: string, data: byte[]): Result<int32> {
  let observed: int32 = 0;
  const watch = ffi.RegisterRetainedWatch(label, data, (current: int32): boolean => { observed = current; return true; })?;
  const accepted = ffi.FireRetained();
  watch.Close()?;
  return ok(observed + accepted);
}
`,
		"cmd/main.go": `package main
import (
  "errors"
  "sync/atomic"
  "time"
  app "registeredffi.test/app"
  ffi "registeredffi.test"
)
func assert(ok bool) { if !ok { panic("registered callback assertion failed") } }
func main() {
  watch, err := ffi.RegisterWatch(nil); assert(watch == nil && errors.Is(err, ffi.ErrNilCallback))
  var firstCalls, secondCalls int32
  first, err := ffi.RegisterWatch(func(value int32) bool { firstCalls += value; return true }); assert(err == nil)
  second, err := ffi.RegisterWatch(func(value int32) bool { secondCalls += value; return false }); assert(err == nil)
  assert(ffi.Fire(3) == 1 && firstCalls == 3 && secondCalls == 3)
  assert(first.Close() == nil)
  assert(ffi.Fire(4) == 0 && firstCalls == 3 && secondCalls == 7)
  assert(first.CallbackError() == nil)
  assert(errors.Is(first.Close(), ffi.ErrClosedCallbackRegistration))
  assert(second.Close() == nil && ffi.Fire(5) == 0)
  var nilWatch *ffi.Watch
  assert(errors.Is(nilWatch.Close(), ffi.ErrClosedCallbackRegistration) && errors.Is(nilWatch.CallbackError(), ffi.ErrClosedCallbackRegistration))

  slots := make([]*ffi.Watch, 4)
  for index := range slots { slots[index], err = ffi.RegisterWatch(func(value int32) bool { return true }); assert(err == nil) }
  overflow, err := ffi.RegisterWatch(func(value int32) bool { return true }); var status *ffi.StatusError
  assert(overflow == nil && errors.As(err, &status) && status.Code == 61 && status.Function == "RegisterWatch")
  for _, slot := range slots { assert(slot.Close() == nil) }

  retryCalls := 0
  retry, err := ffi.RegisterWatch(func(value int32) bool { retryCalls++; return true }); assert(err == nil)
  ffi.FailNextUnregister(); err = retry.Close(); assert(errors.As(err, &status) && status.Code == 62 && status.Function == "Watch.Close")
  assert(ffi.Fire(1) == 1 && retryCalls == 1)
  assert(retry.Close() == nil && ffi.Fire(1) == 0)

  inputCalls, windowCalls := int32(0), int32(0)
  inputWatch, err := ffi.RegisterTopicWatch(ffi.TopicInput, func(value int32) bool { inputCalls += value; return true }); assert(err == nil)
  windowWatch, err := ffi.RegisterTopicWatch(ffi.TopicWindow, func(value int32) bool { windowCalls += value; return true }); assert(err == nil)
  assert(ffi.FireTopic(ffi.TopicInput, 3) == 1 && inputCalls == 3 && windowCalls == 0)
  assert(ffi.FireTopic(ffi.TopicWindow, 5) == 1 && inputCalls == 3 && windowCalls == 5)
  assert(inputWatch.Close() == nil && windowWatch.Close() == nil && ffi.FireTopic(ffi.TopicInput, 7) == 0)
  unknownWatch, err := ffi.RegisterTopicWatch(ffi.Topic(99), func(value int32) bool { return value == 9 }); assert(err == nil && ffi.FireTopic(ffi.Topic(99), 9) == 1 && unknownWatch.Close() == nil)
  filter := ffi.Filter{Minimum: -2, Maximum: 2}
  filtered, err := ffi.RegisterFilteredWatch(ffi.TopicInput, filter, func(value int32) bool { return true }); assert(err == nil)
  filter.Minimum = 100; filter.Maximum = 200
  assert(ffi.FireFiltered(ffi.TopicInput, -2) && ffi.FireFiltered(ffi.TopicInput, 2) && !ffi.FireFiltered(ffi.TopicInput, 3) && !ffi.FireFiltered(ffi.TopicWindow, 0))
  assert(filtered.Close() == nil)

  var nilResource *ffi.Resource
  resourceWatch, err := ffi.RegisterResourceWatch(nilResource, ffi.TopicInput, func(value int32) bool { return true }); assert(resourceWatch == nil && errors.Is(err, ffi.ErrClosedHandle))
  nilCallbackResource, err := ffi.NewResource(); assert(err == nil)
  resourceWatch, err = ffi.RegisterResourceWatch(nilCallbackResource, ffi.TopicInput, nil); assert(resourceWatch == nil && errors.Is(err, ffi.ErrNilCallback) && nilCallbackResource.Close() == nil)
  resource, err := ffi.NewResource(); assert(err == nil)
  resourceCalls := 0
  resourceWatch, err = ffi.RegisterResourceWatch(resource, ffi.TopicWindow, func(value int32) bool { resourceCalls += int(value); return true }); assert(err == nil)
  rejectedResource, err := ffi.NewResource(); assert(err == nil)
  rejectedWatch, err := ffi.RegisterResourceWatch(rejectedResource, ffi.TopicInput, func(value int32) bool { return true }); assert(rejectedWatch == nil && errors.As(err, &status) && status.Code == 92 && rejectedResource.Close() == nil)
  assert(errors.Is(resource.Close(), ffi.ErrHandleHasActiveRegistrations))
  accepted, err := ffi.FireResource(resource, ffi.TopicInput, 3); assert(accepted == 0 && err == nil && resourceCalls == 0)
  accepted, err = ffi.FireResource(resource, ffi.TopicWindow, 4); assert(accepted == 1 && err == nil && resourceCalls == 4)
  ffi.FailNextResourceUnregister(); err = resourceWatch.Close(); assert(errors.As(err, &status) && status.Code == 96 && errors.Is(resource.Close(), ffi.ErrHandleHasActiveRegistrations))
  accepted, err = ffi.FireResource(resource, ffi.TopicWindow, 5); assert(accepted == 1 && err == nil && resourceCalls == 9)
  assert(resourceWatch.Close() == nil && resource.Close() == nil)
  closedWatch, err := ffi.RegisterResourceWatch(resource, ffi.TopicInput, func(value int32) bool { return true }); assert(closedWatch == nil && errors.Is(err, ffi.ErrClosedHandle))

  retainedWatch, err := ffi.RegisterRetainedWatch("bad\x00label", []byte{1}, func(value int32) bool { return true }); assert(retainedWatch == nil && errors.Is(err, ffi.ErrEmbeddedNUL))
  retainedData := []byte{1, 2, 3}
  retainedValue := int32(-1)
  retainedWatch, err = ffi.RegisterRetainedWatch("sdl", retainedData, func(value int32) bool { retainedValue = value; return true }); assert(err == nil)
  retainedData[0] = 100
  rejectedRetained, err := ffi.RegisterRetainedWatch("other", []byte{9}, func(value int32) bool { return true }); assert(rejectedRetained == nil && errors.As(err, &status) && status.Code == 101)
  assert(ffi.FireRetained() == 1 && retainedValue == 9)
  ffi.FailNextRetainedUnregister(); err = retainedWatch.Close(); assert(errors.As(err, &status) && status.Code == 102 && ffi.FireRetained() == 1 && retainedValue == 9)
  assert(retainedWatch.Close() == nil && ffi.FireRetained() == 0)
  emptyRetained, err := ffi.RegisterRetainedWatch("", nil, func(value int32) bool { retainedValue = value; return true }); assert(err == nil && ffi.FireRetained() == 1 && retainedValue == 0 && emptyRetained.Close() == nil)

  panicCalls := 0
  panicking, err := ffi.RegisterWatch(func(value int32) bool { panicCalls++; panic("registered exploded") }); assert(err == nil)
  assert(ffi.Fire(1) == 0 && panicCalls == 1)
  var callbackPanic *ffi.CallbackPanicError
  assert(errors.As(panicking.CallbackError(), &callbackPanic) && callbackPanic.Function == "Watch" && callbackPanic.Value == "registered exploded")
  assert(ffi.Fire(2) == 0 && panicCalls == 1)
  assert(panicking.Close() == nil && errors.As(panicking.CallbackError(), &callbackPanic))

  entered := make(chan struct{}); release := make(chan struct{}); fireDone := make(chan struct{}); closeDone := make(chan error, 1)
  blocking, err := ffi.RegisterWatch(func(value int32) bool { close(entered); <-release; return true }); assert(err == nil)
  go func() { assert(ffi.Fire(9) == 1); close(fireDone) }()
  <-entered
  go func() { closeDone <- blocking.Close() }()
  select { case <-closeDone: panic("Close returned while callback was in flight"); case <-time.After(20 * time.Millisecond): }
  close(release); <-fireDone; assert(<-closeDone == nil); assert(ffi.Fire(9) == 0)

  var parallelCalls atomic.Int32
  parallel, err := ffi.RegisterWatch(func(value int32) bool { parallelCalls.Add(1); return true }); assert(err == nil)
  assert(ffi.FireParallel(0, 500) == 1000 && parallelCalls.Load() == 1000)
  assert(parallel.Close() == nil)

  total, err := app.CountOnce(42); assert(total == 43 && err == nil)
  total, err = app.CountTopic(ffi.TopicWindow, 8); assert(total == 9 && err == nil)
  total, err = app.CountResource(10); assert(total == 11 && err == nil)
  total, err = app.CountRetained("raylib", []byte{4, 5}); assert(total == 16 && err == nil)
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generatedApp, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.otm")}, "app")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("generate registered callback OnsenTamago wrapper: err=%v diagnostics=%v", err, diagnostics)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generatedApp, 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"run", "-buildvcs=false", "./cmd"}
	if os.Getenv("ONTAMA_DIFFERENTIAL_RACE") == "1" {
		arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
	}
	command := exec.Command("go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("registered callback incoming C FFI failed: %v\n%s\n%s", err, output, artifacts.Source)
	}
}

func TestIncomingCFFIThreadAffineUsesOneOSThread(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler is not available")
	}
	root := t.TempDir()
	artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion": 1,
  "package": "affineffi",
  "header": "fixture.h",
  "threadPolicy": "threadAffine",
  "handles": [{"name":"Resource", "cType":"fixture_resource", "release":"fixture_resource_free"}],
  "callbacks": [{"name":"AffineCallback", "lifetime":"callScoped", "parameters":[{"name":"value","type":"uint64"}], "result":"uint64"},{"name":"RegisteredAffineCallback", "lifetime":"registered", "parameters":[{"name":"value","type":"uint64"}], "result":"uint64"},{"name":"AffineTextCallback", "lifetime":"callScoped", "parameters":[{"name":"title","type":"copiedCString"},{"name":"data","type":"copiedBytes"}], "result":"uint64"},{"name":"AffineMutateCallback", "lifetime":"callScoped", "parameters":[{"name":"data","type":"inoutBytes"}], "result":"void"},{"name":"AffineOwnedCallback", "lifetime":"registered", "parameters":[{"name":"seed","type":"byte"}], "result":"ownedBytes"},{"name":"AffineOwnedTextCallback", "lifetime":"registered", "parameters":[{"name":"seed","type":"byte"}], "result":"ownedCString"},{"name":"AffineOwnedArrayCallback", "lifetime":"registered", "parameters":[{"name":"seed","type":"int32"}], "result":"ownedArray", "resultElement":"int32"}],
  "callbackRegistrations": [{"name":"AffineWatch", "callback":"RegisteredAffineCallback", "register":"fixture_affine_watch_register", "unregister":"fixture_affine_watch_unregister"},{"name":"AffineResourceWatch", "callback":"RegisteredAffineCallback", "parameters":[{"name":"resource","type":"Resource"}], "register":"fixture_affine_resource_watch_register", "unregister":"fixture_affine_resource_watch_unregister"},{"name":"AffineRetainedWatch", "callback":"RegisteredAffineCallback", "parameters":[{"name":"label","type":"retainedCString"},{"name":"data","type":"retainedBytes"}], "register":"fixture_affine_retained_watch_register", "unregister":"fixture_affine_retained_watch_unregister"},{"name":"AffineOwnedWatch", "callback":"AffineOwnedCallback", "register":"fixture_affine_owned_register", "unregister":"fixture_affine_owned_unregister"},{"name":"AffineOwnedTextWatch", "callback":"AffineOwnedTextCallback", "register":"fixture_affine_owned_text_register", "unregister":"fixture_affine_owned_text_unregister"},{"name":"AffineOwnedArrayWatch", "callback":"AffineOwnedArrayCallback", "register":"fixture_affine_owned_array_register", "unregister":"fixture_affine_owned_array_unregister"}],
  "functions": [
    {"name":"ThreadToken", "symbol":"fixture_thread_token", "parameters":[], "result":"uint64", "convention":"direct"},
    {"name":"SerializedProbe", "symbol":"fixture_serialized_probe", "parameters":[], "result":"int32", "convention":"direct"},
    {"name":"Record", "symbol":"fixture_record", "parameters":[], "result":"void", "convention":"direct"},
    {"name":"TitleLength", "symbol":"fixture_title_length", "parameters":[{"name":"title","type":"cstring"}], "result":"cInt32", "convention":"direct"},
    {"name":"SetTitle", "symbol":"fixture_set_title", "parameters":[{"name":"title","type":"cstring"}], "result":"void", "convention":"status"},
    {"name":"MakeThreadName", "symbol":"fixture_make_thread_name", "parameters":[], "result":"ownedCString", "resultRelease":"fixture_string_free", "convention":"statusOut"},
    {"name":"NewResource", "symbol":"fixture_resource_new", "parameters":[], "result":"Resource", "convention":"statusOut"},
    {"name":"ResourceValue", "symbol":"fixture_resource_value", "parameters":[{"name":"resource","type":"Resource"}], "result":"uint64", "convention":"statusOut"},
    {"name":"Draw", "symbol":"fixture_resource_draw", "parameters":[{"name":"resource","type":"Resource"}], "result":"void", "convention":"status"},
    {"name":"FailStatus", "symbol":"fixture_fail_status", "parameters":[], "result":"void", "convention":"status"},
    {"name":"ReleaseMismatch", "symbol":"fixture_release_mismatch", "parameters":[], "result":"boolean", "convention":"direct"},
    {"name":"Invoke", "symbol":"fixture_invoke", "parameters":[{"name":"value","type":"uint64"},{"name":"callback","type":"AffineCallback"}], "result":"uint64", "convention":"direct"},
    {"name":"EmitAffineText", "symbol":"fixture_emit_affine_text", "parameters":[{"name":"mode","type":"byte"},{"name":"callback","type":"AffineTextCallback"}], "result":"uint64", "convention":"direct"},
    {"name":"MutateAffine", "symbol":"fixture_mutate_affine", "parameters":[{"name":"mode","type":"byte"},{"name":"callback","type":"AffineMutateCallback"}], "result":"void", "convention":"direct"},
    {"name":"AffineMutationChecksum", "symbol":"fixture_affine_mutation_checksum", "parameters":[], "result":"int32", "convention":"direct"},
    {"name":"FireAffineWatch", "symbol":"fixture_affine_watch_fire", "parameters":[{"name":"value","type":"uint64"}], "result":"uint64", "convention":"direct"},
    {"name":"FireAffineResourceWatch", "symbol":"fixture_affine_resource_watch_fire", "parameters":[{"name":"value","type":"uint64"}], "result":"uint64", "convention":"direct"},
    {"name":"FireAffineRetainedWatch", "symbol":"fixture_affine_retained_watch_fire", "parameters":[], "result":"uint64", "convention":"direct"},
    {"name":"FailNextAffineRetainedUnregister", "symbol":"fixture_affine_retained_fail_next_unregister", "parameters":[], "result":"void", "convention":"direct"}
    ,{"name":"FireAffineOwned", "symbol":"fixture_affine_owned_fire", "parameters":[{"name":"seed","type":"byte"}], "result":"int32", "convention":"direct"}
    ,{"name":"HoldAffineOwned", "symbol":"fixture_affine_owned_hold", "parameters":[{"name":"seed","type":"byte"}], "result":"int32", "convention":"direct"}
    ,{"name":"ReleaseHeldAffineOwned", "symbol":"fixture_affine_owned_release_held", "parameters":[], "result":"int32", "convention":"direct"}
    ,{"name":"AffineOwnedReleaseCount", "symbol":"fixture_affine_owned_release_count", "parameters":[], "result":"int32", "convention":"direct"}
    ,{"name":"AffineOwnedThreadMismatch", "symbol":"fixture_affine_owned_thread_mismatch", "parameters":[], "result":"boolean", "convention":"direct"}
    ,{"name":"FireAffineOwnedText", "symbol":"fixture_affine_owned_text_fire", "parameters":[{"name":"seed","type":"byte"}], "result":"int32", "convention":"direct"}
    ,{"name":"HoldAffineOwnedText", "symbol":"fixture_affine_owned_text_hold", "parameters":[{"name":"seed","type":"byte"}], "result":"int32", "convention":"direct"}
    ,{"name":"ReleaseHeldAffineOwnedText", "symbol":"fixture_affine_owned_text_release_held", "parameters":[], "result":"int32", "convention":"direct"}
    ,{"name":"AffineOwnedTextReleaseCount", "symbol":"fixture_affine_owned_text_release_count", "parameters":[], "result":"int32", "convention":"direct"}
    ,{"name":"AffineOwnedTextThreadMismatch", "symbol":"fixture_affine_owned_text_thread_mismatch", "parameters":[], "result":"boolean", "convention":"direct"}
    ,{"name":"FireAffineOwnedArray", "symbol":"fixture_affine_owned_array_fire", "parameters":[{"name":"seed","type":"int32"}], "result":"int32", "convention":"direct"}
    ,{"name":"HoldAffineOwnedArray", "symbol":"fixture_affine_owned_array_hold", "parameters":[{"name":"seed","type":"int32"}], "result":"int32", "convention":"direct"}
    ,{"name":"ReleaseHeldAffineOwnedArray", "symbol":"fixture_affine_owned_array_release_held", "parameters":[], "result":"int32", "convention":"direct"}
    ,{"name":"AffineOwnedArrayReleaseCount", "symbol":"fixture_affine_owned_array_release_count", "parameters":[], "result":"int32", "convention":"direct"}
    ,{"name":"AffineOwnedArrayThreadMismatch", "symbol":"fixture_affine_owned_array_thread_mismatch", "parameters":[], "result":"boolean", "convention":"direct"}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(artifacts.Source)
	for _, want := range []string{"runtime.LockOSThread()", "func ontamaCFFIDo(call func())", "func ThreadToken() uint64", "func ontamaCFFIRawThreadToken() uint64", "ontamaCFFIDo(func()", "ontamaCFFIRawClose", "func MakeThreadName() (string, error)", "func ontamaCFFIRawMakeThreadName() (string, error)", "defer C.fixture_string_free(output)", "func Invoke(value uint64, callback AffineCallback) (uint64, error)", "func ontamaCFFIRawInvoke", "func EmitAffineText(mode byte, callback AffineTextCallback) (uint64, error)", "func ontamaCFFIRawEmitAffineText", "func MutateAffine(mode byte, callback AffineMutateCallback) error", "func ontamaCFFIRawMutateAffine", "func RegisterAffineWatch", "ontamaCFFIRawRegisterAffineWatch", "func RegisterAffineResourceWatch(resource *Resource", "ontamaCFFIRawRegisterAffineResourceWatch", "func RegisterAffineRetainedWatch(label string, data []byte", "ontamaCFFIRawRegisterAffineRetainedWatch", "func RegisterAffineOwnedWatch(callback AffineOwnedCallback)", "ontamaCFFIRawRegisterAffineOwnedWatch", "func RegisterAffineOwnedTextWatch(callback AffineOwnedTextCallback)", "ontamaCFFIRawRegisterAffineOwnedTextWatch", "func RegisterAffineOwnedArrayWatch(callback AffineOwnedArrayCallback)", "ontamaCFFIRawRegisterAffineOwnedArrayWatch"} {
		if !strings.Contains(generated, want) {
			t.Errorf("thread-affine generated C FFI does not contain %q:\n%s", want, generated)
		}
	}
	files := map[string]string{
		"go.mod":           "module affineffi.test\n\ngo 1.23\n",
		"generated_ffi.go": generated,
		"fixture.h": `#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef struct fixture_resource fixture_resource;
uint64_t fixture_thread_token(void);
int32_t fixture_serialized_probe(void);
void fixture_record(void);
int fixture_title_length(const char *title);
int32_t fixture_set_title(const char *title);
int32_t fixture_make_thread_name(char **output);
void fixture_string_free(char *value);
int32_t fixture_resource_new(fixture_resource **output);
int32_t fixture_resource_value(fixture_resource *resource, uint64_t *output);
int32_t fixture_resource_draw(fixture_resource *resource);
int32_t fixture_fail_status(void);
void fixture_resource_free(fixture_resource *resource);
bool fixture_release_mismatch(void);
typedef uint64_t (*fixture_affine_callback)(uint64_t value, void *context);
uint64_t fixture_invoke(uint64_t value, fixture_affine_callback callback, void *context);
typedef uint64_t (*fixture_affine_text_callback)(const char *title, const uint8_t *data, size_t data_length, void *context);
uint64_t fixture_emit_affine_text(uint8_t mode, fixture_affine_text_callback callback, void *context);
typedef void (*fixture_affine_mutate_callback)(uint8_t *data, size_t data_length, void *context);
void fixture_mutate_affine(uint8_t mode, fixture_affine_mutate_callback callback, void *context);
int32_t fixture_affine_mutation_checksum(void);
typedef uint64_t (*fixture_registered_affine_callback)(uint64_t value, void *context);
int32_t fixture_affine_watch_register(fixture_registered_affine_callback callback, void *context);
int32_t fixture_affine_watch_unregister(fixture_registered_affine_callback callback, void *context);
uint64_t fixture_affine_watch_fire(uint64_t value);
int32_t fixture_affine_resource_watch_register(fixture_resource *resource, fixture_registered_affine_callback callback, void *context);
int32_t fixture_affine_resource_watch_unregister(fixture_resource *resource, fixture_registered_affine_callback callback, void *context);
uint64_t fixture_affine_resource_watch_fire(uint64_t value);
int32_t fixture_affine_retained_watch_register(char *label, uint8_t *data, size_t data_length, fixture_registered_affine_callback callback, void *context);
int32_t fixture_affine_retained_watch_unregister(char *label, uint8_t *data, size_t data_length, fixture_registered_affine_callback callback, void *context);
uint64_t fixture_affine_retained_watch_fire(void);
void fixture_affine_retained_fail_next_unregister(void);
typedef uint8_t *(*fixture_affine_owned_callback)(uint8_t seed, size_t *output_length, void *context);
typedef void (*fixture_affine_owned_release)(uint8_t *data);
int32_t fixture_affine_owned_register(fixture_affine_owned_callback callback, fixture_affine_owned_release release, void *context);
int32_t fixture_affine_owned_unregister(fixture_affine_owned_callback callback, fixture_affine_owned_release release, void *context);
int32_t fixture_affine_owned_fire(uint8_t seed);
int32_t fixture_affine_owned_hold(uint8_t seed);
int32_t fixture_affine_owned_release_held(void);
int32_t fixture_affine_owned_release_count(void);
bool fixture_affine_owned_thread_mismatch(void);
typedef char *(*fixture_affine_owned_text_callback)(uint8_t seed, void *context);
typedef void (*fixture_affine_owned_text_release)(char *text);
int32_t fixture_affine_owned_text_register(fixture_affine_owned_text_callback callback, fixture_affine_owned_text_release release, void *context);
int32_t fixture_affine_owned_text_unregister(fixture_affine_owned_text_callback callback, fixture_affine_owned_text_release release, void *context);
int32_t fixture_affine_owned_text_fire(uint8_t seed);
int32_t fixture_affine_owned_text_hold(uint8_t seed);
int32_t fixture_affine_owned_text_release_held(void);
int32_t fixture_affine_owned_text_release_count(void);
bool fixture_affine_owned_text_thread_mismatch(void);
typedef int32_t *(*fixture_affine_owned_array_callback)(int32_t seed, size_t *output_length, void *context);
typedef void (*fixture_affine_owned_array_release)(int32_t *values);
int32_t fixture_affine_owned_array_register(fixture_affine_owned_array_callback callback, fixture_affine_owned_array_release release, void *context);
int32_t fixture_affine_owned_array_unregister(fixture_affine_owned_array_callback callback, fixture_affine_owned_array_release release, void *context);
int32_t fixture_affine_owned_array_fire(int32_t seed);
int32_t fixture_affine_owned_array_hold(int32_t seed);
int32_t fixture_affine_owned_array_release_held(void);
int32_t fixture_affine_owned_array_release_count(void);
bool fixture_affine_owned_array_thread_mismatch(void);
`,
		"fixture.c": `#include "fixture.h"
#include <stdlib.h>
static _Thread_local int thread_marker;
static int active;
static int records;
static bool release_mismatch;
static char *owned_string;
static uint64_t owned_string_thread;
static fixture_registered_affine_callback affine_watch_callback;
static void *affine_watch_context;
static uint64_t affine_watch_thread;
static fixture_resource *affine_resource_watch_resource;
static fixture_registered_affine_callback affine_resource_watch_callback;
static void *affine_resource_watch_context;
static uint64_t affine_resource_watch_thread;
static char *affine_retained_label;
static uint8_t *affine_retained_data;
static size_t affine_retained_data_length;
static fixture_registered_affine_callback affine_retained_callback;
static void *affine_retained_context;
static uint64_t affine_retained_thread;
static bool fail_next_affine_retained_unregister;
static uint64_t affine_text_thread;
static uint8_t affine_mutation_data[2]; static int32_t affine_mutation_checksum;
static fixture_affine_owned_callback affine_owned_callback; static fixture_affine_owned_release affine_owned_release; static void *affine_owned_context; static uint64_t affine_owned_thread; static uint8_t *affine_owned_held; static size_t affine_owned_held_length; static int32_t affine_owned_release_count; static bool affine_owned_thread_mismatch;
static fixture_affine_owned_text_callback affine_owned_text_callback; static fixture_affine_owned_text_release affine_owned_text_release; static void *affine_owned_text_context; static uint64_t affine_owned_text_thread; static char *affine_owned_text_held; static int32_t affine_owned_text_release_count; static bool affine_owned_text_thread_mismatch;
static fixture_affine_owned_array_callback affine_owned_array_callback; static fixture_affine_owned_array_release affine_owned_array_release; static void *affine_owned_array_context; static uint64_t affine_owned_array_thread; static int32_t *affine_owned_array_held; static size_t affine_owned_array_held_length; static int32_t affine_owned_array_release_count; static bool affine_owned_array_thread_mismatch;
struct fixture_resource { uint64_t thread; };
uint64_t fixture_thread_token(void) { return (uint64_t)(uintptr_t)&thread_marker; }
int32_t fixture_serialized_probe(void) {
  if (__sync_fetch_and_add(&active, 1) != 0) { __sync_fetch_and_sub(&active, 1); return -1; }
  for (volatile int index = 0; index < 100000; index++) {}
  __sync_fetch_and_sub(&active, 1);
  return 0;
}
void fixture_record(void) { records++; }
int fixture_title_length(const char *title) { int length = 0; while (title[length] != 0) length++; return length; }
int32_t fixture_set_title(const char *title) { records += fixture_title_length(title) > 0 ? 1 : 0; return 0; }
int32_t fixture_make_thread_name(char **output) {
  owned_string = (char *)malloc(7);
  if (!owned_string) return 39;
  const char value[7] = "affine";
  for (int index = 0; index < 7; index++) owned_string[index] = value[index];
  owned_string_thread = fixture_thread_token();
  *output = owned_string;
  return 0;
}
void fixture_string_free(char *value) {
  if (value != owned_string || fixture_thread_token() != owned_string_thread) release_mismatch = true;
  free(value);
  owned_string = NULL;
}
int32_t fixture_resource_new(fixture_resource **output) {
  fixture_resource *resource = (fixture_resource *)malloc(sizeof(fixture_resource));
  if (!resource) return 40;
  resource->thread = fixture_thread_token();
  *output = resource;
  return 0;
}
int32_t fixture_resource_value(fixture_resource *resource, uint64_t *output) {
  if (!resource || resource->thread != fixture_thread_token()) return 41;
  *output = resource->thread;
  return 0;
}
int32_t fixture_resource_draw(fixture_resource *resource) {
  if (!resource || resource->thread != fixture_thread_token()) return 41;
  return 0;
}
int32_t fixture_fail_status(void) { return 55; }
void fixture_resource_free(fixture_resource *resource) {
  if (resource && resource->thread != fixture_thread_token()) release_mismatch = true;
  free(resource);
}
bool fixture_release_mismatch(void) { return release_mismatch; }
uint64_t fixture_invoke(uint64_t value, fixture_affine_callback callback, void *context) { return callback(value, context); }
uint64_t fixture_emit_affine_text(uint8_t mode, fixture_affine_text_callback callback, void *context) { static char title[] = "affine"; static uint8_t data[] = {4, 5}; if (affine_text_thread == 0) affine_text_thread = fixture_thread_token(); if (fixture_thread_token() != affine_text_thread) return UINT64_MAX; if (mode == 1) return callback(NULL, data, 2, context); title[0] = 'a'; data[0] = 4; uint64_t value = callback(title, data, 2, context); title[0] = 'X'; data[0] = 99; return value; }
void fixture_mutate_affine(uint8_t mode, fixture_affine_mutate_callback callback, void *context) { affine_mutation_data[0] = 7; affine_mutation_data[1] = 8; if (mode == 1) callback(NULL, 2, context); else callback(affine_mutation_data, 2, context); affine_mutation_checksum = affine_mutation_data[0] + affine_mutation_data[1]; }
int32_t fixture_affine_mutation_checksum(void) { return affine_mutation_checksum; }
int32_t fixture_affine_watch_register(fixture_registered_affine_callback callback, void *context) { if (affine_watch_callback) return 71; affine_watch_callback = callback; affine_watch_context = context; affine_watch_thread = fixture_thread_token(); return 0; }
int32_t fixture_affine_watch_unregister(fixture_registered_affine_callback callback, void *context) { if (fixture_thread_token() != affine_watch_thread) return 72; if (callback != affine_watch_callback || context != affine_watch_context) return 73; affine_watch_callback = NULL; affine_watch_context = NULL; return 0; }
uint64_t fixture_affine_watch_fire(uint64_t value) { if (!affine_watch_callback || fixture_thread_token() != affine_watch_thread) return 0; return affine_watch_callback(value, affine_watch_context); }
int32_t fixture_affine_resource_watch_register(fixture_resource *resource, fixture_registered_affine_callback callback, void *context) { if (!resource || resource->thread != fixture_thread_token() || affine_resource_watch_callback) return 74; affine_resource_watch_resource = resource; affine_resource_watch_callback = callback; affine_resource_watch_context = context; affine_resource_watch_thread = fixture_thread_token(); return 0; }
int32_t fixture_affine_resource_watch_unregister(fixture_resource *resource, fixture_registered_affine_callback callback, void *context) { if (fixture_thread_token() != affine_resource_watch_thread || resource != affine_resource_watch_resource || callback != affine_resource_watch_callback || context != affine_resource_watch_context) return 75; affine_resource_watch_resource = NULL; affine_resource_watch_callback = NULL; affine_resource_watch_context = NULL; return 0; }
uint64_t fixture_affine_resource_watch_fire(uint64_t value) { if (!affine_resource_watch_callback || fixture_thread_token() != affine_resource_watch_thread) return 0; return affine_resource_watch_callback(value, affine_resource_watch_context); }
int32_t fixture_affine_retained_watch_register(char *label, uint8_t *data, size_t data_length, fixture_registered_affine_callback callback, void *context) { if (affine_retained_callback) return 77; if (!label || (data_length == 0 ? data != NULL : data == NULL)) return 78; affine_retained_label = label; affine_retained_data = data; affine_retained_data_length = data_length; affine_retained_callback = callback; affine_retained_context = context; affine_retained_thread = fixture_thread_token(); return 0; }
int32_t fixture_affine_retained_watch_unregister(char *label, uint8_t *data, size_t data_length, fixture_registered_affine_callback callback, void *context) { if (fixture_thread_token() != affine_retained_thread) return 79; if (fail_next_affine_retained_unregister) { fail_next_affine_retained_unregister = false; return 80; } if (label != affine_retained_label || data != affine_retained_data || data_length != affine_retained_data_length || callback != affine_retained_callback || context != affine_retained_context) return 81; affine_retained_label = NULL; affine_retained_data = NULL; affine_retained_data_length = 0; affine_retained_callback = NULL; affine_retained_context = NULL; return 0; }
uint64_t fixture_affine_retained_watch_fire(void) { if (!affine_retained_callback || fixture_thread_token() != affine_retained_thread) return 0; uint64_t value = 0; while (affine_retained_label[value] != 0) value++; for (size_t index = 0; index < affine_retained_data_length; index++) value += affine_retained_data[index]; return affine_retained_callback(value, affine_retained_context); }
void fixture_affine_retained_fail_next_unregister(void) { fail_next_affine_retained_unregister = true; }
int32_t fixture_affine_owned_register(fixture_affine_owned_callback callback, fixture_affine_owned_release release, void *context) { if (affine_owned_callback) return 83; affine_owned_callback = callback; affine_owned_release = release; affine_owned_context = context; affine_owned_thread = fixture_thread_token(); return 0; }
int32_t fixture_affine_owned_unregister(fixture_affine_owned_callback callback, fixture_affine_owned_release release, void *context) { if (fixture_thread_token() != affine_owned_thread || callback != affine_owned_callback || release != affine_owned_release || context != affine_owned_context) return 84; affine_owned_callback = NULL; affine_owned_context = NULL; return 0; }
static int32_t fixture_affine_owned_sum(uint8_t *data, size_t length) { int32_t sum = 0; for (size_t index = 0; index < length; index++) sum += data[index]; return sum; }
int32_t fixture_affine_owned_fire(uint8_t seed) { if (!affine_owned_callback || fixture_thread_token() != affine_owned_thread) return -1; size_t length = 0; uint8_t *data = affine_owned_callback(seed, &length, affine_owned_context); int32_t sum = fixture_affine_owned_sum(data, length); if (data) { if (fixture_thread_token() != affine_owned_thread) affine_owned_thread_mismatch = true; affine_owned_release(data); affine_owned_release_count++; } return sum; }
int32_t fixture_affine_owned_hold(uint8_t seed) { if (!affine_owned_callback || affine_owned_held) return -1; affine_owned_held = affine_owned_callback(seed, &affine_owned_held_length, affine_owned_context); return (int32_t)affine_owned_held_length; }
int32_t fixture_affine_owned_release_held(void) { if (!affine_owned_held) return 0; if (fixture_thread_token() != affine_owned_thread) affine_owned_thread_mismatch = true; int32_t sum = fixture_affine_owned_sum(affine_owned_held, affine_owned_held_length); affine_owned_release(affine_owned_held); affine_owned_release_count++; affine_owned_held = NULL; affine_owned_held_length = 0; return sum; }
int32_t fixture_affine_owned_release_count(void) { return affine_owned_release_count; }
bool fixture_affine_owned_thread_mismatch(void) { return affine_owned_thread_mismatch; }
int32_t fixture_affine_owned_text_register(fixture_affine_owned_text_callback callback, fixture_affine_owned_text_release release, void *context) { if (affine_owned_text_callback) return 85; affine_owned_text_callback = callback; affine_owned_text_release = release; affine_owned_text_context = context; affine_owned_text_thread = fixture_thread_token(); return 0; }
int32_t fixture_affine_owned_text_unregister(fixture_affine_owned_text_callback callback, fixture_affine_owned_text_release release, void *context) { if (fixture_thread_token() != affine_owned_text_thread || callback != affine_owned_text_callback || release != affine_owned_text_release || context != affine_owned_text_context) return 86; affine_owned_text_callback = NULL; affine_owned_text_context = NULL; return 0; }
int32_t fixture_affine_owned_text_fire(uint8_t seed) { if (!affine_owned_text_callback || fixture_thread_token() != affine_owned_text_thread) return -1; char *text = affine_owned_text_callback(seed, affine_owned_text_context); if (!text) return 0; if (fixture_thread_token() != affine_owned_text_thread) affine_owned_text_thread_mismatch = true; int32_t length = fixture_title_length(text); affine_owned_text_release(text); affine_owned_text_release_count++; return length; }
int32_t fixture_affine_owned_text_hold(uint8_t seed) { if (!affine_owned_text_callback || affine_owned_text_held) return -1; affine_owned_text_held = affine_owned_text_callback(seed, affine_owned_text_context); return affine_owned_text_held ? fixture_title_length(affine_owned_text_held) : -2; }
int32_t fixture_affine_owned_text_release_held(void) { if (!affine_owned_text_held) return 0; if (fixture_thread_token() != affine_owned_text_thread) affine_owned_text_thread_mismatch = true; int32_t length = fixture_title_length(affine_owned_text_held); affine_owned_text_release(affine_owned_text_held); affine_owned_text_release_count++; affine_owned_text_held = NULL; return length; }
int32_t fixture_affine_owned_text_release_count(void) { return affine_owned_text_release_count; }
bool fixture_affine_owned_text_thread_mismatch(void) { return affine_owned_text_thread_mismatch; }
int32_t fixture_affine_owned_array_register(fixture_affine_owned_array_callback callback, fixture_affine_owned_array_release release, void *context) { if (affine_owned_array_callback) return 87; affine_owned_array_callback = callback; affine_owned_array_release = release; affine_owned_array_context = context; affine_owned_array_thread = fixture_thread_token(); return 0; }
int32_t fixture_affine_owned_array_unregister(fixture_affine_owned_array_callback callback, fixture_affine_owned_array_release release, void *context) { if (fixture_thread_token() != affine_owned_array_thread || callback != affine_owned_array_callback || release != affine_owned_array_release || context != affine_owned_array_context) return 88; affine_owned_array_callback = NULL; affine_owned_array_context = NULL; return 0; }
static int32_t fixture_affine_owned_array_sum(int32_t *values, size_t length) { int32_t sum = 0; for (size_t index = 0; index < length; index++) sum += values[index]; return sum; }
int32_t fixture_affine_owned_array_fire(int32_t seed) { if (!affine_owned_array_callback || fixture_thread_token() != affine_owned_array_thread) return -1; size_t length = 0; int32_t *values = affine_owned_array_callback(seed, &length, affine_owned_array_context); int32_t sum = fixture_affine_owned_array_sum(values, length); if (values) { if (fixture_thread_token() != affine_owned_array_thread) affine_owned_array_thread_mismatch = true; affine_owned_array_release(values); affine_owned_array_release_count++; } return sum; }
int32_t fixture_affine_owned_array_hold(int32_t seed) { if (!affine_owned_array_callback || affine_owned_array_held) return -1; affine_owned_array_held = affine_owned_array_callback(seed, &affine_owned_array_held_length, affine_owned_array_context); return (int32_t)affine_owned_array_held_length; }
int32_t fixture_affine_owned_array_release_held(void) { if (!affine_owned_array_held) return 0; if (fixture_thread_token() != affine_owned_array_thread) affine_owned_array_thread_mismatch = true; int32_t sum = fixture_affine_owned_array_sum(affine_owned_array_held, affine_owned_array_held_length); affine_owned_array_release(affine_owned_array_held); affine_owned_array_release_count++; affine_owned_array_held = NULL; affine_owned_array_held_length = 0; return sum; }
int32_t fixture_affine_owned_array_release_count(void) { return affine_owned_array_release_count; }
bool fixture_affine_owned_array_thread_mismatch(void) { return affine_owned_array_thread_mismatch; }
`,
		"app/binding.otm": `import go ffi from "affineffi.test";
function Token(): uint64 { return ffi.ThreadToken(); }
function TitleSize(title: string): Result<int32> {
  const size = ffi.TitleLength(title)?;
  return ok(size);
}
function ThreadName(): Result<string> {
  const name = ffi.MakeThreadName()?;
  return ok(name);
}
function Draw(resource: *ffi.Resource): Result<void> {
  ffi.Draw(resource)?;
  return ok();
}
class Renderer {
  constructor(private resource: *ffi.Resource) {}
  public static function create(): Result<Renderer> {
    const resource = ffi.NewResource()?;
    return ok(new Renderer(resource));
  }
  public function draw(): Result<void> {
    ffi.Draw(this.resource)?;
    return ok();
  }
  public function close(): Result<void> {
    this.resource.Close()?;
    return ok();
  }
}
function ExerciseRenderer(): Result<void> {
  const renderer = Renderer.create()?;
  renderer.draw()?;
  renderer.close()?;
  return ok();
}
`,
		"cmd/main.go": `package main
import (
  "errors"
  "sync"
  app "affineffi.test/app"
  ffi "affineffi.test"
)
func assert(ok bool) { if !ok { panic("thread-affine FFI assertion failed") } }
func main() {
  first := app.Token(); assert(first != 0)
  var wait sync.WaitGroup
  tokens := make(chan uint64, 64)
  for index := 0; index < 64; index++ {
    wait.Add(1)
    go func() { defer wait.Done(); assert(ffi.SerializedProbe() == 0); tokens <- ffi.ThreadToken() }()
  }
  wait.Wait(); close(tokens)
  for token := range tokens { assert(token == first) }
  ffi.Record()
  assert(ffi.SetTitle("raylib") == nil)
  size, err := app.TitleSize("温泉"); assert(size == 6 && err == nil)
  size, err = ffi.TitleLength("bad\x00title"); assert(size == 0 && errors.Is(err, ffi.ErrEmbeddedNUL))
  name, err := app.ThreadName(); assert(name == "affine" && err == nil)
  resource, err := ffi.NewResource(); assert(err == nil)
  assert(app.Draw(resource) == nil)
  token, err := ffi.ResourceValue(resource); assert(token == first && err == nil)
  var status *ffi.StatusError
  err = ffi.FailStatus(); assert(errors.As(err, &status) && status.Code == 55 && status.Function == "FailStatus")
  invoked, err := ffi.Invoke(41, func(value uint64) uint64 { return value + 1 }); assert(invoked == 42 && err == nil)
  invoked, err = ffi.Invoke(1, func(value uint64) uint64 { panic("affine callback panic") }); var callbackPanic *ffi.CallbackPanicError; assert(invoked == 0 && errors.As(err, &callbackPanic) && callbackPanic.Function == "Invoke")
  affineTitle := ""; var affineData []byte
  affineText, err := ffi.EmitAffineText(0, func(title string, data []byte) uint64 { affineTitle = title; affineData = data; return uint64(len(title) + int(data[0]) + int(data[1])) }); assert(affineText == 15 && err == nil && affineTitle == "affine" && affineData[0] == 4 && affineData[1] == 5)
  affineTextCalls := 0; var inputError *ffi.CallbackInputError
  affineText, err = ffi.EmitAffineText(1, func(title string, data []byte) uint64 { affineTextCalls++; return 1 }); assert(affineText == 0 && affineTextCalls == 0 && errors.As(err, &inputError) && inputError.Function == "EmitAffineText" && inputError.Parameter == "title")
  err = ffi.MutateAffine(0, func(data []byte) { data[0] = 34 }); assert(err == nil && ffi.AffineMutationChecksum() == 42)
  affineMutationCalls := 0; err = ffi.MutateAffine(1, func(data []byte) { affineMutationCalls++ }); assert(affineMutationCalls == 0 && errors.As(err, &inputError) && inputError.Function == "MutateAffine" && ffi.AffineMutationChecksum() == 15)
  affineWatch, err := ffi.RegisterAffineWatch(func(value uint64) uint64 { return value + 2 }); assert(err == nil && ffi.FireAffineWatch(40) == 42)
  assert(affineWatch.CallbackError() == nil && affineWatch.Close() == nil && ffi.FireAffineWatch(40) == 0)
  affineResource, err := ffi.NewResource(); assert(err == nil)
  affineResourceWatch, err := ffi.RegisterAffineResourceWatch(affineResource, func(value uint64) uint64 { return value + 3 }); assert(err == nil && ffi.FireAffineResourceWatch(39) == 42)
  assert(errors.Is(affineResource.Close(), ffi.ErrHandleHasActiveRegistrations))
  assert(affineResourceWatch.Close() == nil && ffi.FireAffineResourceWatch(39) == 0 && affineResource.Close() == nil)
  retainedData := []byte{2, 4}
  affineRetainedWatch, err := ffi.RegisterAffineRetainedWatch("gpu", retainedData, func(value uint64) uint64 { return value + 1 }); assert(err == nil)
  retainedData[0] = 100
  rejectedRetainedWatch, err := ffi.RegisterAffineRetainedWatch("busy", []byte{9}, func(value uint64) uint64 { return value }); assert(rejectedRetainedWatch == nil && errors.As(err, &status) && status.Code == 77)
  assert(ffi.FireAffineRetainedWatch() == 10)
  ffi.FailNextAffineRetainedUnregister(); err = affineRetainedWatch.Close(); assert(errors.As(err, &status) && status.Code == 80 && ffi.FireAffineRetainedWatch() == 10)
  assert(affineRetainedWatch.Close() == nil && ffi.FireAffineRetainedWatch() == 0)
  emptyRetainedWatch, err := ffi.RegisterAffineRetainedWatch("", nil, func(value uint64) uint64 { return value + 1 }); assert(err == nil && ffi.FireAffineRetainedWatch() == 1 && emptyRetainedWatch.Close() == nil)
  embeddedNULWatch, err := ffi.RegisterAffineRetainedWatch("bad\x00label", nil, func(value uint64) uint64 { return value }); assert(embeddedNULWatch == nil && errors.Is(err, ffi.ErrEmbeddedNUL))
  affineOwned, err := ffi.RegisterAffineOwnedWatch(func(seed byte) []byte { return []byte{seed, seed + 1} }); assert(err == nil && ffi.FireAffineOwned(20) == 41 && ffi.AffineOwnedReleaseCount() == 1)
  assert(ffi.HoldAffineOwned(30) == 2 && affineOwned.Close() == nil && ffi.ReleaseHeldAffineOwned() == 61 && ffi.AffineOwnedReleaseCount() == 2 && !ffi.AffineOwnedThreadMismatch())
  affineOwnedText, err := ffi.RegisterAffineOwnedTextWatch(func(seed byte) string { return string([]byte{seed, seed + 1}) }); assert(err == nil && ffi.FireAffineOwnedText(65) == 2 && ffi.AffineOwnedTextReleaseCount() == 1)
  assert(ffi.HoldAffineOwnedText(67) == 2 && affineOwnedText.Close() == nil && ffi.ReleaseHeldAffineOwnedText() == 2 && ffi.AffineOwnedTextReleaseCount() == 2 && !ffi.AffineOwnedTextThreadMismatch())
  affineOwnedArray, err := ffi.RegisterAffineOwnedArrayWatch(func(seed int32) []int32 { return []int32{seed, seed + 1} }); assert(err == nil && ffi.FireAffineOwnedArray(20) == 41 && ffi.AffineOwnedArrayReleaseCount() == 1)
  assert(ffi.HoldAffineOwnedArray(30) == 2 && affineOwnedArray.Close() == nil && ffi.ReleaseHeldAffineOwnedArray() == 61 && ffi.AffineOwnedArrayReleaseCount() == 2 && !ffi.AffineOwnedArrayThreadMismatch())
  assert(resource.Close() == nil)
  assert(errors.Is(ffi.Draw(resource), ffi.ErrClosedHandle))
  assert(errors.Is(ffi.Draw(nil), ffi.ErrClosedHandle))
  assert(errors.Is(resource.Close(), ffi.ErrClosedHandle))
  assert(!ffi.ReleaseMismatch())
  assert(app.ExerciseRenderer() == nil)
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generatedApp, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.otm")}, "app")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("generate thread-affine OnsenTamago wrapper: err=%v diagnostics=%v", err, diagnostics)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generatedApp, 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"run", "-buildvcs=false", "./cmd"}
	if os.Getenv("ONTAMA_DIFFERENTIAL_RACE") == "1" {
		arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
	}
	command := exec.Command("go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("thread-affine incoming C FFI failed: %v\n%s\n%s", err, output, artifacts.Source)
	}
}

func TestIncomingCFFIRaylibStyleLoadUnloadShim(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler is not available")
	}
	root := t.TempDir()
	artifacts, err := GenerateCFFI([]byte(`{
  "schemaVersion":1,
  "package":"raylibshimffi",
  "header":"fixture.h",
  "threadPolicy":"serialized",
  "callbacks":[{"name":"LoadFileDataCallback","lifetime":"registered","parameters":[{"name":"path","type":"copiedCString"}],"result":"ownedBytes"}],
  "callbackRegistrations":[{"name":"FileDataHooks","callback":"LoadFileDataCallback","register":"shim_file_data_register","unregister":"shim_file_data_unregister"}],
  "functions":[
    {"name":"RaylibLoadChecksum","symbol":"fake_raylib_load_checksum","parameters":[{"name":"path","type":"cstring"}],"result":"int32","convention":"direct"},
    {"name":"RaylibHoldFile","symbol":"fake_raylib_hold_file","parameters":[{"name":"path","type":"cstring"}],"result":"int32","convention":"direct"},
    {"name":"RaylibReleaseHeldFile","symbol":"fake_raylib_release_held_file","parameters":[],"result":"int32","convention":"direct"},
    {"name":"RaylibUnloadCount","symbol":"fake_raylib_unload_count","parameters":[],"result":"int32","convention":"direct"}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(artifacts.Source)
	for _, want := range []string{
		"type LoadFileDataCallback func(path string) []byte",
		"typedef uint8_t * (*ontama_cffi_callback_LoadFileDataCallback_fn)(const char *value0, size_t *output_length, void *context)",
		"shim_file_data_register(ontama_cffi_callback_LoadFileDataCallback_bridge, ontama_cffi_callback_LoadFileDataCallback_release",
		"func RegisterFileDataHooks(callback LoadFileDataCallback)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("Raylib-style generated FFI does not contain %q:\n%s", want, generated)
		}
	}
	files := map[string]string{
		"go.mod":           "module raylibshimffi.test\n\ngo 1.23\n",
		"generated_ffi.go": generated,
		"fixture.h": `#include <stdint.h>
#include <stddef.h>
typedef uint8_t *(*shim_load_file_data_callback)(const char *path, size_t *output_length, void *context);
typedef void (*shim_unload_file_data_callback)(uint8_t *data);
int32_t shim_file_data_register(shim_load_file_data_callback load, shim_unload_file_data_callback unload, void *context);
int32_t shim_file_data_unregister(shim_load_file_data_callback load, shim_unload_file_data_callback unload, void *context);
int32_t fake_raylib_load_checksum(const char *path);
int32_t fake_raylib_hold_file(const char *path);
int32_t fake_raylib_release_held_file(void);
int32_t fake_raylib_unload_count(void);
`,
		"fixture.c": `#include "fixture.h"
#include <limits.h>
typedef unsigned char *(*fake_raylib_load_callback)(const char *path, int *bytes_read);
typedef void (*fake_raylib_unload_callback)(unsigned char *data);
static fake_raylib_load_callback raylib_load_callback;
static fake_raylib_unload_callback raylib_unload_callback;
static shim_load_file_data_callback shim_load_callback;
static shim_unload_file_data_callback shim_unload_callback;
static void *shim_context;
static unsigned char *held_data;
static int held_length;
static int32_t unload_count;
static void SetLoadFileDataCallback(fake_raylib_load_callback callback) { raylib_load_callback = callback; }
static void SetUnloadFileDataCallback(fake_raylib_unload_callback callback) { raylib_unload_callback = callback; }
static unsigned char *shim_raylib_load(const char *path, int *bytes_read) {
  size_t length = 0;
  unsigned char *data = shim_load_callback(path, &length, shim_context);
  if (length > INT_MAX) { if (data) shim_unload_callback(data); *bytes_read = 0; return NULL; }
  *bytes_read = (int)length;
  return data;
}
static void shim_raylib_unload(unsigned char *data) { shim_unload_callback(data); unload_count++; }
int32_t shim_file_data_register(shim_load_file_data_callback load, shim_unload_file_data_callback unload, void *context) {
  if (shim_load_callback) return 181;
  shim_load_callback = load; shim_unload_callback = unload; shim_context = context;
  SetLoadFileDataCallback(shim_raylib_load); SetUnloadFileDataCallback(shim_raylib_unload);
  return 0;
}
int32_t shim_file_data_unregister(shim_load_file_data_callback load, shim_unload_file_data_callback unload, void *context) {
  if (load != shim_load_callback || unload != shim_unload_callback || context != shim_context) return 182;
  shim_load_callback = NULL; shim_context = NULL; SetLoadFileDataCallback(NULL);
  return 0;
}
static int32_t checksum(unsigned char *data, int length) { int32_t value = 0; for (int index = 0; index < length; index++) value += data[index]; return value; }
int32_t fake_raylib_load_checksum(const char *path) {
  if (!raylib_load_callback) return -1;
  int length = -1; unsigned char *data = raylib_load_callback(path, &length);
  if (length == 0) return data == NULL ? 0 : -2;
  if (!data || length < 0) return -3;
  int32_t value = checksum(data, length); raylib_unload_callback(data); return value;
}
int32_t fake_raylib_hold_file(const char *path) {
  if (!raylib_load_callback || held_data) return -1;
  held_length = -1; held_data = raylib_load_callback(path, &held_length); return held_length;
}
int32_t fake_raylib_release_held_file(void) {
  if (!held_data) return held_length == 0 ? 0 : -1;
  int32_t value = checksum(held_data, held_length); raylib_unload_callback(held_data); held_data = NULL; held_length = 0; return value;
}
int32_t fake_raylib_unload_count(void) { return unload_count; }
`,
		"app/binding.otm": `import go ffi from "raylibshimffi.test";
function LoadAsset(path: string): Result<int32> {
  const hooks = ffi.RegisterFileDataHooks((requested: string): byte[] => [1, 2, 3, 4])?;
  const checksum = ffi.RaylibLoadChecksum(path)?;
  hooks.Close()?;
  return ok(checksum);
}
`,
		"cmd/main.go": `package main
import (
  "errors"
  app "raylibshimffi.test/app"
  ffi "raylibshimffi.test"
)
func assert(ok bool) { if !ok { panic("Raylib-style shim assertion failed") } }
func main() {
  var observedPath string
  hooks, err := ffi.RegisterFileDataHooks(func(path string) []byte { observedPath = path; if path == "empty" { return []byte{} }; return []byte{0, 1, 127, 255} }); assert(err == nil)
  rejected, err := ffi.RegisterFileDataHooks(func(path string) []byte { return nil }); var status *ffi.StatusError; assert(rejected == nil && errors.As(err, &status) && status.Code == 181)
  checksum, err := ffi.RaylibLoadChecksum("textures/温泉.png"); assert(checksum == 383 && err == nil && observedPath == "textures/温泉.png" && ffi.RaylibUnloadCount() == 1)
  checksum, err = ffi.RaylibLoadChecksum("empty"); assert(checksum == 0 && err == nil && ffi.RaylibUnloadCount() == 1)
  length, err := ffi.RaylibHoldFile("held.bin"); assert(length == 4 && err == nil && hooks.Close() == nil)
  checksum = ffi.RaylibReleaseHeldFile(); assert(checksum == 383 && ffi.RaylibUnloadCount() == 2)
  checksum, err = ffi.RaylibLoadChecksum("after-close"); assert(checksum == -1 && err == nil)
  checksum, err = app.LoadAsset("asset.bin"); assert(checksum == 10 && err == nil && ffi.RaylibUnloadCount() == 3)
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generatedApp, diagnostics, err := EmitGo([]string{filepath.Join(root, "app", "binding.otm")}, "app")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("generate Raylib-style OnsenTamago wrapper: err=%v diagnostics=%v", err, diagnostics)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "generated.go"), generatedApp, 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"run", "-buildvcs=false", "./cmd"}
	if os.Getenv("ONTAMA_DIFFERENTIAL_RACE") == "1" {
		arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
	}
	command := exec.Command("go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Raylib-style incoming C FFI failed: %v\n%s\n%s", err, output, artifacts.Source)
	}
}

func TestIncomingCFFIRejectsInvalidManifestMatrix(t *testing.T) {
	validFunction := `"functions":[{"name":"Value","symbol":"value","parameters":[],"result":"int32","convention":"direct"}]`
	tests := []struct {
		name string
		data string
		want string
	}{
		{"unknown field", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe",` + validFunction + `,"unknown":true}`, "unknown field"},
		{"schema", `{"schemaVersion":2,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe",` + validFunction + `}`, "schema version"},
		{"package", `{"schemaVersion":1,"package":"bad-name","header":"fixture.h","threadPolicy":"threadSafe",` + validFunction + `}`, "package name"},
		{"keyword package", `{"schemaVersion":1,"package":"type","header":"fixture.h","threadPolicy":"threadSafe",` + validFunction + `}`, "package name"},
		{"header injection", `{"schemaVersion":1,"package":"binding","header":"fixture.h\n#error injected","threadPolicy":"threadSafe",` + validFunction + `}`, "single-line"},
		{"policy", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"unknown",` + validFunction + `}`, "threadPolicy"},
		{"unknown target", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","targets":[{"goos":"templeos","ldFlags":["-llib"]}],` + validFunction + `}`, "unsupported C FFI target"},
		{"duplicate target", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","targets":[{"goos":"linux","ldFlags":["-lone"]},{"goos":"linux","ldFlags":["-ltwo"]}],` + validFunction + `}`, "duplicate C FFI target"},
		{"empty target", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","targets":[{"goos":"linux"}],` + validFunction + `}`, "must declare cFlags or ldFlags"},
		{"target flag injection", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","targets":[{"goos":"linux","ldFlags":["-llib\n#error"]}],` + validFunction + `}`, "single-line"},
		{"empty functions", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[]}`, "at least one"},
		{"duplicate", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"one","parameters":[],"result":"int32","convention":"direct"},{"name":"Value","symbol":"two","parameters":[],"result":"int32","convention":"direct"}]}`, "duplicate"},
		{"unexported function", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"value","symbol":"value","parameters":[],"result":"int32","convention":"direct"}]}`, "must be identifiers"},
		{"reserved function", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"StatusError","symbol":"value","parameters":[],"result":"int32","convention":"direct"}]}`, "must be identifiers"},
		{"reserved parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"output","type":"int32"}],"result":"int32","convention":"statusOut"}]}`, "invalid or duplicate"},
		{"machine int", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"input","type":"int"}],"result":"int32","convention":"direct"}]}`, "unsupported type"},
		{"borrowed bytes result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"borrowedBytes","convention":"direct"}]}`, "unsupported result type"},
		{"retained string function parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"label","type":"retainedCString"}],"result":"void","convention":"direct"}]}`, "unsupported type"},
		{"retained bytes function parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"data","type":"retainedBytes"}],"result":"void","convention":"direct"}]}`, "unsupported type"},
		{"copied string function parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"label","type":"copiedCString"}],"result":"void","convention":"direct"}]}`, "unsupported type"},
		{"copied bytes function parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"data","type":"copiedBytes"}],"result":"void","convention":"direct"}]}`, "unsupported type"},
		{"inout bytes function parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"data","type":"inoutBytes"}],"result":"void","convention":"direct"}]}`, "unsupported type"},
		{"retained string result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"retainedCString","convention":"direct"}]}`, "unsupported result type"},
		{"owned cstring without release", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedCString","convention":"statusOut"}]}`, "resultRelease"},
		{"owned cstring direct", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedCString","resultRelease":"free_value","convention":"direct"}]}`, "must use statusOut"},
		{"owned cstring element", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedCString","resultElement":"byte","resultRelease":"free_value","convention":"statusOut"}]}`, "may not declare resultElement"},
		{"owned cstring parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[{"name":"input","type":"ownedCString"}],"result":"void","convention":"direct"}]}`, "unsupported type"},
		{"owned bytes without release", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedBytes","convention":"statusOut"}]}`, "resultRelease"},
		{"owned bytes direct", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedBytes","resultRelease":"free_value","convention":"direct"}]}`, "must use statusOut"},
		{"unexpected result release", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"int32","resultRelease":"free_value","convention":"direct"}]}`, "only for ownedCString, ownedBytes, or ownedArray"},
		{"owned array without element", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedArray","resultRelease":"free_value","convention":"statusOut"}]}`, "supported resultElement"},
		{"owned array string element", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedArray","resultElement":"cstring","resultRelease":"free_value","convention":"statusOut"}]}`, "supported resultElement"},
		{"owned array direct", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"ownedArray","resultElement":"uint32","resultRelease":"free_value","convention":"direct"}]}`, "must use statusOut"},
		{"void status out", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"void","convention":"statusOut"}]}`, "non-void"},
		{"non-void status", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"value","parameters":[],"result":"int32","convention":"status"}]}`, "void result"},
		{"invalid handle", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","handles":[{"name":"Handle","cType":"bad*","release":"free_handle"}],` + validFunction + `}`, "handle name"},
		{"direct handle", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","handles":[{"name":"Handle","cType":"handle","release":"free_handle"}],"functions":[{"name":"Use","symbol":"use","parameters":[{"name":"handle","type":"Handle"}],"result":"int32","convention":"direct"}]}`, "must use statusOut"},
		{"two handles", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","handles":[{"name":"Handle","cType":"handle","release":"free_handle"}],"functions":[{"name":"Use","symbol":"use","parameters":[{"name":"left","type":"Handle"},{"name":"right","type":"Handle"}],"result":"int32","convention":"statusOut"}]}`, "at most one"},
		{"flag comment injection", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","cFlags":["-DVALUE=*/package injected/*"],` + validFunction + `}`, "single-line"},
		{"enum float underlying", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","enums":[{"name":"Mode","cType":"mode","underlying":"float32"}],` + validFunction + `}`, "integer underlying"},
		{"enum duplicate value", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","enums":[{"name":"Mode","cType":"mode","underlying":"cInt32","values":[{"name":"ModeOne","symbol":"MODE_ONE"},{"name":"ModeOne","symbol":"MODE_TWO"}]}],` + validFunction + `}`, "invalid or duplicate value"},
		{"empty struct", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Point","cType":"point","fields":[]}],` + validFunction + `}`, "at least one field"},
		{"struct string field", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Text","cType":"text","fields":[{"name":"Value","cName":"value","type":"cstring"}]}],` + validFunction + `}`, "unsupported POD type"},
		{"struct duplicate C field", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Point","cType":"point","fields":[{"name":"X","cName":"value","type":"float32"},{"name":"Y","cName":"value","type":"float32"}]}],` + validFunction + `}`, "invalid or duplicate field"},
		{"recursive struct", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Node","cType":"node","fields":[{"name":"Next","cName":"next","type":"Node"}]}],` + validFunction + `}`, "recursive by-value"},
		{"empty tagged union", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[]}],` + validFunction + `}`, "at least one variant"},
		{"tagged union duplicate type", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Event","cType":"event_struct","fields":[{"name":"Value","cName":"value","type":"int32"}]}],"taggedUnions":[{"name":"Event","cType":"event_union","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "unique exported identifiers"},
		{"tagged union invalid C type", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event*","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "unique exported identifiers"},
		{"tagged union duplicate C type", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Payload","cType":"event","fields":[{"name":"Value","cName":"value","type":"int32"}]}],"taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "unique exported identifiers"},
		{"tagged union unexported tag", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "invalid tag field"},
		{"tagged union C keyword field", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"switch","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "invalid tag field"},
		{"tagged union float tag", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"float32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "unsupported integer or enum type"},
		{"tagged union overlaid scalar", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"overlaidTag":true,"variants":[{"name":"Value","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "requires POD struct variant"},
		{"tagged union missing variant tags", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"int32","tags":[]}]}],` + validFunction + `}`, "invalid variant"},
		{"tagged union string variant", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Text","cName":"text","type":"cstring","tags":["EVENT_TEXT"]}]}],` + validFunction + `}`, "invalid variant"},
		{"tagged union unknown variant", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"value","type":"Missing","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "invalid variant"},
		{"tagged union duplicate Go field", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Type","cName":"value","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "invalid variant"},
		{"tagged union duplicate C field", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"Value","cName":"type","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "invalid variant"},
		{"tagged union duplicate tag symbol", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","taggedUnions":[{"name":"Event","cType":"event","tag":{"name":"Type","cName":"type","type":"uint32"},"variants":[{"name":"First","cName":"first","type":"int32","tags":["EVENT_VALUE"]},{"name":"Second","cName":"second","type":"int32","tags":["EVENT_VALUE"]}]}],` + validFunction + `}`, "duplicate tag symbol"},
		{"callback unexported name", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"visit","lifetime":"callScoped","parameters":[],"result":"void"}],` + validFunction + `}`, "unique exported identifier"},
		{"callback duplicate type name", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","enums":[{"name":"Visit","cType":"visit_kind","underlying":"int32"}],"callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],` + validFunction + `}`, "unique exported identifier"},
		{"callback missing lifetime", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","parameters":[],"result":"void"}],` + validFunction + `}`, "lifetime must be callScoped or registered"},
		{"callback retained lifetime", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"retained","parameters":[],"result":"void"}],` + validFunction + `}`, "lifetime must be callScoped or registered"},
		{"callback string result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"cstring"}],` + validFunction + `}`, "unsupported scalar or enum result"},
		{"callback copied string result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"copiedCString"}],` + validFunction + `}`, "unsupported scalar or enum result"},
		{"callback inout bytes result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"inoutBytes"}],` + validFunction + `}`, "unsupported scalar or enum result"},
		{"call-scoped callback owned bytes result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"ownedBytes"}],` + validFunction + `}`, "requires registered lifetime"},
		{"call-scoped callback owned string result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"ownedCString"}],` + validFunction + `}`, "requires registered lifetime"},
		{"call-scoped callback owned array result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"ownedArray","resultElement":"int32"}],` + validFunction + `}`, "requires registered lifetime"},
		{"owned array callback without element", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"ownedArray"}],` + validFunction + `}`, "requires a supported resultElement"},
		{"owned array callback unsupported element", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"ownedArray","resultElement":"cstring"}],` + validFunction + `}`, "requires a supported resultElement"},
		{"scalar callback with result element", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"int32","resultElement":"int32"}],` + validFunction + `}`, "may declare resultElement only for ownedArray"},
		{"callback struct result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","structs":[{"name":"Point","cType":"point","fields":[{"name":"X","cName":"x","type":"int32"}]}],"callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"Point"}],` + validFunction + `}`, "unsupported scalar or enum result"},
		{"callback string parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"cstring"}],"result":"void"}],` + validFunction + `}`, "unsupported scalar, enum, POD, or tagged-union type"},
		{"callback retained string parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"retainedCString"}],"result":"void"}],` + validFunction + `}`, "unsupported scalar, enum, POD, or tagged-union type"},
		{"callback retained bytes parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"retainedBytes"}],"result":"void"}],` + validFunction + `}`, "unsupported scalar, enum, POD, or tagged-union type"},
		{"callback void parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"void"}],"result":"void"}],` + validFunction + `}`, "unsupported scalar, enum, POD, or tagged-union type"},
		{"callback duplicate parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[{"name":"value","type":"int32"},{"name":"value","type":"int64"}],"result":"void"}],` + validFunction + `}`, "invalid or duplicate parameter"},
		{"callback used as result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"functions":[{"name":"Value","symbol":"value","parameters":[],"result":"Visit","convention":"direct"}]}`, "unsupported result type"},
		{"two callback parameters", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"first","type":"Visit"},{"name":"second","type":"Visit"}],"result":"void","convention":"direct"}]}`, "at most one callScoped callback"},
		{"callback with string result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"visit","type":"Visit"}],"result":"cstring","convention":"direct"}]}`, "may not combine"},
		{"callback with owned result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"visit","type":"Visit"}],"result":"ownedBytes","resultRelease":"free_value","convention":"statusOut"}]}`, "may not combine"},
		{"callback with owned cstring result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"visit","type":"Visit"}],"result":"ownedCString","resultRelease":"free_value","convention":"statusOut"}]}`, "may not combine"},
		{"callback with handle result", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"handles":[{"name":"Handle","cType":"handle","release":"free_handle"}],"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"visit","type":"Visit"}],"result":"Handle","convention":"statusOut"}]}`, "may not combine"},
		{"registered callback without registration", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],` + validFunction + `}`, "requires a callbackRegistration"},
		{"registration unknown callback", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbackRegistrations":[{"name":"Watch","callback":"Missing","register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "must reference a registered callback"},
		{"registration callScoped callback", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"callScoped","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "must reference a registered callback"},
		{"registration invalid name", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"watch","callback":"Visit","register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unique exported identifier"},
		{"registration invalid symbol", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch-add","unregister":"watch_remove"}],` + validFunction + `}`, "symbols must be identifiers"},
		{"registration duplicate parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"topic","type":"int32"},{"name":"topic","type":"int64"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "invalid or duplicate parameter"},
		{"registration callback parameter name", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"callback","type":"int32"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "invalid or duplicate parameter"},
		{"registration string parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"label","type":"cstring"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"registration bytes parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"data","type":"borrowedBytes"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"registration copied string parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"label","type":"copiedCString"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"registration copied bytes parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"data","type":"copiedBytes"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"registration inout bytes parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"data","type":"inoutBytes"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"registration two handle parameters", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"handles":[{"name":"Handle","cType":"handle","release":"handle_free"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"left","type":"Handle"},{"name":"right","type":"Handle"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "at most one handle parameter"},
		{"registration callback typed parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"nested","type":"Visit"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"registration unknown parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","parameters":[{"name":"value","type":"Missing"}],"register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unsupported value type"},
		{"duplicate registration", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove"},{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove"}],` + validFunction + `}`, "unique exported identifier"},
		{"registered callback as call parameter", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove"}],"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"visit","type":"Visit"}],"result":"void","convention":"direct"}]}`, "only callScoped callback parameters"},
		{"registration generated name collision", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove"}],"functions":[{"name":"RegisterWatch","symbol":"value","parameters":[],"result":"void","convention":"direct"}]}`, "duplicate C FFI function"},
		{"trailing", `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe",` + validFunction + `} {}`, "multiple JSON"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GenerateCFFI([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
