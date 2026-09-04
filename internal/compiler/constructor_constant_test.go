package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorConstantProofsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "constructor_constants.km")
	input := `
class User { constructor(public name: string) {} }
class NegatedHolder {
  private user: User;
  private visits: int[];
  constructor() {
    while (!false) {
      this.user = new User("negated");
      this.visits = [1];
      break;
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}
class NumericHolder {
  private user: User;
  private visits: int[];
  constructor() {
    for (; (1 + 2) * 3 === 9 && int32(4) < int32(5); ) {
      this.user = new User("numeric");
      this.visits = [1, 2];
      break;
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}
class StringComparisonHolder {
  private user: User;
  constructor() {
    while (("a" + "b") < "b" && "same" !== "different") {
      this.user = new User("string-comparison");
      break;
    }
  }
  public function name(): string { return this.user.name; }
}
class StringRangeHolder {
  private user: User;
  private runes: int32[];
  constructor() {
    this.runes = [];
    for (const rune of "温" + "泉") {
      this.user = new User("string-range");
      this.runes = append(this.runes, rune);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.runes); }
}
class AppendedRangeHolder {
  private user: User;
  private values: int[];
  constructor() {
    this.values = [];
    for (const value of append(makeSlice[int](0), 7)) {
      this.user = new User("append-item");
      this.values = append(this.values, value);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.values); }
}
class SpreadRangeHolder {
  private user: User;
  private values: int[];
  constructor() {
    this.values = [];
    for (const value of append(makeSlice[int](0), [3, 4]...)) {
      this.user = new User("append-spread");
      this.values = append(this.values, value);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.values); }
}
class MadeSliceRangeHolder {
  private user: User;
  private visits: int[];
  constructor() {
    this.visits = [];
    for (const value of makeSlice[int](1 + 1)) {
      this.user = new User("make-slice");
      this.visits = append(this.visits, value);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}
function negatedName(): string { return new NegatedHolder().name(); }
function negatedCount(): int { return new NegatedHolder().count(); }
function numericName(): string { return new NumericHolder().name(); }
function numericCount(): int { return new NumericHolder().count(); }
function stringComparison(): string { return new StringComparisonHolder().name(); }
function stringRangeName(): string { return new StringRangeHolder().name(); }
function stringRangeCount(): int { return new StringRangeHolder().count(); }
function appendedRangeName(): string { return new AppendedRangeHolder().name(); }
function appendedRangeCount(): int { return new AppendedRangeHolder().count(); }
function spreadRangeName(): string { return new SpreadRangeHolder().name(); }
function spreadRangeCount(): int { return new SpreadRangeHolder().count(); }
function madeSliceRangeName(): string { return new MadeSliceRangeHolder().name(); }
function madeSliceRangeCount(): int { return new MadeSliceRangeHolder().count(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "constructorconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"for !false", "(1+2)*3 == 9", `for _, rune := range "温" + "泉"`} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated Go does not preserve %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
func Negated() (string, int) {
  name, count := "", 0
  for !false { name = "negated"; count = 1; break }
  return name, count
}
func Numeric() (string, int) {
  name, count := "", 0
  for ; (1+2)*3 == 9 && int32(4) < int32(5); { name = "numeric"; count = 2; break }
  return name, count
}
func StringComparison() string {
  name := ""
  for "a"+"b" < "b" && "same" != "different" { name = "string-comparison"; break }
  return name
}
func StringRange() (string, int) {
  name, count := "", 0
  for range "温"+"泉" { name = "string-range"; count++ }
  return name, count
}
func AppendedRange() (string, int) {
  name, count := "", 0
  for range append(make([]int, 0), 7) { name = "append-item"; count++ }
  return name, count
}
func SpreadRange() (string, int) {
  name, count := "", 0
  for range append(make([]int, 0), []int{3, 4}...) { name = "append-spread"; count++ }
  return name, count
}
func MadeSliceRange() (string, int) {
  name, count := "", 0
  for range make([]int, 1+1) { name = "make-slice"; count++ }
  return name, count
}
`
	testSource := `package constructorconstants
import (
  "testing"
  reference "constructor-constants.test/reference"
)
func TestConstructorConstants(t *testing.T) {
  wantName, wantCount := reference.Negated()
  if got := negatedName(); got != wantName { t.Fatalf("negatedName = %q, equivalent Go = %q", got, wantName) }
  if got := negatedCount(); got != wantCount { t.Fatalf("negatedCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.Numeric()
  if got := numericName(); got != wantName { t.Fatalf("numericName = %q, equivalent Go = %q", got, wantName) }
  if got := numericCount(); got != wantCount { t.Fatalf("numericCount = %d, equivalent Go = %d", got, wantCount) }
  if got, want := stringComparison(), reference.StringComparison(); got != want { t.Fatalf("stringComparison = %q, equivalent Go = %q", got, want) }
  wantName, wantCount = reference.StringRange()
  if got := stringRangeName(); got != wantName { t.Fatalf("stringRangeName = %q, equivalent Go = %q", got, wantName) }
  if got := stringRangeCount(); got != wantCount { t.Fatalf("stringRangeCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.AppendedRange()
  if got := appendedRangeName(); got != wantName { t.Fatalf("appendedRangeName = %q, equivalent Go = %q", got, wantName) }
  if got := appendedRangeCount(); got != wantCount { t.Fatalf("appendedRangeCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.SpreadRange()
  if got := spreadRangeName(); got != wantName { t.Fatalf("spreadRangeName = %q, equivalent Go = %q", got, wantName) }
  if got := spreadRangeCount(); got != wantCount { t.Fatalf("spreadRangeCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.MadeSliceRange()
  if got := madeSliceRangeName(); got != wantName { t.Fatalf("madeSliceRangeName = %q, equivalent Go = %q", got, wantName) }
  if got := madeSliceRangeCount(); got != wantCount { t.Fatalf("madeSliceRangeCount = %d, equivalent Go = %d", got, wantCount) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-constants.test", generated, referenceSource, testSource)
}
