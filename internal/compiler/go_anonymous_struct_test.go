package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnonymousGoStructAPIsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "anonymous_struct.km")
	input := `
import go context from "context";
import go runtime from "runtime";

function canceledContextSignals(): boolean {
  const [ctx, cancel] = context.WithCancel(context.Background());
  cancel();
  select { case <-ctx.Done() { return true; } }
}
function runtimeSizeClass(): int {
  let stats: runtime.MemStats = runtime.MemStats{};
  runtime.ReadMemStats(&stats);
  const first = () => stats.BySize[1];
  return int(first().Size);
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "anonymousstruct")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	normalized := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{"context.WithCancel", "<-ctx.Done()", "stats.BySize[1]", "func() struct", "Size uint32", "Mallocs uint64", "Frees uint64"} {
		if !strings.Contains(normalized, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "context"
  "runtime"
)
func CanceledContextSignals() bool {
  ctx, cancel := context.WithCancel(context.Background())
  cancel()
  select { case <-ctx.Done(): return true }
}
func RuntimeSizeClass() int {
  var stats runtime.MemStats
  runtime.ReadMemStats(&stats)
  first := func() struct { Size uint32; Mallocs uint64; Frees uint64 } { return stats.BySize[1] }
  return int(first().Size)
}
`
	testSource := `package anonymousstruct
import (
  "testing"
  reference "anonymousstruct.test/reference"
)
func TestAnonymousStructInterop(t *testing.T) {
	if got, want := canceledContextSignals(), reference.CanceledContextSignals(); got != want { t.Errorf("canceled context = %v, Go = %v", got, want) }
	if got, want := runtimeSizeClass(), reference.RuntimeSizeClass(); got != want || got <= 0 { t.Errorf("runtime size class = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "anonymousstruct.test", generated, referenceSource, testSource)
}
