package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorLengthSwitchProofsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "constructor_length_switch.km")
	input := `
class User { constructor(public name: string) {} }

class DefaultRangeHolder {
  private user: User;
  private visits: int[];
  constructor(values: int[]) {
    this.visits = [];
    switch (len(values)) {
      case 0 { this.user = new User("empty"); }
      default {
        for (const value of values) {
          this.user = new User("nonempty");
          this.visits = append(this.visits, value);
        }
      }
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}

class PositiveCaseHolder {
  private user: User;
  private visits: int[];
  constructor(values: int[]) {
    this.visits = [];
    switch (len(values)) {
      case 1, 1 + 1 {
        for (const value of values) {
          this.user = new User("short");
          this.visits = append(this.visits, value);
        }
      }
      default { this.user = new User("other"); }
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}

class StringDefaultRangeHolder {
  private user: User;
  private runes: int32[];
  constructor(value: string) {
    this.runes = [];
    switch (len(value)) {
      default {
        for (const rune of value) {
          this.user = new User("text");
          this.runes = append(this.runes, rune);
        }
      }
      case 1 - 1 { this.user = new User("empty"); }
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.runes); }
}

function defaultRangeName(values: int[]): string { return new DefaultRangeHolder(values).name(); }
function defaultRangeCount(values: int[]): int { return new DefaultRangeHolder(values).count(); }
function positiveCaseName(values: int[]): string { return new PositiveCaseHolder(values).name(); }
function positiveCaseCount(values: int[]): int { return new PositiveCaseHolder(values).count(); }
function stringDefaultRangeName(value: string): string { return new StringDefaultRangeHolder(value).name(); }
function stringDefaultRangeCount(value: string): int { return new StringDefaultRangeHolder(value).count(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "constructorlengthswitch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"switch len(values)", "case 0:", "case 1, 1 + 1:", "switch len(value)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference

func DefaultRange(values []int) (string, int) {
  name := ""
  visits := 0
  switch len(values) {
  case 0:
    name = "empty"
  default:
    for range values { name = "nonempty"; visits++ }
  }
  return name, visits
}

func PositiveCase(values []int) (string, int) {
  name := ""
  visits := 0
  switch len(values) {
  case 1, 1 + 1:
    for range values { name = "short"; visits++ }
  default:
    name = "other"
  }
  return name, visits
}

func StringDefaultRange(value string) (string, int) {
  name := ""
  runes := 0
  switch len(value) {
  default:
    for range value { name = "text"; runes++ }
  case 1 - 1:
    name = "empty"
  }
  return name, runes
}
`
	testSource := `package constructorlengthswitch
import (
  "testing"
  reference "constructor-length-switch.test/reference"
)

func TestConstructorLengthSwitch(t *testing.T) {
  for _, values := range [][]int{nil, {}, {7}, {4, 5}, {1, 2, 3}, {-2, 0, 8, 13}} {
    wantName, wantCount := reference.DefaultRange(values)
    if got := defaultRangeName(values); got != wantName {
      t.Errorf("defaultRangeName(%v) = %q, equivalent Go = %q", values, got, wantName)
    }
    if got := defaultRangeCount(values); got != wantCount {
      t.Errorf("defaultRangeCount(%v) = %d, equivalent Go = %d", values, got, wantCount)
    }
    wantName, wantCount = reference.PositiveCase(values)
    if got := positiveCaseName(values); got != wantName {
      t.Errorf("positiveCaseName(%v) = %q, equivalent Go = %q", values, got, wantName)
    }
    if got := positiveCaseCount(values); got != wantCount {
      t.Errorf("positiveCaseCount(%v) = %d, equivalent Go = %d", values, got, wantCount)
    }
  }
  for _, value := range []string{"", "a", "温", "onsen", "温泉たまご"} {
    wantName, wantCount := reference.StringDefaultRange(value)
    if got := stringDefaultRangeName(value); got != wantName {
      t.Errorf("stringDefaultRangeName(%q) = %q, equivalent Go = %q", value, got, wantName)
    }
    if got := stringDefaultRangeCount(value); got != wantCount {
      t.Errorf("stringDefaultRangeCount(%q) = %d, equivalent Go = %d", value, got, wantCount)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-length-switch.test", generated, referenceSource, testSource)
}
