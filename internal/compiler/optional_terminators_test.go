package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionalTerminatorsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "main.km")
	files := map[string]string{
		"helper.km": "alias Count = int\nfunction twice(value: Count): Count { return value * 2 }",
		"main.km": `import { twice } from "./helper"
import go strings from "strings"
interface Reader { function read(): int }
class Box implements Reader {
  public value: int = 1
  constructor(seed: int) { this.value = seed }
  public function read(): int { return this.value }
}
let trace = ""
function mark(value: string): void { trace += value }
function work(seed: int): int {
  defer mark("defer")
  const next = (value: int): int => {
    return twice(value)
  }
  let sum = next
  (seed)
  + 1
  const box: Reader = new Box(sum)
  sum = box.read()
  for (let i = 0; i < 4; i++) {
    if (i == 1) { continue }
    sum += i
  }
  while (true) { sum++
    break
  }
  switch (sum % 2) {
    case 0 { sum += 10
      fallthrough
    }
    default { sum += 20 }
  }
  try { mark("try")
    throw new Exception("expected")
  } catch (_: error) { mark("catch") } finally { mark("finally") }
  return sum
}
function Run(seed: int): string {
  trace = ""
  const result = work(seed)
  return strings.Repeat("x", result) + trace
}
function ReturnBoundary(): int {
  let count = 0
  const stop = (): void => {
    return
    count++
  }
  stop()
  return count
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{path}, "terminators")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "strings"
func Run(seed int) string {
  trace := ""
  work := func() int {
    defer func() { trace += "defer" }()
    twice := func(value int) int { return value * 2 }
    sum := twice(seed) + 1
    for i := 0; i < 4; i++ { if i == 1 { continue }; sum += i }
    for { sum++; break }
    switch sum % 2 { case 0: sum += 10; fallthrough; default: sum += 20 }
    trace += "trycatchfinally"
    return sum
  }
  result := work()
  return strings.Repeat("x", result) + trace
}
func ReturnBoundary() int {
  count := 0
  stop := func() { return; count++ }
  stop()
  return count
}
`
	comparison := `package terminators
import (
  "testing"
  reference "optional-terminators.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, seed := range []int{0, 1, 2, 10} {
    if got, want := Run(seed), reference.Run(seed); got != want { t.Errorf("Run(%d)=%q want %q", seed, got, want) }
  }
  if got, want := ReturnBoundary(), reference.ReturnBoundary(); got != want { t.Errorf("return boundary=%d want %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, root, "optional-terminators.test", generated, reference, comparison)
}

func TestNewlineReturnReportsSourceDiagnostic(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "return.km")
	result, err := CheckFilesWithOverlay([]string{path}, map[string]string{
		path: "function value(): int { return\n42 }",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("bare return in int function must fail")
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Span.Path != path || strings.Contains(diagnostic.Message, "generated Go") {
			t.Fatal(diagnostic)
		}
	}
}
