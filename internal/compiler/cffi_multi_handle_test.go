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

func TestIncomingCFFIMultipleHandles(t *testing.T) {
	requireCCompiler(t)
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine"} {
		t.Run(policy, func(t *testing.T) {
			root := t.TempDir()
			manifest := `{
  "schemaVersion":1,
  "package":"ffi",
  "header":"fixture.h",
  "threadPolicy":"` + policy + `",
  "handles":[
    {"name":"Image","cType":"fixture_image","release":"fixture_image_free"},
    {"name":"Canvas","cType":"fixture_canvas","release":"fixture_canvas_free"}
  ],
  "functions":[
    {"name":"NewImage","symbol":"fixture_image_new","parameters":[{"name":"value","type":"int32"}],"result":"Image","convention":"statusOut"},
    {"name":"NewCanvas","symbol":"fixture_canvas_new","parameters":[{"name":"value","type":"int32"}],"result":"Canvas","convention":"statusOut"},
    {"name":"CombineImages","symbol":"fixture_image_combine","parameters":[{"name":"left","type":"Image"},{"name":"right","type":"Image"}],"result":"int32","convention":"statusOut"},
    {"name":"DrawImage","symbol":"fixture_image_draw","parameters":[{"name":"canvas","type":"Canvas"},{"name":"image","type":"Image"}],"result":"int32","convention":"statusOut"},
    {"name":"DrawImageStatus","symbol":"fixture_image_draw_status","parameters":[{"name":"canvas","type":"Canvas"},{"name":"image","type":"Image"}],"result":"void","convention":"status"}
  ]
}`
			artifacts, err := GenerateCFFI([]byte(manifest))
			if err != nil {
				t.Fatal(err)
			}
			generated := string(artifacts.Source)
			for _, want := range []string{"atomic.Uint64", "sort.Slice(kinmokuseiHandleLocks", "kinmokuseiLockedHandles", "id: kinmokuseiCFFIHandleID.Add(1)"} {
				if !strings.Contains(generated, want) {
					t.Errorf("generated multi-handle FFI misses %q", want)
				}
			}
			files := map[string]string{
				"go.mod":           "module fixture.test\n\ngo 1.23\n",
				"generated_ffi.go": generated,
				"fixture.h": `#include <stdint.h>
typedef struct fixture_image fixture_image;
typedef struct fixture_canvas fixture_canvas;
int32_t fixture_image_new(int32_t value, fixture_image **output);
int32_t fixture_canvas_new(int32_t value, fixture_canvas **output);
int32_t fixture_image_combine(fixture_image *left, fixture_image *right, int32_t *output);
int32_t fixture_image_draw(fixture_canvas *canvas, fixture_image *image, int32_t *output);
int32_t fixture_image_draw_status(fixture_canvas *canvas, fixture_image *image);
void fixture_image_free(fixture_image *image);
void fixture_canvas_free(fixture_canvas *canvas);
`,
				"fixture.c": `#include "fixture.h"
#include <stdlib.h>
struct fixture_image { int32_t value; };
struct fixture_canvas { int32_t value; };
int32_t fixture_image_new(int32_t value, fixture_image **output) {
  *output = calloc(1, sizeof(fixture_image));
  if (!*output) return 1;
  (*output)->value = value;
  return 0;
}
int32_t fixture_canvas_new(int32_t value, fixture_canvas **output) {
  *output = calloc(1, sizeof(fixture_canvas));
  if (!*output) return 1;
  (*output)->value = value;
  return 0;
}
int32_t fixture_image_combine(fixture_image *left, fixture_image *right, int32_t *output) {
  *output = left->value + right->value;
  return 0;
}
int32_t fixture_image_draw(fixture_canvas *canvas, fixture_image *image, int32_t *output) {
  *output = canvas->value + image->value;
  return 0;
}
int32_t fixture_image_draw_status(fixture_canvas *canvas, fixture_image *image) {
  return canvas->value + image->value == 13 ? 0 : 7;
}
void fixture_image_free(fixture_image *image) { free(image); }
void fixture_canvas_free(fixture_canvas *canvas) { free(canvas); }
`,
				"cmd/main.go": `package main
import (
  "errors"
  "fixture.test"
  "sync"
)
func assert(ok bool) { if !ok { panic("multi-handle mismatch") } }
func main() {
  left, err := ffi.NewImage(2); assert(err == nil)
  right, err := ffi.NewImage(3); assert(err == nil)
  canvas, err := ffi.NewCanvas(10); assert(err == nil)
  same, err := ffi.CombineImages(left, left); assert(err == nil && same == 4)
  combined, err := ffi.CombineImages(left, right); assert(err == nil && combined == 5)
  drawn, err := ffi.DrawImage(canvas, right); assert(err == nil && drawn == 13)
  assert(ffi.DrawImageStatus(canvas, right) == nil)
  combined, err = ffi.CombineImages(nil, right); assert(combined == 0 && errors.Is(err, ffi.ErrClosedHandle))
  combined, err = ffi.CombineImages(left, nil); assert(combined == 0 && errors.Is(err, ffi.ErrClosedHandle))
  drawn, err = ffi.DrawImage(nil, right); assert(drawn == 0 && errors.Is(err, ffi.ErrClosedHandle))
  var workers sync.WaitGroup
  for i := 0; i < 16; i++ {
    workers.Add(2)
    go func() { defer workers.Done(); for j := 0; j < 100; j++ { result, err := ffi.CombineImages(left, right); assert(err == nil && result == 5) } }()
    go func() { defer workers.Done(); for j := 0; j < 100; j++ { result, err := ffi.CombineImages(right, left); assert(err == nil && result == 5) } }()
  }
  workers.Wait()
  for i := 0; i < 16; i++ {
    workers.Add(1)
    go func() {
      defer workers.Done()
      for j := 0; j < 100; j++ {
        result, err := ffi.CombineImages(left, right)
        assert((err == nil && result == 5) || (result == 0 && errors.Is(err, ffi.ErrClosedHandle)))
      }
    }()
  }
  assert(left.Close() == nil)
  workers.Wait()
  combined, err = ffi.CombineImages(left, right)
  assert(combined == 0 && errors.Is(err, ffi.ErrClosedHandle))
  assert(right.Close() == nil && canvas.Close() == nil)
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
			arguments := []string{"run", "-buildvcs=false", "./cmd"}
			if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE") == "1" {
				arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
			}
			command := exec.CommandContext(ctx, "go", arguments...)
			command.Dir = root
			command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, "go-cache"), "CGO_ENABLED=1")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("multi-handle generated package failed: %v\n%s\n%s", err, output, generated)
			}
		})
	}
}
