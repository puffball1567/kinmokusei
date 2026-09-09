package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstructorArrowReturnsCompileAndRun(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()
	path := filepath.Join(temp, "constructor_arrow.km")
	input := `
class Holder {
  private callback: () => int;
  private value: int;
  constructor(seed: int) {
    const [parsed, failure] = ((): Result<int> => { return ok(seed); })();
    this.value = parsed;
    const step = (): int => {
      const increment = (): int => { return 2; };
      this.value += increment();
      return this.value;
    };
    const ignore = (): void => { return; };
    ignore();
    this.callback = step;
  }
  public function next(): int { return this.callback(); }
}
function run(seed: int): int[] {
  const holder = new Holder(seed);
  return [holder.next(), holder.next(), holder.next()];
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "constructorarrow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type holder struct { value int; callback func() int }
func newHolder(seed int) *holder {
  parsed, _ := func() (int, error) { return seed, nil }()
  h := &holder{value: parsed}
  step := func() int {
    increment := func() int { return 2 }
    h.value += increment()
    return h.value
  }
  ignore := func() { return }
  ignore()
  h.callback = step
  return h
}
func Run(seed int) []int {
  h := newHolder(seed)
  return []int{h.callback(), h.callback(), h.callback()}
}
`
	comparison := `package constructorarrow
import (
  "reflect"
  "testing"
  reference "constructor-arrow.test/reference"
)
func TestCallbacks(t *testing.T) {
  for _, seed := range []int{-10, 0, 1, 100} {
    if got, want := run(seed), reference.Run(seed); !reflect.DeepEqual(got, want) {
      t.Errorf("run(%d) = %v, equivalent Go = %v", seed, got, want)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-arrow.test", generated, reference, comparison)
}
