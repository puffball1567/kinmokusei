package compiler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Keep resource measurements in a separate process: other compiler tests run
// in parallel, and process-wide allocation counters would include their work.
func TestNestedArrayCheckResources(t *testing.T) {
	if os.Getenv("KINMOKUSEI_ARRAY_RESOURCES_HELPER") == "1" {
		path := filepath.Join(t.TempDir(), "nested.km")
		for _, depth := range []int{64, 1000, 2000, 8000} {
			for _, leaf := range []string{"", "1"} {
				input := nestedArrayResourceSource(depth, leaf)
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				runtime.ReadMemStats(&after)
				if err != nil {
					t.Fatal(err)
				}
				allocated := after.TotalAlloc - before.TotalAlloc
				t.Logf("depth=%d leaf=%q allocated=%d bytes", depth, leaf, allocated)
				// Generous across supported toolchains/targets, but well below
				// the reported 0.3 GB at 1,000 and 1.2 GB at 2,000 levels.
				if allocated > 128<<20 {
					t.Fatalf("nested array checking allocated %d bytes", allocated)
				}
				if depth == 64 {
					if leaf == "" {
						if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "cannot infer the element type of an empty array" {
							t.Fatalf("expected ordinary empty-array inference diagnostic: %v", result.Diagnostics)
						}
					} else if len(result.Diagnostics) != 0 {
						t.Fatalf("ordinary array nesting rejected: %v", result.Diagnostics)
					}
					continue
				}
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "syntax nesting is too deep" || result.Diagnostics[0].Span.Path != path {
					t.Fatalf("expected located depth diagnostic: %v", result.Diagnostics)
				}
				if result.Program == nil || len(result.Program.Declarations) != 0 {
					t.Fatal("over-deep array reached later compiler passes")
				}
			}
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNestedArrayCheckResources$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), "KINMOKUSEI_ARRAY_RESOURCES_HELPER=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("nested array resource check failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}

func nestedArrayResourceSource(depth int, leaf string) string {
	return "const x = " + strings.Repeat("[", depth) + leaf + strings.Repeat("]", depth) + ";"
}

func BenchmarkNestedArrayCheck(b *testing.B) {
	for _, depth := range []int{64, 1000, 2000, 8000} {
		b.Run(fmt.Sprint(depth), func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "nested.km")
			input := nestedArrayResourceSource(depth, "")
			overlay := map[string]string{path: input}
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for range b.N {
				result, err := CheckFilesWithOverlay([]string{path}, overlay)
				if err != nil {
					b.Fatal(err)
				}
				if depth == 64 {
					if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "cannot infer the element type of an empty array" {
						b.Fatal(result.Diagnostics)
					}
				} else if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "syntax nesting is too deep" {
					b.Fatalf("expected depth diagnostic: %v", result.Diagnostics)
				}
			}
		})
	}
}
