package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstructorRangeDeclarationsCompileAndRun(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()
	path := filepath.Join(temp, "constructor_declarations.km")
	input := `
class User { constructor(public value: int) {} }
class SliceHolder {
  private user: User;
  constructor(values: int[], enabled: boolean) {
    if (len(values) > 0) {
      const count = len(values);
      let weight = count + 1;
      if (enabled) {
        const offset = weight;
        for (const value of values) { this.user = new User(value + offset); }
      } else {
        const offset = -weight;
        for (const value of values) { this.user = new User(value + offset); }
      }
    } else { this.user = new User(0); }
  }
  public function value(): int { return this.user.value; }
}
class MapHolder {
  private user: User;
  private total: int;
  constructor(values: Map<string, int>) {
    this.total = 0;
    switch (len(values)) {
      case 0 { this.user = new User(0); }
      default {
        const count = len(values);
        const weight = count * 2;
        for (const [key, value] of values) {
          this.user = new User(weight);
          this.total += value;
        }
      }
    }
  }
  public function value(): int { return this.user.value + this.total; }
}
class TextHolder {
  private user: User;
  private count: int;
  constructor(text: string) {
    this.count = 0;
    if (len(text) === 0) { throw new Exception("empty"); }
    const bytes = len(text);
    const offset = bytes + 1;
    for (const rune of text) {
      this.user = new User(offset);
      this.count++;
    }
  }
  public function value(): int { return this.user.value + this.count; }
}
function sliceValue(values: int[], enabled: boolean): int { return new SliceHolder(values, enabled).value(); }
function mapValue(values: Map<string, int>): int { return new MapHolder(values).value(); }
function textValue(text: string): int { return new TextHolder(text).value(); }
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "constructordeclarations")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func SliceValue(values []int, enabled bool) int {
  result := 0
  if len(values) > 0 {
    count := len(values)
    weight := count + 1
    if enabled {
      offset := weight
      for _, value := range values { result = value + offset }
    } else {
      offset := -weight
      for _, value := range values { result = value + offset }
    }
  }
  return result
}
func MapValue(values map[string]int) int {
  result, total := 0, 0
  switch len(values) {
  case 0:
    result = 0
  default:
    count := len(values)
    weight := count * 2
    for _, value := range values { result = weight; total += value }
  }
  return result + total
}
func TextValue(text string) int {
  if len(text) == 0 { panic("empty") }
  bytes := len(text)
  offset := bytes + 1
  result, count := 0, 0
  for range text { result = offset; count++ }
  return result + count
}
`
	comparison := `package constructordeclarations
import (
  "testing"
  reference "constructor-declarations.test/reference"
)
func outcome(f func() int) (value int, panicked bool) {
  panicked = true
  defer func() { recover() }()
  value = f()
  panicked = false
  return
}
func TestDeclarations(t *testing.T) {
  for _, values := range [][]int{nil, {}, {7}, {-2, 3, 9}} {
    for _, enabled := range []bool{false, true} {
      if got, want := sliceValue(values, enabled), reference.SliceValue(values, enabled); got != want {
        t.Errorf("slice(%v, %v) = %d, equivalent Go = %d", values, enabled, got, want)
      }
    }
  }
  for _, values := range []map[string]int{nil, {}, {"a": 7}, {"a": -2, "b": 3, "c": 9}} {
    if got, want := mapValue(values), reference.MapValue(values); got != want {
      t.Errorf("map(%v) = %d, equivalent Go = %d", values, got, want)
    }
  }
  for _, text := range []string{"", "a", "abc", "温泉", "a温🌸", "\xff"} {
    got, gotPanic := outcome(func() int { return textValue(text) })
    want, wantPanic := outcome(func() int { return reference.TextValue(text) })
    if got != want || gotPanic != wantPanic {
      t.Errorf("text(%q) = (%d, %v), equivalent Go = (%d, %v)", text, got, gotPanic, want, wantPanic)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-declarations.test", generated, reference, comparison)
}
